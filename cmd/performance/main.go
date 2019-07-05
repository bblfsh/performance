package main

import (
	"fmt"
	"os"

	"github.com/bblfsh/performance/cmd/performance/endtoend"
	"github.com/bblfsh/performance/cmd/performance/parseandstore"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "performance",
		Short: "Performance test utilities for bblfshd and drivers",
	}

	rootCmd.AddCommand(parseandstore.Cmd(), endtoend.Cmd())
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
