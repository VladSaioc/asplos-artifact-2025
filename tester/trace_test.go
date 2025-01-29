package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractTraceDeadlocks(t *testing.T) {
	t1 := []byte(``)
	trace, err := ExtractTrace(t1)
	require.EqualValues(t, Trace{}, trace)
	require.NoError(t, err)

	t1 = []byte(`partial deadlock!`)
	trace, _ = ExtractTrace(t1)
	require.EqualValues(t, Trace{}, trace)
	require.NoError(t, err)

	t1 = []byte(`partial deadlock! goroutine 1: foo
gc %: a
gc 1 @2s%: , 3 P
gc 1 @2s a%: , 3 P
gc 1 @2s 0%: 1+2+3 ms clock, 1+2/3/4+5 ms cpu, a->b->badheap->100->100->99 MB, s80 MB stacks, 3 P (forced)
gc 1 @2s 0%: 1+2+3 ms clock, 1+2/3/4+5 ms cpu, 100->100->99 MB, badstack80 MB stacks, 3 P (forced)
gc 1 @2s 0%: 1+2+3 ms clock, 1+2/3/4+5 ms cpu, 100->100->99 MB, 80 MB stacks, 3 P (forced)
Final goroutine count: asd
Final goroutine count: 30
`)
	trace, err = ExtractTrace(t1)
	require.ErrorContains(t, err, "GC cycle `gc 1 @2s%: , 3 P` is ill-formatted left of the colon")
	require.ErrorContains(t, err, "strconv.ParseFloat: parsing \"a\": invalid syntax")
	require.ErrorContains(t, err, "strconv.Atoi: parsing \"badstack80\": invalid syntax")
	require.ErrorContains(t, err, "strconv.Atoi: parsing \"badheap\": invalid syntax")
	require.EqualValues(t, Trace{
		Deadlocks: []TraceDeadlock{{FunctionName: "foo"}},
		GCMessages: []gcTrace{{
			cycle:          1,
			cpuUtilization: 0,
			timeUnit:       "ms",
			clockMarkTime:  2,
			clockTermTime:  3,
			cpuMarkTime:    9,
			cpuTermTime:    5,
			memUnit:        "MB",
			liveHeap:       99,
			stackSize:      80,
			processors:     3,
		}},
		NumGoroutines: 30,
	}, trace)
}

func TestDeadlocksAtFunction(t *testing.T) {
	trace := Trace{
		Deadlocks: []TraceDeadlock{
			{FunctionName: "foo"},
			{FunctionName: "bar"},
			{FunctionName: "foo"},
		},
	}

	require.Equal(t, 2, trace.DeadlocksAtFunction("foo"))
	require.Equal(t, 1, trace.DeadlocksAtFunction("bar"))
	require.Equal(t, 0, trace.DeadlocksAtFunction("baz"))
}

func TestRemoveGoGCTra(t *testing.T) {
	const (
		withoutStart = `foo
bar`
		withStart = `foo
` + startRun + `
` + withoutStart
	)

	require.EqualValues(t, withoutStart, RemoveGoGCTrace(strings.NewReader(string(withoutStart))))
	require.EqualValues(t, startRun+"\n"+withoutStart, RemoveGoGCTrace(strings.NewReader(string(withStart))))
}
