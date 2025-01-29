package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	numberOfRepeats      int
	goCompiler           = "go"
	baselineCompiler     = "golf"
	parallelism          = runtime.GOMAXPROCS(0)
	perf                 = false
	testFiles            = "tests"
	matchExamplesStr     = ""
	dontMatchExamplesStr = ""
	reportDest           = ""
)

func RunBenchmark() {
	matchExamples := regexp.MustCompile(matchExamplesStr)
	dontMatchExamples := regexp.MustCompile(dontMatchExamplesStr)
	start := time.Now()
	fullReport := Report{
		ticket: make(chan struct{}, parallelism),
	}

	compiledTargets, mu := make(map[string]struct{}), &sync.Mutex{}

	// Get every configuration
	for i := 1; i <= numberOfRepeats; i++ {
		fullReport.Add(1)
		go func(i int) {
			defer fullReport.Done()
			wg := &sync.WaitGroup{}
			RESULT := fmt.Sprintf("results-%d", i)
			// Clean up previous results.
			os.RemoveAll(RESULT)
			fmt.Println("Running round", i, "of", numberOfRepeats)

			for c := range EmitConfigurations() {
				wg.Add(1)
				go func(c Config) {
					defer wg.Done()

					cwg := &sync.WaitGroup{}
					filepath.WalkDir(testFiles, func(p string, d fs.DirEntry, err error) error {
						// If an error is encountered, bail early.
						if err != nil {
							return nil
						}
						// Ignore directories.
						if d == nil || d.IsDir() {
							return nil
						}

						// Ignore non-"main.go" files, or examples that match the "don't match" regular expression
						if dontMatchExamplesStr != "" && dontMatchExamples.MatchString(p) || path.Base(p) != "main.go" {
							return nil
						}

						// Ignore files that don't match the "match" regular expression.
						// If the "match" regular expression is empty, match everything.
						if !matchExamples.MatchString(p) {
							return nil
						}

						// Run all tests concurrently.
						cwg.Add(1)
						go func(p string) {
							defer func() {
								<-fullReport.ticket
								cwg.Done()
							}()
							fullReport.ticket <- struct{}{}
							// Create result directory.
							traceFile := path.Join(path.Dir(path.Join(RESULT, strings.TrimPrefix(p, testFiles))), c.Name())
							traceDir := path.Dir(traceFile)

							report := TargetReport{
								Config:    c,
								TraceFile: traceFile,
								Repeat:    i,
							}

							// Create all needed parent directories if they don't exist.
							if err := os.MkdirAll(traceDir, os.ModePerm); err != nil {
								report.Exception = errors.New("Failed to create parent directories: " + traceFile + ", Configuration:" + err.Error())
								fullReport.Append(report)
								return
							}

							// Construct a buffer into which to dump the resulting trace.
							fs, err := os.Create(traceFile + ".tmp")
							if err != nil {
								report.Exception = errors.New("Failed to create result file: " + traceFile + ": " + err.Error())
								fullReport.Append(report)
								return
							}

							report.ExpectedDeadlocks, err = getDeadlockExpectations(path.Dir(p), c)
							// Failure to extract expected deadlock annotations should be fatal.
							if err != nil {
								log.Fatal("Failed to get expected deadlocks:", err)
							}

							done := make(chan struct{})
							gocmd := goCompiler
							if perf && !c.HasDeadlockDetection() {
								gocmd = baselineCompiler
							}
							compileGo := exec.Command(gocmd, "build", "main.go")
							compileGo.Dir = path.Dir(p)
							compileGo.Env = append(os.Environ(), "GO_GCFLAGS=-race")
							compileGo.Stderr = fs

							runGo := exec.Command("./main")
							runGo.Dir = path.Dir(p)
							runGo.Stdout = fs
							runGo.Stderr = fs
							runGo.Env = append(append(os.Environ(), c.Flags()...), "GO_GCFLAGS=-race", "GOTRACEBACK=system")
							go func() {
								defer func() { done <- struct{}{} }()
								mu.Lock()
								if _, ok := compiledTargets[path.Dir(p)]; !ok {
									// Clean out old binary.
									os.RemoveAll(path.Dir(p) + "/main")
									if err := compileGo.Run(); err != nil {
										mu.Unlock()
										fmt.Println(path.Dir(p), "["+c.String()+"] Compile:", err)
										report.Exception = errors.New("compilation failure")
										return
									}
									compiledTargets[path.Dir(p)] = struct{}{}
								}
								mu.Unlock()
								if err := runGo.Run(); err != nil {
									report.Exception = errors.New("runtime failure")
									fmt.Println(c.String()+" Run:", traceFile, err)
								}
							}()

							select {
							case <-time.After(200 * time.Second):
								report.Exception = errors.New("go runtime timed out")
								report.ExpectedDeadlocks.Target = path.Dir(p)
								fullReport.Append(report)
								if compileGo != nil && compileGo.Process != nil && compileGo.Process.Pid > 0 {
									compileGo.Process.Signal(os.Interrupt)
								}
								if runGo != nil && runGo.Process != nil && runGo.Process.Pid > 0 {
									runGo.Process.Signal(syscall.SIGQUIT)
								}
								return
							case <-done:
							}
							fs.Close()

							fs, _ = os.Open(traceFile + ".tmp")
							report.RawTrace = RemoveGoGCTrace(fs)
							fs.Close()

							os.Remove(traceFile + ".tmp")

							report.Trace, err = ExtractTrace(report.RawTrace)
							report.Exception = errors.Join(report.Exception, err)

							if err := report.EmitToFile(); err != nil {
								log.Fatal("Failed to write trace to file:", err)
							}

							// If deadlock detection is disabled, we should not have any deadlock reports.
							// Otherwise, we are dealing with a serious implementation bug.
							if !c.HasDeadlockDetection() {
								if len(report.Trace.Deadlocks) > 0 {
									log.Fatal("Target: ", report.Target, " Found deadlocks in trace when deadlock detection is disabled!")
								}
								fullReport.Append(report)
								return
							}

							if len(report.Deadlocks) == 0 {
								if report.IsDeadlock() {
									report.Exception = errors.Join(report.Exception, errors.New("missing deadlock annotations in deadlock example"))
									fullReport.Append(report)
								}
								return
							}

							fullReport.Append(report)
						}(p)
						return nil
					})

					cwg.Wait()
					fmt.Println("Done with configuration [", i, "] :", c.String())
				}(c)
			}
			wg.Wait()
		}(i)
	}
	fullReport.Wait()
	fmt.Println("Done with all configurations!")
	fmt.Println("Whole benchmark took:", time.Since(start))
	if reportDest == "" {
		fmt.Println(fullReport.String())

		fmt.Println()
		fmt.Println("Performance results:")
		fmt.Println(fullReport.OverheadMeasurements())
		return
	}

	if !perf {
		content := fullReport.String()
		content += "\n" + fullReport.DirAggregates("deadlock/cgo-examples")
		content += "\n" + fullReport.DirAggregates("deadlock/gobench/goker/blocking")
		if err := os.WriteFile(reportDest, []byte(content), os.ModePerm); err != nil {
			log.Fatal("Failed to write report:", err)
		}
	} else {
		content := fullReport.OverheadMeasurements()
		if err := os.WriteFile(reportDest+"-perf.csv", []byte(content), os.ModePerm); err != nil {
			log.Fatal("Failed to write report:", err)
		}
	}
}

