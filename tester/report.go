package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Report is a struct that holds a comprehensive report of the analysis.
type Report struct {
	sync.WaitGroup
	sync.Mutex
	ticket  chan struct{}
	Results map[string]*TargetReport
}

func (r *Report) Append(report TargetReport) {
	defer r.Unlock()
	r.Lock()
	if r.Results == nil {
		r.Results = make(map[string]*TargetReport)
	}
	r.Results[report.TraceFile] = &report
}

// TargetReport is a struct that holds a report of a single target execution.
type TargetReport struct {
	Config
	Name      string
	TraceFile string
	Exception error
	Repeat    int
	ExpectedDeadlocks
	Trace    Trace
	RawTrace []byte
	Diff     *DeadlockDifferential
}

// GetDeadlockToggleReport retrieves the name of the report for the
// target with the deadlock detection toggled:
//
//	If the report is "off", it produces the target report name "collect"
//	If the report is "collect/monitor", it produces the target report name "off"
func (r *TargetReport) GetDeadlockToggleReport() string {
	return strings.ReplaceAll(r.TraceFile, r.Config.Name(), r.Config.WithToggledDeadlockDetection().Name())
}

// IsBuggy returns true if the target directory path contains `buggy`.
func (r *TargetReport) IsBuggy() bool {
	return strings.Contains(r.ExpectedDeadlocks.Target, string(os.PathSeparator)+"buggy"+string(os.PathSeparator))
}

// IsBuggy returns true if the target directory path contains `deadlock`.
func (r *TargetReport) IsDeadlock() bool {
	return strings.Contains(r.ExpectedDeadlocks.Target, string(os.PathSeparator)+"deadlock"+string(os.PathSeparator))
}

// IsCorrect returns true if the target directory path contains `correct`.
func (r *TargetReport) IsCorrect() bool {
	return strings.Contains(r.ExpectedDeadlocks.Target, string(os.PathSeparator)+"correct"+string(os.PathSeparator))
}

// Emit content of target execution trace to trace file.
func (r *TargetReport) EmitToFile() error {
	content := fmt.Sprintf("Ran %s with configuration:\n%s\n\n", r.ExpectedDeadlocks.Target, r.Config) +
		string(r.RawTrace)

	return os.WriteFile(r.TraceFile, []byte(content), os.ModePerm)
}

func (r *TargetReport) CompareWithTrace() {
	diff := r.ExpectedDeadlocks.CompareWithTrace(r.Trace)
	r.Diff = &diff
}

func (r *TargetReport) String() string {
	const (
		REPEAT = iota
		TARGET
		CONFIG
		DEADLOCKS
		EXCEPTIONS
		COMMENT
	)

	content := make([][]string, 0, 2)
	content = append(content, []string{
		REPEAT:     strconv.Itoa(r.Repeat),
		TARGET:     r.Target,
		CONFIG:     r.Config.Name(),
		DEADLOCKS:  "No mismatches",
		EXCEPTIONS: "-",
		COMMENT:    "",
	})

	annotationsExpectDeadlock := r.DeadlockShouldBeFound()

	if r.IsCorrect() && annotationsExpectDeadlock {
		content[0][COMMENT] = "Annotations expected deadlock in correct example; "
	}
	if r.IsCorrect() && len(r.Trace.Deadlocks) > 0 {
		content[0][COMMENT] += "Deadlock found in correct example trace; "
	}
	if r.IsDeadlock() && !annotationsExpectDeadlock {
		content[0][COMMENT] += "Missing deadlock annotation in deadlock example; "
	}
	if content[0][COMMENT] == "" {
		content[0][COMMENT] = "-"
	}
	if r.Exception != nil {
		content[0][EXCEPTIONS] = "[" + r.Exception.Error() + "] "
	}

	if msg, ok := r.TraceHasExceptions(); ok {
		content[0][EXCEPTIONS] += msg
	}

	r.CompareWithTrace()
	if len(r.Diff.Mismatches) > 0 {
		slices.SortFunc(r.Diff.Mismatches, func(i, j DeadlockMismatch) int {
			return strings.Compare(i.String(), j.String())
		})
		mismatches := make([]DeadlockMismatch, 0, len(r.Diff.Mismatches))
		for _, mismatch := range r.Diff.Mismatches {
			if mismatch.Unexpected {
				mismatches = append(mismatches, mismatch)
			}
		}

		if len(mismatches) > 0 {
			content[0][DEADLOCKS] = mismatches[0].String()
			for i, mismatch := range mismatches[1:] {
				if !mismatch.Unexpected {
					continue
				}
				content = append(content, make([]string, COMMENT+1))
				content[i+1][DEADLOCKS] = mismatch.String()
				content[i+1][REPEAT] = strconv.Itoa(r.Repeat)
				content[i+1][TARGET] = r.Target
				content[i+1][CONFIG] = r.Config.Name()
				if r.Exception != nil {
					content[i+1][EXCEPTIONS] = "[" + r.Exception.Error() + "] "
				}
				if msg, ok := r.TraceHasExceptions(); ok {
					content[i+1][COMMENT] += msg
				}
			}
		}
	}

	lines := []string{}
	for _, row := range content {
		if row[DEADLOCKS] == "No mismatches" && row[EXCEPTIONS] == "-" {
			continue
		}
		lines = append(lines, strings.Join(row, ",\t"))
	}
	return strings.Join(lines, "@:@")
}

