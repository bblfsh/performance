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
	// DriverNativeLevel is a metrics tag that represents benchmarks being run over native language driver
	DriverNativeLevel = "driver-native"
	// TransformsLevel is a metrics tag that represents benchmarks being run over transformations layer
	TransformsLevel = "transforms"

	// FileFilterPrefix is a fileFilterPrefix of file that would be filtered from the list of files in a directory.
	// Currently we use benchmark fixtures, file name pattern in this case is bench_*.${extension}
	FileFilterPrefix = "bench_"
)

var (
	// ErrCannotInstallCustomDriver is used when driver installation process has failed or test conditions
	// do not allow to install it
	ErrCannotInstallCustomDriver = errors.NewKind("custom driver cannot be installed: %v")

	errCmdFailed = errors.NewKind("command failed: %v, output: %v")
)

// Benchmark is a wrapper around parse.Benchmark and serves for formatting and arranging data before storing
type Benchmark struct {
	Benchmark parse.Benchmark
}

// NewBenchmark is a constructor for Benchmark
func NewBenchmark(pb *parse.Benchmark, trimPrefixes ...string) Benchmark {
	pb.Name = parseBenchmarkName(pb.Name, trimPrefixes...)
	return Benchmark{*pb}
}

// BenchmarkResultToBenchmark converts b *testing.BenchmarkResult *parse.Benchmark for further storing
func BenchmarkResultToBenchmark(name string, b *testing.BenchmarkResult, trimPrefixes ...string) Benchmark {
	return NewBenchmark(&parse.Benchmark{
		Name:              name,
		N:                 b.N,
		NsPerOp:           float64(b.NsPerOp()),
		AllocedBytesPerOp: uint64(b.AllocedBytesPerOp()),
		AllocsPerOp:       uint64(b.AllocsPerOp()),
	}, trimPrefixes...)
}

// parseBenchmarkName removes the path and suffixes from benchmark info
// Example: BenchmarkGoDriver/transform/accumulator_factory-4 -> accumulator_factory
func parseBenchmarkName(name string, trimPrefixes ...string) string {
	for _, tp := range trimPrefixes {
		name = strings.TrimPrefix(name, tp)
	}
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

func stringInSlice(s string, strSlice []string) bool {
	for _, sl := range strSlice {
		if strings.HasSuffix(s, sl) {
			return true
		}
	}
	return false
}

// GetFiles is a simple "get files by pattern" function
// Purpose: filter required fixtures
func GetFiles(pref string, exclusionSuffixes []string, dirs ...string) ([]string, error) {
	var res []string
	for _, d := range dirs {
		matches, err := filepath.Glob(filepath.Join(d, pref+"*"))
		if err != nil {
			return nil, err
		}

		for _, m := range matches {
			if stringInSlice(m, exclusionSuffixes) {
				continue
			}
			res = append(res, m)
		}
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
	log.Debugf("command output: %v", string(data))
	if err != nil {
		return errCmdFailed.New(err, string(data))
	}

	return nil
}

// Bench wraps given function into function that performs benchmark over it
func Bench(f func()) func(b *testing.B) {
	return func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			f()
		}
	}
}