func makeFlags() {
	flag.BoolVar(&perf, "perf", false, "Run performance tests.")
	flag.IntVar(&parallelism, "parallelism", runtime.GOMAXPROCS(0), "Number of parallel tests to run.")
	flag.StringVar(&baselineCompiler, "baseline", "go", "Path to executable of baseline Go compiler/runtime. Defaults to `go`.")
	flag.StringVar(&goCompiler, "go", "go", "Path to executable of Go compiler/runtime. Defaults to system `go`.")
	flag.StringVar(&matchExamplesStr, "match", "", "Only run tests that match the given regular expression.")
	flag.StringVar(&dontMatchExamplesStr, "dontmatch", "", "Don't run tests that match the given regular expression.")
	flag.StringVar(&testFiles, "tests", testFiles, "Direct the tester to a directory of benchmarks.")
	flag.IntVar(&numberOfRepeats, "repeats", 1, "Number of times to repeat each configuration test.")
	flag.StringVar(&reportDest, "report", "", "Destination file for final report.")

	flag.Parse()

	if goCompiler != "" && goCompiler != "go" {
		if goCompilerAbs, err := filepath.Abs(goCompiler); err != nil {
			log.Fatal("Failed to get absolute path of Go compiler:", err)
		} else {
			goCompiler = goCompilerAbs
		}
	}

	if perf {
		// Only run on one core for performance tests.
		defaultvalues[2] = []configvalue{maxProcs1}
	}
}

func main() {
	makeFlags()

	RunBenchmark()
}
