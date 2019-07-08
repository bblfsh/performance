package storage

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/src-d/envconfig"
	"gopkg.in/src-d/go-errors.v1"
	"gopkg.in/src-d/go-log.v1"
	// https://github.com/influxdata/influxdb1-client/issues/5
	"github.com/orourkedd/influxdb1-client/client"
	"golang.org/x/tools/benchmark/parse"
)

const envPrefix = "influx"

type InfluxClient struct {
	client.Client
	conf Config
}

type Config struct {
	Driver       string
	Commit       string
	influxConfig influxConfig
}

type influxConfig struct {
	Address  string
	Username string
	Password string
	Db       string
	// The measurement acts as a container for tags, fields, and the time column, and the measurement name is the description of the data that are stored in the associated fields.
	// Measurement names are strings, and, for any SQL users out there, a measurement is conceptually similar to a table.
	Measurement string
}

var (
	errGetClientFailed = errors.NewKind("cannot get influx db client")
	errDumpFailed      = errors.NewKind("cannot dump batch points")
)

func NewClient(driver, commit string) (*InfluxClient, error) {
	var influxConfig influxConfig
	if err := envconfig.Process(envPrefix, &influxConfig); err != nil {
		return nil, err
	}

	c, err := influxDBClient(influxConfig)
	if err != nil {
		return nil, errGetClientFailed.Wrap(err)
	}

	return &InfluxClient{
		Client: c,
		conf: Config{
			Driver:       driver,
			Commit:       commit,
			influxConfig: influxConfig,
		},
	}, nil
}

func (c *InfluxClient) Dump(benchmarks ...*parse.Benchmark) error {
	wrapErr := func(err error) error { return errDumpFailed.Wrap(err) }

	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  c.conf.influxConfig.Db,
		Precision: "s",
	})
	if err != nil {
		return wrapErr(err)
	}

	eventTime := time.Now()
	for _, b := range benchmarks {
		tags := map[string]string{
			"driver": c.conf.Driver,
			"name":   parseBenchmarkName(b.Name),
			"commit": c.conf.Commit,
		}

		fields := map[string]interface{}{
			"n":        b.N,
			"per_op_s": b.NsPerOp / 1e9,
			// https://github.com/influxdata/influxdb/issues/7801
			"per_op_bytes_alloc": int(b.AllocedBytesPerOp),
			"per_op_alloc":       int(b.AllocsPerOp),
		}

		point, err := client.NewPoint(
			c.conf.influxConfig.Measurement,
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
	spl := strings.SplitAfterN(name, "/", -1)
	if len(spl) == 0 {
		return name
	}
	name = spl[len(spl)-1]
	for _, s := range []string{"-", "."} {
		name = strings.Split(filepath.Base(name), s)[0]
	}

	return name
}
