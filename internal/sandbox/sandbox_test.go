package sandbox

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/kage-run/kage/internal/policy"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestSandbox_EchoOutput(t *testing.T) {
	s := NewSandbox(Config{
		Name:    "test-echo",
		Command: []string{"echo", "hello world"},
		Policy:  policy.DefaultPolicy(),
	}, testLogger())

	if err := s.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	select {
	case <-s.Wait():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for process")
	}

	st := s.Status()
	if st.State != StateExited {
		t.Fatalf("expected StateExited, got %s", st.State)
	}
	if st.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", st.ExitCode)
	}

	lines := s.Output(10)
	if len(lines) != 1 || lines[0] != "hello world" {
		t.Fatalf("unexpected output: %v", lines)
	}
}

func TestSandbox_StopRunningProcess(t *testing.T) {
	s := NewSandbox(Config{
		Name:    "test-sleep",
		Command: []string{"sleep", "60"},
		Policy:  policy.DefaultPolicy(),
	}, testLogger())

	if err := s.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	st := s.Status()
	if st.State != StateRunning {
		t.Fatalf("expected StateRunning, got %s", st.State)
	}
	if st.PID == 0 {
		t.Fatal("expected non-zero PID")
	}

	if err := s.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	st = s.Status()
	if st.State != StateStopped {
		t.Fatalf("expected StateStopped, got %s", st.State)
	}
}

func TestSandbox_BadCommand(t *testing.T) {
	s := NewSandbox(Config{
		Name:    "test-bad",
		Command: []string{"/nonexistent/binary"},
		Policy:  policy.DefaultPolicy(),
	}, testLogger())

	err := s.Start()
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestSandbox_DoubleStart(t *testing.T) {
	s := NewSandbox(Config{
		Name:    "test-double",
		Command: []string{"sleep", "60"},
		Policy:  policy.DefaultPolicy(),
	}, testLogger())

	if err := s.Start(); err != nil {
		t.Fatalf("first start failed: %v", err)
	}
	defer s.Stop()

	err := s.Start()
	if err == nil {
		t.Fatal("expected error on double start")
	}
}

func TestSandbox_StopIdleIsNoop(t *testing.T) {
	s := NewSandbox(Config{
		Name:    "test-idle",
		Command: []string{"echo", "hi"},
		Policy:  policy.DefaultPolicy(),
	}, testLogger())

	// Stop before start should be a no-op
	if err := s.Stop(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSandbox_ID(t *testing.T) {
	s := NewSandbox(Config{
		Name:    "my-agent",
		Command: []string{"echo"},
		Policy:  policy.DefaultPolicy(),
	}, testLogger())

	if s.ID() != "my-agent" {
		t.Fatalf("expected ID 'my-agent', got %q", s.ID())
	}
}
