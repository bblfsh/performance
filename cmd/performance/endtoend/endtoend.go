package endtoend

import (
	"context"
	"io/ioutil"
	"os"
	"os/signal"
	"testing"

	"github.com/bblfsh/go-client/v4"
	"github.com/bblfsh/performance/util"
	"github.com/bblfsh/performance/util/docker"
	"github.com/bblfsh/performance/util/storage"
	"github.com/spf13/cobra"
	"golang.org/x/tools/benchmark/parse"
	"gopkg.in/src-d/go-errors.v1"
	"gopkg.in/src-d/go-log.v1"
)

// TODO blacklist https://github.com/bblfsh/go-driver/blob/57cc13f9a962dc47829c381a61d4372d37b38358/driver/fixtures/fixtures_test.go#L24

const prefix = "bench_"

var (
	errGrpcClient = errors.NewKind("cannot get grpc client")
	errGetFiles   = errors.NewKind("cannot get files")
	errBenchmark  = errors.NewKind("cannot perform benchmark over the file %s: %v")
)

// Cmd return configured end to end command
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "end-to-end [--driver=<driver-name>] [--commit=<commit-id>] [--extension=<files-extension>] <directory ...>",
		Aliases: []string{"e2e"},
		Args:    cobra.MinimumNArgs(1),
		Short:   "run bblfshd container and perform benchmark tests, store results in influx db",
		Example: `WARNING! To access influx db corresponding environment variables should be set.
Full example of usage script is the following:

export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
performance end-to-end --driver=go --commit=3d9682b6a3c51db91896ad516bc521cce49ffe10 --extension=".go" /var/testdata/benchmarks`,
		RunE: util.RunESilenced(func(cmd *cobra.Command, args []string) error {
			driver, _ := cmd.Flags().GetString("driver")
			commit, _ := cmd.Flags().GetString("commit")
			extension, _ := cmd.Flags().GetString("extension")

			// for debug purposes with externally spinning container
			containerAddress := os.Getenv("BBLFSHD_LOCAL")
			if containerAddress == "" {
				log.Debugf("running bblfshd container")
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
			if err != nil {
				return errGrpcClient.Wrap(err)
			}

			files, err := util.GetFiles(prefix, extension, args...)
			if err != nil {
				return errGetFiles.Wrap(err)
			}

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
			defer func() { _ = storageClient.Close() }()
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
	flags.StringP("driver", "d", "not-specified", "name of the language current driver relates to")
	flags.StringP("commit", "c", "not-specified", "commit id that's being tested")
	flags.StringP("extension", "e", "", "file extension to be filtered")

	return cmd
}

func benchFile(ctx context.Context, c *bblfsh.Client, driver string, path string) (*testing.BenchmarkResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	// let's do not hold the descriptor while benchmark's running
	if err := f.Close(); err != nil {
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
