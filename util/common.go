package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"golang.org/x/tools/benchmark/parse"
	"gopkg.in/src-d/go-errors.v1"
)

type RunE func(cmd *cobra.Command, args []string) error

// https://github.com/spf13/cobra/issues/340
func RunESilenced(f RunE) RunE {
	return func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return f(cmd, args)
	}
}

func GetFiles(pref, ext string, dirs ...string) ([]string, error) {
	var res []string
	for _, d := range dirs {
		matches, err := filepath.Glob(filepath.Join(d, pref+"*"+ext))
		if err != nil {
			return nil, err
		}
		res = append(res, matches...)
	}
	return res, nil
}

func BenchmarkResultToParseBenchmark(name string, b *testing.BenchmarkResult) *parse.Benchmark {
	return &parse.Benchmark{
		Name:              name,
		N:                 b.N,
		NsPerOp:           float64(b.NsPerOp()),
		AllocedBytesPerOp: uint64(b.AllocedBytesPerOp()),
		AllocsPerOp:       uint64(b.AllocsPerOp()),
	}
}

func WrapErr(err error, kinds ...*errors.Kind) error {
	if err == nil {
		return nil
	}
	for _, k := range kinds {
		err = k.Wrap(err)
	}
	return err
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
