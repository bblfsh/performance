package performance

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"golang.org/x/tools/benchmark/parse"
	"gopkg.in/src-d/go-errors.v1"
	"gopkg.in/src-d/go-log.v1"
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

var errCmdFailed = errors.NewKind("command failed: %v, output: %v")

// Benchmark is a wrapper around parse.Benchmark and serves for formatting and arranging data before storing
type Benchmark struct {
	Benchmark parse.Benchmark
}

// NewBenchmark is a constructor for Benchmark
func NewBenchmark(pb *parse.Benchmark) Benchmark {
	pb.Name = parseBenchmarkName(pb.Name)
	return Benchmark{*pb}
}

// BenchmarkResultToBenchmark converts b *testing.BenchmarkResult *parse.Benchmark for further storing
func BenchmarkResultToBenchmark(name string, b *testing.BenchmarkResult) Benchmark {
	return NewBenchmark(&parse.Benchmark{
		Name:              name,
		N:                 b.N,
		NsPerOp:           float64(b.NsPerOp()),
		AllocedBytesPerOp: uint64(b.AllocedBytesPerOp()),
		AllocsPerOp:       uint64(b.AllocsPerOp()),
	})
}

// parseBenchmarkName removes the path and suffixes from benchmark info
// Example: BenchmarkGoDriver/transform/accumulator_factory-4 -> accumulator_factory
func parseBenchmarkName(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.IndexAny(name, "-."); i >= 0 {
		return name[:i]
	}
	return name
}

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

// SplitStringMap splits map[string]string to arrays of keys and arrays of values
func SplitStringMap(m map[string]string) ([]string, []string) {
	var (
		keys   []string
		values []string
	)
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, k := range keys {
		values = append(values, m[k])
	}

	return keys, values
}

// ExecCmd executes the specified Bash script. If execution fails, the error contains
// the combined output from stdout and stderr of the script.
// Do not use this for scripts that produce a large volume of output.
func ExecCmd(command string) error {
	cmd := exec.Command("bash")
	cmd.Stdin = strings.NewReader(command)

	data, err := cmd.CombinedOutput()
	log.Debugf("command output %v", string(data))
	if err != nil {
		return errCmdFailed.New(err, string(data))
	}

	return nil
}
