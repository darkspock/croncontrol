package executor

import (
	"context"
	"testing"
)

type fakeBlockingMethod struct {
	result            Result
	err               error
	lastExecuteParams ExecuteParams
	lastKillHandle    Handle
	supportsKill      bool
	supportsHeartbeat bool
}

func (m *fakeBlockingMethod) Execute(_ context.Context, params ExecuteParams) (Result, error) {
	m.lastExecuteParams = params
	return m.result, m.err
}

func (m *fakeBlockingMethod) Kill(_ context.Context, handle Handle) error {
	m.lastKillHandle = handle
	return nil
}

func (m *fakeBlockingMethod) SupportsKill() bool {
	return m.supportsKill
}

func (m *fakeBlockingMethod) SupportsHeartbeat() bool {
	return m.supportsHeartbeat
}

func TestRegisterBlockingWrapsLegacyMethod(t *testing.T) {
	reg := NewRegistry()
	legacy := &fakeBlockingMethod{
		result: Result{Stdout: "ok"},
	}

	reg.RegisterBlocking("fake", legacy)

	method, ok := reg.Get("fake")
	if !ok {
		t.Fatal("expected registered method")
	}
	if method.IsAsync() {
		t.Fatal("blocking adapter should not be async")
	}

	startResult, err := method.Start(context.Background(), StartParams{
		RunID:        "run_123",
		WorkspaceID:  "ws_123",
		MethodConfig: map[string]any{"k": "v"},
		Environment:  map[string]string{"ENV": "1"},
		APIBaseURL:   "https://api.example.com",
	})
	if err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}
	if startResult.Result == nil {
		t.Fatal("expected terminal result from blocking adapter")
	}
	if startResult.Handle.MethodName != "fake" {
		t.Fatalf("expected handle method name fake, got %q", startResult.Handle.MethodName)
	}
	if startResult.Handle.RunID != "run_123" {
		t.Fatalf("expected run id run_123, got %q", startResult.Handle.RunID)
	}
	if got := legacy.lastExecuteParams.RunID; got != "run_123" {
		t.Fatalf("expected Execute run id run_123, got %q", got)
	}
	if got := legacy.lastExecuteParams.WorkspaceID; got != "ws_123" {
		t.Fatalf("expected Execute workspace id ws_123, got %q", got)
	}
}

func TestBlockingAdapterDelegatesKillAndRejectsPoll(t *testing.T) {
	legacy := &fakeBlockingMethod{
		supportsKill:      true,
		supportsHeartbeat: true,
	}

	method := AdaptBlocking("fake", legacy)
	handle := Handle{MethodName: "fake", RunID: "run_456"}

	if err := method.Kill(context.Background(), handle); err != nil {
		t.Fatalf("unexpected kill error: %v", err)
	}
	if legacy.lastKillHandle.RunID != "run_456" {
		t.Fatalf("expected kill run id run_456, got %q", legacy.lastKillHandle.RunID)
	}
	if !method.SupportsKill() {
		t.Fatal("expected SupportsKill to be delegated")
	}
	if !method.SupportsHeartbeat() {
		t.Fatal("expected SupportsHeartbeat to be delegated")
	}

	if _, err := method.Poll(context.Background(), handle, PollCursor{}); err != ErrPollUnsupported {
		t.Fatalf("expected ErrPollUnsupported, got %v", err)
	}
}
