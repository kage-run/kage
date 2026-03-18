package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kage-run/kage/internal/daemon"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var (
		name       string
		policyPath string
		workdir    string
		detach     bool
	)

	cmd := &cobra.Command{
		Use:   "run [flags] -- <command> [args...]",
		Short: "Run a command in a Kage sandbox",
		Long:  "Start a sandboxed process managed by the Kage daemon.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				name = "default"
			}

			if workdir == "" {
				var err error
				workdir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("getting working directory: %w", err)
				}
			}

			client, err := ensureDaemon()
			if err != nil {
				return err
			}

			params := daemon.StartParams{
				Name:       name,
				Command:    args,
				Workdir:    workdir,
				PolicyPath: policyPath,
			}

			var result daemon.StartResult
			if err := client.Call(daemon.MethodSandboxStart, params, &result); err != nil {
				return fmt.Errorf("starting sandbox: %w", err)
			}

			if detach {
				fmt.Printf("Started %s (PID %d)\n", result.Name, result.PID)
				return nil
			}

			// Foreground mode: stream output and handle signals
			return followAndWait(client, name)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name for this sandbox (default: \"default\")")
	cmd.Flags().StringVar(&policyPath, "policy", "", "Path to policy YAML file")
	cmd.Flags().StringVar(&workdir, "workdir", "", "Working directory for the command")
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run in background")

	return cmd
}

func followAndWait(client *daemon.Client, name string) error {
	// Set up signal handler to stop sandbox on Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	go func() {
		<-sigCh
		client.Call(daemon.MethodSandboxStop, daemon.StopParams{Name: name}, nil)
	}()

	// Stream output
	stream, err := client.Stream(daemon.MethodSandboxOutputFollow, daemon.OutputFollowParams{Name: name})
	if err != nil {
		return fmt.Errorf("following output: %w", err)
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var line string
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			// Not JSON, print raw
			fmt.Println(scanner.Text())
			continue
		}
		fmt.Println(line)
	}

	return nil
}
