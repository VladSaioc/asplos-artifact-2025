# Dynamic Partial Deadlock Detection and Recovery via Garbage Collection ASPLOS 2025 Artifact

## Introduction

Golf is an extension to the baseline Go runtime that allows it to detect *partial deadlocks*, i.e., goroutines (threads managed by the Go runtime) which are definitely blocked for all possible executions at runtime.

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
colima start --cpu 10 --memory 16 --disk 50 --arch amd64
```

The build script automatically perform a best effort attempt at setting this up if run with `-apple-silicon` as the first argument, e.g.:
```
./run.sh -apple-silicon
```

**NOTE:** The overhead introduced when running the container through Colima is 10-15x. Assume this overhead for all following time estimations.

## Getting Started

### Building Docker image

Estimated time: <1 hour

In order to run the artifact, run the following script at the root of this repository:
```
./run.sh
```

The script will build the Docker image, installing the Go runtime with the Golf extension, the baseline version of Go from which the Golf extension is derived, and the infrastructure necessary to run Golf on microbenchmarks.

### Experimental workflow

The experiment sets up two versions of the Go runtime (a baseline version at `./baseline`, and one with the Golf extension `./golf`), and supplies them to a testing harness (in `./tester`) so they may be used on microbenchmarks.

#### Go runtimes

Each version of Go can be compiled by running:
```
bash ./baseline/src/make.bash # For baseline
bash ./golf/src/make.bash # For Golf
```
The Go compiler and runtime binaries are found at `./{baseline,golf}/bin/go`.

The Golf extensions to the Go runtime are found in the following files in `./golf/src`:
* **Major changes**:
  * `runtime/mgc.go` - live goroutine discovery (`gcDiscoverMoreStackRoots`) and deadlock detection (`detectPartialDeadlocks`).
  * `runtime/mgcmark.go` - goroutine address obfuscations (`allGsSnapshotSortedForGc`, `unblockingWaitReason`, `gcMask`, `gcUnmask`) and deadlocked goroutine shutdown (`gcGoexit`).
  * `runtime/sema.go` - deadlock detection and reclaim for semaphores.
  * `runtime/runtime2.go` - goroutine status extensions.
  * `sync/{runtime,waitgroup}.go` - (small but important) support WaitGroup deadlock detection.
* Minor changes at `src/runtime{preempt,proc,runtime1,trace2,trace2status,traceback}.go`

#### Testing harness

All the testing harness files are found in `./tester`, with `./main.go` as the entry source file. The testing harness can be built manually by running `go build .` (Go >1.21 recommended) from within `./tester`. The produced binary is `./golf-tester`.

Within `./tester`, examples of Go programs are found in `./test`. Example programs are split between *deadlocking* and *correct* examples, found in the `deadlock` and `correct` subdirectories, respectively. Each example program has its own subdirectory, which contains a standalone `main.go` file.

The core microbenchmark suites are found in the subdirectories `cgo-examples` and `goker`, extracted from [Saioc et al.](https://github.com/VladSaioc/common-goroutine-leak-patterns) and [Ting Yuan et al.](https://github.com/timmyyuan/gobench), respectively. Aside from the deadlocking examples, a subset of the microbenchmarks also feature correct examples (see above).

After running the testing harness, all resulting execution tracees will be included in subdirectories of `./tester` titled `results-<n>`, where `<n>` identifies the execution run index. Each directory mimics the subdirectory structure of `tests`.

The trace files have numes such as `gcddtrace-0-gcdetectdeadlocks-0-GOMAXPROCS-1`, which identify the runtime configuration used to run the example, e.g., `GOMAXPROCS-4` implies that the example was configured with 4 logical cores. Partial deadlocks are reported as `partial deadlock! ...`.

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

Entries are only added if unexpected partial deadlocks are detected by Golf, or if the microbenchmark raises an exception.

In order, each column represents the following:
1. `Repeat round` identifies each repetition of a microbenchmark execution, e.g., the value of 42, if this is the 42nd out of 100 executions of some benchmark B.
2. `Target` identifies the microbenchmark by file path.
3. `Configuration` lists the runtime configuration with which the microbenchmark was executed, e.g., number of concurrent processes, or GC flags, up to, and including the enabling of deadlock detection.
4. `Deadlock mismatches` lists all the unexpected deadlocks reported by Golf, with entries such as `Unexpected DL: main.NCastLeakFixed.func1`.
5. `Exceptions` lists any exceptions raised by the microbenchmark at runtime.
6. `Comments` lists any additional comments (it is mostly used to give more context to runtime exceptions).

It is expected that `results` does **NOT** include any unexpected deadlock reports.

Occasionally, `etcd/7443` will fail with a runtime exception of `send on closed channel`. This is an issue inherent to the microbenchmark, not one caused by Golf.

The file `results` continues with an aggregated report like the following:
```
Benchmark	1P	2P	4P	10P	Total
goker/etcd/7443:129	0	0	0	0	0.00%
goker/etcd/7443:216	0	0	0	0	0.00%
goker/etcd/7443:222	0	0	0	0	0.00%
goker/etcd/7443:226	0	0	0	0	0.00%
goker/etcd/7443:96	0	0	0	0	0.00%
goker/grpc/1460:83	5	5	5	4	95.00%
goker/grpc/1460:85	5	5	5	4	95.00%
goker/grpc/3017:106	0	5	5	5	75.00%
goker/grpc/3017:71	0	5	5	5	75.00%
goker/grpc/3017:97	0	5	5	5	75.00%
goker/hugo/3251:54	5	5	5	4	95.00%
goker/hugo/3251:62	5	5	5	4	95.00%
goker/moby/27782:213	5	1	3	5	70.00%
goker/moby/27782:65	5	1	3	5	70.00%
Remaining 107 go instruction (68 benchmarks)					100.00%
Aggregated	93.65%	94.76%	95.40%	95.40%	94.80%
```

This corresponds with the results presented in Table 1, answering **RQ1** ***(a)*** (the example above uses 5 microbenchmark execution repetitions), and showing the efficacy of Golf on the microbenchmarks. The expectation is that the results only include entries from `goker` (the paper omits the `goker/` prefix for the **Benchmark line** entries), and **NO** `cgo-examples` entries. The total number of entries (number of rows plus the `x` value in `Remaining x go instructions`) must be 121.

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

The expected result is that Golf GC performance hovers roughly in the same ballpark as baseline. Golf is typically outperformed by the baseline GC for examples without partial deadlocks, but without a signficant penalty. However, it may significantly outperform the baseline GC for examples with partial deadlocks. Values vary, depending on scheduling non-determinism, but should exceed 1 millisecond.

###  Experiment customization

Write your own program in its own subdirectory at `./tester/tests`. If the example is expected to produce deadlocks, include it in the `deadlock` subdirectory. Otherwise include it in the `correct` subdirectory.

Each example is identified by its own directory. Inside it, create a `main.go` file that roughly follows this template:
```
package main

