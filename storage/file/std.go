package file

import (
	"github.com/bblfsh/performance"
	"github.com/bblfsh/performance/storage"

	"gopkg.in/src-d/go-errors.v1"
)

// TODO(lwsanty): https://github.com/bblfsh/performance/issues/5

// Kind is a string that represents file
const Kind = "file"

var errNotImplemented = errors.NewKind("%v: not implemented")

type fileClient struct{}

func init() {
	storage.Register(Kind, NewClient)
}

// NewClient is a constructor for file
func NewClient() (storage.Client, error) {
	return &fileClient{}, nil
}

// Dump prints given benchmark results with tags to file
func (c *fileClient) Dump(tags map[string]string, benchmarks ...performance.Benchmark) error {
	return errNotImplemented.New()
}

// Close closes connection to file
func (c *fileClient) Close() error {
	return errNotImplemented.New(Kind)
}
