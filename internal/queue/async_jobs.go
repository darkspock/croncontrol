package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/dbutil"
	"github.com/croncontrol/croncontrol/internal/executor"
	"github.com/croncontrol/croncontrol/internal/jobstate"
	"github.com/croncontrol/croncontrol/internal/logging"
)

type asyncJobPollRow struct {
	ID               string
	WorkspaceID      string
	QueueID          string
	Attempt          int32
	State            string
	AttemptID        string
	AttemptStartedAt time.Time
	ExecutionHandle  []byte
	StdoutOffset     int64
	StderrOffset     int64
	ExecutionMethod  string
	MaxAttempts      int32
	RetryBackoff     string
	MaxResponseSize  int32
}

func (p *Processor) saveJobExecutionHandle(ctx context.Context, jobID string, handle executor.Handle) error {
	data, err := json.Marshal(handle)
	if err != nil {
		return fmt.Errorf("marshal job handle: %w", err)
	}
	return p.queries.SaveJobExecutionHandle(ctx, db.SaveJobExecutionHandleParams{
		ID:              jobID,
		ExecutionHandle: data,
	})
}

func (p *Processor) clearJobExecutionHandle(ctx context.Context, jobID string) error {
	return p.queries.ClearJobExecutionHandle(ctx, jobID)
}

func (p *Processor) updateJobOffsets(ctx context.Context, jobID string, stdoutOffset, stderrOffset int64) error {
	return p.queries.UpdateJobOffsets(ctx, db.UpdateJobOffsetsParams{
		ID:           jobID,
		StdoutOffset: stdoutOffset,
		StderrOffset: stderrOffset,
	})
}

func (p *Processor) appendJobAttemptResponseChunk(ctx context.Context, attemptID, chunk string) error {
	if attemptID == "" || chunk == "" {
		return nil
	}
	body := chunk
	return p.queries.AppendJobAttemptResponseChunk(ctx, db.AppendJobAttemptResponseChunkParams{
		ID:           attemptID,
		ResponseBody: &body,
	})
}

func (p *Processor) loadPersistedJobHandle(ctx context.Context, jobID string) (executor.Handle, bool, error) {
	row, err := p.queries.GetJobExecutionHandle(ctx, jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return executor.Handle{}, false, nil
		}
		return executor.Handle{}, false, err
	}
	var handle executor.Handle
	if err := json.Unmarshal(row.ExecutionHandle, &handle); err != nil {
		return executor.Handle{}, false, fmt.Errorf("unmarshal job handle: %w", err)
	}
	return handle, true, nil
}

func (p *Processor) listAsyncJobs(ctx context.Context) ([]asyncJobPollRow, error) {
	rows, err := p.queries.ListActiveAsyncJobs(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]asyncJobPollRow, 0, len(rows))
	for _, row := range rows {
		if !row.StartedAt.Valid {
			slog.Warn("queue: skipping async job without started_at", "job_id", row.ID)
			continue
		}
		result = append(result, asyncJobPollRow{
			ID:               row.ID,
			WorkspaceID:      row.WorkspaceID,
			QueueID:          row.QueueID,
			Attempt:          row.Attempt,
			State:            row.State,
			AttemptID:        row.AttemptID,
			AttemptStartedAt: row.StartedAt.Time,
			ExecutionHandle:  row.ExecutionHandle,
			StdoutOffset:     row.StdoutOffset,
			StderrOffset:     row.StderrOffset,
			ExecutionMethod:  row.ExecutionMethod,
			MaxAttempts:      row.MaxAttempts,
			RetryBackoff:     row.RetryBackoff,
			MaxResponseSize:  row.MaxResponseSize,
		})
	}
	return result, nil
}

func (p *Processor) processKillRequests(ctx context.Context) {
	rows, err := p.queries.ListKillRequestedAsyncJobIDs(ctx)
	if err != nil {
		slog.Error("queue: list kill requested async jobs failed", "error", err)
		return
	}
	for _, jobID := range rows {
		handle, ok, err := p.loadPersistedJobHandle(ctx, jobID)
		if err != nil || !ok {
			continue
		}
		method, ok := p.registry.Get(handle.MethodName)
		if !ok {
			continue
		}
		if err := method.Kill(ctx, handle); err != nil {
			slog.Error("queue: kill async job failed", "job_id", jobID, "error", err)
		}
	}
}

func (p *Processor) processAsyncJobs(ctx context.Context) {
	rows, err := p.listAsyncJobs(ctx)
	if err != nil {
		slog.Error("queue: list async jobs failed", "error", err)
		return
	}
	for _, row := range rows {
		p.pollAsyncJob(ctx, row)
	}
}

