package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:   "kage",
		Short: "Secure, observe, and remotely control AI agents",
		Long:  "Secure, observe, and control your AI agents from anywhere.",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	rootCmd.AddCommand(
		newDaemonCmd(),
		newRunCmd(),
		newPsCmd(),
		newStopCmd(),
		newOutputCmd(),
	)

	rootCmd.Version = version

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
