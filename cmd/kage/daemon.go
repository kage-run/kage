package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/kage-run/kage/internal/daemon"
	"github.com/spf13/cobra"
)

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the Kage daemon",
	}

	cmd.AddCommand(
		newDaemonStartCmd(),
		newDaemonStopCmd(),
		newDaemonStatusCmd(),
	)

	return cmd
}

func newDaemonStartCmd() *cobra.Command {
	var detach bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Kage daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if detach {
				return startDaemonBackground()
			}
			return startDaemonForeground()
		},
	}

	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run daemon in background")
	return cmd
}

func newDaemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the Kage daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := daemon.NewClient(daemon.SocketPath())
			if err := client.Call(daemon.MethodDaemonStop, nil, nil); err != nil {
				return err
			}
			fmt.Println("Daemon stopped.")
			return nil
		},
	}
}

func newDaemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := daemon.NewClient(daemon.SocketPath())
			var result daemon.DaemonStatusResult
			if err := client.Call(daemon.MethodDaemonStatus, nil, &result); err != nil {
				fmt.Println("Daemon is not running.")
				return nil
			}
			fmt.Printf("Status:     running\n")
			fmt.Printf("Version:    %s\n", result.Version)
			fmt.Printf("Uptime:     %s\n", result.Uptime)
			fmt.Printf("Sandboxes:  %d\n", result.SandboxCount)
			return nil
		},
	}
}

func startDaemonForeground() error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	d := daemon.New(logger)
	return d.Run(context.Background(), daemon.SocketPath())
}

func startDaemonBackground() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".kage")
	os.MkdirAll(logDir, 0700)
	logFile, err := os.OpenFile(filepath.Join(logDir, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("opening daemon log: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(exe, "daemon", "start")
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting daemon: %w", err)
	}

	// Wait for socket to appear
	socketPath := daemon.SocketPath()
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		client := daemon.NewClient(socketPath)
		var result daemon.DaemonStatusResult
		if err := client.Call(daemon.MethodDaemonStatus, nil, &result); err == nil {
			fmt.Printf("Daemon started (PID %d).\n", cmd.Process.Pid)
			return nil
		}
	}

	return fmt.Errorf("daemon did not start within 3 seconds")
}

// ensureDaemon auto-starts the daemon if not running.
// Returns a connected client.
func ensureDaemon() (*daemon.Client, error) {
	socketPath := daemon.SocketPath()
	client := daemon.NewClient(socketPath)

	// Check if daemon is running
	var status daemon.DaemonStatusResult
	if err := client.Call(daemon.MethodDaemonStatus, nil, &status); err == nil {
		return client, nil
	}

	// Auto-start
	fmt.Fprintln(os.Stderr, "Starting daemon...")
	if err := startDaemonBackground(); err != nil {
		return nil, fmt.Errorf("auto-starting daemon: %w", err)
	}

	return client, nil
}
