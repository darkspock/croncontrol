// Package ssm implements the AWS Systems Manager execution method.
//
// Canonical rules (from docs/product-specification.md):
// - Targeting supports instance IDs and tags.
// - Target must resolve to exactly one instance.
// - SSM profiles are reusable workspace resources.
// - Heartbeat supported (remote process calls heartbeat API).
package ssm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/croncontrol/croncontrol/internal/executor"
)

var _ executor.Method = (*Method)(nil)
var _ executor.BlockingMethod = (*Method)(nil)

// ProfileLoader loads SSM profile data (region, role_arn) by profile ID.
type ProfileLoader func(ctx context.Context, profileID string) (region string, roleARN string, err error)

// Method implements SSM execution.
type Method struct {
	loadProfile ProfileLoader

	// Active commands keyed by RunID for concurrent foreground kill support.
	activeCommands sync.Map // map[string]*activeSSMRef
}

type activeSSMRef struct {
	commandID *string
	client    *awsssm.Client
}

type profileConfig struct {
	profileID string
	region    string
	roleARN   string
}

type asyncHandleData struct {
	InstanceID string `json:"instance_id"`
	ProfileID  string `json:"profile_id,omitempty"`
	Region     string `json:"region"`
	RoleARN    string `json:"role_arn,omitempty"`
	BasePath   string `json:"base_path"`
	Pid        int    `json:"pid"`
}

type startPayload struct {
	Pid      int    `json:"pid"`
	BasePath string `json:"base_path"`
}

type pollPayload struct {
	State        string `json:"state"`
	ExitCode     *int   `json:"exit_code,omitempty"`
	StdoutB64    string `json:"stdout_b64,omitempty"`
	StderrB64    string `json:"stderr_b64,omitempty"`
	StdoutOffset int64  `json:"stdout_offset"`
	StderrOffset int64  `json:"stderr_offset"`
}

// New creates a new SSM execution method.
func New(loader ProfileLoader) *Method {
	return &Method{loadProfile: loader}
}

// Start launches the SSM execution.
// By default, SSM runs detached and returns a durable handle for polling.
// Set method_config.detach=false to force foreground blocking execution.
func (m *Method) Start(ctx context.Context, params executor.StartParams) (executor.StartResult, error) {
	if !isDetached(params.MethodConfig) {
		result, err := m.Execute(ctx, executor.ExecuteParams{
			RunID:        params.RunID,
			WorkspaceID:  params.WorkspaceID,
			MethodConfig: params.MethodConfig,
			Environment:  params.Environment,
			APIBaseURL:   params.APIBaseURL,
		})
		return executor.StartResult{
			Handle: executor.Handle{
				MethodName: "ssm",
				RunID:      params.RunID,
				Data:       map[string]any{},
			},
			AcceptedAt: time.Now().UTC(),
			Result:     &result,
		}, err
	}

	command, _ := params.MethodConfig["command"].(string)
	if command == "" {
		result := executor.Result{Error: fmt.Errorf("ssm: command is required")}
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "ssm", RunID: params.RunID, Data: map[string]any{}},
			Result: &result,
		}, nil
	}

	profile, err := m.resolveProfile(ctx, params.MethodConfig)
	if err != nil {
		result := executor.Result{Error: err}
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "ssm", RunID: params.RunID, Data: map[string]any{}},
			Result: &result,
		}, nil
	}

	client, err := m.buildClient(ctx, profile)
	if err != nil {
		result := executor.Result{Error: err}
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "ssm", RunID: params.RunID, Data: map[string]any{}},
			Result: &result,
		}, nil
	}

	instanceID, err := resolveTargetInstanceID(ctx, client, params.MethodConfig)
	if err != nil {
		result := executor.Result{Error: err}
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "ssm", RunID: params.RunID, Data: map[string]any{}},
			Result: &result,
		}, nil
	}

	basePath := remoteBasePath(params.RunID, params.MethodConfig)
	envPrefix := buildEnvPrefix(params.Environment, params.RunID, params.APIBaseURL)
	startScript := buildDetachedStartScript(basePath, envPrefix+command)

	result, err := runSSMCommand(ctx, client, instanceID, startScript, params.RunID, false)
	if err != nil {
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "ssm", RunID: params.RunID, Data: map[string]any{}},
			Result: &executor.Result{Error: err},
		}, nil
	}
	if result.Error != nil || !result.IsSuccess() {
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "ssm", RunID: params.RunID, Data: map[string]any{}},
			Result: &result,
		}, nil
	}

	var payload startPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &payload); err != nil {
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "ssm", RunID: params.RunID, Data: map[string]any{}},
			Result: &executor.Result{Error: fmt.Errorf("ssm: parse detached start payload: %w", err), Stdout: result.Stdout, Stderr: result.Stderr},
		}, nil
	}

	handle := executor.Handle{
		MethodName: "ssm",
		RunID:      params.RunID,
		Data: map[string]any{
			"instance_id": instanceID,
			"profile_id":  profile.profileID,
			"region":      profile.region,
			"role_arn":    profile.roleARN,
			"base_path":   payload.BasePath,
			"pid":         payload.Pid,
		},
	}

	return executor.StartResult{
		Handle:     handle,
		AcceptedAt: time.Now().UTC(),
		Result:     nil,
	}, nil
}

