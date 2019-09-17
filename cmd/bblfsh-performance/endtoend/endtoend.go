package endtoend

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
	prom_pushgateway "github.com/bblfsh/performance/storage/prom-pushgateway"

	"github.com/spf13/cobra"
	"gopkg.in/src-d/go-log.v1"
)

const (
	fileFilterPrefix = performance.FileFilterPrefix

	bblfshDefaultConfTag = "latest-drivers"
)

// TODO(lwsanty): https://github.com/bblfsh/performance/issues/2
// Cmd return configured end to end command
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "end-to-end [--language=<language>] [--commit=<commit-id>] [--docker-tag=<docker-tag>] [--storage=<storage>] <directory ...>",
		Aliases: []string{"e2e"},
		Args:    cobra.MinimumNArgs(1),
		Short:   "run bblfshd container and perform benchmark tests, store results into a given storage",
		Example: `To use external bblfshd set BBLFSHD_LOCAL=${bblfshd_address}

WARNING! To access storage corresponding environment variables should be set.
Full examples of usage scripts are following:

# for prometheus pushgateway
export PROM_ADDRESS="localhost:9091"
export PROM_JOB=pushgateway
./bblfsh-performance end-to-end \
--language=go \
--commit=096361d09049c27e829fd5a6658f1914fd3b62ac \
--filter-prefix="bench_" \
--storage="prom" \
/var/testdata/benchmarks

# for influx db
export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
./bblfsh-performance end-to-end \
--language=go \
--commit=096361d09049c27e829fd5a6658f1914fd3b62ac \
--filter-prefix="bench_" \
--storage="influxdb" \
/var/testdata/benchmarks`,
		RunE: performance.RunESilenced(func(cmd *cobra.Command, args []string) error {
			language, _ := cmd.Flags().GetString("language")
			commit, _ := cmd.Flags().GetString("commit")
			excludeSubstrings, _ := cmd.Flags().GetStringSlice("exclude-suffixes")
			stor, _ := cmd.Flags().GetString("storage")
			filterPrefix, _ := cmd.Flags().GetString("filter-prefix")
			customDriver, _ := cmd.Flags().GetBool("custom-driver")

			if _, err := storage.ValidateKind(stor); err != nil {
				return err
			}

			// for debug purposes with externally spinning container
			containerAddress := os.Getenv("BBLFSHD_LOCAL")
			if containerAddress == "" {
				tag, _ := cmd.Flags().GetString("docker-tag")
				log.Debugf("running bblfshd %s container", tag)
				if tag == bblfshDefaultConfTag && customDriver {
					return performance.ErrCannotInstallCustomDriver.New("bblfshd tag is set to " + bblfshDefaultConfTag + ": all drivers are pre-installed")
				}

				addr, closer, err := docker.RunBblfshd(tag)
				if err != nil {
					return err
				}
				defer closer()
				containerAddress = addr

				if customDriver {
					if err := docker.InstallDriver(language, commit); err != nil {
						return performance.ErrCannotInstallCustomDriver.New(err)
					}
				}
			}

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
				Address:           containerAddress,
				Commit:            commit,
				ExcludeSubstrings: excludeSubstrings,
				Dirs:              args,
				FilterPrefix:      filterPrefix,
				Language:          language,
				Level:             performance.BblfshdLevel,
				Storage:           stor,
			})
		}),
	}

	flags := cmd.Flags()
	flags.StringP("language", "l", "", "name of the language to be tested")
	flags.StringP("commit", "c", "", "commit id that's being tested and will be used as a tag in performance report")
	flags.StringSlice("exclude-suffixes", []string{".legacy", ".native", ".uast"}, "file suffixes to be excluded")
	flags.StringP("docker-tag", "t", bblfshDefaultConfTag, "bblfshd docker image tag to be tested")
	flags.String("filter-prefix", fileFilterPrefix, "file prefix to be filtered")
	flags.StringP("storage", "s", prom_pushgateway.Kind, "storage kind to store the results"+
		fmt.Sprintf("(%s, %s, %s)", prom_pushgateway.Kind, influxdb.Kind, file.Kind))
	flags.Bool("custom-driver", false, "if this flag is set to true CLI pulls corresponding language driver repo's commit, builds docker image and installs it onto the bblfsh container")

	return cmd
}
