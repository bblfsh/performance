package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"testing"

	"github.com/bblfsh/performance"

	"github.com/bblfsh/sdk/v3/driver"
	"github.com/bblfsh/sdk/v3/driver/native"
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

func run(ctx context.Context, fixtures, resultsFile, filterPrefix string) (gerr error) {
	client := native.NewDriver(native.UTF8)
	if gerr := client.Start(); gerr != nil {
		return fmt.Errorf("failed to start driver: %v", gerr)
	}
	defer client.Close()

	files, gerr := performance.GetFiles(filterPrefix, excludeSubstrings, fixtures)
	if gerr != nil {
		return fmt.Errorf("cannot get files: %v", gerr)
	} else if len(files) == 0 {
		return fmt.Errorf("no files detected: %v", gerr)
	}

	var benchmarks []performance.Benchmark
	for _, f := range files {
		log.Debugf("benching file: %s", f)
		bRes, gerr := benchFile(ctx, client, f)
		if gerr != nil {
			return fmt.Errorf("cannot perform benchmark over the file %v: %v", f, gerr)
		}
		benchmarks = append(benchmarks, performance.BenchmarkResultToBenchmark(f, bRes, filterPrefix))
	}

	data, gerr := json.Marshal(benchmarks)
	if gerr != nil {
		return fmt.Errorf("failed to marshal results: %v", gerr)
	}

	file := resultsFile
	f, gerr := os.Create(file)
	if gerr != nil {
		return fmt.Errorf("failed to marshal results: %v", gerr)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Infof("failed to close file %v: %v", file, err)
			gerr = fmt.Errorf("err: %v, closeErr: %v", gerr, err)
		}
	}()

	if _, gerr := f.Write(data); gerr != nil {
		return fmt.Errorf("failed to write to file %v: %v", file, gerr)
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