// Poll checks a detached SSM execution by reading its remote temp directory.
func (m *Method) Poll(ctx context.Context, handle executor.Handle, cursor executor.PollCursor) (executor.PollResult, error) {
	data, profile, err := m.profileFromHandle(ctx, handle)
	if err != nil {
		return executor.PollResult{}, err
	}

	client, err := m.buildClient(ctx, profile)
	if err != nil {
		return executor.PollResult{}, err
	}

	pollScript := buildDetachedPollScript(data.BasePath, cursor.StdoutOffset, cursor.StderrOffset)
	result, err := runSSMCommand(ctx, client, data.InstanceID, pollScript, handle.RunID, false)
	if err != nil {
		return executor.PollResult{}, err
	}
	if result.Error != nil {
		return executor.PollResult{}, result.Error
	}

	var payload pollPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &payload); err != nil {
		return executor.PollResult{}, fmt.Errorf("ssm: parse poll payload: %w", err)
	}

	stdoutChunk, err := decodeB64(payload.StdoutB64)
	if err != nil {
		return executor.PollResult{}, err
	}
	stderrChunk, err := decodeB64(payload.StderrB64)
	if err != nil {
		return executor.PollResult{}, err
	}

	pollResult := executor.PollResult{
		Cursor: executor.PollCursor{
			StdoutOffset: payload.StdoutOffset,
			StderrOffset: payload.StderrOffset,
		},
		StdoutChunk: stdoutChunk,
		StderrChunk: stderrChunk,
		ExitCode:    payload.ExitCode,
	}

	switch payload.State {
	case "running":
		pollResult.State = executor.RemoteRunning
	case "completed":
		pollResult.State = executor.RemoteCompleted
	case "failed":
		pollResult.State = executor.RemoteFailed
	case "killed":
		pollResult.State = executor.RemoteKilled
	default:
		pollResult.State = executor.RemoteFailed
		pollResult.Error = fmt.Errorf("ssm: unknown poll state %q", payload.State)
	}
	return pollResult, nil
}

