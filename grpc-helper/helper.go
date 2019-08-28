package grpc_helper

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/bblfsh/performance"
	"github.com/bblfsh/performance/storage"

	bblfsh "github.com/bblfsh/go-client/v4"
	"gopkg.in/src-d/go-errors.v1"
	"gopkg.in/src-d/go-log.v1"
)

var (
	errGRPCClient      = errors.NewKind("cannot get grpc client")
	errGetFiles        = errors.NewKind("cannot get files")
	errBenchmark       = errors.NewKind("cannot perform benchmark over the file %v: %v")
	errNoFilesDetected = errors.NewKind("no files detected")
	errWarmUpFailed    = errors.NewKind("warmup for file %v has failed: %v")
)

// BenchmarkGRPCMeta collects metadata that is required for:
// - GRPC client connection
// - files filtering
// - proper push to storage
type BenchmarkGRPCMeta struct {
	// Address represents the address of GRPC server
	Address string
	// Commit represents commit id that's being tested, used as a label for storage
	Commit string
	// ExcludeSubstrings is a filtering option that defines substrings which files cannot contain to be filtered
	ExcludeSubstrings []string
	// Dirs defines the array of directories to be listed for files for further filtering
	Dirs []string
	// FilterPrefix is a filtering option that defines name prefix files should contain to be filtered
	FilterPrefix string
	// Language represents tested language, is used during the request and as a label for storage
	Language string
	// Level represents the level of bblfsh architecture tested(either end-to-end, driver, driver-native, transformations)
	// is used as a label during the storage
	Level string
	// Storage represents storage type to be used
	Storage string
}

// BenchmarkGRPCAndStore performs steps
// 1) creates client to GRPC server
// 2) filters files from a given directories
// 3) runs warm up request
// 4) runs benchmarks using the filtered files
// 5) stores results to a given storage
func BenchmarkGRPCAndStore(ctx context.Context, meta BenchmarkGRPCMeta) error {
	client, err := bblfsh.NewClientContext(ctx, meta.Address)
	if err != nil {
		return errGRPCClient.Wrap(err)
	}
	defer client.Close()

	files, err := performance.GetFiles(meta.FilterPrefix, meta.ExcludeSubstrings, meta.Dirs...)
	if err != nil {
		return errGetFiles.Wrap(err)
	} else if len(files) == 0 {
		return errNoFilesDetected.New()
	}

	warmUpFile := files[0]
	log.Debugf("🔥warming up the language %s using file %s", meta.Language, warmUpFile)
	warmUpTime, err := warmUpDriver(ctx, client, meta.Language, warmUpFile)
	if err != nil {
		return errWarmUpFailed.New(warmUpFile, err)
	}
	log.Debugf("warm up done for file %s in %v", warmUpFile, warmUpTime)

	var benchmarks []performance.Benchmark
	for _, f := range files {
		log.Debugf("benching file: %s", f)
		bRes, err := benchFile(ctx, client, meta.Language, f)
		if err != nil {
			return errBenchmark.New(f, err)
		}
		benchmarks = append(benchmarks, performance.BenchmarkResultToBenchmark(f, bRes, meta.FilterPrefix))
	}

	// store data
	storageClient, err := storage.NewClient(meta.Storage)
	if err != nil {
		return err
	}
	defer storageClient.Close()

	return storageClient.Dump(map[string]string{
		"language": meta.Language,
		"commit":   meta.Commit,
		"level":    meta.Level,
	}, benchmarks...)
}

func warmUpDriver(ctx context.Context, c *bblfsh.Client, language string, path string) (time.Duration, error) {
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

	res := testing.Benchmark(performance.Bench(func() {
		_, _, err := c.NewParseRequest().Context(ctx).Language(language).Content(string(data)).UAST()
		if err != nil {
			panic(err)
		}
	}))
	return &res, nil
}
