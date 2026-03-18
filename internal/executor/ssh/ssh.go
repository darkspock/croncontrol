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
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/croncontrol/croncontrol/internal/executor"
)

// Compile-time contract.
var _ executor.Method = (*Method)(nil)

// CredentialLoader loads decrypted SSH private key bytes and metadata by credential ID.
// strictHostKey indicates whether host key verification is enforced for this credential.
type CredentialLoader func(ctx context.Context, credentialID string) (privateKey []byte, username string, port int, strictHostKey bool, err error)

// Method implements SSH execution.
type Method struct {
	loadCredential CredentialLoader

	// Active session tracking for kill support, keyed by RunID.
	sessions sync.Map // map[string]*ssh.Session
}

// New creates a new SSH execution method.
func New(loader CredentialLoader) *Method {
	return &Method{loadCredential: loader}
}

func (m *Method) Execute(ctx context.Context, params executor.ExecuteParams) (executor.Result, error) {
	cfg := params.MethodConfig
	start := time.Now()

	command, _ := cfg["command"].(string)
	if command == "" {
		return executor.Result{Error: fmt.Errorf("ssh: command is required")}, nil
	}

	host, _ := cfg["host"].(string)
	credentialID, _ := cfg["ssh_credential_id"].(string)

	// Discovery mode: call URL to dynamically resolve host (and optionally port/user)
	if discoveryURL, ok := cfg["discovery_url"].(string); ok && discoveryURL != "" {
		resolved, err := discover(ctx, discoveryURL)
		if err != nil {
			return executor.Result{Error: fmt.Errorf("ssh discovery failed: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
		}
		if resolved.Host != "" {
			host = resolved.Host
		}
		if resolved.Port > 0 {
			cfg["port"] = float64(resolved.Port)
		}
		if resolved.User != "" {
			cfg["username"] = resolved.User
		}
	}

	if host == "" {
		return executor.Result{Error: fmt.Errorf("ssh: host is required")}, nil
	}

	// Load credential
	if credentialID == "" {
		return executor.Result{Error: fmt.Errorf("ssh: ssh_credential_id is required")}, nil
	}

	privateKey, username, port, strictHostKey, err := m.loadCredential(ctx, credentialID)
	if err != nil {
		return executor.Result{Error: fmt.Errorf("ssh: load credential: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
	}

	// Override from config
	if u, ok := cfg["username"].(string); ok && u != "" {
		username = u
	}
	if p, ok := cfg["port"].(float64); ok && p > 0 {
		port = int(p)
	}
	if port == 0 {
		port = 22
	}

	// Parse key
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return executor.Result{Error: fmt.Errorf("ssh: parse key: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
	}

	// Build host key callback
	hostKeyCallback := buildHostKeyCallback(strictHostKey, cfg)

	// Connect
	sshConfig := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCallback,
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return executor.Result{Error: fmt.Errorf("ssh: connect to %s: %w", addr, err), DurationMs: time.Since(start).Milliseconds()}, nil
	}
	defer func() {
		m.sessions.Delete(params.RunID)
		client.Close()
	}()

	// Create session
	session, err := client.NewSession()
	if err != nil {
		return executor.Result{Error: fmt.Errorf("ssh: create session: %w", err), DurationMs: time.Since(start).Milliseconds()}, nil
	}
	defer session.Close()

	// Track active session for kill support (keyed by RunID for concurrency safety)
	m.sessions.Store(params.RunID, session)

	// Set environment variables
	if params.Environment != nil {
		for k, v := range params.Environment {
			session.Setenv(k, v)
		}
	}
	session.Setenv("CRONCONTROL_RUN_ID", params.RunID)
	if params.APIBaseURL != "" {
		session.Setenv("CRONCONTROL_API_URL", params.APIBaseURL)
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Run command
	err = session.Run(command)
	durationMs := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
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

func (m *Method) Kill(_ context.Context, handle executor.Handle) error {
	val, ok := m.sessions.Load(handle.RunID)
	if !ok {
		return fmt.Errorf("ssh: no active session for run %s", handle.RunID)
	}
	session := val.(*ssh.Session)

	// Send SIGTERM to the remote process, then SIGKILL if needed.
	if err := session.Signal(ssh.SIGTERM); err != nil {
		// Some servers don't support Signal; close the session to force termination.
		slog.Warn("ssh: SIGTERM failed, closing session", "error", err)
		return session.Close()
	}
	return nil
}

// buildHostKeyCallback returns the appropriate host key callback based on configuration.
// When strictHostKey is true:
//   - If method_config contains "host_key" (base64 public key), verify against it.
//   - Otherwise, reject the connection.
//
// When strictHostKey is false, accept any host key (for development/testing).
func buildHostKeyCallback(strict bool, cfg map[string]any) ssh.HostKeyCallback {
	if !strict {
		return ssh.InsecureIgnoreHostKey()
	}

	// Check if a known host key is provided in method_config
	if hostKeyB64, ok := cfg["host_key"].(string); ok && hostKeyB64 != "" {
		hostKeyBytes, err := base64.StdEncoding.DecodeString(hostKeyB64)
		if err == nil {
			pubKey, err := ssh.ParsePublicKey(hostKeyBytes)
			if err == nil {
				return ssh.FixedHostKey(pubKey)
			}
			slog.Warn("ssh: failed to parse host_key from config, rejecting connections", "error", err)
		} else {
			slog.Warn("ssh: failed to decode host_key base64 from config, rejecting connections", "error", err)
		}
	}

	// Strict mode with no known key: reject all connections
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		return fmt.Errorf("ssh: strict host key verification enabled but no host_key configured in method_config (host: %s, key type: %s)", hostname, key.Type())
	}
}

func (m *Method) SupportsKill() bool     { return true }
func (m *Method) SupportsHeartbeat() bool { return true }

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

	// Validate and resolve host to prevent DNS rebinding.
	// We resolve once here and use the resolved IP as the host for ssh.Dial.
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

	// Use the resolved IP directly to prevent DNS rebinding between validation and connection.
	result.Host = ips[0]

	return &result, nil
}