// Execute runs SSM in foreground mode and blocks until completion.
func (m *Method) Execute(ctx context.Context, params executor.ExecuteParams) (executor.Result, error) {
	command, _ := params.MethodConfig["command"].(string)
	if command == "" {
		return executor.Result{Error: fmt.Errorf("ssm: command is required")}, nil
	}

	profile, err := m.resolveProfile(ctx, params.MethodConfig)
	if err != nil {
		return executor.Result{Error: err}, nil
	}

	client, err := m.buildClient(ctx, profile)
	if err != nil {
		return executor.Result{Error: err}, nil
	}

	instanceID, err := resolveTargetInstanceID(ctx, client, params.MethodConfig)
	if err != nil {
		return executor.Result{Error: err}, nil
	}

	fullCommand := buildEnvPrefix(params.Environment, params.RunID, params.APIBaseURL) + command
	start := time.Now()
	sendOut, err := client.SendCommand(ctx, &awsssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		Targets: []ssmtypes.Target{{
			Key:    aws.String("InstanceIds"),
			Values: []string{instanceID},
		}},
		Parameters: map[string][]string{
			"commands": {fullCommand},
		},
		TimeoutSeconds: aws.Int32(3600),
		Comment:        aws.String(fmt.Sprintf("CronControl run %s", params.RunID)),
	})
	if err != nil {
		return executor.Result{Error: fmt.Errorf("ssm: send command: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
	}

	commandID := sendOut.Command.CommandId
	m.activeCommands.Store(params.RunID, &activeSSMRef{commandID: commandID, client: client})
	defer m.activeCommands.Delete(params.RunID)

	result, err := pollCommandResult(ctx, client, aws.ToString(commandID), instanceID)
	result.DurationMs = time.Since(start).Milliseconds()
	if err != nil {
		return executor.Result{Error: fmt.Errorf("ssm: poll result: %w", err), DurationMs: result.DurationMs}, nil
	}
	return result, nil
}

func (m *Method) Kill(ctx context.Context, handle executor.Handle) error {
	if val, ok := m.activeCommands.Load(handle.RunID); ok {
		ref := val.(*activeSSMRef)
		_, err := ref.client.CancelCommand(ctx, &awsssm.CancelCommandInput{
			CommandId: ref.commandID,
		})
		return err
	}

	data, profile, err := m.profileFromHandle(ctx, handle)
	if err != nil {
		return err
	}
	client, err := m.buildClient(ctx, profile)
	if err != nil {
		return err
	}
	killScript := buildDetachedKillScript(data.BasePath, data.Pid)
	result, err := runSSMCommand(ctx, client, data.InstanceID, killScript, handle.RunID, false)
	if err != nil {
		return err
	}
	return result.Error
}

func (m *Method) SupportsKill() bool      { return true }
func (m *Method) SupportsHeartbeat() bool { return true }
func (m *Method) IsAsync() bool           { return true }

func (m *Method) resolveProfile(ctx context.Context, cfg map[string]any) (profileConfig, error) {
	profile := profileConfig{
		profileID: getString(cfg, "ssm_profile_id"),
		region:    "us-east-1",
	}
	if profile.profileID != "" && m.loadProfile != nil {
		region, roleARN, err := m.loadProfile(ctx, profile.profileID)
		if err != nil {
			return profileConfig{}, fmt.Errorf("ssm: load profile: %w", err)
		}
		if region != "" {
			profile.region = region
		}
		profile.roleARN = roleARN
	}
	if region := getString(cfg, "region"); region != "" {
		profile.region = region
	}
	return profile, nil
}

func (m *Method) profileFromHandle(ctx context.Context, handle executor.Handle) (asyncHandleData, profileConfig, error) {
	data := asyncHandleData{
		InstanceID: getString(handle.Data, "instance_id"),
		ProfileID:  getString(handle.Data, "profile_id"),
		Region:     getString(handle.Data, "region"),
		RoleARN:    getString(handle.Data, "role_arn"),
		BasePath:   getString(handle.Data, "base_path"),
		Pid:        getInt(handle.Data, "pid"),
	}
	if data.InstanceID == "" || data.BasePath == "" {
		return data, profileConfig{}, fmt.Errorf("ssm: incomplete handle data")
	}
	profile := profileConfig{
		profileID: data.ProfileID,
		region:    data.Region,
		roleARN:   data.RoleARN,
	}
	if profile.region == "" {
		profile.region = "us-east-1"
	}
	if profile.profileID != "" && m.loadProfile != nil {
		region, roleARN, err := m.loadProfile(ctx, profile.profileID)
		if err == nil {
			if region != "" {
				profile.region = region
			}
			if roleARN != "" {
				profile.roleARN = roleARN
			}
		}
	}
	return data, profile, nil
}

func (m *Method) buildClient(ctx context.Context, profile profileConfig) (*awsssm.Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(profile.region))
	if err != nil {
		return nil, fmt.Errorf("ssm: load aws config: %w", err)
	}
	if profile.roleARN != "" {
		stsClient := sts.NewFromConfig(awsCfg)
		creds := stscreds.NewAssumeRoleProvider(stsClient, profile.roleARN)
		awsCfg.Credentials = aws.NewCredentialsCache(creds)
	}
	return awsssm.NewFromConfig(awsCfg), nil
}

func resolveTargetInstanceID(ctx context.Context, client *awsssm.Client, cfg map[string]any) (string, error) {
	if instanceID := getString(cfg, "instance_id"); instanceID != "" {
		return instanceID, nil
	}

	tagKey := getString(cfg, "tag_key")
	tagValue := getString(cfg, "tag_value")
	if tagKey == "" || tagValue == "" {
		return "", fmt.Errorf("ssm: instance_id or tag_key+tag_value is required")
	}

	out, err := client.DescribeInstanceInformation(ctx, &awsssm.DescribeInstanceInformationInput{
		Filters: []ssmtypes.InstanceInformationStringFilter{
			{Key: aws.String("tag-key"), Values: []string{tagKey}},
			{Key: aws.String("tag:" + tagKey), Values: []string{tagValue}},
		},
	})
	if err != nil {
		return "", fmt.Errorf("ssm: describe instances: %w", err)
	}
	if len(out.InstanceInformationList) != 1 {
		return "", fmt.Errorf("ssm: target resolved to %d instances, expected exactly 1", len(out.InstanceInformationList))
	}
	return aws.ToString(out.InstanceInformationList[0].InstanceId), nil
}

