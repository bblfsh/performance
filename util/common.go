package util

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"golang.org/x/tools/benchmark/parse"
	"gopkg.in/src-d/go-errors.v1"
)

// RunE is a type that represents a standard Run function for cobra commands
type RunE func(cmd *cobra.Command, args []string) error

const (
	// BblfshdLevel is a metrics tag that represents benchmarks being run over bblfshd container
	BblfshdLevel = "bblfshd"
	// DriverLevel is a metrics tag that represents benchmarks being run over language driver container
	DriverLevel = "driver"
	// TransformsLevel is a metrics tag that represents benchmarks being run over transformations layer
	TransformsLevel = "transforms"
)

// TODO(lwsanty): https://github.com/spf13/cobra/issues/340
// RunESilenced is a wrapper over standard cobra's RunE function
// Purpose: hide the command usage output in the case of internal error inside the command
func RunESilenced(f RunE) RunE {
	return func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return f(cmd, args)
	}
}

// GetFiles is a simple "get files by pattern" function
// Purpose: filter required fixtures
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

// BenchmarkResultToParseBenchmark converts b *testing.BenchmarkResult *parse.Benchmark for further storing
func BenchmarkResultToParseBenchmark(name string, b *testing.BenchmarkResult) *parse.Benchmark {
	return &parse.Benchmark{
		Name:              name,
		N:                 b.N,
		NsPerOp:           float64(b.NsPerOp()),
		AllocedBytesPerOp: uint64(b.AllocedBytesPerOp()),
		AllocsPerOp:       uint64(b.AllocsPerOp()),
	}
}

// WrapErr wraps given error with a given amount of error kinds. Works in inside-to-outside direction
func WrapErr(err error, kinds ...*errors.Kind) error {
	if err == nil {
		return nil
	}
	for _, k := range kinds {
		err = k.Wrap(err)
	}
	return err
}
