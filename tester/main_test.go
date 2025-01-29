package main

import (
	// "os"
	"testing"
)

func TestRunBenchmarks(t *testing.T) {
	matchExamplesStr = "(cgo-examples|simple|corner-cases)"
	numberOfRepeats = 1

	RunBenchmark()

	// os.RemoveAll("results-1")
}

func TestMakeFlag(t *testing.T) {
	makeFlags()
}
