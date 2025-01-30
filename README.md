# Dynamic Partial Deadlock Detection and Recovery via Garbage Collection ASPLOS 2025 Artifact

## Introduction

Golf is an extension to the regular Go runtime that allows it to detect *partial deadlocks*, i.e., goroutines (threads managed by the Go runtime) which are definitely blocked for all possible executions at runtime.

This artifact supports the claims in the paper about the microbenchmark evaluation. It does not support the claims about the example service and large enterprise code base test suites, as those are proprietary.

In order the supported claims are:
* Demonstrating the efficacy of Golf in the detecting the partial deadlocks in the microbenchmark suite.
* Demonstrating that the runtime overhead is not signficantly larger than the baseline, and may even outperform the baseline GC for buggy examples.

## Requirements

### Hardware

Recommended specifications: 8 cores, 32 GB RAM, 20 GB disk space, x86-64 architectures are recommended. Apple silicon machines use the ARM architecture and may run the Docker benchmark with some caveats (described below).

### Software

Recommended OS is Linux. Docker must be installed, and the Docker daemon in a running state. Recommended Docker version is 26.1.4.

##### MacOS on Apple silicon machines

Running the benchmark on Apple silicon (M1/M2/M3) machines with MacOS, has additional prerequisites because of the underlying ARM architecture. Some dependencies are only available for x86-64, so the following steps are recommended before attempting to build and run the Docker image:

1. Install Colima:

```shell
brew install colima
```

2. Create a VM with the following specifications (numeric values may be adjusted):

```shell
colima start --cpu 4 --memory 16 --disk 50 --arch amd64
```

The build script automatically perform a best effort attempt at setting this up if run with `-apple-silicon` as the first argument, e.g.:
```
./run.sh -apple-silicon
```

**NOTE:** The overhead introduced when running the container through Colima is 10-15x. Assume this overhead for all following time estimations.

## Getting Started

### Building Docker image

Estimated time: 50 minutes

In order to run the artifact, run the following script at the root of this repository:
```
./run.sh
```

The script will build the Docker image, installing the Go runtime with the Golf extension, the baseline version of Go from which the Golf extension is derived, and the infrastructure necessary to run Golf on microbenchmarks.

### Inspecting results

Once `./run.sh` finishes, the script starts a session within the Docker container at `/usr/app/tester`, where the experimental results are found.

Due to non-determinism and flakiness, it is expected that the results will vary slightly between executions, and, implicitly, compared to those presented in the paper.

#### Microbenchmark coverage

The microbenchmarks include two suites: `cgo-examples` and `goker`, extracted from Saioc et al.[19], and Ting Yuan et al.[28], respectively.

To print the microbenchmark coverage report, run the following:
```
cat results
```

The file `results` begins with entries in a table with the following columns:
```
Repeat round, Target, Configuration, Deadlock mismatches, Exceptions, Comment
```

Entries are only added if there are mismatches between the expected and actual partial deadlocks detected by Golf, or if the microbenchmark raises an exception.

In order, each column represents the following:
1. `Repeat round` identifies each repetition of a microbenchmark execution, e.g., the value of 42, if this is the 42nd out of 100 executions of some benchmark B.
2. `Target` identifies the microbenchmark by file path.
3. `Configuration` lists the runtime configuration with which the microbenchmark was executed, e.g., number of concurrent processes, or GC flags, up to, and including the enabling of deadlock detection.
4. `Deadlock mismatches` lists all the mismatched entries between expected and unexpected deadlocks.
If an unexpected partial deadlock is reclaimed, the report will include an entry such as `Unexpected DL: main.NCastLeakFixed.func1`.
5. `Exceptions` lists any exceptions raised by the microbenchmark at runtime.
6. `Comments` lists any additional comments (it is mostly used to give more context to runtime exceptions).

The expected outcome is that `results` does **NOT** include any entries corresponding to unexpected deadlock reports (`Unexpected DL`).

Occasionally, `etcd/7443` will fail with a runtime exception of `send on closed channel`. This is an issue inherent to the microbenchmark, not one caused by Golf.

The file `results` continues with an aggregated report like the following:
```
Benchmark	1P	2P	4P	10P	Total
goker/etcd/7443/main.go/129:2	0	0	0	1	5.00%
goker/etcd/7443/main.go/216:4	0	0	0	1	5.00%
goker/etcd/7443/main.go/222:5	0	0	0	1	5.00%
goker/etcd/7443/main.go/226:4	0	0	0	1	5.00%
goker/etcd/7443/main.go/96:2	0	0	0	1	5.00%
goker/grpc/3017/main.go/106:5	0	5	5	5	75.00%
goker/grpc/3017/main.go/71:3	0	5	5	5	75.00%
goker/grpc/3017/main.go/97:4	0	5	5	5	75.00%
goker/hugo/3251/main.go/54:4	5	5	5	4	95.00%
goker/hugo/3251/main.go/62:7	5	5	5	4	95.00%
goker/kubernetes/62464/main.go/115:4	5	5	5	4	95.00%
goker/kubernetes/62464/main.go/117:4	5	5	5	4	95.00%
goker/moby/27782/main.go/213:2	5	2	3	4	70.00%
goker/moby/27782/main.go/65:2	5	2	3	4	70.00%
goker/moby/33781/main.go/39:5	5	5	4	4	90.00%
Remaining 111 go instruction (67 benchmarks)					100.00%
Aggregated	93.65%	95.08%	95.24%	95.71%	94.92%
```

This corresponds with the results presented in Table 1, answering **RQ1** ***(a)*** (the example above uses 5 microbenchmark execution repetitions), and showing the efficacy of Golf on the microbenchmarks. The expectation is that the results only include entries from `goker` (the paper omits the `goker` prefix in the table), and **NO** `cgo-examples` entries.

The aggregated detection rate value at cell **Aggregated/Total** is expected to be above `90%`, with a median value of `~94%` if the experiment is repeated.

#### Golf overhead

To print the full microbenchmark overhead report, run the following:
```
cat results-perf.csv
```

The resulting CSV contains the following columns:
```
Target,	Mark clock OFF (μs),	Mark clock ON (μs),	CPU utilization OFF (%),	CPU utilization ON (%)
```

Each are described as follows:
* `Target` is an individual microbenchmark execution. It includes the name of the benchmark and the runtime configuration.
* `Mark clock OFF (μs)` is the average duration in microseconds for the baseline GC to complete the marking phase.
* `Mark clock ON (μs)` is the average duration in microseconds for the Golf GC to complete the marking phase.
* `CPU utilization OFF (%)` is the proportional CPU utilization required to run the baseline GC for the microbenchmark.
* `CPU utilization ON (%)` is the proportional CPU utilization required to run the Golf GC for the microbenchmark.

The `OFF` and `ON` columns refer to two separate microbenchmark executions, which otherwise have the same configurations, but where one uses the baseline GC, while the other uses the Golf GC.

The microbenchmark performance overhead also produces a plot from the `Mark clock` columns, and dumps it in a `.tex` file at `results.tex`. To export it, follow these steps:
1. First exit the Docker container:
```
root:/usr/app/tester# exit
```
2. List the Docker container ids:
```
docker ps
```
3. Find the latest container where the `IMAGE` value is `golf`:
```
CONTAINER ID    IMAGE      COMMAND   CREATED          STATUS          PORTS     NAMES
<ID>            golf      ...
```
4. Copy the image from the docker container to the source system, using the container ID:
```
docker cp <ID>:/usr/app/tester/results.tex ./results.tex
```
5. Render `./results.tex` as a PDF with the TeX compiler of your choice.
6. Remove the container when done:
```
docker rm <ID>
```
