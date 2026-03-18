package main

import (
	"bufio"
	"encoding/json"
	"fmt"

	"github.com/kage-run/kage/internal/daemon"
	"github.com/spf13/cobra"
)

func newOutputCmd() *cobra.Command {
	var (
		lines  int
		follow bool
	)

	cmd := &cobra.Command{
		Use:   "output <name>",
		Short: "Show output from a sandbox",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			client := daemon.NewClient(daemon.SocketPath())

			if follow {
				return streamOutput(client, name)
			}

			var result daemon.OutputResult
			params := daemon.OutputParams{Name: name, Lines: lines}
			if err := client.Call(daemon.MethodSandboxOutput, params, &result); err != nil {
				return err
			}

			for _, line := range result.Lines {
				fmt.Println(line)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&lines, "lines", "n", 20, "Number of lines to show")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Stream output in real-time")

	return cmd
}

func streamOutput(client *daemon.Client, name string) error {
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
			fmt.Println(scanner.Text())
			continue
		}
		fmt.Println(line)
	}

	return scanner.Err()
}
