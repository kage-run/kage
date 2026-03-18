package main

import (
	"fmt"

	"github.com/kage-run/kage/internal/daemon"
	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a running sandbox",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			client := daemon.NewClient(daemon.SocketPath())

			var result daemon.StopResult
			if err := client.Call(daemon.MethodSandboxStop, daemon.StopParams{Name: name}, &result); err != nil {
				return err
			}

			fmt.Printf("Stopped %s (%s)\n", result.Name, result.State)
			return nil
		},
	}
}
