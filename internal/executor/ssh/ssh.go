// Package ssh implements the SSH execution method.
//
// Canonical rules (from docs/product-specification.md):
// - Key-based auth only.
// - Strict host key verification.
// - SSH credentials are reusable workspace resources.
// - Host/target stays inline on the process or queue method_config.
// - Heartbeat supported (remote process calls heartbeat API).
package ssh

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/croncontrol/croncontrol/internal/executor"
)

var _ executor.Method = (*Method)(nil)
var _ executor.BlockingMethod = (*Method)(nil)

// CredentialLoader loads decrypted SSH private key bytes and metadata by credential ID.
// strictHostKey indicates whether host key verification is enforced for this credential.
type CredentialLoader func(ctx context.Context, credentialID string) (privateKey []byte, username string, port int, strictHostKey bool, err error)

// Method implements SSH execution.
type Method struct {
	loadCredential CredentialLoader

	// Active session tracking for foreground kill support, keyed by RunID.
	sessions sync.Map // map[string]*gossh.Session
}

type connectionConfig struct {
	host          string
	username      string
	port          int
	privateKey    []byte
	strictHostKey bool
	hostKeyB64    string
	credentialID  string
}

type asyncHandleData struct {
	Host          string `json:"host"`
	Username      string `json:"username"`
	Port          int    `json:"port"`
	CredentialID  string `json:"credential_id,omitempty"`
	StrictHostKey bool   `json:"strict_host_key"`
	HostKeyB64    string `json:"host_key,omitempty"`
	BasePath      string `json:"base_path"`
	Pid           int    `json:"pid"`
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

// New creates a new SSH execution method.
func New(loader CredentialLoader) *Method {
	return &Method{loadCredential: loader}
}

// Start launches the SSH execution.
// By default, SSH runs detached and returns a durable handle for polling.
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
				MethodName: "ssh",
				RunID:      params.RunID,
				Data:       map[string]any{},
			},
			AcceptedAt: time.Now().UTC(),
			Result:     &result,
		}, err
	}

	cfg, err := m.resolveConnectionConfig(ctx, params.MethodConfig)
	if err != nil {
		result := executor.Result{Error: err}
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "ssh", RunID: params.RunID, Data: map[string]any{}},
			Result: &result,
		}, nil
	}

	command, _ := params.MethodConfig["command"].(string)
	if command == "" {
		result := executor.Result{Error: fmt.Errorf("ssh: command is required")}
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "ssh", RunID: params.RunID, Data: map[string]any{}},
			Result: &result,
		}, nil
	}

	client, err := m.connect(cfg, params.MethodConfig)
	if err != nil {
		result := executor.Result{Error: err}
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "ssh", RunID: params.RunID, Data: map[string]any{}},
			Result: &result,
		}, nil
	}
	defer client.Close()

	basePath := remoteBasePath(params.RunID, params.MethodConfig)
	startScript := buildDetachedStartScript(basePath, command)

	stdout, stderr, err := runRemoteCommand(client, nil, nil, params.RunID, params.APIBaseURL, startScript)
	if err != nil {
		result := executor.Result{
			Error:  fmt.Errorf("ssh: start detached command: %w", err),
			Stdout: stdout,
			Stderr: stderr,
		}
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "ssh", RunID: params.RunID, Data: map[string]any{}},
			Result: &result,
		}, nil
	}

	var payload startPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
		result := executor.Result{
			Error:  fmt.Errorf("ssh: parse detached start payload: %w", err),
			Stdout: stdout,
			Stderr: stderr,
		}
		return executor.StartResult{
			Handle: executor.Handle{MethodName: "ssh", RunID: params.RunID, Data: map[string]any{}},
			Result: &result,
		}, nil
	}

	handle := executor.Handle{
		MethodName: "ssh",
		RunID:      params.RunID,
		Data: map[string]any{
			"host":            cfg.host,
			"username":        cfg.username,
			"port":            cfg.port,
			"credential_id":   cfg.credentialID,
			"strict_host_key": cfg.strictHostKey,
			"host_key":        cfg.hostKeyB64,
			"base_path":       payload.BasePath,
			"pid":             payload.Pid,
		},
	}

	return executor.StartResult{
		Handle:     handle,
		AcceptedAt: time.Now().UTC(),
		Result:     nil,
	}, nil
}

