package parseandstore

import (
	"os"

	"github.com/bblfsh/performance/util"
	"github.com/bblfsh/performance/util/storage"
	"github.com/spf13/cobra"
	"golang.org/x/tools/benchmark/parse"
)

// Cmd return configured parse-and-store command
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "parse-and-store [--driver=<driver-name>] [--commit=<commit-id>] <file ...>",
		Aliases: []string{"parse", "p", "dump", "store"},
		Args:    cobra.MinimumNArgs(1),
		Short:   "parse file(s) with golang benchmark output and store it in influx db",
		Example: `WARNING! To access influx db corresponding environment variables should be set.
Full example of usage script is the following:

export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
performance parse-and-store --driver=go --commit=3d9682b6a3c51db91896ad516bc521cce49ffe10 /var/log/bench0 /var/log/bench1`,
		RunE: util.RunESilenced(func(cmd *cobra.Command, args []string) error {
			driver, _ := cmd.Flags().GetString("driver")
			commit, _ := cmd.Flags().GetString("commit")
			c, err := storage.NewClient(driver, commit)
			defer func() { _ = c.Close() }()
			if err != nil {
				return err
			}

			// TODO parallelize
			for _, p := range args {
				benchmarks, err := getBenchmarks(p)
				if err != nil {
					return err
				}
				if err := c.Dump(benchmarks...); err != nil {
					return err
				}
			}

			return nil
		}),
	}

	// TODO reuse this flags for several commands maybe?
	flags := cmd.Flags()
	flags.StringP("driver", "d", "not-specified", "name of the language current driver relates to")
	flags.StringP("commit", "c", "not-specified", "commit id that's being tested")

	return cmd
}

func getBenchmarks(path string) ([]*parse.Benchmark, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var result []*parse.Benchmark
	set, err := parse.ParseSet(f)
	if err != nil {
		return nil, err
	}
	for _, s := range set {
		result = append(result, s...)
	}
	return result, nil
}
