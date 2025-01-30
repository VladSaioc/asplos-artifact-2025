package main

import (
	"errors"
	"go/ast"
	"go/token"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReportAppend(t *testing.T) {
	r := &Report{}
	r.Append(TargetReport{})

	require.Len(t, r.Results, 1)
}

func TestTargetReportIsBuggy(t *testing.T) {
	r := TargetReport{}
	require.False(t, r.IsBuggy())

	r = TargetReport{
		ExpectedDeadlocks: ExpectedDeadlocks{
			Target: path.Join("foo", "buggy", "bar"),
		},
	}

	require.True(t, r.IsBuggy())
}

func TestTargetReportIsDeadlock(t *testing.T) {
	r := TargetReport{}
	require.False(t, r.IsDeadlock())

	r = TargetReport{
		ExpectedDeadlocks: ExpectedDeadlocks{
			Target: path.Join("foo", "deadlock", "bar"),
		},
	}

	require.True(t, r.IsDeadlock())
}

func TestTargetReportIsCorrect(t *testing.T) {
	r := TargetReport{}
	require.False(t, r.IsCorrect())

	r = TargetReport{
		ExpectedDeadlocks: ExpectedDeadlocks{
			Target: path.Join("foo", "correct", "bar"),
		},
	}

	require.True(t, r.IsCorrect())
}

func TestTargetReportEmitToFile(t *testing.T) {
	r := TargetReport{
		RawTrace:  []byte("foo"),
		TraceFile: path.Join(t.TempDir(), "foo"),
		Config:    Config{maxProcs1},
		ExpectedDeadlocks: ExpectedDeadlocks{
			Target: "foo",
		},
	}

	require.NoError(t, r.EmitToFile())
	require.FileExists(t, r.TraceFile)
	content, err := os.ReadFile(r.TraceFile)
	require.NoError(t, err)
	require.Equal(t, "Ran foo with configuration:\nGOMAXPROCS=1 GODEBUG=gctrace=1\n\nfoo", string(content))

}

func TestTargetReportString(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		r := TargetReport{
			ExpectedDeadlocks: ExpectedDeadlocks{},
			Repeat:            1,
			Config:            Config{maxProcs1},
			Trace:             Trace{},
		}

		require.Empty(t, r.String())
	})

	t.Run("bare-bones", func(t *testing.T) {
		r := TargetReport{
			Exception: errors.New("exception"),
			ExpectedDeadlocks: ExpectedDeadlocks{
				Target: "foo",
			},
			Repeat: 1,
			Config: Config{maxProcs1},
			Trace: Trace{
				Deadlocks: []TraceDeadlock{
					{FunctionName: "foo"},
					{FunctionName: "bar"},
				},
			},
		}

		msg := strings.Split(r.String(), "@:@")
		require.Len(t, msg, 2)
		require.Equal(t, "1,\tfoo,\tGOMAXPROCS-1,\tUnexpected DL: bar (1),\t[exception] ,\t-", msg[0])
		require.Equal(t, "1,\tfoo,\tGOMAXPROCS-1,\tUnexpected DL: foo (1),\t[exception] ,\t", msg[1])
	})

	t.Run("bad correct report", func(t *testing.T) {
		r := TargetReport{
			Exception: errors.New("exception"),
			ExpectedDeadlocks: ExpectedDeadlocks{
				Target: path.Join("foo", "correct", "foo"),
				Deadlocks: []ExpectedDeadlock{{
					FunctionName: "foo",
					Expression: &ast.BinaryExpr{
						X: &ast.Ident{
							Name: "x",
						},
						Op: token.GEQ,
						Y: &ast.BasicLit{
							Kind:  token.INT,
							Value: "0",
						},
					}},
				},
			},
			Repeat: 1,
			Config: Config{maxProcs1},
			Trace: Trace{
				Deadlocks: []TraceDeadlock{
					{FunctionName: "foo"},
					{FunctionName: "bar"},
				},
			},
		}

		msg := strings.Split(r.String(), "@:@")
		require.Len(t, msg, 1)
		require.Equal(t, strings.Join([]string{
			"1",
			path.Join("foo", "correct", "foo"),
			"GOMAXPROCS-1",
			"Unexpected DL: bar (1)",
			"[exception] ",
			"Annotations expected deadlock in correct example; Deadlock found in correct example trace; ",
		}, ",\t"), msg[0])
	})

	t.Run("bad deadlock report", func(t *testing.T) {
		r := TargetReport{
			Exception: errors.New("exception"),
			ExpectedDeadlocks: ExpectedDeadlocks{
				Target: path.Join("foo", "deadlock", "foo"),
			},
			Repeat: 1,
			Config: Config{maxProcs1},
			Trace: Trace{
				Deadlocks: []TraceDeadlock{
					{FunctionName: "foo"},
				},
			},
			RawTrace: []byte("fatal error: fatal"),
		}

		msg := strings.Split(r.String(), "@:@")
		require.Len(t, msg, 1)
		require.Equal(t, strings.Join([]string{
			"1",
			path.Join("foo", "deadlock", "foo"),
			"GOMAXPROCS-1",
			"Unexpected DL: foo (1)",
			"[exception] fatal",
			"Missing deadlock annotation in deadlock example; ",
		}, ",\t"), msg[0])
	})
}

func TestReportString(t *testing.T) {
	r := &Report{
		Results: map[string]*TargetReport{
			"a": {
				ExpectedDeadlocks: ExpectedDeadlocks{},
				Repeat:            1,
				Config:            Config{maxProcs1, deadlockDetectionCollect},
				Trace:             Trace{},
			},
			"b": {
				Exception: errors.New("exception"),
				ExpectedDeadlocks: ExpectedDeadlocks{
					Target: path.Join("foo", "deadlock", "foo"),
				},
				Repeat: 1,
				Config: Config{maxProcs1, deadlockDetectionCollect},
				Trace: Trace{
					Deadlocks: []TraceDeadlock{
						{FunctionName: "foo"},
					},
				},
				RawTrace: []byte("fatal error: fatal"),
			}},
	}

	msg := strings.Split(r.String(), "\n")
	require.Len(t, msg, 8)
	require.Equal(t, strings.Join([]string{
		"Repeat round",
		"Configuration",
		"Target",
		"Deadlock mismatches",
		"Exceptions",
		"Comment",
	}, ",\t"), msg[0])
	require.Equal(t, strings.Join([]string{
		"1",
		path.Join("foo", "deadlock", "foo"),
		"GOMAXPROCS-1-gcdetectdeadlocks-1",
		"Unexpected DL: foo (1)",
		"[exception] fatal",
		"Missing deadlock annotation in deadlock example; ",
	}, ",\t"), msg[1])

	require.Equal(t, "Correct guesses: 0/1 (0.00%)", msg[3])
	require.Equal(t, "Correct deadlocks: 0/0 (NaN%)", msg[4])
	require.Equal(t, "Correct not deadlocks: 0/1", msg[5])
	require.Equal(t, "Incorrect guesses: 1 (100.00%)", msg[6])
}
