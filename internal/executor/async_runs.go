package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	db "github.com/croncontrol/croncontrol/internal/db"
	"github.com/croncontrol/croncontrol/internal/dbutil"
	"github.com/croncontrol/croncontrol/internal/id"
	"github.com/croncontrol/croncontrol/internal/metrics"
	"github.com/croncontrol/croncontrol/internal/runstate"
)

type asyncRunPollRow struct {
	ID              string
	WorkspaceID     string
	ProcessID       string
	Attempt         int32
	State           string
	StartedAt       time.Time
	AttemptID       string
	ExecutionHandle []byte
	StdoutOffset    int64
	StderrOffset    int64
	ExecutionMethod string
}

func (o *Orchestrator) saveRunExecutionHandle(ctx context.Context, runID string, handle Handle) error {
	data, err := json.Marshal(handle)
	if err != nil {
		return fmt.Errorf("marshal run handle: %w", err)
	}
	return o.queries.SaveRunExecutionHandle(ctx, db.SaveRunExecutionHandleParams{
		ID:              runID,
		ExecutionHandle: data,
	})
}

func (o *Orchestrator) clearRunExecutionHandle(ctx context.Context, runID string) error {
	return o.queries.ClearRunExecutionHandle(ctx, runID)
}

func (o *Orchestrator) updateRunOffsets(ctx context.Context, runID string, stdoutOffset, stderrOffset int64) error {
	return o.queries.UpdateRunOffsets(ctx, db.UpdateRunOffsetsParams{
		ID:           runID,
		StdoutOffset: stdoutOffset,
		StderrOffset: stderrOffset,
	})
}

func (o *Orchestrator) loadPersistedRunHandle(ctx context.Context, runID string) (Handle, bool, error) {
	row, err := o.queries.GetRunExecutionHandle(ctx, runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Handle{}, false, nil
		}
		return Handle{}, false, err
	}
	var handle Handle
	if err := json.Unmarshal(row.ExecutionHandle, &handle); err != nil {
		return Handle{}, false, fmt.Errorf("unmarshal run handle: %w", err)
	}
	return handle, true, nil
}

func (o *Orchestrator) listActiveAsyncRuns(ctx context.Context) ([]asyncRunPollRow, error) {
	rows, err := o.queries.ListActiveAsyncRuns(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]asyncRunPollRow, 0, len(rows))
	for _, row := range rows {
		if !row.StartedAt.Valid {
			slog.Warn("executor: skipping async run without started_at", "run_id", row.ID)
			continue
		}
		result = append(result, asyncRunPollRow{
			ID:              row.ID,
			WorkspaceID:     row.WorkspaceID,
			ProcessID:       row.ProcessID,
			Attempt:         row.Attempt,
			State:           row.State,
			StartedAt:       row.StartedAt.Time,
			AttemptID:       row.AttemptID,
			ExecutionHandle: row.ExecutionHandle,
			StdoutOffset:    row.StdoutOffset,
			StderrOffset:    row.StderrOffset,
			ExecutionMethod: row.ExecutionMethod,
		})
	}
	return result, nil
}

func (o *Orchestrator) processAsyncRuns(ctx context.Context) {
	rows, err := o.listActiveAsyncRuns(ctx)
	if err != nil {
		slog.Error("executor: list async runs failed", "error", err)
		return
	}
	var wg sync.WaitGroup
	for _, row := range rows {
		wg.Add(1)
		go func(row asyncRunPollRow) {
			defer wg.Done()
			o.pollAsyncRun(ctx, row)
		}(row)
	}
	wg.Wait()
}