func runSSMCommand(ctx context.Context, client *awsssm.Client, instanceID, command, runID string, trackActive bool) (executor.Result, error) {
	start := time.Now()
	sendOut, err := client.SendCommand(ctx, &awsssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		Targets: []ssmtypes.Target{{
			Key:    aws.String("InstanceIds"),
			Values: []string{instanceID},
		}},
		Parameters: map[string][]string{
			"commands": {command},
		},
		TimeoutSeconds: aws.Int32(3600),
		Comment:        aws.String(fmt.Sprintf("CronControl run %s", runID)),
	})
	if err != nil {
		return executor.Result{Error: fmt.Errorf("ssm: send command: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
	}

	commandID := sendOut.Command.CommandId
	if trackActive {
		active := &activeSSMRef{commandID: commandID, client: client}
		// Caller must clean map; this helper cannot.
		_ = active
	}

	result, err := pollCommandResult(ctx, client, aws.ToString(commandID), instanceID)
	result.DurationMs = time.Since(start).Milliseconds()
	if err != nil {
		return executor.Result{Error: fmt.Errorf("ssm: poll result: %w", err), DurationMs: result.DurationMs}, nil
	}
	return result, nil
}

func pollCommandResult(ctx context.Context, client *awsssm.Client, commandID, instanceID string) (executor.Result, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return executor.Result{}, ctx.Err()
		case <-ticker.C:
			out, err := client.GetCommandInvocation(ctx, &awsssm.GetCommandInvocationInput{
				CommandId:  aws.String(commandID),
				InstanceId: aws.String(instanceID),
			})
			if err != nil {
				if strings.Contains(err.Error(), "InvocationDoesNotExist") {
					continue
				}
				return executor.Result{}, err
			}

			switch out.Status {
			case ssmtypes.CommandInvocationStatusSuccess:
				exitCode := 0
				return executor.Result{
					ExitCode: &exitCode,
					Stdout:   aws.ToString(out.StandardOutputContent),
					Stderr:   aws.ToString(out.StandardErrorContent),
				}, nil
			case ssmtypes.CommandInvocationStatusFailed, ssmtypes.CommandInvocationStatusTimedOut:
				exitCode := int(out.ResponseCode)
				if exitCode == 0 {
					exitCode = 1
				}
				return executor.Result{
					ExitCode: &exitCode,
					Stdout:   aws.ToString(out.StandardOutputContent),
					Stderr:   aws.ToString(out.StandardErrorContent),
					Error:    fmt.Errorf("ssm command %s: %s", out.Status, aws.ToString(out.StatusDetails)),
				}, nil
			case ssmtypes.CommandInvocationStatusCancelled:
				exitCode := 137
				return executor.Result{
					ExitCode: &exitCode,
					Stdout:   aws.ToString(out.StandardOutputContent),
					Stderr:   aws.ToString(out.StandardErrorContent),
					Error:    fmt.Errorf("ssm command cancelled"),
				}, nil
			default:
				continue
			}
		}
	}
}

var validEnvKeyRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func buildEnvPrefix(env map[string]string, runID, apiBaseURL string) string {
	var b strings.Builder
	for k, v := range env {
		if !validEnvKeyRe.MatchString(k) {
			slog.Warn("ssm: skipping invalid env var key", "key", k)
			continue
		}
		b.WriteString("export ")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(shellQuote(v))
		b.WriteString("; ")
	}
	if runID != "" {
		b.WriteString("export CRONCONTROL_RUN_ID=")
		b.WriteString(shellQuote(runID))
		b.WriteString("; ")
	}
	if apiBaseURL != "" {
		b.WriteString("export CRONCONTROL_API_URL=")
		b.WriteString(shellQuote(apiBaseURL))
		b.WriteString("; ")
	}
	return b.String()
}

func isDetached(cfg map[string]any) bool {
	if v, ok := cfg["detach"].(bool); ok {
		return v
	}
	if v, ok := cfg["wait"].(bool); ok {
		return !v
	}
	return true
}

