package main

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
)

const (
	// This string demarcates when the runtime starts the user code
	// in the trace.
	//
	// This is useful when running the examples with GC trace enabled,
	// so that the trace can be cleaned up to remove the GC trace produced
	// by the Go compiler itself.
	startRun = "Starting run..."

	// This is the prefix to the final message, which contains the number of goroutines.
	finalGos = "Final goroutine count: "
)

// TraceDeadlock represents a deadlock that occurred in the trace,
// identified by function name.
type TraceDeadlock struct {
	FunctionName string
	Count        int
}

// Trace represents a collection of TraceDeadlocks that occured in the trace.
type Trace struct {
	Deadlocks     []TraceDeadlock
	GCMessages    []gcTrace
	NumGoroutines int
}

// GCPerf represents the performance metrics of the garbage collector.
type GCPerf struct {
	gcCycles          int
	avgUtilization    float64
	avgUtilizationOn  float64
	avgUtilizationOff float64
	avgMarkClock      float64
	avgMarkClockOn    float64
	avgMarkClockOff   float64
	finalHeapSize     int
	finalStackSize    int
	finalGoroutines   int
}

// ExtractTrace extracts the deadlocks from the given raw trace.
//
// The deadlocks are identified by the presence of the string with the format:
//
//	"partial deadlock! goroutine <goid>: <function name>"
//
// All function names are collected and added to the Trace.
// Errors resulting from ill-formed GC messages are aggregated.
func ExtractTrace(rawTrace []byte) (trace Trace, err error) {
	for _, line := range strings.Split(string(rawTrace), "\n") {
		switch {
		case strings.Contains(line, "partial deadlock!"):
			parts1 := strings.Split(line, "partial deadlock! ")
			if len(parts1) < 2 {
				continue
			}

			part1 := genericParam.ReplaceAllString(parts1[1], "")
			parts2 := strings.Split(part1, " ")
			// Will never give an out of bounds exception
			if len(parts2) > 2 {
				trace.Deadlocks = append(trace.Deadlocks, TraceDeadlock{FunctionName: parts2[2]})
			}
		case gcTraceRe.MatchString(line):
			if gc, gcErr := ParseGCTrace(line); gcErr == nil {
				trace.GCMessages = append(trace.GCMessages, gc)
			} else {
				err = errors.Join(err, gcErr)
			}
		case strings.HasPrefix(line, finalGos):
			trace.NumGoroutines, _ = strconv.Atoi(strings.TrimPrefix(line, finalGos))
		}
	}
	return
}

// DeadlocksAtFunction returns the number of deadlocks that occurred at the given function name.
func (t Trace) DeadlocksAtFunction(name string) int {
	count := 0
	for _, deadlock := range t.Deadlocks {
		if deadlock.FunctionName == name {
			count++
		}
	}
	return count
}

// RemoveGoGCTrace removes the Go compiler GC trace from the given trace
// by discarding all lines before the last line containing startRun.
func RemoveGoGCTrace(trace io.Reader) []byte {
	lines := []string{}
	for scanner := bufio.NewScanner(trace); scanner.Scan(); {
		line := scanner.Text()
		if strings.Contains(line, startRun) {
			lines = []string{}
		}
		lines = append(lines, line)
	}

	return []byte(strings.Join(lines, "\n"))
}

// TabulateGCPerf produces the GC performance metrics,
// as determined by the GC messages extracted from the trace.
func (t Trace) GetGCPerf() (perf GCPerf) {
	if t.GCMessages == nil {
		return
	}

	lastGC := t.GCMessages[len(t.GCMessages)-1]

	perf.gcCycles = lastGC.cycle
	perf.finalHeapSize = lastGC.liveHeap
	perf.finalStackSize = lastGC.stackSize
	perf.finalGoroutines = t.NumGoroutines

	// Compute the average CPU utilization
	avgCPU := float64(0)
	for _, gc := range t.GCMessages {
		avgCPU += gc.cpuUtilization
	}
	perf.avgUtilization = avgCPU / float64(len(t.GCMessages))

	// Compute average timings across GC cycles
	getAvg := func(get func(gcTrace) float64) float64 {
		var avg float64
		for _, gc := range t.GCMessages {
			avg += get(gc)
		}
		return avg / float64(len(t.GCMessages))
	}

	perf.avgMarkClock = getAvg(func(gc gcTrace) float64 { return gc.clockMarkTime })

	return
}

// GetPerfDelta computes the performance delta between two GCPerf instances.
func GetPerfDelta(off, on GCPerf) GCPerf {
	return GCPerf{
		avgUtilizationOn:  on.avgUtilization,
		avgUtilizationOff: off.avgUtilization,
		avgMarkClockOff:   off.avgMarkClock,
		avgMarkClockOn:    on.avgMarkClock,
		finalHeapSize:     off.finalHeapSize - on.finalHeapSize,
		finalStackSize:    off.finalStackSize - on.finalStackSize,
		finalGoroutines:   off.finalGoroutines - on.finalGoroutines,
	}
}