func (p *Processor) pollAsyncJob(ctx context.Context, row asyncJobPollRow) {
	var handle executor.Handle
	if err := json.Unmarshal(row.ExecutionHandle, &handle); err != nil {
		slog.Error("queue: invalid async job handle", "job_id", row.ID, "error", err)
		return
	}
	if handle.MethodName == "" {
		handle.MethodName = row.ExecutionMethod
	}
	method, ok := p.registry.Get(handle.MethodName)
	if !ok {
		slog.Error("queue: unknown async method", "job_id", row.ID, "method", handle.MethodName)
		return
	}

	poll, err := method.Poll(ctx, handle, executor.PollCursor{
		StdoutOffset: row.StdoutOffset,
		StderrOffset: row.StderrOffset,
	})
	if err != nil {
		if errors.Is(err, executor.ErrPollUnsupported) {
			return
		}
		slog.Error("queue: poll failed", "job_id", row.ID, "method", handle.MethodName, "error", err)
		return
	}

	stdoutOffset := poll.Cursor.StdoutOffset
	stderrOffset := poll.Cursor.StderrOffset
	if stdoutOffset == 0 && poll.StdoutChunk != "" {
		stdoutOffset = row.StdoutOffset + int64(len(poll.StdoutChunk))
	}
	if stderrOffset == 0 && poll.StderrChunk != "" {
		stderrOffset = row.StderrOffset + int64(len(poll.StderrChunk))
	}
	if stdoutOffset != row.StdoutOffset || stderrOffset != row.StderrOffset {
		_ = p.updateJobOffsets(ctx, row.ID, stdoutOffset, stderrOffset)
	}

	if poll.ResponseBodyChunk != "" {
		_ = p.appendJobAttemptResponseChunk(ctx, row.AttemptID, poll.ResponseBodyChunk)
	}

	if poll.State == "" || poll.State == executor.RemoteRunning {
		return
	}

	result := executor.Result{
		ExitCode:     poll.ExitCode,
		ResponseBody: poll.ResponseBodyChunk,
		Error:        poll.Error,
	}

	killed := poll.State == executor.RemoteKilled
	if poll.State == executor.RemoteFailed && result.Error == nil && result.ExitCode == nil {
		result.Error = fmt.Errorf("remote execution failed")
	}
	p.finalizeJobResult(ctx, row.ID, row.AttemptID, row.Attempt, row.State, row.AttemptStartedAt, row.MaxAttempts, row.RetryBackoff, row.MaxResponseSize, result, killed)
}

func (p *Processor) finalizeJobResult(
	ctx context.Context,
	jobID string,
	attemptID string,
	attempt int32,
	currentState string,
	startedAt time.Time,
	maxAttempts int32,
	retryBackoff string,
	maxResponseSize int32,
	result executor.Result,
	killed bool,
) {
	finished := time.Now().UTC()
	durationMs := finished.Sub(startedAt).Milliseconds()

	var responseCode *int32
	if result.ResponseCode != nil {
		rc := int32(*result.ResponseCode)
		responseCode = &rc
	}

	var errMsg *string
	if result.Error != nil {
		s := result.Error.Error()
		errMsg = &s
	}

	respBody := result.ResponseBody
	truncated := false
	maxSize := int(maxResponseSize)
	if maxSize > 0 && len(respBody) > maxSize {
		respBody = respBody[:maxSize]
		truncated = true
	}

	if attemptID != "" {
		p.queries.FinishJobAttempt(ctx, db.FinishJobAttemptParams{
			ID:           attemptID,
			FinishedAt:   dbutil.Timestamptz(finished),
			DurationMs:   &durationMs,
			ResponseCode: responseCode,
			ResponseBody: &respBody,
			Truncated:    truncated,
			ErrorMessage: errMsg,
		})
	}

	if p.logBackend != nil {
		var rc *int
		if responseCode != nil {
			v := int(*responseCode)
			rc = &v
		}
		em := ""
		if errMsg != nil {
			em = *errMsg
		}
		fin := finished
		p.logBackend.WriteJobAttempt(ctx, jobID, logging.JobAttemptLog{
			AttemptNumber: int(attempt),
			StartedAt:     startedAt,
			FinishedAt:    &fin,
			DurationMs:    durationMs,
			ResponseCode:  rc,
			ResponseBody:  respBody,
			ErrorMessage:  em,
		})
	}

	_ = p.clearJobExecutionHandle(ctx, jobID)

	if killed {
		// Only transition to killed if the job is actually in kill_requested state.
		// If something else already transitioned it (e.g. timeout), treat as normal failure.
		if currentState == string(jobstate.KillRequested) {
			p.queries.UpdateJobState(ctx, db.UpdateJobStateParams{
				ID:         jobID,
				State:      string(jobstate.Killed),
				DurationMs: &durationMs,
			})
			return
		}
		slog.Warn("queue: poll reported killed but job not in kill_requested state, treating as failure",
			"job_id", jobID, "current_state", currentState)
		// Fall through to normal success/failure handling
	}

	if result.IsSuccess() {
		p.queries.UpdateJobState(ctx, db.UpdateJobStateParams{
			ID:         jobID,
			State:      string(jobstate.Completed),
			DurationMs: &durationMs,
		})
		return
	}

	if attempt < maxAttempts {
		backoffs := getBackoffListFromString(retryBackoff)
		nextAt := calculateBackoff(finished, attempt, backoffs)
		p.queries.UpdateJobState(ctx, db.UpdateJobStateParams{
			ID:            jobID,
			State:         string(jobstate.Retrying),
			Attempt:       &attempt,
			NextAttemptAt: dbutil.Timestamptz(nextAt),
		})
		return
	}

	p.failJob(ctx, jobID, durationMs)
}
