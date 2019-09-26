package drivernative

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/bblfsh/performance"
	"github.com/bblfsh/performance/docker"
	"github.com/bblfsh/performance/storage"
	"github.com/bblfsh/performance/storage/file"
	"github.com/bblfsh/performance/storage/influxdb"
	"github.com/bblfsh/performance/storage/pushgateway"

	"github.com/spf13/cobra"
	"gopkg.in/src-d/go-log.v1"
)

const (
	containerTmp      = "/tmp"
	containerFixtures = containerTmp + "/fixtures"
	results           = "results.txt"
)

// Cmd return configured driver-native command
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "driver-native [--language=<language>] [--commit=<commit-id>] [--storage=<storage>] [--filter-prefix=<filter-prefix>] [--native=<path-to-native>] <directory>",
		Aliases: []string{"dn", "native"},
		Args:    cobra.MinimumNArgs(1),
		Short:   "run language driver container and perform benchmark tests over the native driver, store results into a given storage",
		Example: `WARNING! Requires native-driver-performance binary to be build
WARNING! To access storage corresponding environment variables should be set.
Full examples of usage scripts are following:

# for prometheus pushgateway
export PROM_ADDRESS="localhost:9091"
export PROM_JOB=pushgateway
./bblfsh-performance driver-native \
--language go \
--commit 096361d09049c27e829fd5a6658f1914fd3b62ac \
--native /home/lwsanty/goproj/lwsanty/performance/cmd/native-driver-performance/native-driver-performance \
/var/testdata/fixtures

# for influx db
export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
./bblfsh-performance driver-native \
--language go \
--commit 096361d09049c27e829fd5a6658f1914fd3b62ac \
--native /home/lwsanty/goproj/lwsanty/performance/cmd/native-driver-performance/native-driver-performance \
--storage=influxdb \
/var/testdata/fixtures
`,
		RunE: performance.RunESilenced(func(cmd *cobra.Command, args []string) error {
			language, _ := cmd.Flags().GetString("language")
			commit, _ := cmd.Flags().GetString("commit")
			stor, _ := cmd.Flags().GetString("storage")
			filterPrefix, _ := cmd.Flags().GetString("filter-prefix")
			native, _ := cmd.Flags().GetString("native")

			fixtures := args[0]
			execDst := getSubTmp(filepath.Base(native))
			resultsPath := getSubTmp(results)

			log.Debugf("validating storage")
			if _, err := storage.ValidateKind(stor); err != nil {
				return err
			}

			log.Debugf("download and build driver")
			image, err := docker.DownloadAndBuildDriver(language, commit)
			if err != nil {
				return err
			}

			log.Debugf("run driver container")
			driver, err := docker.RunDriver(image, fixtures+":"+containerFixtures)
			if err != nil {
				return err
			}
			defer driver.Close()

			// prepare context
			ctx, cancel := context.WithCancel(context.Background())
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			defer func() {
				signal.Stop(c)
				cancel()
			}()
			go func() {
				select {
				case <-c:
					cancel()
				case <-ctx.Done():
				}
			}()

			log.Debugf("copying file %v to container's dst: %v", native, execDst)
			if err := driver.Upload(ctx, native, execDst); err != nil {
				return err
			}

			log.Debugf("executing command on driver")
			//if err := driver.Exec(ctx, "sh", "-c", "dfgdfg"); err != nil {
			if err := driver.Exec(ctx,
				[]string{"LOG_LEVEL=debug"},
				execDst,
				"--filter-prefix="+filterPrefix,
				"--fixtures="+containerFixtures,
				"--results="+resultsPath,
			); err != nil {
				return err
			}

			log.Debugf("getting results")
			data, err := driver.GetResults(ctx, resultsPath)
			if err != nil {
				return err
			}

			var benchmarks []performance.Benchmark
			if err := json.Unmarshal(data, &benchmarks); err != nil {
				return err
			}

			// store data
			storageClient, err := storage.NewClient(stor)
			if err != nil {
				return err
			}
			defer storageClient.Close()

			if err := storageClient.Dump(map[string]string{
				"language": language,
				"commit":   commit,
				"level":    performance.DriverNativeLevel,
			}, benchmarks...); err != nil {
				return err
			}

			return nil
		}),
	}

	flags := cmd.Flags()
	flags.StringP("native", "n", "/root/utils/native-driver-test", "path to native driver performance util")
	flags.StringP("language", "l", "", "name of the language to be tested")
	flags.StringP("commit", "c", "", "commit id that's being tested and will be used as a tag in performance report")
	flags.StringSlice("exclude-suffixes", []string{".legacy", ".native", ".uast"}, "file suffixes to be excluded")
	flags.String("filter-prefix", performance.FileFilterPrefix, "file prefix to be filtered")
	flags.StringP("storage", "s", pushgateway.Kind, fmt.Sprintf("storage kind to store the results(%s, %s, %s)", pushgateway.Kind, influxdb.Kind, file.Kind))

	return cmd
}

func getSubTmp(name string) string {
	return filepath.Join(containerTmp, name)
}
