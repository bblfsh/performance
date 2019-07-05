# Babelfish performance testing

## Currently supports only 2 commands

### parse-and-store
```bash
./bblfsh-performance parse-and-store --help
parse file(s) with golang benchmark output and store it in influx db

Usage:
  bblfsh-performance parse-and-store [--language=<language>] [--commit=<commit-id>] <file ...> [flags]

Aliases:
  parse-and-store, pas, parse-and-dump

Examples:
WARNING! To access influx db corresponding environment variables should be set.
Full example of usage script is the following:

export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
bblfsh-performance parse-and-store --language=go --commit=3d9682b /var/log/bench0 /var/log/bench1

Flags:
  -c, --commit string     commit id that's being tested and will be used as a tag in performance report
  -h, --help              help for parse-and-store
  -l, --language string   name of the language to be tested
```

#####Command usage
Either locally or in CI:
1) pull driver repo
2) perform benchmarks over the fixtures
3) save output test benchmark output to the file
4) run `bblfsh-performance parse-and-store` and pass the filepath(s) as an argument


### end-2-end
```bash
./bblfsh-performance end-to-end --help
run bblfshd container and perform benchmark tests, store results in influx db

Usage:
  bblfsh-performance end-to-end [--language=<language>] [--commit=<commit-id>] [--extension=<files-extension>] [--docker-tag=<docker-tag>] <directory ...> [flags]

Aliases:
  end-to-end, e2e

Examples:
To use external bblfshd set BBLFSHD_LOCAL=${bblfshd_address}

WARNING! To access influx db corresponding environment variables should be set.
Full example of usage script is the following:

export INFLUX_ADDRESS="http://localhost:8086"
export INFLUX_USERNAME=""
export INFLUX_PASSWORD=""
export INFLUX_DB=mydb
export INFLUX_MEASUREMENT=benchmark
bblfsh-performance end-to-end --language=go --commit=3d9682b --extension=".go" /var/testdata/benchmarks

Flags:
  -c, --commit string       commit id that's being tested and will be used as a tag in performance report
  -t, --docker-tag string   bblfshd docker image tag to be tested (default "latest-drivers")
  -e, --extension string    file extension to be filtered
  -h, --help                help for end-to-end
  -l, --language string     name of the language to be tested
```