func (o *Orchestrator) pollAsyncRun(ctx context.Context, row asyncRunPollRow) {
	var handle Handle
	if err := json.Unmarshal(row.ExecutionHandle, &handle); err != nil {
		slog.Error("executor: invalid async run handle", "run_id", row.ID, "error", err)
		return
	}
	if handle.MethodName == "" {
		handle.MethodName = row.ExecutionMethod
	}

	method, ok := o.registry.Get(handle.MethodName)
	if !ok {
		slog.Error("executor: unknown async method", "run_id", row.ID, "method", handle.MethodName)
		return
	}

	poll, err := method.Poll(ctx, handle, PollCursor{
		StdoutOffset: row.StdoutOffset,
		StderrOffset: row.StderrOffset,
	})
	if err != nil {
		if errors.Is(err, ErrPollUnsupported) {
			return
		}
		slog.Error("executor: poll failed", "run_id", row.ID, "method", handle.MethodName, "error", err)
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

	appendFailed := false
	if poll.StdoutChunk != "" {
		if _, err := o.queries.AppendRunOutput(ctx, db.AppendRunOutputParams{
			ID:      id.New("out_"),
			RunID:   row.ID,
			Stream:  "stdout",
			Content: poll.StdoutChunk,
		}); err != nil {
			slog.Error("executor: append stdout failed, will retry on next poll", "run_id", row.ID, "error", err)
			appendFailed = true
		} else if o.logBackend != nil {
			o.logBackend.WriteRunOutput(ctx, row.ID, "stdout", poll.StdoutChunk)
		}
	}
	if poll.StderrChunk != "" {
		if _, err := o.queries.AppendRunOutput(ctx, db.AppendRunOutputParams{
			ID:      id.New("out_"),
			RunID:   row.ID,
			Stream:  "stderr",
			Content: poll.StderrChunk,
		}); err != nil {
			slog.Error("executor: append stderr failed, will retry on next poll", "run_id", row.ID, "error", err)
			appendFailed = true
		} else if o.logBackend != nil {
			o.logBackend.WriteRunOutput(ctx, row.ID, "stderr", poll.StderrChunk)
		}
	}
	if poll.ResponseBodyChunk != "" {
		if _, err := o.queries.AppendRunOutput(ctx, db.AppendRunOutputParams{
			ID:      id.New("out_"),
			RunID:   row.ID,
			Stream:  "stdout",
			Content: poll.ResponseBodyChunk,
		}); err != nil {
			slog.Error("executor: append response body failed, will retry on next poll", "run_id", row.ID, "error", err)
			appendFailed = true
		} else if o.logBackend != nil {
			o.logBackend.WriteRunOutput(ctx, row.ID, "stdout", poll.ResponseBodyChunk)
		}
	}
	// Skip updating offsets if any append failed so the chunk will be retried on next poll
	if !appendFailed && (stdoutOffset != row.StdoutOffset || stderrOffset != row.StderrOffset) {
		_ = o.updateRunOffsets(ctx, row.ID, stdoutOffset, stderrOffset)
	}

	if poll.State == "" || poll.State == RemoteRunning {
		return
	}

	proc, err := o.queries.GetProcess(ctx, db.GetProcessParams{
		ID:          row.ProcessID,
		WorkspaceID: row.WorkspaceID,
	})
	if err != nil {
		slog.Error("executor: get process for async finalize failed", "run_id", row.ID, "error", err)
		return
	}

	result := Result{
		ExitCode:     poll.ExitCode,
		Stdout:       poll.StdoutChunk,
		Stderr:       poll.StderrChunk,
		ResponseBody: poll.ResponseBodyChunk,
		Error:        poll.Error,
	}

	switch poll.State {
	case RemoteKilled:
		o.finalizeRunKilled(ctx, row, proc, result)
	case RemoteCompleted:
		o.finalizeRunResult(ctx, row.ID, row.WorkspaceID, row.AttemptID, row.Attempt, row.StartedAt, proc, result)
	case RemoteFailed:
		if result.Error == nil && result.ExitCode == nil {
			result.Error = fmt.Errorf("remote execution failed")
		}
		o.finalizeRunResult(ctx, row.ID, row.WorkspaceID, row.AttemptID, row.Attempt, row.StartedAt, proc, result)
	}
}

func (o *Orchestrator) finalizeRunKilled(ctx context.Context, row asyncRunPollRow, proc db.Process, result Result) {
	// Only transition to killed if the run is actually in kill_requested state.
	// If something else already transitioned it, treat as a normal failure instead.
	if row.State != string(runstate.KillRequested) {
		slog.Warn("executor: poll reported killed but run not in kill_requested state, treating as failure",
			"run_id", row.ID, "current_state", row.State)
		if result.Error == nil {
			result.Error = fmt.Errorf("process was killed")
		}
		o.finalizeRunResult(ctx, row.ID, row.WorkspaceID, row.AttemptID, row.Attempt, row.StartedAt, proc, result)
		return
	}

	finished := time.Now().UTC()
	durationMs := finished.Sub(row.StartedAt).Milliseconds()

	var exitCode *int32
	if result.ExitCode != nil {
		ec := int32(*result.ExitCode)
		exitCode = &ec
	}
	if row.AttemptID != "" {
		var errMsg *string
		if result.Error != nil {
			msg := result.Error.Error()
			errMsg = &msg
		}
		o.queries.FinishRunAttempt(ctx, db.FinishRunAttemptParams{
			ID:           row.AttemptID,
			FinishedAt:   dbutil.Timestamptz(finished),
			DurationMs:   &durationMs,
			ExitCode:     exitCode,
			ErrorMessage: errMsg,
		})
	}

	ec := int32(137)
	if exitCode != nil {
		ec = *exitCode
	}
	_ = o.clearRunExecutionHandle(ctx, row.ID)
	o.queries.UpdateRunState(ctx, db.UpdateRunStateParams{
		ID:         row.ID,
		State:      string(runstate.Killed),
		FinishedAt: dbutil.Timestamptz(finished),
		DurationMs: &durationMs,
		ExitCode:   &ec,
	})

	run, err := o.queries.GetRun(ctx, db.GetRunParams{ID: row.ID, WorkspaceID: row.WorkspaceID})
	if err == nil {
		o.handleTerminal(ctx, run, proc, runstate.Killed)
	}
}

func (o *Orchestrator) finalizeRunResult(
	ctx context.Context,
	runID string,
	workspaceID string,
	attemptID string,
	attempt int32,
	startedAt time.Time,
	proc db.Process,
	result Result,
) {
	finished := time.Now().UTC()
	durationMs := finished.Sub(startedAt).Milliseconds()

	var exitCode *int32
	if result.ExitCode != nil {
		ec := int32(*result.ExitCode)
		exitCode = &ec
	}

	var errMsg *string
	if result.Error != nil {
		s := result.Error.Error()
		errMsg = &s
	} else if !result.IsSuccess() {
		var msg string
		if result.ResponseCode != nil {
			msg = fmt.Sprintf("HTTP %d", *result.ResponseCode)
			if result.ResponseBody != "" {
				body := result.ResponseBody
				if len(body) > 200 {
					body = body[:200] + "..."
				}
				msg += ": " + body
			}
		} else if result.ExitCode != nil {
			msg = fmt.Sprintf("exit code %d", *result.ExitCode)
			if result.Stderr != "" {
				stderr := result.Stderr
				if len(stderr) > 200 {
					stderr = stderr[:200] + "..."
				}
				msg += ": " + stderr
			}
		} else {
			msg = "execution failed (unknown reason)"
		}
		errMsg = &msg
	}

	if attemptID != "" {
		o.queries.FinishRunAttempt(ctx, db.FinishRunAttemptParams{
			ID:           attemptID,
			FinishedAt:   dbutil.Timestamptz(finished),
			DurationMs:   &durationMs,
			ExitCode:     exitCode,
			ErrorMessage: errMsg,
		})
	}

	if result.Stdout != "" {
		o.queries.AppendRunOutput(ctx, db.AppendRunOutputParams{
			ID:      id.New("out_"),
			RunID:   runID,
			Stream:  "stdout",
			Content: result.Stdout,
		})
		if o.logBackend != nil {
			o.logBackend.WriteRunOutput(ctx, runID, "stdout", result.Stdout)
		}
	}
	if result.Stderr != "" {
		o.queries.AppendRunOutput(ctx, db.AppendRunOutputParams{
			ID:      id.New("out_"),
			RunID:   runID,
			Stream:  "stderr",
			Content: result.Stderr,
		})
		if o.logBackend != nil {
			o.logBackend.WriteRunOutput(ctx, runID, "stderr", result.Stderr)
		}
	}
	if result.ResponseBody != "" {
		o.queries.AppendRunOutput(ctx, db.AppendRunOutputParams{
			ID:      id.New("out_"),
			RunID:   runID,
			Stream:  "stdout",
			Content: result.ResponseBody,
		})
		if o.logBackend != nil {
			o.logBackend.WriteRunOutput(ctx, runID, "stdout", result.ResponseBody)
		}
	}

	_ = o.clearRunExecutionHandle(ctx, runID)

	if result.IsSuccess() {
		o.queries.UpdateRunState(ctx, db.UpdateRunStateParams{
			ID:         runID,
			State:      string(runstate.Completed),
			FinishedAt: dbutil.Timestamptz(finished),
			DurationMs: &durationMs,
			ExitCode:   exitCode,
		})
		metrics.RunsCompleted.Inc()
		metrics.RunDuration.Observe(float64(durationMs) / 1000.0)
		run, err := o.queries.GetRun(ctx, db.GetRunParams{ID: runID, WorkspaceID: workspaceID})
		if err == nil {
			o.handleTerminal(ctx, run, proc, runstate.Completed)
		}
		return
	}

	if attempt < proc.MaxAttempts {
		nextAt := calculateBackoff(finished, attempt, parseBackoffList(proc.RetryBackoff))
		o.queries.UpdateRunState(ctx, db.UpdateRunStateParams{
			ID:            runID,
			State:         string(runstate.Retrying),
			Attempt:       &attempt,
			NextAttemptAt: dbutil.Timestamptz(nextAt),
			DurationMs:    &durationMs,
		})
		return
	}

	ec := int32(-1)
	if exitCode != nil {
		ec = *exitCode
	}
	o.queries.UpdateRunState(ctx, db.UpdateRunStateParams{
		ID:         runID,
		State:      string(runstate.Failed),
		FinishedAt: dbutil.Timestamptz(finished),
		DurationMs: &durationMs,
		ExitCode:   &ec,
	})
	metrics.RunsFailed.Inc()
	metrics.RunDuration.Observe(float64(durationMs) / 1000.0)
	run, err := o.queries.GetRun(ctx, db.GetRunParams{ID: runID, WorkspaceID: workspaceID})
	if err == nil {
		o.handleTerminal(ctx, run, proc, runstate.Failed)
	}
}