// Poll checks a detached SSH execution by reading its remote temp directory.
func (m *Method) Poll(ctx context.Context, handle executor.Handle, cursor executor.PollCursor) (executor.PollResult, error) {
	cfg, handleData, err := m.connectionConfigFromHandle(ctx, handle)
	if err != nil {
		return executor.PollResult{}, err
	}

	client, err := m.connect(cfg, map[string]any{"host_key": handleData.HostKeyB64})
	if err != nil {
		return executor.PollResult{}, err
	}
	defer client.Close()

	pollScript := buildDetachedPollScript(handleData.BasePath, cursor.StdoutOffset, cursor.StderrOffset)
	stdout, stderr, err := runRemoteCommand(client, nil, nil, handle.RunID, "", pollScript)
	if err != nil {
		return executor.PollResult{}, fmt.Errorf("ssh: poll command failed: %w (stderr: %s)", err, stderr)
	}

	var payload pollPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
		return executor.PollResult{}, fmt.Errorf("ssh: parse poll payload: %w", err)
	}

	stdoutChunk, err := decodeB64(payload.StdoutB64)
	if err != nil {
		return executor.PollResult{}, err
	}
	stderrChunk, err := decodeB64(payload.StderrB64)
	if err != nil {
		return executor.PollResult{}, err
	}

	result := executor.PollResult{
		Cursor: executor.PollCursor{
			StdoutOffset: payload.StdoutOffset,
			StderrOffset: payload.StderrOffset,
		},
		StdoutChunk: stdoutChunk,
		StderrChunk: stderrChunk,
	}

	switch payload.State {
	case "running":
		result.State = executor.RemoteRunning
	case "completed":
		result.State = executor.RemoteCompleted
	case "failed":
		result.State = executor.RemoteFailed
	case "killed":
		result.State = executor.RemoteKilled
	default:
		result.State = executor.RemoteFailed
		result.Error = fmt.Errorf("ssh: unknown poll state %q", payload.State)
	}
	result.ExitCode = payload.ExitCode
	return result, nil
}

// Execute runs SSH in foreground mode and blocks until completion.
func (m *Method) Execute(ctx context.Context, params executor.ExecuteParams) (executor.Result, error) {
	cfg, err := m.resolveConnectionConfig(ctx, params.MethodConfig)
	if err != nil {
		return executor.Result{Error: err}, nil
	}

	command, _ := params.MethodConfig["command"].(string)
	if command == "" {
		return executor.Result{Error: fmt.Errorf("ssh: command is required")}, nil
	}

	start := time.Now()
	client, err := m.connect(cfg, params.MethodConfig)
	if err != nil {
		return executor.Result{Error: err, DurationMs: time.Since(start).Milliseconds()}, nil
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return executor.Result{Error: fmt.Errorf("ssh: create session: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
	}
	defer session.Close()

	m.sessions.Store(params.RunID, session)
	defer m.sessions.Delete(params.RunID)

	for k, v := range params.Environment {
		_ = session.Setenv(k, v)
	}
	_ = session.Setenv("CRONCONTROL_RUN_ID", params.RunID)
	if params.APIBaseURL != "" {
		_ = session.Setenv("CRONCONTROL_API_URL", params.APIBaseURL)
	}

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(command)
	durationMs := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*gossh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			return executor.Result{
				Error:      fmt.Errorf("ssh: run command: %w", err),
				Stdout:     stdout.String(),
				Stderr:     stderr.String(),
				DurationMs: durationMs,
			}, nil
		}
	}

	return executor.Result{
		ExitCode:   &exitCode,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		DurationMs: durationMs,
	}, nil
}

