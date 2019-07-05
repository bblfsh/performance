package storage

import (
	"strings"
	"time"

	"github.com/orourkedd/influxdb1-client/client"
	"github.com/src-d/envconfig"
	"golang.org/x/tools/benchmark/parse"
	"gopkg.in/src-d/go-errors.v1"
	"gopkg.in/src-d/go-log.v1"
)

const envPrefix = "influx"

// InfluxClient embeds influxdb client itself and also contains the configuration info
type InfluxClient struct {
	client.Client
	influxConfig influxConfig
}

type influxConfig struct {
	Address  string
	Username string
	Password string
	Db       string
	// Measurement acts as a container for tags, fields, and the time column, and the measurement name is the description of the data that are stored in the associated fields.
	// Measurement names are strings, and, for any SQL users out there, a measurement is conceptually similar to a table.
	Measurement string
}

var (
	errGetClientFailed = errors.NewKind("cannot get influx db client")
	errDumpFailed      = errors.NewKind("cannot dump batch points")
)

// TODO(lwsanty): client should become interface in the future to support several storages
// NewClient is a constructor for InfluxClient, uses environment variables to get influxConfig
func NewClient() (*InfluxClient, error) {
	var influxConfig influxConfig
	if err := envconfig.Process(envPrefix, &influxConfig); err != nil {
		return nil, err
	}

	c, err := influxDBClient(influxConfig)
	if err != nil {
		return nil, errGetClientFailed.Wrap(err)
	}

	return &InfluxClient{
		Client:       c,
		influxConfig: influxConfig,
	}, nil
}

// Dump stores given benchmark results with tags to influxdb
func (c *InfluxClient) Dump(tags map[string]string, benchmarks ...*parse.Benchmark) error {
	wrapErr := func(err error) error { return errDumpFailed.Wrap(err) }

	if tags == nil {
		tags = make(map[string]string)
	}

	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  c.influxConfig.Db,
		Precision: "s",
	})
	if err != nil {
		return wrapErr(err)
	}

	eventTime := time.Now()
	for _, b := range benchmarks {
		tags["name"] = parseBenchmarkName(b.Name)
		fields := map[string]interface{}{
			"n":              b.N,
			"per_op_seconds": b.NsPerOp / 1e9,
			// https://github.com/influxdata/influxdb/issues/7801
			"per_op_alloc_bytes": int(b.AllocedBytesPerOp),
			"per_op_alloc":       int(b.AllocsPerOp),
		}

		point, err := client.NewPoint(
			c.influxConfig.Measurement,
			tags,
			fields,
			eventTime,
		)
		if err != nil {
			return wrapErr(err)
		}
		log.Debugf("batch -> add point %+v\n", point)
		bp.AddPoint(point)
	}
	if err := c.Write(bp); err != nil {
		return wrapErr(err)
	}

	return nil
}

func influxDBClient(conf influxConfig) (client.Client, error) {
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     conf.Address,
		Username: conf.Username,
		Password: conf.Password,
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

func parseBenchmarkName(name string) string {
	spl := strings.Split(name, "/")
	name = spl[len(spl)-1]
	for _, s := range []string{"-", "."} {
		name = strings.Split(name, s)[0]
	}
	return name
}