import (
  "runtime"
  "time"
  // Other libraries
)

func init() {
  fmt.Println("Starting run...")
}

// Custom functions go here
// ...

func main() {
  defer func() {
    // Puts the program to sleep for some time.
    // The recommended duration depends on the program.
    time.Sleep(...)
    // Forces a GC cycle
    runtime.GC()
  }()

  // Main program execution goes here
  ...
}
```

You can then navigate to your directory and invoke the Golf runtime directly:
```
GODEBUG=gctrace=1,gcdetectdeadlocks=1 <artifact>/golf/bin/go main.go
```

If your example contains any partial deadlocks, they will be reported similarly to the following:
```
partial deadlock! goroutine 535: <function name> Stack size: <N> bytes
runtime.gopark(...)
	.../golf/src/runtime/proc.go:402
runtime.chansend(...)
	.../golf/src/runtime/chan.go:259
runtime.chansend1(0x0?, 0x0?)
	.../golf/src/runtime/chan.go:145
...
```

In order to validate the example with the testing harness, you must annotate the program with `// deadlocks: e` comments at key points, where `e` is either a Go integer constant (e.g., `// deadlocks: 10` signals that precisely 10 partial deadlocks are expected), or the inequality `x > 0` signalling that at least one deadlock is expected (the value of `x` is instantiated as the number of reports produced at runtime).

The annotation identifies individual goroutines, but its placement depends on the signature of the goroutine function. For functions without formal parameters, place the annotation inside the function body. Examples:
```
// For anonymous functions:

go func() {
  // deadlocks: x > 0
  ...
}()


// For named functions:
func foo() {
  // deadlocks: x > 0
  ...
}
...
go foo() // Do not place the annotation at the `go` instruction in this case.
```


For functions with formal parameters, place the annotation above the `go` instruction. Examples:
```
// deadlocks: x > 0
go func(x int){
  ...
}(10)

// deadlocks: x > 0
go willDeadlock(x, y, z)
// deadlocks: x > 0
go obj.deadlockingMethod()
```

The testing harness can be given the following input flags:
* `-parallelism` (integer) - Dictate how many virtual cores the harness may use.
* `-baseline` (file path) - Path to a baseline version of the Go runtime binary. Only needed when measuring performance.
* `-golf` (file path) - Path to Go (with Golf) runtime binary
* `-match` (regular expression, in quotes) - Only include a subset of the examples in `tests` with a path that matches the regular expression.
* `-dontmatch`  (regular expression, in quotes) - Ignore examples whenever their path matches this regular expression.
* `-repeats ` (integer) - How many times to repeat the entire microbenchmark execution in the same run.
* `-report` (file name) - Generates a report of the run results at the given file path.
* `-perf` - When provided, runs Golf in performance measurement mode, to compare against the baseline. The resulting reports are generated at `<-report>-perf.csv` (CSV file) and `<-report>-perf.tex` (TeX plot).

To target only your own examples with the testing harness, supply `-match` with a regular expression that matches the sub-paths to your examples in `tests`, e.g., if your example is `tests/deadlock/foo/bar`, any of  `-match foo`, `-match bar`, or `-match foo/bar` work.
