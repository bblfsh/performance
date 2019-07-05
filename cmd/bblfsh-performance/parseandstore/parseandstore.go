package parseandstore

import (
	"fmt"
	"os"

	"github.com/bblfsh/performance"
	"github.com/bblfsh/performance/storage"
	"github.com/bblfsh/performance/storage/file"
	"github.com/bblfsh/performance/storage/influxdb"
	"github.com/bblfsh/performance/storage/prom-pushgateway"

	"github.com/spf13/cobra"
	"golang.org/x/tools/benchmark/parse"
)

// Cmd return configured parse-and-store command
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "parse-and-store [--language=<language>] [--commit=<commit-id>] [--storage=<storage>] <file ...>",
		Aliases: []string{"pas", "parse-and-dump"},
		Args:    cobra.MinimumNArgs(1),
		Short:   "parse file(s) with golang benchmark output and store it into a given storage",
		Example: `WARNING! To access storage corresponding environment variables should be set.
Full examples of usage scripts are following:

# for prometheus pushgateway
export PROM_ADDRESS="localhost:9091"
export PROM_JOB=pushgateway
bblfsh-performance parse-and-store --language=go --commit=3d9682b --storage="prom" /var/log/bench0 /var/log/bench1

# for influx db
export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
bblfsh-performance parse-and-store --language=go --commit=3d9682b --storage="influxdb" /var/log/bench0 /var/log/bench1`,
		RunE: performance.RunESilenced(func(cmd *cobra.Command, args []string) error {
			language, _ := cmd.Flags().GetString("language")
			commit, _ := cmd.Flags().GetString("commit")
			stor, _ := cmd.Flags().GetString("storage")

			c, err := storage.NewClient(stor)
			if err != nil {
				return err
			}
			defer c.Close()

			// TODO(lwsanty): parallelize
			for _, p := range args {
				benchmarks, err := getBenchmarks(p)
				if err != nil {
					return err
				}
				if err := c.Dump(map[string]string{
					"language": language,
					"commit":   commit,
					"level":    performance.TransformsLevel,
				}, benchmarks...); err != nil {
					return err
				}
			}

			return nil
		}),
	}

	// TODO(lwsanty): reuse this flags for several commands maybe?
	flags := cmd.Flags()
	flags.StringP("language", "l", "", "name of the language to be tested")
	flags.StringP("commit", "c", "", "commit id that's being tested and will be used as a tag in performance report")
	flags.StringP("storage", "s", prom_pushgateway.Kind, "storage kind to store the results"+
		fmt.Sprintf("(%s, %s, %s)", prom_pushgateway.Kind, influxdb.Kind, file.Kind))

	return cmd
}

func getBenchmarks(path string) ([]performance.Benchmark, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var result []performance.Benchmark
	set, err := parse.ParseSet(f)
	if err != nil {
		return nil, err
	}
	for _, s := range set {
		var benchmarkSet []performance.Benchmark
		for _, b := range s {
			benchmarkSet = append(benchmarkSet, performance.NewBenchmark(b))
		}

		result = append(result, benchmarkSet...)
	}
	return result, nil
}