func remoteBasePath(runID string, cfg map[string]any) string {
	root := getString(cfg, "remote_tmp_dir")
	if root == "" {
		root = "/var/tmp/croncontrol"
	}
	return strings.TrimRight(root, "/") + "/" + runID
}

func buildDetachedStartScript(basePath, command string) string {
	return fmt.Sprintf(`
set -eu
base=%s
mkdir -p "$base"
: > "$base/stdout.log"
: > "$base/stderr.log"
date -u +"%%Y-%%m-%%dT%%H:%%M:%%SZ" > "$base/started_at"
nohup sh -lc %s >>"$base/stdout.log" 2>>"$base/stderr.log" < /dev/null &
pid=$!
printf "%%s\n" "$pid" > "$base/pid"
printf '{"pid":%%s,"base_path":"%%s"}\n' "$pid" "$base"
`, shellQuote(basePath), shellQuote(buildDetachedRunnerCommand(basePath, command)))
}

func buildDetachedRunnerCommand(basePath, command string) string {
	return fmt.Sprintf(`%s; code=$?; printf "%%s" "$code" > %s; date -u +"%%Y-%%m-%%dT%%H:%%M:%%SZ" > %s; exit "$code"`,
		command,
		shellQuote(basePath+"/exit_code"),
		shellQuote(basePath+"/finished_at"),
	)
}

func buildDetachedPollScript(basePath string, stdoutOffset, stderrOffset int64) string {
	return fmt.Sprintf(`
set -eu
base=%s
stdout_offset=%d
stderr_offset=%d
stdout_file="$base/stdout.log"
stderr_file="$base/stderr.log"
pid_file="$base/pid"
exit_file="$base/exit_code"
kill_file="$base/kill_requested"

stdout_size=0
stderr_size=0
stdout_b64=""
stderr_b64=""

if [ -f "$stdout_file" ]; then
  stdout_size=$(wc -c < "$stdout_file" | tr -d ' ')
fi
if [ -f "$stderr_file" ]; then
  stderr_size=$(wc -c < "$stderr_file" | tr -d ' ')
fi
if [ "$stdout_size" -gt "$stdout_offset" ]; then
  stdout_b64=$(dd if="$stdout_file" bs=1 skip="$stdout_offset" 2>/dev/null | base64 | tr -d '\n')
fi
if [ "$stderr_size" -gt "$stderr_offset" ]; then
  stderr_b64=$(dd if="$stderr_file" bs=1 skip="$stderr_offset" 2>/dev/null | base64 | tr -d '\n')
fi

state="running"
exit_code="null"
pid=""
if [ -f "$pid_file" ]; then
  pid=$(cat "$pid_file")
fi

if [ -f "$exit_file" ]; then
  exit_code=$(cat "$exit_file")
  if [ "$exit_code" = "0" ] && [ ! -f "$kill_file" ]; then
    state="completed"
  elif [ -f "$kill_file" ]; then
    state="killed"
  else
    state="failed"
  fi
elif [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
  state="running"
elif [ -f "$kill_file" ]; then
  state="killed"
else
  state="failed"
fi

printf '{"state":"%%s","exit_code":%%s,"stdout_b64":"%%s","stderr_b64":"%%s","stdout_offset":%%s,"stderr_offset":%%s}\n' \
  "$state" "$exit_code" "$stdout_b64" "$stderr_b64" "$stdout_size" "$stderr_size"
`, shellQuote(basePath), stdoutOffset, stderrOffset)
}

func buildDetachedKillScript(basePath string, fallbackPID int) string {
	return fmt.Sprintf(`
set -eu
base=%s
pid_file="$base/pid"
kill_file="$base/kill_requested"
printf "kill\n" > "$kill_file"
pid=%d
if [ -f "$pid_file" ]; then
  pid=$(cat "$pid_file")
fi
if [ "$pid" -gt 0 ] 2>/dev/null; then
  kill -TERM "$pid" 2>/dev/null || true
  sleep 5
  if kill -0 "$pid" 2>/dev/null; then
    kill -KILL "$pid" 2>/dev/null || true
  fi
fi
`, shellQuote(basePath), fallbackPID)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func decodeB64(v string) (string, error) {
	if v == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return "", fmt.Errorf("ssm: decode base64 chunk: %w", err)
	}
	return string(data), nil
}

func getString(cfg map[string]any, key string) string {
	v, _ := cfg[key].(string)
	return v
}

func getInt(cfg map[string]any, key string) int {
	switch v := cfg[key].(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(v)
		return i
	default:
		return 0
	}
}
