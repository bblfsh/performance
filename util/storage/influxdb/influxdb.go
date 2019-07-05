package influxdb

import (
	"time"

	"github.com/bblfsh/performance/util"
	"github.com/bblfsh/performance/util/storage"
	"github.com/orourkedd/influxdb1-client/client"
	"github.com/src-d/envconfig"
	"golang.org/x/tools/benchmark/parse"
	"gopkg.in/src-d/go-errors.v1"
	"gopkg.in/src-d/go-log.v1"
)

// Kind is a string that represents influxdb
const Kind = "influxdb"

func init() {
	storage.Register(Kind, NewClient)
}

// influxClient embeds influxdb client itself and also contains the configuration info
type influxClient struct {
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

// NewClient is a constructor for influxClient, uses environment variables to get influxConfig
func NewClient() (storage.Client, error) {
	var influxConfig influxConfig
	if err := envconfig.Process("influx", &influxConfig); err != nil {
		return nil, err
	}

	c, err := influxDBClient(influxConfig)
	if err != nil {
		return nil, errGetClientFailed.Wrap(err)
	}

	return &influxClient{
		Client:       c,
		influxConfig: influxConfig,
	}, nil
}

// Dump stores given benchmark results with tags to influxdb
func (c *influxClient) Dump(tags map[string]string, benchmarks ...*parse.Benchmark) error {
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
		tags["name"] = util.ParseBenchmarkName(b.Name)
		fields := map[string]interface{}{
			"n":                  b.N,
			storage.PerOpSeconds: b.NsPerOp / 1e9,
			// https://github.com/influxdata/influxdb/issues/7801
			storage.PerOpAllocBytes: int(b.AllocedBytesPerOp),
			storage.PerOpAllocs:     int(b.AllocsPerOp),
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

func (c *influxClient) Close() error {
	return c.Client.Close()
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
