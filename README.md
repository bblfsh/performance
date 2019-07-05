# Babelfish performance testing

## Currently supports only 2 commands

### parse-and-store
```bash
Alexandrs-MacBook-Pro:performance Alexandr$ ./performance parse-and-store  --help
parse file(s) with golang benchmark output and store it in influx db

Usage:
  performance parse-and-store [--driver=<driver-name>] [--commit=<commit-id>] <file ...> [flags]

Aliases:
  parse-and-store, parse, p, dump, store

Examples:
WARNING! To access influx db corresponding environment variables should be set.
Full example of usage script is the following:

export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
performance parse-and-store --driver=go --commit=3d9682b6a3c51db91896ad516bc521cce49ffe10 /var/log/bench0 /var/log/bench1

Flags:
  -c, --commit string   commit id that's being tested (default "not-specified")
  -d, --driver string   name of the language current driver relates to (default "not-specified")
  -h, --help            help for parse-and-store
```


### end-2-end
```bash
./performance end-to-end --help
run bblfshd container and perform benchmark tests, store results in influx db

Usage:
  performance end-to-end [--driver=<driver-name>] [--commit=<commit-id>] [--extension=<files-extension>] <directory ...> [flags]

Aliases:
  end-to-end, e2e

Examples:
WARNING! To access influx db corresponding environment variables should be set.
Full example of usage script is the following:

export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
performance end-to-end --driver=go --commit=3d9682b6a3c51db91896ad516bc521cce49ffe10 --extension=".go" /var/testdata/benchmarks

Flags:
  -c, --commit string      commit id that's being tested (default "not-specified")
  -d, --driver string      name of the language current driver relates to (default "not-specified")
  -e, --extension string   file extension to be filtered
  -h, --help               help for end-to-end
```