func (r *Report) DirAggregates(p string) string {
	var correctDeadlocks, expectedDeadlocks, correctGuesses, incorrectGuesses int
	for _, report := range r.Results {
		if strings.Contains(report.Target, p) && report.HasDeadlockDetection() {
			correctGuesses += report.Diff.CorrectDeadlockFound + report.Diff.CorrectNoDeadlockFound
			correctDeadlocks += report.Diff.CorrectDeadlockFound
			incorrectGuesses += len(report.Diff.Mismatches)
			for _, dl := range report.Deadlocks {
				if dl.DeadlockShouldBeFound() {
					expectedDeadlocks++
				}
			}
		}
	}

	content := fmt.Sprintf("Directory: %s\n", p)
	content += fmt.Sprintf("Correct guesses: %d/%d (%.2f%%)\n", correctGuesses, correctGuesses+incorrectGuesses, float64(correctGuesses)/float64(correctGuesses+incorrectGuesses)*100)
	content += fmt.Sprintf("Correct deadlocks: %d (%.2f%%)\n", correctDeadlocks, float64(correctDeadlocks)/float64(expectedDeadlocks)*100)

	return content
}

func (r *Report) String() string {
	content := strings.Join([]string{
		"Repeat round",
		"Target",
		"Configuration",
		"Deadlock mismatches",
		"Exceptions",
		"Comment",
	}, ",\t")

	reportSlice := make([]*TargetReport, 0, len(r.Results))
	for _, report := range r.Results {
		if report.HasDeadlockDetection() {
			reportSlice = append(reportSlice, report)
		}
	}

	slices.SortFunc(reportSlice, func(r1, r2 *TargetReport) int {
		return strings.Compare(r1.TraceFile, r2.TraceFile)
	})

	reportStrings := make([]string, 0, len(r.Results))
	for i, report := range reportSlice {
		if str := report.String(); str != "" {
			reportStrings = append(reportStrings, str)
		}
		reportSlice[i] = report
	}

	slices.Sort(reportStrings)
	for i, report := range reportStrings {
		reportStrings[i] = strings.Replace(report, "@:@", ",\n", -1)
	}

	content += "\n" + strings.Join(reportStrings, "\n")

	var (
		correctDeadlocks, expectedDeadlocks int
		correctGuesses                      int
		incorrectGuesses                    int
	)
	for _, report := range reportSlice {
		correctGuesses += report.Diff.CorrectDeadlockFound + report.Diff.CorrectNoDeadlockFound
		correctDeadlocks += report.Diff.CorrectDeadlockFound
		incorrectGuesses += len(report.Diff.Mismatches)
		for _, dl := range report.Deadlocks {
			if dl.DeadlockShouldBeFound() {
				expectedDeadlocks++
			}
		}
	}

	// totalGuesses := correctGuesses + incorrectGuesses
	// content += "\n\n" + fmt.Sprintf("Correct guesses: %d/%d (%.2f%%)\n", correctGuesses, totalGuesses, float64(correctGuesses)/float64(totalGuesses)*100) +
	// 	fmt.Sprintf("Correct deadlocks: %d/%d (%.2f%%)\n", correctDeadlocks, expectedDeadlocks, float64(correctDeadlocks)/float64(expectedDeadlocks)*100) +
	// 	fmt.Sprintf("Correct not deadlocks: %d/%d\n", correctGuesses-correctDeadlocks, totalGuesses-expectedDeadlocks) +
	// 	fmt.Sprintf("Incorrect guesses: %d (%.2f%%)\n", incorrectGuesses, float64(incorrectGuesses)/float64(totalGuesses)*100)

	return content
}

