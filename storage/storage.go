package storage

import (
	"github.com/bblfsh/performance"
	"gopkg.in/src-d/go-errors.v1"
)

const (
	// PerOpSeconds represents metric of seconds per operation
	PerOpSeconds = "per_op_seconds"
	// PerOpAllocBytes represents metric of bytes allocated per operation
	PerOpAllocBytes = "per_op_alloc_bytes"
	// PerOpAllocs represents metric of allocations per operation
	PerOpAllocs = "per_op_allocs"
)

// Constructor is a type that represents function of default storage client Constructor
type Constructor func() (Client, error)

var (
	// constructors is a map of all supported storage client constructors
	constructors = make(map[string]Constructor)

	errNotSupported = errors.NewKind("storage kind %v is not supported")
)

// Client is an interface for storage clients
type Client interface {
	// Dump stores given benchmark results with tags to storage
	Dump(tags map[string]string, benchmarks ...performance.Benchmark) error
	// Close closes client's connection to the storage if needed
	Close() error
}

// Register updates the map of known storage clients constructors
func Register(kind string, c Constructor) {
	constructors[kind] = c
}

// NewClient takes a given kind and creates related storage client
func NewClient(kind string) (Client, error) {
	c, err := ValidateKind(kind)
	if err != nil {
		return nil, err
	}
	return c()
}

// ValidateKind checks if a given kind is supported
// This method should be useful when long-term tests are performed
// so kind can be checked much earlier then storage client acquired
// and prevent the situation when tests passed and store failed because kind is not supported
func ValidateKind(kind string) (Constructor, error) {
	c, ok := constructors[kind]
	if !ok {
		return nil, errNotSupported.New(kind)
	}

	return c, nil
}
