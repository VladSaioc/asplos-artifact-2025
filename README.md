# Dynamic Partial Deadlock Detection and Recovery via Garbage Collection (Paper 597) ASPLOS 2025 Artifact

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
The following is an example of an expected deadlock that was not reported: ``[Expected: x > 0; Actual: 0] at n-cast/main.go:24:4 (main.NCastLeak.func1)``. The goroutine is syntactically created at `n-cast/main.go:24:4`, and at runtime, its name is `main.NCastLeak.func1`.
The expression `x > 0` states that at least one partial deadlock was expected at this goroutine for the microbenchmark named `n-cast`, but none were detected (hence: `Actual: 0`).
The `Expected` expression is extracted from a code comment in the microbenchmark source file.
If an unexpected partial deadlock is reclaimed, the report will include an entry such as `Unexpected DL: main.NCastLeakFixed.func1`.
5. `Exceptions` lists any exceptions raised by the microbenchmark at runtime.
6. `Comments` lists any additional comments (it is mostly used to give more context to runtime exceptions).

In `results`, most entries will correspond with expected deadlocks which are not reported by Golf. These mismatches are expected, and correspond with the flakiness described in Section 6.2, ***RQ1 (a)***. This is especially true for all entries explicitly listed in Table 1.

The expected outcome is that `results` does **NOT** include any entries corresponding to unexpected deadlock reports (`Unexpected DL`).

Occasionally, `deadlock/gobench/goker/blocking/etcd/7443` will fail with a runtime exception of `send on closed channel`. This is a runtime exception inherent to the microbenchmark, not one caused by Golf.

The file `results` concludes with an aggregate report similar to the following:
```
Correct guesses: 4042/4180 (96.70%)
Correct deadlocks: 2262/2400 (94.25%)
Correct not deadlocks: 1780/1780
Incorrect guesses: 138 (3.30%)

Directory: deadlock/cgo-examples
Correct guesses: 220/220 (100.00%)
Correct deadlocks: 140 (100.00%)

Directory: deadlock/gobench/goker/blocking
Correct guesses: 2162/2300 (94.00%)
Correct deadlocks: 2122 (93.89%)
```

`Correct guesses` signals how many reports are correctly guessed, when compared with the expected value (the left- and right-hand sides of `/`, rspectively). The values change depending on the stated number of microbenchmark execution repetitions, e.g., a suite execution with `100` expected guesses, totals to `5000` expected guess if set to repeat `50` times.

`Correct deadlocks` counts how many expected deadlocks are correctly reported, followed by a percentage. The percentage of this value roughly corresponds with the that in Table 1, at cell **Aggregated (%)/Total**.

`Correct not deadlocks` counts how many syntactical goroutines were expected to not a cause a deadlock, and, therefore, not lead to any Golf reports.

`Incorrect guesses` is the difference between `Correct guesses` and the total number of expected guesses.

Below the total report, are the aggregated efficacy metrics for the two microbenchmark suites described in the paper. The entries at `deadlocks/cgo-examples` and `deadlocks/gobench/goker/blocking` are related to the microbenchmarks extracted from Saioc et al.[19], and Ting Yuan et al.[28], respectively.
It is expected that the values for `Correct guesses` are at `100%`, consistently, for `deadlocks/cgo-examples`, and `93-95%` for `deadlocks/gobench/goker/blocking`.

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