// Tabulated produces a tabulated report of the analysis.
func (r *Report) Tabulated() string {
	type tabEntry struct {
		target  string
		goro    string
		pconfig []int
		total   float64
	}

	procconfigs := make(map[int]int)
	for i, v := range defaultvalues[PROCS] {
		procconfigs[int(v.(maxProcs))] = i
	}

	entries := make(map[string]tabEntry)
	for _, report := range r.Results {
		if !report.HasDeadlockDetection() {
			continue
		}

	DEADLOCKS:
		for _, dl := range report.Deadlocks {
			if !dl.DeadlockShouldBeFound() {
				// Skip `deadlocks: {0, false}` annotations.
				continue
			}
			pos := fmt.Sprintf("%s:%d", report.Target, dl.Line)
			entry, ok := entries[pos]
			if !ok {
				entry = tabEntry{
					target:  report.Target,
					goro:    pos,
					pconfig: make([]int, len(procconfigs)),
				}
			}

			for _, mismatch := range report.Diff.Mismatches {
				if dl.Line == mismatch.Line {
					entries[pos] = entry
					continue DEADLOCKS
				}
			}

			entry.pconfig[procconfigs[report.Config.Ps()]]++
			entries[pos] = entry
		}
	}

	entriesSlice := make([]tabEntry, 0, len(entries))
	correctTargets := make(map[string]struct{})
	aggregated := tabEntry{
		pconfig: make([]int, len(procconfigs)),
	}

	for _, entry := range entries {
		var total float64
		for i, p := range entry.pconfig {
			total += float64(p)
			aggregated.pconfig[i] += p
		}
		total = total / (float64(len(procconfigs) * numberOfRepeats)) * 100
		if total == 100 {
			correctTargets[entry.target] = struct{}{}
			continue
		}
		entry.total = total
		entriesSlice = append(entriesSlice, entry)
	}

	slices.SortFunc(entriesSlice, func(e1, e2 tabEntry) int {
		return strings.Compare(e1.goro, e2.goro)
	})

	content := make([]string, 1, len(entries))

	header := make([]string, len(procconfigs)+2)
	header[0] = "Benchmark"
	for p, i := range procconfigs {
		header[i+1] = strconv.Itoa(p) + "P"
	}
	header[len(header)-1] = "Total"
	content[0] = strings.Join(header, "\t")

	for _, entry := range entriesSlice {
		tabulated := make([]string, len(procconfigs)+2)
		prettyTarget := strings.TrimPrefix(entry.goro, "tests/deadlock/")
		prettyTarget = strings.TrimPrefix(prettyTarget, "gobench/")
		prettyTarget = strings.Replace(prettyTarget, "blocking/", "", 1)
		tabulated[0] = prettyTarget
		for i, p := range entry.pconfig {
			tabulated[i+1] = strconv.Itoa(p)
		}
		tabulated[len(tabulated)-1] = strconv.FormatFloat(entry.total, 'f', 2, 64) + "%"
		content = append(content, strings.Join(tabulated, "\t"))
	}

	remainingTabulated := make([]string, len(procconfigs)+2)
	remainingTabulated[0] = fmt.Sprintf("Remaining %d go instruction (%d benchmarks)", len(entries)-len(entriesSlice), len(correctTargets))
	remainingTabulated[len(remainingTabulated)-1] = strconv.FormatFloat(100, 'f', 2, 64) + "%"
	content = append(content, strings.Join(remainingTabulated, "\t"))

	aggregatedTabulated, aggregatedtotal := make([]string, len(procconfigs)+2), float64(0)
	aggregatedTabulated[0] = "Aggregated"
	for i, p := range aggregated.pconfig {
		aggregatedTabulated[i+1] = strconv.FormatFloat(float64(p)/float64(len(entries)*numberOfRepeats)*100, 'f', 2, 64) + "%"
		aggregatedtotal += float64(p)
	}
	aggregatedTabulated[len(aggregatedTabulated)-1] = strconv.FormatFloat(aggregatedtotal/float64(len(entries)*numberOfRepeats*len(procconfigs))*100, 'f', 2, 64) + "%"
	content = append(content, strings.Join(aggregatedTabulated, "\t"))

	return strings.Join(content, "\n")
}

