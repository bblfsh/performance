package prom_pushgateway

import (
	"github.com/bblfsh/performance/util"
	"github.com/bblfsh/performance/util/storage"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/src-d/envconfig"
	"golang.org/x/tools/benchmark/parse"
)

// TODO logs, comments, commands doc

// Kind is a string that represents prometheus pushgateway
const Kind = "prom"

type metrics map[string]*prometheus.SummaryVec

type promClient struct {
	*push.Pusher
}

type promConfig struct {
	Address string
	Job     string
}

func init() {
	storage.Register(Kind, NewClient)
}

// NewClient is a constructor that uses environment variables to create a wrapper around *push.Pusher,
func NewClient() (storage.Client, error) {
	var promConfig promConfig
	if err := envconfig.Process("prom", &promConfig); err != nil {
		return nil, err
	}
	return &promClient{push.New(promConfig.Address, promConfig.Job)}, nil
}

// Dump stores given benchmark results with tags to prometheus pushgateway
func (c *promClient) Dump(tags map[string]string, benchmarks ...*parse.Benchmark) error {
	labels, values := util.SplitStringMap(tags)
	labels = append([]string{"name"}, labels...)

	metrics := getMetrics(labels)
	for _, b := range benchmarks {
		tmpValues := append([]string{util.ParseBenchmarkName(b.Name)}, values...)

		metrics[storage.PerOpSeconds].WithLabelValues(tmpValues...).Observe(float64(b.NsPerOp / 1e9))
		metrics[storage.PerOpAllocBytes].WithLabelValues(tmpValues...).Observe(float64(b.AllocedBytesPerOp))
		metrics[storage.PerOpAllocs].WithLabelValues(tmpValues...).Observe(float64(b.AllocsPerOp))
	}

	c.collector(metrics)
	return c.Add()
}

// Close is an implementation of interface
// there're no connections should be closed
func (c *promClient) Close() error { return nil }

func (c *promClient) collector(ms metrics) {
	for _, m := range ms {
		c.Collector(m)
	}
}

func getMetrics(labels []string) metrics {
	return metrics{
		storage.PerOpSeconds:    getMetric(storage.PerOpSeconds, labels),
		storage.PerOpAllocBytes: getMetric(storage.PerOpAllocBytes, labels),
		storage.PerOpAllocs:     getMetric(storage.PerOpAllocs, labels),
	}
}

func getMetric(name string, labels []string) *prometheus.SummaryVec {
	return prometheus.NewSummaryVec(
		prometheus.SummaryOpts{Name: name},
		labels,
	)
}
