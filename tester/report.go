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
		GCYCLES
		MARKCLOCKOFF
		MARKCLOCKON
		CPUTILOFF
		CPUTILON
		TERMCLOCK
		MARKCPU
		TERMCPU
		HEAP
		STACK
		UTILIZATION
		NUMGOS
	)

	content := make([][]string, 1, 2)
	content[0] = []string{
		TARGET:       "Target",
		GCYCLES:      "GC cycles",
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
			GCYCLES:      strconv.Itoa(perfDelta.gcCycles),
			MARKCLOCKOFF: strconv.FormatFloat(perfDelta.avgMarkCPUOff, 'f', 2, 64),
			MARKCLOCKON:  strconv.FormatFloat(perfDelta.avgMarkCPUOn, 'f', 2, 64),
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
		return int(p1.avgMarkCPUOff*100 - p2.avgMarkCPUOff*100)
	})

	contentOn := make([]string, 0, len(perfs))
	contentOff := make([]string, 0, len(perfs))
	for i, perf := range perfs {
		contentOn = append(contentOn, fmt.Sprintf("(%f, %f)", float64(i)/5, math.Log10(perf.avgMarkCPUOff)))
		contentOff = append(contentOff, fmt.Sprintf("(%f, %f)", float64(i)/5, math.Log10(perf.avgMarkCPUOn)))
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

// OverheadMeasurementsBoxplot produces a boxplot comparing the performance
// of the GC with deadlock enabled and disabled at equivalent runtime configurations.
// The output is a .tex file that can be compiled to a PDF, and resembles the evaluation
// in the paper.
func (r *Report) OverheadMeasurementsBoxplot() string {
	reportSlice := make([]*TargetReport, 0, len(r.Results))
	for _, report := range r.Results {
		if report.HasDeadlockDetection() {
			reportSlice = append(reportSlice, report)
		}
	}

	// Compute performance deltas
	perfsCorrect, perfsDeadlock := make([]GCPerf, 0, len(reportSlice)), make([]GCPerf, 0, len(reportSlice))
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
		switch {
		case report.IsCorrect():
			perfsCorrect = append(perfsCorrect, GetPerfDelta(offPerf, onPerf))
		case report.IsDeadlock():
			perfsDeadlock = append(perfsDeadlock, GetPerfDelta(offPerf, onPerf))
		}
	}

	slices.SortFunc(perfsCorrect, func(p1, p2 GCPerf) int {
		p1Norm := NormalizeSlowdown(p1.avgMarkCPUOff, p1.avgMarkCPUOn)
		p2Norm := NormalizeSlowdown(p2.avgMarkCPUOff, p2.avgMarkCPUOn)
		return int(p1Norm*100 - p2Norm*100)
	})

	slices.SortFunc(perfsDeadlock, func(p1, p2 GCPerf) int {
		p1Norm := NormalizeSlowdown(p1.avgMarkCPUOff, p1.avgMarkCPUOn)
		p2Norm := NormalizeSlowdown(p2.avgMarkCPUOff, p2.avgMarkCPUOn)
		return int(p1Norm*100 - p2Norm*100)
	})

	// Normalize slowdowns
	perfsCorrectDataLog, perfsDeadlockDataLog := make([]float64, 0, len(perfsCorrect)), make([]float64, 0, len(perfsDeadlock))
	for _, perf := range perfsCorrect {
		perfsCorrectDataLog = append(perfsCorrectDataLog, NormalizeSlowdown(perf.avgMarkCPUOff, perf.avgMarkCPUOn))
	}
	for _, perf := range perfsDeadlock {
		perfsDeadlockDataLog = append(perfsDeadlockDataLog, NormalizeSlowdown(perf.avgMarkCPUOff, perf.avgMarkCPUOn))
	}

	// Get procentual slowdowns
	perfsCorrectPct, perfsDeadlockPct := make([]float64, 0, len(perfsCorrect)), make([]float64, 0, len(perfsDeadlock))
	for _, perf := range perfsCorrect {
		perfsCorrectPct = append(perfsCorrectPct, perf.avgMarkCPUOn/perf.avgMarkCPUOff)
	}
	for _, perf := range perfsDeadlock {
		perfsDeadlockPct = append(perfsDeadlockPct, perf.avgMarkCPUOn/perf.avgMarkCPUOff)
	}

	maxDeadlockTime := slices.MaxFunc(perfsDeadlock, func(p1, p2 GCPerf) int {
		return int(p1.avgMarkCPUOn - p2.avgMarkCPUOn)
	})
	maxCorrectTime := slices.MaxFunc(perfsCorrect, func(p1, p2 GCPerf) int {
		return int(p1.avgMarkCPUOn - p2.avgMarkCPUOn)
	})

	// Box plots
	correctBoxPct := BoxPlotMetrics(perfsCorrectPct)
	deadlockBoxPct := BoxPlotMetrics(perfsDeadlockPct)

	correctBoxLog := BoxPlotMetrics(perfsCorrectDataLog)
	deadlockBoxLog := BoxPlotMetrics(perfsDeadlockDataLog)

	q0CorrectOutliers, q4CorrectOutliers := correctBoxLog.SmallOutliers, correctBoxLog.LargeOutliers
	q0DeadlockOutliers, q4DeadlockOutliers := deadlockBoxLog.SmallOutliers, deadlockBoxLog.LargeOutliers

	correctOutlierString := make([]string, 0, len(q0CorrectOutliers)+len(q4CorrectOutliers))
	for _, outlier := range q0CorrectOutliers {
		correctOutlierString = append(correctOutlierString, strconv.FormatFloat(outlier, 'f', -1, 64))
	}
	for _, outlier := range q4CorrectOutliers {
		correctOutlierString = append(correctOutlierString, strconv.FormatFloat(outlier, 'f', -1, 64))
	}

	deadlockOutlierString := make([]string, 0, len(q0DeadlockOutliers)+len(q4DeadlockOutliers))
	for _, outlier := range q0DeadlockOutliers {
		deadlockOutlierString = append(deadlockOutlierString, strconv.FormatFloat(outlier, 'f', -1, 64))
	}
	for _, outlier := range q4DeadlockOutliers {
		deadlockOutlierString = append(deadlockOutlierString, strconv.FormatFloat(outlier, 'f', -1, 64))
	}

	getPctRow := func(pb, pg float64, fi func(int) int) string {
		pCorrect, pDeadlock := perfsCorrect[fi(len(perfsCorrect))], perfsDeadlock[fi(len(perfsDeadlock))]
		return strconv.FormatFloat(pb, 'f', 2, 64) +
			` (` + strconv.FormatFloat(pCorrect.avgMarkCPUOff, 'f', 2, 64) + "µs $\\implies$ " + strconv.FormatFloat(pCorrect.avgMarkCPUOn, 'f', 2, 64) + `µs)` +
			` & ` + strconv.FormatFloat(pg, 'f', 2, 64) +
			` (` + strconv.FormatFloat(pDeadlock.avgMarkCPUOff, 'f', 2, 64) + "µs $\\implies$ " + strconv.FormatFloat(pDeadlock.avgMarkCPUOn, 'f', 2, 64) + `µs)`
	}

	// Return template
	return `\documentclass{standalone}
\usepackage{amsmath}
\usepackage{tikz}
\usepackage{pgfplots}
\usepgfplotslibrary{statistics}
\usepackage{pgfplotstable}
\pgfplotsset{compat=1.18}
\usetikzlibrary{calc,trees,positioning,arrows,chains,shapes.geometric,%
    decorations.pathreplacing,decorations.pathmorphing,patterns,shapes,%
    matrix,shapes.symbols}

\begin{document}


\begin{tikzpicture}
    \begin{axis} [
		boxplot/draw direction=y,
		xtick={1,2},
		xticklabels={\textbf{Correct}, \textbf{Deadlocking}},
		ylabel={Slowdown},
		ymajorgrids=true,
		ymin=-3.5,ymax=3.5,
		ytick={-3,-2,-1,0,1,2,3},
		yticklabels={$0.1\times$, $0.5\times$, $0.9\times$, $1\times$, $1.1\times$, $2\times$, $10\times$},
    ]
	\addplot+ [
        pattern=north east lines,
        boxplot/every box/.style={draw=black},
        boxplot/every whisker/.style={thick,black},
        boxplot/every median/.style={ultra thick,black},
        boxplot prepared={
			lower whisker=` + strconv.FormatFloat(correctBoxLog.Q0, 'f', -1, 64) + `,
			lower quartile=` + strconv.FormatFloat(correctBoxLog.Q1, 'f', -1, 64) + `,
			median=` + strconv.FormatFloat(correctBoxLog.Q2, 'f', -1, 64) + `,
			upper quartile=` + strconv.FormatFloat(correctBoxLog.Q3, 'f', -1, 64) + `,
			upper whisker=` + strconv.FormatFloat(correctBoxLog.Q4, 'f', -1, 64) + `,
        },
        every mark/.append style={
                fill=black!0,
                fill opacity=0,
                draw=black,
            },
	] table [row sep=\\,y index=0] {
		data\\ ` + strings.Join(correctOutlierString, `\\ `) + `\\
	};

	\addplot+ [
		pattern=north east lines,
		boxplot/every median/.style={thick},
		boxplot/every box/.style={draw=black},
		boxplot/every whisker/.style={thick,black},
		boxplot/every median/.style={ultra thick,black},
		every mark/.append style={
			fill=black!0,
			fill opacity=0,
			draw=black,
		},
		boxplot prepared={
				lower whisker=` + strconv.FormatFloat(deadlockBoxLog.Q0, 'f', -1, 64) + `,
				lower quartile=` + strconv.FormatFloat(deadlockBoxLog.Q1, 'f', -1, 64) + `,
				median=` + strconv.FormatFloat(deadlockBoxLog.Q2, 'f', -1, 64) + `,
				upper quartile=` + strconv.FormatFloat(deadlockBoxLog.Q3, 'f', -1, 64) + `,
				upper whisker=` + strconv.FormatFloat(deadlockBoxLog.Q4, 'f', -1, 64) + `,
		},
	] table [row sep=\\,y index=0] {
		data\\ ` + strings.Join(deadlockOutlierString, `\\ `) + `\\
	};
	\end{axis}
\end{tikzpicture}

\begin{tabular}{|c|c|c|}
	\hline
	\textbf{Metric} & \textbf{Correct} & \textbf{Deadlocking}
	\\
	\hline
	\textbf{Min} & ` + getPctRow(correctBoxPct.Min, deadlockBoxPct.Min, func(_ int) int { return 0 }) + `
	\\
	\hline
	\textbf{P1} & ` + getPctRow(correctBoxPct.Q0, deadlockBoxPct.Q0, func(l int) int { return l / 100 }) + `
	\\
	\hline
	\textbf{P25} & ` + getPctRow(correctBoxPct.Q1, deadlockBoxPct.Q1, func(l int) int { return l / 4 }) + `
	\\
	\hline
	\textbf{P50} & ` + getPctRow(correctBoxPct.Q2, deadlockBoxPct.Q2, func(l int) int { return l / 2 }) + `
	\\
	\hline
	\textbf{P75} & ` + getPctRow(correctBoxPct.Q3, deadlockBoxPct.Q3, func(l int) int { return l * 3 / 4 }) + `
	\\
	\hline
	\textbf{P99} & ` + getPctRow(correctBoxPct.Q4, deadlockBoxPct.Q4, func(l int) int { return l * 99 / 100 }) + `
	\\
	\hline
	\textbf{Max} & ` + getPctRow(correctBoxPct.Max, deadlockBoxPct.Max, func(l int) int { return l - 1 }) + `
	\\
	\hline
	\textbf{Max (absolute)} & ` + strconv.FormatFloat(maxCorrectTime.avgMarkCPUOn, 'f', 2, 64) + ` (` +
		strconv.FormatFloat(maxCorrectTime.avgMarkCPUOn/maxCorrectTime.avgMarkCPUOff, 'f', 2, 64) +
		`$\times$) & ` + strconv.FormatFloat(maxDeadlockTime.avgMarkCPUOn, 'f', 2, 64) + ` (` +
		strconv.FormatFloat(maxDeadlockTime.avgMarkCPUOn/maxDeadlockTime.avgMarkCPUOff, 'f', 2, 64) +
		`$\times$)
	\\
	\hline
\end{tabular}

\end{document}
`
}
