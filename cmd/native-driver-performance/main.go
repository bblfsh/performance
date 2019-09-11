package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"os/signal"
	"testing"

	"github.com/bblfsh/performance"

	"github.com/bblfsh/sdk/v3/driver"
	"github.com/bblfsh/sdk/v3/driver/native"
	"gopkg.in/src-d/go-errors.v1"
	"gopkg.in/src-d/go-log.v1"
)

var excludeSubstrings = []string{".legacy", ".native", ".uast"}

func main() {
	// TODO: fixtures filters and so on
	fixtures := flag.String("fixtures", "", "path to fixtures directory")
	resultsFile := flag.String("results", "", "path to file to store benchmark results")
	filterPrefix := flag.String("filter-prefix", "", "file prefix to be filtered")

	flag.Parse()

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

	if err := run(ctx, *fixtures, *resultsFile, *filterPrefix); err != nil {
		log.Infof("run failed: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, fixtures, resultsFile, filterPrefix string) error {
	client := native.NewDriver(native.UTF8)
	if err := client.Start(); err != nil {
		return errors.NewKind("failed to start driver").Wrap(err)
	}
	defer client.Close()

	files, err := performance.GetFiles(filterPrefix, excludeSubstrings, fixtures)
	if err != nil {
		return errors.NewKind("cannot get files").Wrap(err)
	} else if len(files) == 0 {
		return errors.NewKind("no files detected").New()
	}

	var benchmarks []performance.Benchmark
	for _, f := range files {
		log.Debugf("benching file: %s", f)
		bRes, err := benchFile(ctx, client, f)
		if err != nil {
			return errors.NewKind("cannot perform benchmark over the file %v: %v").New(f, err)
		}
		benchmarks = append(benchmarks, performance.BenchmarkResultToBenchmark(f, bRes, filterPrefix))
	}

	data, err := json.Marshal(benchmarks)
	if err != nil {
		return errors.NewKind("failed to marshal results").Wrap(err)
	}

	file := resultsFile
	f, err := os.Create(file)
	if err != nil {
		return errors.NewKind("failed to marshal results").Wrap(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Infof("failed to close file %v: %v", file, err)
		}
	}()

	if _, err := f.Write(data); err != nil {
		return errors.NewKind("failed to write to file %v: %v").New(file, err)
	}

	return nil
}

func benchFile(ctx context.Context, driver driver.Native, path string) (*testing.BenchmarkResult, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	res := testing.Benchmark(performance.Bench(func() {
		_, err := driver.Parse(ctx, string(data))
		if err != nil {
			panic(err)
		}
	}))
	return &res, nil
}
