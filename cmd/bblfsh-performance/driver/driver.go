package driver

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/bblfsh/performance"
	"github.com/bblfsh/performance/docker"
	helper "github.com/bblfsh/performance/grpc-helper"
	"github.com/bblfsh/performance/storage"
	"github.com/bblfsh/performance/storage/file"
	"github.com/bblfsh/performance/storage/influxdb"
	"github.com/bblfsh/performance/storage/pushgateway"

	"github.com/spf13/cobra"
	"gopkg.in/src-d/go-log.v1"
)

// Cmd return configured driver-native command
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "driver [--language=<language>] [--commit=<commit-id>] [--storage=<storage>] [--filter-prefix=<filter-prefix>] <directory>",
		Aliases: []string{"d"},
		Args:    cobra.MinimumNArgs(1),
		Short:   "run language driver container and perform benchmark tests over the driver, store results into a given storage",
		Example: `WARNING! To access storage corresponding environment variables should be set.
Full examples of usage scripts are following:

# for prometheus pushgateway
export PROM_ADDRESS="localhost:9091"
export PROM_JOB=pushgateway
./bblfsh-performance driver \
--language go \
--commit 096361d09049c27e829fd5a6658f1914fd3b62ac \
/var/testdata/fixtures

# for influx db
export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
./bblfsh-performance driver \
--language go \
--commit 096361d09049c27e829fd5a6658f1914fd3b62ac \
--storage=influxdb \
/var/testdata/fixtures
`,
		RunE: performance.RunESilenced(func(cmd *cobra.Command, args []string) error {
			language, _ := cmd.Flags().GetString("language")
			commit, _ := cmd.Flags().GetString("commit")
			excludeSubstrings, _ := cmd.Flags().GetStringSlice("exclude-suffixes")
			stor, _ := cmd.Flags().GetString("storage")
			filterPrefix, _ := cmd.Flags().GetString("filter-prefix")

			if _, err := storage.ValidateKind(stor); err != nil {
				return err
			}

			log.Debugf("download and build driver")
			image, err := docker.DownloadAndBuildDriver(language, commit)
			if err != nil {
				return err
			}

			log.Debugf("run driver container")
			driver, err := docker.RunDriver(image)
			if err != nil {
				return err
			}
			defer driver.Close()

			// prepare context
			ctx, cancel := context.WithCancel(context.Background())
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			defer func() {
				signal.Stop(c)
				cancel()
			}()
			go func() {
				select {
				case <-c:
					cancel()
				case <-ctx.Done():
				}
			}()

			return helper.BenchmarkGRPCAndStore(ctx, helper.BenchmarkGRPCMeta{
				Address:           driver.Address,
				Commit:            commit,
				ExcludeSubstrings: excludeSubstrings,
				Dirs:              args,
				FilterPrefix:      filterPrefix,
				Language:          language,
				Level:             performance.DriverLevel,
				Storage:           stor,
			})
		}),
	}

	flags := cmd.Flags()
	flags.StringP("language", "l", "", "name of the language to be tested")
	flags.StringP("commit", "c", "", "commit id that's being tested and will be used as a tag in performance report")
	flags.StringSlice("exclude-suffixes", []string{".legacy", ".native", ".uast"}, "file suffixes to be excluded")
	flags.String("filter-prefix", performance.FileFilterPrefix, "file prefix to be filtered")
	flags.StringP("storage", "s", pushgateway.Kind, fmt.Sprintf("storage kind to store the results(%s, %s, %s)", pushgateway.Kind, influxdb.Kind, file.Kind))

	return cmd
}
