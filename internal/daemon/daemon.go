package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/kage-run/kage/internal/policy"
	"github.com/kage-run/kage/internal/sandbox"
)

// Version is set at build time.
var Version = "dev"

// Daemon is the long-lived process that owns all sandboxes and serves the socket API.
type Daemon struct {
	mu        sync.RWMutex
	sandboxes map[string]*sandbox.Sandbox
	server    *Server
	logger    *slog.Logger
	startedAt time.Time
	ctx       context.Context
	cancel    context.CancelFunc
}

// New creates a new Daemon instance.
func New(logger *slog.Logger) *Daemon {
	return &Daemon{
		sandboxes: make(map[string]*sandbox.Sandbox),
		logger:    logger,
	}
}

// SocketPath returns the default socket path based on the current user.
func SocketPath() string {
	if os.Getuid() == 0 {
		return "/var/run/kage/kage.sock"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kage", "kage.sock")
}

// Run starts the daemon, binds the socket, and blocks until shutdown.
func (d *Daemon) Run(ctx context.Context, socketPath string) error {
	d.ctx, d.cancel = context.WithCancel(ctx)
	d.startedAt = time.Now()

	// Ensure directory exists
	dir := filepath.Dir(socketPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating socket directory %s: %w", dir, err)
	}

	// Check for stale socket
	if err := cleanStaleSocket(socketPath); err != nil {
		return fmt.Errorf("cleaning stale socket: %w", err)
	}

	// Start server
	srv, err := NewServer(socketPath, d, d.logger)
	if err != nil {
		return fmt.Errorf("creating server: %w", err)
	}
	d.server = srv

	d.logger.Info("daemon started", "socket", socketPath, "pid", os.Getpid())

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(d.ctx)
	}()

	select {
	case <-d.ctx.Done():
	case sig := <-sigCh:
		d.logger.Info("received signal", "signal", sig)
	case err := <-errCh:
		if err != nil && d.ctx.Err() == nil {
			return fmt.Errorf("server error: %w", err)
		}
	}

	return d.Shutdown()
}

// Shutdown stops all sandboxes and closes the server.
func (d *Daemon) Shutdown() error {
	d.logger.Info("shutting down daemon")
	d.cancel()

	// Stop all sandboxes in parallel
	d.mu.RLock()
	var wg sync.WaitGroup
	for name, sb := range d.sandboxes {
		wg.Add(1)
		go func(name string, sb *sandbox.Sandbox) {
			defer wg.Done()
			if err := sb.Stop(); err != nil {
				d.logger.Warn("error stopping sandbox", "name", name, "error", err)
			}
		}(name, sb)
	}
	d.mu.RUnlock()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		d.logger.Warn("timed out waiting for sandboxes to stop")
	}

	if d.server != nil {
		return d.server.Close()
	}
	return nil
}

// StartSandbox creates and starts a new sandbox.
func (d *Daemon) StartSandbox(name string, command []string, workdir, policyPath string, env []string) (*sandbox.ProcessStatus, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check for existing sandbox
	if existing, ok := d.sandboxes[name]; ok {
		st := existing.Status()
		switch st.State {
		case sandbox.StateRunning, sandbox.StateStopping:
			return nil, fmt.Errorf("sandbox %q is already running", name)
		default:
			// Replace exited/stopped/failed sandbox
			d.logger.Info("replacing sandbox", "name", name, "old_state", st.State.String())
		}
	}

	// Load policy
	var pol *policy.Policy
	if policyPath != "" {
		var err error
		pol, err = policy.LoadFromFile(policyPath)
		if err != nil {
			return nil, fmt.Errorf("loading policy: %w", err)
		}
	} else {
		pol = policy.DefaultPolicy()
	}

	sb := sandbox.NewSandbox(sandbox.Config{
		Name:    name,
		Command: command,
		Policy:  pol,
		Workdir: workdir,
		Env:     env,
	}, d.logger)

	if err := sb.Start(); err != nil {
		return nil, err
	}

	d.sandboxes[name] = sb

	st := sb.Status()
	return &st, nil
}

// StopSandbox stops a sandbox by name.
func (d *Daemon) StopSandbox(name string) error {
	d.mu.RLock()
	sb, ok := d.sandboxes[name]
	d.mu.RUnlock()

	if !ok {
		return fmt.Errorf("sandbox %q not found", name)
	}
	return sb.Stop()
}

// GetSandbox returns a sandbox by name.
func (d *Daemon) GetSandbox(name string) (*sandbox.Sandbox, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	sb, ok := d.sandboxes[name]
	if !ok {
		return nil, fmt.Errorf("sandbox %q not found", name)
	}
	return sb, nil
}

// ListSandboxes returns info about all sandboxes.
func (d *Daemon) ListSandboxes() []SandboxInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	infos := make([]SandboxInfo, 0, len(d.sandboxes))
	for name, sb := range d.sandboxes {
		st := sb.Status()
		info := SandboxInfo{
			Name:     name,
			PID:      st.PID,
			State:    st.State.String(),
			ExitCode: st.ExitCode,
		}
		if !st.StartedAt.IsZero() {
			info.StartedAt = st.StartedAt.Format(time.RFC3339)
			if st.State == sandbox.StateRunning {
				info.Uptime = time.Since(st.StartedAt).Truncate(time.Second).String()
			}
		}
		infos = append(infos, info)
	}
	return infos
}

// DaemonStatus returns the daemon's own status.
func (d *Daemon) DaemonStatus() DaemonStatusResult {
	d.mu.RLock()
	count := len(d.sandboxes)
	d.mu.RUnlock()

	return DaemonStatusResult{
		Version:      Version,
		Uptime:       time.Since(d.startedAt).Truncate(time.Second).String(),
		SandboxCount: count,
	}
}

// cleanStaleSocket removes a socket file if it exists but no daemon is listening.
func cleanStaleSocket(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	// Try to connect
	conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err == nil {
		conn.Close()
		return fmt.Errorf("daemon already running (socket %s is active)", path)
	}

	// Stale socket — remove it
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing stale socket %s: %w", path, err)
	}
	return nil
}