func (m *Method) Kill(ctx context.Context, handle executor.Handle) error {
	if val, ok := m.sessions.Load(handle.RunID); ok {
		session := val.(*gossh.Session)
		if err := session.Signal(gossh.SIGTERM); err != nil {
			slog.Warn("ssh: SIGTERM failed, closing session", "error", err)
			return session.Close()
		}
		return nil
	}

	cfg, handleData, err := m.connectionConfigFromHandle(ctx, handle)
	if err != nil {
		return err
	}

	client, err := m.connect(cfg, map[string]any{"host_key": handleData.HostKeyB64})
	if err != nil {
		return err
	}
	defer client.Close()

	killScript := buildDetachedKillScript(handleData.BasePath, handleData.Pid)
	_, stderr, err := runRemoteCommand(client, nil, nil, handle.RunID, "", killScript)
	if err != nil {
		return fmt.Errorf("ssh: detached kill failed: %w (stderr: %s)", err, stderr)
	}
	return nil
}

func (m *Method) SupportsKill() bool      { return true }
func (m *Method) SupportsHeartbeat() bool { return true }
func (m *Method) IsAsync() bool           { return true }

// discoveryResult is the expected response from a discovery URL.
type discoveryResult struct {
	Host string `json:"host"`
	Port int    `json:"port,omitempty"`
	User string `json:"user,omitempty"`
}

func discover(ctx context.Context, url string) (*discoveryResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("discovery returned %d: %s", resp.StatusCode, string(body))
	}

	var result discoveryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode discovery response: %w", err)
	}

	if result.Host == "" {
		return nil, fmt.Errorf("discovery response missing 'host'")
	}

	ips, err := net.LookupHost(result.Host)
	if err != nil {
		return nil, fmt.Errorf("discovery host DNS resolution failed: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("discovery host resolved to no addresses: %s", result.Host)
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
			return nil, fmt.Errorf("discovery returned private/loopback host: %s (%s)", result.Host, ipStr)
		}
	}

	result.Host = ips[0]
	return &result, nil
}

func (m *Method) resolveConnectionConfig(ctx context.Context, cfg map[string]any) (connectionConfig, error) {
	commandHost, _ := cfg["host"].(string)

	if discoveryURL, ok := cfg["discovery_url"].(string); ok && discoveryURL != "" {
		resolved, err := discover(ctx, discoveryURL)
		if err != nil {
			return connectionConfig{}, fmt.Errorf("ssh discovery failed: %w", err)
		}
		if resolved.Host != "" {
			commandHost = resolved.Host
		}
		if resolved.Port > 0 {
			cfg["port"] = float64(resolved.Port)
		}
		if resolved.User != "" {
			cfg["username"] = resolved.User
		}
	}

	if commandHost == "" {
		return connectionConfig{}, fmt.Errorf("ssh: host is required")
	}

	cc := connectionConfig{
		host:       commandHost,
		port:       22,
		hostKeyB64: getString(cfg, "host_key"),
	}

	if username := getString(cfg, "username"); username != "" {
		cc.username = username
	}
	if port := getInt(cfg, "port"); port > 0 {
		cc.port = port
	}

	if credentialID := getString(cfg, "ssh_credential_id"); credentialID != "" {
		if m.loadCredential == nil {
			return connectionConfig{}, fmt.Errorf("ssh: credential loader not configured")
		}
		privateKey, username, port, strictHostKey, err := m.loadCredential(ctx, credentialID)
		if err != nil {
			return connectionConfig{}, fmt.Errorf("ssh: load credential: %w", err)
		}
		cc.privateKey = privateKey
		cc.credentialID = credentialID
		cc.strictHostKey = strictHostKey
		if cc.username == "" {
			cc.username = username
		}
		if getInt(cfg, "port") == 0 && port > 0 {
			cc.port = port
		}
	} else {
		privateKey := getString(cfg, "private_key")
		if privateKey == "" {
			return connectionConfig{}, fmt.Errorf("ssh: ssh_credential_id or inline private_key is required")
		}
		cc.privateKey = []byte(privateKey)
		cc.strictHostKey = getBool(cfg, "strict_host_key", false)
		if cc.username == "" {
			cc.username = getString(cfg, "user")
		}
	}

	if cc.username == "" {
		return connectionConfig{}, fmt.Errorf("ssh: username is required")
	}
	return cc, nil
}

