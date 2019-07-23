# Babelfish performance testing

## Build
```bash
go generate ./...
cd cmd/bblfsh-performance
go build
```

## Currently supports only 2 commands

### parse-and-store
```bash
./bblfsh-performance parse-and-store --help
parse file(s) with golang benchmark output and store it into a given storage

Usage:
  bblfsh-performance parse-and-store [--language=<language>] [--commit=<commit-id>] [--storage=<storage>] <file ...> [flags]

Aliases:
  parse-and-store, pas, parse-and-dump

Examples:
WARNING! To access storage corresponding environment variables should be set.
Full examples of usage scripts are following:

# for prometheus pushgateway
export PROM_ADDRESS="localhost:9091"
export PROM_JOB=pushgateway
bblfsh-performance parse-and-store --language=go --commit=3d9682b --storage="prom" /var/log/bench0 /var/log/bench1

# for influx db
export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
bblfsh-performance parse-and-store --language=go --commit=3d9682b --storage="influxdb" /var/log/bench0 /var/log/bench1

Flags:
  -c, --commit string     commit id that's being tested and will be used as a tag in performance report
  -h, --help              help for parse-and-store
  -l, --language string   name of the language to be tested
  -s, --storage string    storage kind to store the results(prom, influxdb, file) (default "prom")
```

##### Command usage
Either locally or in CI:
1) pull driver repo
2) perform benchmarks over the fixtures
3) save output test benchmark output to the file
4) run `bblfsh-performance parse-and-store` and pass the filepath(s) as an argument


### end-2-end
```bash
./bblfsh-performance end-to-end --help
run bblfshd container and perform benchmark tests, store results into a given storage

Usage:
  bblfsh-performance end-to-end [--language=<language>] [--commit=<commit-id>] [--extension=<files-extension>] [--docker-tag=<docker-tag>] [--storage=<storage>] <directory ...> [flags]

Aliases:
  end-to-end, e2e

Examples:
To use external bblfshd set BBLFSHD_LOCAL=${bblfshd_address}

WARNING! To access storage corresponding environment variables should be set.
Full examples of usage scripts are following:

# for prometheus pushgateway
export PROM_ADDRESS="localhost:9091"
export PROM_JOB=pushgateway
./bblfsh-performance end-to-end --language=go --commit=3d9682b --filter-prefix="bench_" --exclude-substrings=".legacy",".native",".uast" --storage="prom" /var/testdata/benchmarks

# for influx db
export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
bblfsh-performance end-to-end --language=go --commit=3d9682b --filter-prefix="bench_" --exclude-substrings=".legacy",".native",".uast" --storage="influxdb" /var/testdata/benchmarks

Flags:
  -c, --commit string          commit id that's being tested and will be used as a tag in performance report
      --custom-driver          if this flag is set to true CLI pulls corresponding language driver repo's commit, builds docker image and installs it onto the bblfsh container
  -t, --docker-tag string      bblfshd docker image tag to be tested (default "latest-drivers")
      --exclude-substrings strings   file name substrings to be excluded (default [.legacy,.native,.uast])
      --filter-prefix string   file prefix to be filtered (default "bench_")
  -h, --help                   help for end-to-end
  -l, --language string        name of the language to be tested
  -s, --storage string         storage kind to store the results(prom, influxdb, file) (default "prom")
```
