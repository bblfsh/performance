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

// TODO(lwsanty): https://github.com/bblfsh/performance/issues/2
// Cmd return configured end to end command
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "end-to-end [--language=<language>] [--commit=<commit-id>] [--extension=<files-extension>] [--docker-tag=<docker-tag>] <directory ...>",
		Aliases: []string{"e2e"},
		Args:    cobra.MinimumNArgs(1),
		Short:   "run bblfshd container and perform benchmark tests, store results in influx db",
		Example: `To use external bblfshd set BBLFSHD_LOCAL=${bblfshd_address}

WARNING! To access influx db corresponding environment variables should be set.
Full example of usage script is the following:

export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
bblfsh-performance end-to-end --language=go --commit=3d9682b --extension=".go" /var/testdata/benchmarks`,
		RunE: util.RunESilenced(func(cmd *cobra.Command, args []string) error {
			language, _ := cmd.Flags().GetString("language")
			commit, _ := cmd.Flags().GetString("commit")
			extension, _ := cmd.Flags().GetString("extension")

			// for debug purposes with externally spinning container
			containerAddress := os.Getenv("BBLFSHD_LOCAL")
			if containerAddress == "" {
				tag, _ := cmd.Flags().GetString("docker-tag")
				log.Debugf("running bblfshd %s container\n", tag)
				addr, closer, err := docker.RunBblfshd(tag)
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
			defer client.Close()

			files, err := util.GetFiles(prefix, extension, args...)
			if err != nil {
				return errGetFiles.Wrap(err)
			} else if len(files) == 0 {
				return errNoFilesDetected.New()
			}

			warmUpFile := files[0]
			log.Debugf("🔥 warming up the language %s using file %s\n", language, warmUpFile)
			warmUpTime, err := warmUp(ctx, client, language, warmUpFile)
			if err != nil {
				return errWarmUpFailed.New(warmUpFile, err)
			}
			log.Debugf("warm up done for file %s in %v", warmUpFile, warmUpTime)

			var benchmarks []*parse.Benchmark
			for _, f := range files {
				log.Debugf("benching file: %s\n", f)
				bRes, err := benchFile(ctx, client, language, f)
				if err != nil {
					return errBenchmark.New(f, err)
				}
				benchmarks = append(benchmarks, util.BenchmarkResultToParseBenchmark(f, bRes))
			}

			// store data
			storageClient, err := storage.NewClient()
			if err != nil {
				return err
			}
			defer storageClient.Close()

			if err := storageClient.Dump(map[string]string{
				"language": language,
				"commit":   commit,
				"level":    util.BblfshdLevel,
			}, benchmarks...); err != nil {
				return err
			}

			return nil
		}),
	}

	flags := cmd.Flags()
	flags.StringP("language", "l", "", "name of the language to be tested")
	flags.StringP("commit", "c", "", "commit id that's being tested and will be used as a tag in performance report")
	flags.StringP("extension", "e", "", "file extension to be filtered")
	flags.StringP("docker-tag", "t", "latest-drivers", "bblfshd docker image tag to be tested")

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
