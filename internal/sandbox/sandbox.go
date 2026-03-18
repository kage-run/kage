package sandbox

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/kage-run/kage/internal/policy"
)

const defaultBufferLines = 500

// Config holds the configuration for creating a new Sandbox.
type Config struct {
	Name        string
	Command     []string
	Policy      *policy.Policy
	Workdir     string
	Env         []string
	BufferLines int
	TeeStdout   io.Writer
	TeeStderr   io.Writer
}

// Sandbox manages a single supervised agent process.
type Sandbox struct {
	id     string
	config Config
	sup    *supervisor
	output *RingBuffer
	mu     sync.Mutex
	logger *slog.Logger
}

// NewSandbox creates a new sandbox with the given configuration.
func NewSandbox(cfg Config, logger *slog.Logger) *Sandbox {
	bufLines := cfg.BufferLines
	if bufLines <= 0 {
		bufLines = defaultBufferLines
	}

	return &Sandbox{
		id:     cfg.Name,
		config: cfg,
		output: NewRingBuffer(bufLines),
		logger: logger.With("agent", cfg.Name),
	}
}

// Start launches the sandboxed process.
func (s *Sandbox) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sup != nil {
		st := s.sup.getStatus()
		if st.State == StateRunning || st.State == StateStopping {
			return fmt.Errorf("sandbox %q is already running", s.id)
		}
	}

	s.sup = newSupervisor(
		s.config.Command,
		s.config.Workdir,
		s.config.Env,
		s.output,
		s.config.TeeStdout,
		s.config.TeeStderr,
		s.logger,
	)

	if err := s.sup.start(); err != nil {
		return fmt.Errorf("starting sandbox %q: %w", s.id, err)
	}

	return nil
}

// Stop gracefully stops the sandboxed process.
func (s *Sandbox) Stop() error {
	s.mu.Lock()
	sup := s.sup
	s.mu.Unlock()

	if sup == nil {
		return nil
	}
	return sup.stop()
}

// Status returns the current process status.
func (s *Sandbox) Status() ProcessStatus {
	s.mu.Lock()
	sup := s.sup
	s.mu.Unlock()

	if sup == nil {
		return ProcessStatus{State: StateIdle}
	}
	return sup.getStatus()
}

// Output returns the last n lines from the process output.
func (s *Sandbox) Output(n int) []string {
	return s.output.Lines(n)
}

// Follow returns a channel that streams new output lines.
func (s *Sandbox) Follow(ctx context.Context) <-chan string {
	return s.output.Follow(ctx)
}

// Wait returns a channel that closes when the process exits.
func (s *Sandbox) Wait() <-chan struct{} {
	s.mu.Lock()
	sup := s.sup
	s.mu.Unlock()

	if sup == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return sup.done()
}

// ID returns the sandbox's agent ID.
func (s *Sandbox) ID() string {
	return s.id
}
