package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	gcTraceRe = regexp.MustCompile(`^gc \d+ @.*%: .*, \d+ P( \(forced\))?$`)
	// clockRe matches GC message components that GC clock times.
	clockRe = regexp.MustCompile(`[\d+]+ .s clock`)
	// cpuRe matches GC message components that GC CPU times.
	cpuRe = regexp.MustCompile(`[\d+/]+ .s cpu`)
	// heapRe matches GC message components that GC heap sizes.
	heapRe = regexp.MustCompile(`\d+->\d+->\d+ \wB`)
	// stackRe matches GC message components that GC stack sizes.
	stackRe = regexp.MustCompile(`\d+ \wB stacks`)
	// procRe matches GC message components that GC processor counts.
	procRe = regexp.MustCompile(`\d+ P`)
)

// gcTrace represents a single garbage collection trace message.
//
// GC trace messages are emitted by the garbage collector at the end of each
// garbage collection cycle, if the GODEBUG field contains gctrace=1.
// The messages are formatted as follows:
//
//	gc # ... #%: #+#+# μs clock, #+#/#/#+# μs cpu, #->#-># KB, ..., # KB stacks, ... # P
type gcTrace struct {
	// cycle is the garbage collection cycle number (gc #).
	cycle int
	// cpuUtilization is the CPU utilization percentage (#%).
	cpuUtilization float64
	// timeUnit is the unit used for wall-clock and CPU time.
	timeUnit string
	// clockMarkTime is the time spent in the clock (#+#+# μs clock).
	clockMarkTime float64
	// clockTermTime is the time spent performing mark termination (#+#+# μs clock).
	clockTermTime float64
	// cpuMarkTime is the aggregate time spent on CPUs for the marking phase.
	cpuMarkTime float64
	// cpuTermTime is the aggregate time spent on CPUs for mark termination.
	cpuTermTime float64
	// memUnit is the unit used for memory size (heap, stacks).
	memUnit string
	// liveHeap is the size of heap.
	liveHeap int
	// stackSize is the size of stacks.
	stackSize int
	// Number of logical processors.
	processors int
}

// ParseGCTrace takes a GC trace message raw string and parses it into a gcTrace.
// If the message is ill-formatted, an error is returned.
func ParseGCTrace(gcMsg string) (gc gcTrace, err error) {
	var parts []string

	// Split at colon:
	//			  V
	//	gc # @_ #%: ....
	//	   A    A
	//	   |    '- Fractional CPU utilization
	//	   '--- GC cycle
	gcParts := strings.Split(gcMsg, "%: ") // Always has length at least 2
	if parts = strings.Split(gcParts[0], " "); len(parts) < 4 {
		err = fmt.Errorf("GC cycle `%s` is ill-formatted left of the colon",
			gcMsg)
		return
	}
	if gc.cycle, err = strconv.Atoi(parts[1]); err != nil {
		return
	}

	if gc.cpuUtilization, err = strconv.ParseFloat(parts[3], 64); err != nil {
		return
	}

	// Handle right-hand side of colon:
	//	_+#+# μs clock, _+#/#/#+# μs cpu, _->_-># KB, _, # KB stacks, _, # P
	//	              0                 1           2  3            4  5
	for _, part := range strings.Split(gcParts[1], ", ") {
		switch {
		case clockRe.MatchString(part):
			// Extract clock times: _+#+# μs clock
			if parts = strings.Split(part, " "); len(parts) < 3 {
				err = fmt.Errorf("GC cycle `%s` is ill-formatted for clock",
					gcMsg)
				return
			}
			gc.timeUnit = parts[1]
			if parts = strings.Split(parts[0], "+"); len(parts) < 3 {
				err = fmt.Errorf("GC cycle `%s` is ill-formatted for clock GC phases",
					gcMsg)
				return
			}
			if gc.clockMarkTime, err = strconv.ParseFloat(parts[1], 64); err != nil {
				return
			}
			if gc.clockTermTime, err = strconv.ParseFloat(parts[2], 64); err != nil {
				return
			}
		case cpuRe.MatchString(part):
			// Extract CPU times: _+#/#/#+# μs cpu
			if parts = strings.Split(part, " "); len(parts) < 3 {
				err = fmt.Errorf("GC cycle `%s` is ill-formatted for CPU time",
					gcMsg)
				return
			}
			gc.timeUnit = parts[1]
			if parts = strings.Split(parts[0], "+"); len(parts) < 3 {
				err = fmt.Errorf("GC cycle `%s` is ill-formatted for CPU GC phases",
					gcMsg)
				return
			}
			for _, markPart := range strings.Split(parts[1], "/") {
				var markPartTime float64
				if markPartTime, err = strconv.ParseFloat(markPart, 64); err != nil {
					return
				}
				gc.cpuMarkTime += markPartTime
			}
			if gc.cpuTermTime, err = strconv.ParseFloat(parts[2], 64); err != nil {
				return
			}
		case heapRe.MatchString(part):
			// Extract heap sizes: _->_-># KB
			if parts = strings.Split(part, " "); len(parts) < 2 {
				err = fmt.Errorf("GC cycle `%s` is ill-formatted for heap sizes",
					gcMsg)
				return
			}
			if parts = strings.Split(parts[0], "->"); len(parts) < 3 {
				err = fmt.Errorf("GC cycle `%s` is ill-formatted for heap sizes",
					gcMsg)
				return
			}
			if gc.liveHeap, err = strconv.Atoi(parts[2]); err != nil {
				return
			}
		case stackRe.MatchString(part):
			// Extract stack sizes: # KB stacks
			if parts = strings.Split(part, " "); len(parts) < 2 {
				err = fmt.Errorf("GC cycle `%s` is ill-formatted for stack sizes",
					gcMsg)
				return
			}
			if gc.stackSize, err = strconv.Atoi(parts[0]); err != nil {
				return
			}
			gc.memUnit = parts[1]
		case procRe.MatchString(part):
			// Extract processor count: # P
			if parts = strings.Split(part, " "); len(parts) < 2 {
				err = fmt.Errorf("GC cycle `%s` is ill-formatted for processor count",
					gcMsg)
				return
			}
			if gc.processors, err = strconv.Atoi(parts[0]); err != nil {
				return
			}
		}
	}

	return
}
