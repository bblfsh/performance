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
		Use:     "parse-and-store [--driver=<language>] [--commit=<commit-id>] <file ...>",
		Aliases: []string{"pas", "parse-and-dump"},
		Args:    cobra.MinimumNArgs(1),
		Short:   "parse file(s) with golang benchmark output and store it in influx db",
		Example: `WARNING! To access influx db corresponding environment variables should be set.
Full example of usage script is the following:

export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
bblfsh-performance parse-and-store --driver=go --commit=3d9682b /var/log/bench0 /var/log/bench1`,
		RunE: util.RunESilenced(func(cmd *cobra.Command, args []string) error {
			driver, _ := cmd.Flags().GetString("driver")
			commit, _ := cmd.Flags().GetString("commit")
			c, err := storage.NewClient(driver, commit)
			defer c.Close()
			if err != nil {
				return err
			}

			// TODO(lwsanty): parallelize
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

	// TODO(lwsanty): reuse this flags for several commands maybe?
	flags := cmd.Flags()
	flags.StringP("driver", "d", "", "name of the language current driver relates to")
	flags.StringP("commit", "c", "", "commit id that's being tested and will be used as a tag in performance report")

	return cmd
}

func getBenchmarks(path string) ([]*parse.Benchmark, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

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
