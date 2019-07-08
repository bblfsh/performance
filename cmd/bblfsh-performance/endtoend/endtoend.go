package endtoend

import (
	"context"
	"io/ioutil"
	"os"
	"os/signal"
	"testing"
	"time"

	"github.com/bblfsh/go-client/v4"
	"github.com/bblfsh/performance/util"
	"github.com/bblfsh/performance/util/docker"
	"github.com/bblfsh/performance/util/storage"
	"github.com/spf13/cobra"
	"golang.org/x/tools/benchmark/parse"
	"gopkg.in/src-d/go-errors.v1"
	"gopkg.in/src-d/go-log.v1"
)

const prefix = "bench_"

var (
	errGrpcClient      = errors.NewKind("cannot get grpc client")
	errGetFiles        = errors.NewKind("cannot get files")
	errBenchmark       = errors.NewKind("cannot perform benchmark over the file %s: %v")
	errNoFilesDetected = errors.NewKind("no files detected")
	errWarmUpFailed    = errors.NewKind("warmup for file %s has failed: %v")
)

// Cmd return configured end to end command
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "end-to-end [--driver=<language>] [--commit=<commit-id>] [--extension=<files-extension>] <directory ...>",
		Aliases: []string{"e2e"},
		Args:    cobra.MinimumNArgs(1),
		Short:   "run bblfshd container and perform benchmark tests, store results in influx db",
		Example: `To use external bblfshd set BBLFSHD_LOCAL=${bblfshd_address}
Default bblfshd tag is latest-drivers. To use custom bblfshd tag set BBLFSHD_TAG=${custom_tag}

WARNING! To access influx db corresponding environment variables should be set.
Full example of usage script is the following:

export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
bblfsh-performance end-to-end --driver=go --commit=3d9682b --extension=".go" /var/testdata/benchmarks`,
		RunE: util.RunESilenced(func(cmd *cobra.Command, args []string) error {
			driver, _ := cmd.Flags().GetString("driver")
			commit, _ := cmd.Flags().GetString("commit")
			extension, _ := cmd.Flags().GetString("extension")

			// for debug purposes with externally spinning container
			containerAddress := os.Getenv("BBLFSHD_LOCAL")
			if containerAddress == "" {
				log.Debugf("running bblfshd container\n")
				addr, closer, err := docker.Run()
				if err != nil {
					return err
				}
				defer closer()
				containerAddress = addr
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
			defer client.Close()
			if err != nil {
				return errGrpcClient.Wrap(err)
			}

			files, err := util.GetFiles(prefix, extension, args...)
			if err != nil {
				return errGetFiles.Wrap(err)
			}
			if len(files) == 0 {
				return errNoFilesDetected.New()
			}

			warmUpFile := files[0]
			log.Debugf("🔥 warming up the driver %s using file %s", driver, warmUpFile)
			warmUpTime, err := warmUp(ctx, client, driver, warmUpFile)
			if err != nil {
				return errWarmUpFailed.New(warmUpFile, err)
			}
			log.Debugf("warm up done for file %s in %v", warmUpFile, warmUpTime)

			var benchmarks []*parse.Benchmark
			for _, f := range files {
				log.Debugf("benching file: %s\n", f)
				bRes, err := benchFile(ctx, client, driver, f)
				if err != nil {
					return errBenchmark.New(f, err)
				}
				benchmarks = append(benchmarks, util.BenchmarkResultToParseBenchmark(f, bRes))
			}

			// store data
			storageClient, err := storage.NewClient("bblfshd:"+driver, commit)
			defer storageClient.Close()
			if err != nil {
				return err
			}

			if err := storageClient.Dump(benchmarks...); err != nil {
				return err
			}

			return nil
		}),
	}

	flags := cmd.Flags()
	flags.StringP("driver", "d", "", "name of the language current driver relates to")
	flags.StringP("commit", "c", "", "commit id that's being tested and will be used as a tag in performance report")
	flags.StringP("extension", "e", "", "file extension to be filtered")

	return cmd
}

func warmUp(ctx context.Context, c *bblfsh.Client, driver string, path string) (time.Duration, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	_, _, err = c.NewParseRequest().Context(ctx).Language(driver).Content(string(data)).UAST()

	return time.Since(start), err
}

func benchFile(ctx context.Context, c *bblfsh.Client, driver string, path string) (*testing.BenchmarkResult, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	res := testing.Benchmark(bench(func() {
		_, _, err := c.NewParseRequest().Context(ctx).Language(driver).Content(string(data)).UAST()
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
