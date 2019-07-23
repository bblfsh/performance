package endtoend

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"testing"
	"time"

	"github.com/bblfsh/performance"
	"github.com/bblfsh/performance/docker"
	"github.com/bblfsh/performance/storage"
	"github.com/bblfsh/performance/storage/file"
	"github.com/bblfsh/performance/storage/influxdb"
	"github.com/bblfsh/performance/storage/prom-pushgateway"

	"github.com/bblfsh/go-client/v4"
	"github.com/spf13/cobra"
	"gopkg.in/src-d/go-errors.v1"
	"gopkg.in/src-d/go-log.v1"
)

const (
	// fileFilterPrefix is a fileFilterPrefix of file that would be filtered from the list of files in a directory.
	// Currently we use benchmark fixtures, file name pattern in this case is bench_*.${extension}
	fileFilterPrefix = "bench_"

	bblfshDefaultConfTag = "latest-drivers"
)

var (
	errGRPCClient                = errors.NewKind("cannot get grpc client")
	errGetFiles                  = errors.NewKind("cannot get files")
	errBenchmark                 = errors.NewKind("cannot perform benchmark over the file %v: %v")
	errNoFilesDetected           = errors.NewKind("no files detected")
	errWarmUpFailed              = errors.NewKind("warmup for file %v has failed: %v")
	errCannotInstallCustomDriver = errors.NewKind("custom driver cannot be installed: %v")
)

// TODO(lwsanty): https://github.com/bblfsh/performance/issues/2
// Cmd return configured end to end command
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "end-to-end [--language=<language>] [--commit=<commit-id>] [--extension=<files-extension>] [--docker-tag=<docker-tag>] [--storage=<storage>] <directory ...>",
		Aliases: []string{"e2e"},
		Args:    cobra.MinimumNArgs(1),
		Short:   "run bblfshd container and perform benchmark tests, store results into a given storage",
		Example: `To use external bblfshd set BBLFSHD_LOCAL=${bblfshd_address}

WARNING! To access storage corresponding environment variables should be set.
Full examples of usage scripts are following:

# for prometheus pushgateway
export PROM_ADDRESS="localhost:9091"
export PROM_JOB=pushgateway
./bblfsh-performance end-to-end --language=go --commit=3d9682b --filter-prefix="bench_" --exclude-substrings=".legacy",".native",".uast" --storage="prom" /var/testdata/benchmarks

# for influx db
export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
bblfsh-performance end-to-end --language=go --commit=3d9682b --filter-prefix="bench_" --exclude-substrings=".legacy",".native",".uast" --storage="influxdb" /var/testdata/benchmarks`,
		RunE: performance.RunESilenced(func(cmd *cobra.Command, args []string) error {
			language, _ := cmd.Flags().GetString("language")
			commit, _ := cmd.Flags().GetString("commit")
			excludeSubstrings, _ := cmd.Flags().GetStringSlice("exclude-substrings")
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
					return errCannotInstallCustomDriver.New("bblfshd tag is set to " + bblfshDefaultConfTag + ": all drivers are pre-installed")
				}

				addr, closer, err := docker.RunBblfshd(tag)
				if err != nil {
					return err
				}
				defer closer()
				containerAddress = addr

				if customDriver {
					if err := docker.InstallDriver(language, commit); err != nil {
						return errCannotInstallCustomDriver.New(err)
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

			client, err := bblfsh.NewClientContext(ctx, containerAddress)
			if err != nil {
				return errGRPCClient.Wrap(err)
			}
			defer client.Close()

			files, err := performance.GetFiles(filterPrefix, excludeSubstrings, args...)
			if err != nil {
				return errGetFiles.Wrap(err)
			} else if len(files) == 0 {
				return errNoFilesDetected.New()
			}

			warmUpFile := files[0]
			log.Debugf("🔥 warming up the language %s using file %s", language, warmUpFile)
			warmUpTime, err := warmUp(ctx, client, language, warmUpFile)
			if err != nil {
				return errWarmUpFailed.New(warmUpFile, err)
			}
			log.Debugf("warm up done for file %s in %v", warmUpFile, warmUpTime)

			var benchmarks []performance.Benchmark
			for _, f := range files {
				log.Debugf("benching file: %s", f)
				bRes, err := benchFile(ctx, client, language, f)
				if err != nil {
					return errBenchmark.New(f, err)
				}
				benchmarks = append(benchmarks, performance.BenchmarkResultToBenchmark(f, bRes))
			}

			// store data
			storageClient, err := storage.NewClient(stor)
			if err != nil {
				return err
			}
			defer storageClient.Close()

			if err := storageClient.Dump(map[string]string{
				"language": language,
				"commit":   commit,
				"level":    performance.BblfshdLevel,
			}, benchmarks...); err != nil {
				return err
			}

			return nil
		}),
	}

	flags := cmd.Flags()
	flags.StringP("language", "l", "", "name of the language to be tested")
	flags.StringP("commit", "c", "", "commit id that's being tested and will be used as a tag in performance report")
	flags.StringSlice("exclude-substrings", []string{".legacy", ".native", ".uast"}, "file name substrings to be excluded")
	flags.StringP("docker-tag", "t", bblfshDefaultConfTag, "bblfshd docker image tag to be tested")
	flags.String("filter-prefix", fileFilterPrefix, "file prefix to be filtered")
	flags.StringP("storage", "s", prom_pushgateway.Kind, "storage kind to store the results"+
		fmt.Sprintf("(%s, %s, %s)", prom_pushgateway.Kind, influxdb.Kind, file.Kind))
	flags.Bool("custom-driver", false, "if this flag is set to true CLI pulls corresponding language driver repo's commit, builds docker image and installs it onto the bblfsh container")

	return cmd
}

func warmUp(ctx context.Context, c *bblfsh.Client, language string, path string) (time.Duration, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	_, _, err = c.NewParseRequest().Context(ctx).Language(language).Content(string(data)).UAST()

	return time.Since(start), err
}

func benchFile(ctx context.Context, c *bblfsh.Client, language string, path string) (*testing.BenchmarkResult, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	res := testing.Benchmark(bench(func() {
		_, _, err := c.NewParseRequest().Context(ctx).Language(language).Content(string(data)).UAST()
		if err != nil {
			panic(err)
		}
	}))
	return &res, nil
}

func bench(f func()) func(b *testing.B) {
	return func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			f()
		}
	}
}