// OverheadMeasurements produces a report comparing the performance
// of the GC with deadlock enabled and disabled at equivalent runtime configurations.
func (r *Report) OverheadMeasurements() string {
	reportSlice := make([]*TargetReport, 0, len(r.Results))
	for _, report := range r.Results {
		if report.Config.HasDeadlockDetection() {
			reportSlice = append(reportSlice, report)
		}
	}

	slices.SortFunc(reportSlice, func(r1, r2 *TargetReport) int {
		return strings.Compare(r1.TraceFile, r2.TraceFile)
	})

	const (
		TARGET = iota
		MARKCLOCKOFF
		MARKCLOCKON
		CPUTILOFF
		CPUTILON
		TERMCLOCK
		MARKCPU
		TERMCPU
		HEAP
		STACK
		GCYCLES
		UTILIZATION
		NUMGOS
	)

	content := make([][]string, 1, 2)
	content[0] = []string{
		TARGET:       "Target",
		MARKCLOCKOFF: "Mark clock OFF (μs)",
		MARKCLOCKON:  "Mark clock ON (μs)",
		CPUTILOFF:    "CPU utilization OFF (%)",
		CPUTILON:     "CPU utilization ON (%)",
	}

	for _, report := range reportSlice {
		dlOffReport, ok := r.Results[report.GetDeadlockToggleReport()]
		if !ok {
			log.Println("Did not find equivalent report with deadlock detection off for:", report.TraceFile)
			continue
		}

		onPerf := report.Trace.GetGCPerf()
		if onPerf == (GCPerf{}) {
			// Missing GC trace?
			log.Println("Missing GC trace for:", report.TraceFile)
			continue
		}
		offPerf := dlOffReport.Trace.GetGCPerf()
		perfDelta := GetPerfDelta(offPerf, onPerf)

		content = append(content, []string{
			TARGET:       report.TraceFile,
			MARKCLOCKOFF: strconv.FormatFloat(perfDelta.avgMarkClockOff, 'f', 2, 64),
			MARKCLOCKON:  strconv.FormatFloat(perfDelta.avgMarkClockOn, 'f', 2, 64),
			CPUTILOFF:    strconv.FormatFloat(perfDelta.avgUtilizationOff, 'f', 2, 64),
			CPUTILON:     strconv.FormatFloat(perfDelta.avgUtilizationOn, 'f', 2, 64),
		})
	}

	lines := make([]string, 0, len(content))
	for _, row := range content {
		lines = append(lines, strings.Join(row, ",\t"))
	}

	return strings.Join(lines, "\n")
}

// OverheadMeasurementsPlot produces a report comparing the performance
// of the GC with deadlock enabled and disabled at equivalent runtime configurations.
// The output is a .tex file that can be compiled to a PDF, and resembles the evaluation
// in the paper.
func (r *Report) OverheadMeasurementsPlot() string {
	reportSlice := make([]*TargetReport, 0, len(r.Results))
	for _, report := range r.Results {
		if report.Config.HasDeadlockDetection() {
			reportSlice = append(reportSlice, report)
		}
	}

	perfs := make([]GCPerf, 0, len(reportSlice))
	for _, report := range reportSlice {
		dlOffReport, ok := r.Results[report.GetDeadlockToggleReport()]
		if !ok {
			log.Println("Did not find equivalent report with deadlock detection off for:", report.TraceFile)
			continue
		}

		onPerf := report.Trace.GetGCPerf()
		if onPerf == (GCPerf{}) {
			// Missing GC trace?
			log.Println("Missing GC trace for:", report.TraceFile)
			continue
		}
		offPerf := dlOffReport.Trace.GetGCPerf()
		perfs = append(perfs, GetPerfDelta(offPerf, onPerf))
	}

	slices.SortFunc(perfs, func(p1, p2 GCPerf) int {
		return int(p1.avgMarkClockOff*100 - p2.avgMarkClockOff*100)
	})

	contentOn := make([]string, 0, len(perfs))
	contentOff := make([]string, 0, len(perfs))
	for i, perf := range perfs {
		contentOn = append(contentOn, fmt.Sprintf("(%f, %f)", float64(i)/5, math.Log10(perf.avgMarkClockOff)))
		contentOff = append(contentOff, fmt.Sprintf("(%f, %f)", float64(i)/5, math.Log10(perf.avgMarkClockOn)))
	}

	return `
\documentclass{standalone}
\usepackage{tikz}
\usepackage{pgfplots}
\pgfplotsset{compat=1.17}

\begin{document}


\begin{tikzpicture}
	\begin{axis}[
		width=\columnwidth, height=10cm,
		axis x line=middle,
		axis y line=middle,
		xmin=0, xmax=` + strconv.FormatFloat(float64(len(perfs))/5, 'f', 2, 64) + `,
		ymin=2, ymax=6,
		ytick={2.01, 3, 4, 5, 6},
		yticklabels={$10^2$, $10^3$, $10^4$, $10^5$, $10^6$},
		xtick={},
		xticklabels={},
		ylabel={$\mu s$},
		xlabel style={below left},
		xlabel={Benchmarks},
		ylabel style={above left},
		grid=both,
		legend style={
			at={(0.25,-0.1)},
			anchor=north,
			draw=none,
			fill=none,
			legend columns=2,
		},
		legend cell align={left}
		]
		\addplot[smooth,dashed] plot coordinates {
	` + strings.Join(contentOff, " ") + `
	};
	\addlegendentry{Baseline\ \ \ }
	\addplot[smooth] plot coordinates {
	` + strings.Join(contentOn, " ") + `
	};
	\addlegendentry{Golf}
\end{axis}
\end{tikzpicture}

\end{document}
`
}
