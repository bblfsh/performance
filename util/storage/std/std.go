package std

import (
	"github.com/bblfsh/performance/util/storage"
	"golang.org/x/tools/benchmark/parse"
	"gopkg.in/src-d/go-errors.v1"
)

// TODO(lwsanty): https://github.com/bblfsh/performance/issues/5

// Kind is a string that represents stdout
const Kind = "std"

var errNotImplemented = errors.NewKind("%s: not implemented")

type stdClient struct{}

func init() {
	storage.Register(Kind, NewClient)
}

// NewClient is a constructor for stdout
func NewClient() (storage.Client, error) {
	return &stdClient{}, nil
}

// Dump prints given benchmark results with tags to stdout
func (c *stdClient) Dump(tags map[string]string, benchmarks ...*parse.Benchmark) error {
	return errNotImplemented.New()
}

// Close closes connection to stdout
func (c *stdClient) Close() error {
	return errNotImplemented.New(Kind)
}
