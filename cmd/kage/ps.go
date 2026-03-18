package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/kage-run/kage/internal/daemon"
	"github.com/spf13/cobra"
)

func newPsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: "List running sandboxes",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := daemon.NewClient(daemon.SocketPath())
			var result daemon.ListResult
			if err := client.Call(daemon.MethodSandboxList, nil, &result); err != nil {
				return err
			}

			if len(result.Sandboxes) == 0 {
				fmt.Println("No sandboxes.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tPID\tSTATE\tUPTIME\tEXIT")
			for _, s := range result.Sandboxes {
				uptime := s.Uptime
				if uptime == "" {
					uptime = "-"
				}
				exitCode := "-"
				if s.State == "exited" || s.State == "stopped" || s.State == "failed" {
					exitCode = fmt.Sprintf("%d", s.ExitCode)
				}
				fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n", s.Name, s.PID, s.State, uptime, exitCode)
			}
			w.Flush()
			return nil
		},
	}
}
