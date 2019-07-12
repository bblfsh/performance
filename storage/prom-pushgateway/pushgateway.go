package prom_pushgateway

import (
	"time"

	"github.com/bblfsh/performance"
	"github.com/bblfsh/performance/storage"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/src-d/envconfig"
	"gopkg.in/src-d/go-log.v1"
)

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
func (c *promClient) Dump(tags map[string]string, benchmarks ...performance.Benchmark) error {
	labels, values := performance.SplitStringMap(tags)
	labels = append([]string{"name"}, labels...)

	log.Debugf("getting metrics")
	metrics := getMetrics(labels)
	for _, b := range benchmarks {
		bench := b.Benchmark
		tmpValues := append([]string{bench.Name}, values...)

		log.Debugf("observing for the benchmark: %+v", b)
		metrics[storage.PerOpSeconds].WithLabelValues(tmpValues...).Observe(time.Duration(bench.NsPerOp).Seconds())
		metrics[storage.PerOpAllocBytes].WithLabelValues(tmpValues...).Observe(float64(bench.AllocedBytesPerOp))
		metrics[storage.PerOpAllocs].WithLabelValues(tmpValues...).Observe(float64(bench.AllocsPerOp))
	}

	log.Debugf("adding metrics to the pusher")
	c.collector(metrics)
	log.Debugf("pushing metrics")
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