func (m *Method) connectionConfigFromHandle(ctx context.Context, handle executor.Handle) (connectionConfig, asyncHandleData, error) {
	data := asyncHandleData{
		Host:          getString(handle.Data, "host"),
		Username:      getString(handle.Data, "username"),
		Port:          getInt(handle.Data, "port"),
		CredentialID:  getString(handle.Data, "credential_id"),
		StrictHostKey: getBool(handle.Data, "strict_host_key", false),
		HostKeyB64:    getString(handle.Data, "host_key"),
		BasePath:      getString(handle.Data, "base_path"),
		Pid:           getInt(handle.Data, "pid"),
	}
	if data.Host == "" || data.Username == "" || data.BasePath == "" {
		return connectionConfig{}, data, fmt.Errorf("ssh: incomplete handle data")
	}
	if data.Port == 0 {
		data.Port = 22
	}

	cfg := connectionConfig{
		host:          data.Host,
		username:      data.Username,
		port:          data.Port,
		strictHostKey: data.StrictHostKey,
		hostKeyB64:    data.HostKeyB64,
		credentialID:  data.CredentialID,
	}

	if data.CredentialID != "" {
		privateKey, _, _, _, err := m.loadCredential(ctx, data.CredentialID)
		if err != nil {
			return connectionConfig{}, data, fmt.Errorf("ssh: reload credential: %w", err)
		}
		cfg.privateKey = privateKey
	} else {
		return connectionConfig{}, data, fmt.Errorf("ssh: detached handle without credential_id is not supported")
	}

	return cfg, data, nil
}

func (m *Method) connect(cfg connectionConfig, methodCfg map[string]any) (*gossh.Client, error) {
	signer, err := gossh.ParsePrivateKey(cfg.privateKey)
	if err != nil {
		return nil, fmt.Errorf("ssh: parse key: %w", err)
	}

	hostKeyCallback := buildHostKeyCallback(cfg.strictHostKey, methodCfg, cfg.hostKeyB64)
	sshConfig := &gossh.ClientConfig{
		User:            cfg.username,
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCallback,
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", cfg.host, cfg.port)
	client, err := gossh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh: connect to %s: %w", addr, err)
	}
	return client, nil
}

func buildHostKeyCallback(strict bool, cfg map[string]any, fallbackB64 string) gossh.HostKeyCallback {
	if !strict {
		return gossh.InsecureIgnoreHostKey()
	}

	hostKeyB64 := fallbackB64
	if hostKeyB64 == "" {
		hostKeyB64 = getString(cfg, "host_key")
	}
	if hostKeyB64 != "" {
		hostKeyBytes, err := base64.StdEncoding.DecodeString(hostKeyB64)
		if err == nil {
			pubKey, err := gossh.ParsePublicKey(hostKeyBytes)
			if err == nil {
				return gossh.FixedHostKey(pubKey)
			}
			slog.Warn("ssh: failed to parse host_key from config, rejecting connections", "error", err)
		} else {
			slog.Warn("ssh: failed to decode host_key base64 from config, rejecting connections", "error", err)
		}
	}

	return func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		return fmt.Errorf("ssh: strict host key verification enabled but no host_key configured in method_config (host: %s, key type: %s)", hostname, key.Type())
	}
}

func runRemoteCommand(
	client *gossh.Client,
	env map[string]string,
	extraEnv map[string]string,
	runID string,
	apiBaseURL string,
	command string,
) (string, string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("ssh: create session: %w", err)
	}
	defer session.Close()

	for k, v := range env {
		_ = session.Setenv(k, v)
	}
	for k, v := range extraEnv {
		_ = session.Setenv(k, v)
	}
	if runID != "" {
		_ = session.Setenv("CRONCONTROL_RUN_ID", runID)
	}
	if apiBaseURL != "" {
		_ = session.Setenv("CRONCONTROL_API_URL", apiBaseURL)
	}

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	err = session.Run(command)
	return stdout.String(), stderr.String(), err
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
		return "", fmt.Errorf("ssh: decode base64 chunk: %w", err)
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

func getBool(cfg map[string]any, key string, fallback bool) bool {
	v, ok := cfg[key]
	if !ok {
		return fallback
	}
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return strings.EqualFold(b, "true")
	default:
		return fallback
	}
}
