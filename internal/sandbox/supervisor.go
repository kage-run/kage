package sandbox

import (
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// SupervisorState represents the lifecycle state of a supervised process.
type SupervisorState int

const (
	StateIdle     SupervisorState = iota
	StateRunning
	StateStopping
	StateStopped
	StateExited // process exited on its own
	StateFailed // process failed to start
)

func (s SupervisorState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateExited:
		return "exited"
	case StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// ProcessStatus holds the current status of a supervised process.
type ProcessStatus struct {
	State     SupervisorState `json:"state"`
	PID       int             `json:"pid"`
	ExitCode  int             `json:"exit_code"`
	StartedAt time.Time       `json:"started_at"`
	StoppedAt time.Time       `json:"stopped_at,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// supervisor manages the lifecycle of a single child process.
type supervisor struct {
	command   []string
	workdir   string
	env       []string
	output    *RingBuffer
	teeStdout io.Writer
	teeStderr io.Writer
	logger    *slog.Logger

	status atomic.Value // holds ProcessStatus
	doneCh chan struct{}
	mu     sync.Mutex
	cmd    *exec.Cmd
}

func newSupervisor(command []string, workdir string, env []string, output *RingBuffer, teeStdout, teeStderr io.Writer, logger *slog.Logger) *supervisor {
	s := &supervisor{
		command:   command,
		workdir:   workdir,
		env:       env,
		output:    output,
		teeStdout: teeStdout,
		teeStderr: teeStderr,
		logger:    logger,
		doneCh:    make(chan struct{}),
	}
	s.status.Store(ProcessStatus{State: StateIdle})
	return s
}

func (s *supervisor) start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.command) == 0 {
		return fmt.Errorf("empty command")
	}

	cmd := exec.Command(s.command[0], s.command[1:]...)
	cmd.Dir = s.workdir
	if len(s.env) > 0 {
		cmd.Env = append(cmd.Environ(), s.env...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Set up output capture
	var stdoutWriters []io.Writer
	var stderrWriters []io.Writer

	stdoutWriters = append(stdoutWriters, s.output)
	stderrWriters = append(stderrWriters, s.output)

	if s.teeStdout != nil {
		stdoutWriters = append(stdoutWriters, s.teeStdout)
	}
	if s.teeStderr != nil {
		stderrWriters = append(stderrWriters, s.teeStderr)
	}

	cmd.Stdout = io.MultiWriter(stdoutWriters...)
	cmd.Stderr = io.MultiWriter(stderrWriters...)

	if err := cmd.Start(); err != nil {
		s.status.Store(ProcessStatus{
			State: StateFailed,
			Error: err.Error(),
		})
		close(s.doneCh)
		return fmt.Errorf("starting process: %w", err)
	}

	s.cmd = cmd
	now := time.Now()
	s.status.Store(ProcessStatus{
		State:     StateRunning,
		PID:       cmd.Process.Pid,
		StartedAt: now,
	})

	s.logger.Info("process started", "pid", cmd.Process.Pid, "command", s.command)

	// Wait goroutine
	go func() {
		defer close(s.doneCh)
		err := cmd.Wait()

		st := s.getStatus()
		st.StoppedAt = time.Now()

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				st.ExitCode = exitErr.ExitCode()
			} else {
				st.Error = err.Error()
			}
		}

		if st.State == StateStopping {
			st.State = StateStopped
		} else {
			st.State = StateExited
		}

		s.status.Store(st)
		s.logger.Info("process exited", "pid", st.PID, "exit_code", st.ExitCode, "state", st.State.String())
	}()

	return nil
}

func (s *supervisor) stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.getStatus()
	if st.State != StateRunning {
		return nil
	}

	st.State = StateStopping
	s.status.Store(st)

	pid := s.cmd.Process.Pid
	s.logger.Info("sending SIGTERM", "pid", pid)

	// Send SIGTERM to process group
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		s.logger.Warn("SIGTERM failed", "pid", pid, "error", err)
	}

	// Wait up to 10 seconds for graceful exit
	select {
	case <-s.doneCh:
		return nil
	case <-time.After(10 * time.Second):
	}

	// Force kill
	s.logger.Warn("sending SIGKILL", "pid", pid)
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		return fmt.Errorf("sending SIGKILL to pid %d: %w", pid, err)
	}

	<-s.doneCh
	return nil
}

func (s *supervisor) getStatus() ProcessStatus {
	return s.status.Load().(ProcessStatus)
}

func (s *supervisor) done() <-chan struct{} {
	return s.doneCh
}
