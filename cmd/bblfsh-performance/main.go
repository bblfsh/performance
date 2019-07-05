package main

import (
	"fmt"
	"os"

	"github.com/bblfsh/performance/cmd/bblfsh-performance/endtoend"
	"github.com/bblfsh/performance/cmd/bblfsh-performance/parseandstore"
	_ "github.com/bblfsh/performance/storage/influxdb"
	_ "github.com/bblfsh/performance/storage/prom-pushgateway"
	_ "github.com/bblfsh/performance/storage/std"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:     "bblfsh-performance",
		Aliases: []string{"bblfsh-perf"},
		Short:   "Performance test utilities for bblfshd and drivers",
	}

	rootCmd.AddCommand(parseandstore.Cmd(), endtoend.Cmd())
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
