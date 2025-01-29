package main

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluate(t *testing.T) {
	t.Run("bad expression", func(t *testing.T) {
		require.PanicsWithValue(t, "Bad expression: BadExpr (*ast.BadExpr)\n", func() {
			n := 1
			expr := &ast.BadExpr{}
			dl := ExpectedDeadlock{
				Expression: expr,
			}
			dl.evaluate(n, expr)
		})
	})
	t.Run("bad expression (not int or bool)", func(t *testing.T) {
		require.PanicsWithValue(t, "Bad basic literal: foo of kind string\n", func() {
			n := 1
			expr := &ast.BasicLit{
				Kind:  token.STRING,
				Value: "foo",
			}
			dl := ExpectedDeadlock{
				Expression: expr,
			}
			dl.evaluate(n, expr)
		})
	})
	t.Run("1", func(t *testing.T) {
		expr := &ast.BasicLit{
			Kind:  token.INT,
			Value: "1",
		}
		dl := ExpectedDeadlock{
			Expression: expr,
		}
		require.Equal(t, 1, dl.evaluate(0, expr))
	})
	t.Run("(1)", func(t *testing.T) {
		expr := &ast.ParenExpr{
			X: &ast.BasicLit{
				Kind:  token.INT,
				Value: "1",
			},
		}
		dl := ExpectedDeadlock{
			Expression: expr,
		}
		require.Equal(t, 1, dl.evaluate(0, expr))
	})
	t.Run("true", func(t *testing.T) {
		n := 1
		expr := &ast.Ident{
			Name: "true",
		}
		dl := ExpectedDeadlock{
			Expression: expr,
		}
		require.True(t, dl.evaluate(n, expr).(bool))
	})
	t.Run("false", func(t *testing.T) {
		n := 1
		expr := &ast.Ident{
			Name: "false",
		}
		dl := ExpectedDeadlock{
			Expression: expr,
		}
		require.False(t, dl.evaluate(n, expr).(bool))
	})
	t.Run("x", func(t *testing.T) {
		n := 1
		expr := &ast.Ident{
			Name: "x",
		}
		dl := ExpectedDeadlock{
			Expression: expr,
		}
		require.Equal(t, n, dl.evaluate(n, expr))
	})

	testBinaryOp := func(name string, n int, x ast.Expr, op token.Token, y ast.Expr, expected any) {
		t.Run(name, func(t *testing.T) {
			expr := &ast.BinaryExpr{X: x, Op: op, Y: y}
			dl := ExpectedDeadlock{
				Expression: expr,
			}
			require.Equal(t, expected, dl.evaluate(n, expr))
		})
	}

	testBinaryArithm := func(name string, n int, op token.Token, expected any) {
		testBinaryOp(name, n,
			&ast.Ident{
				Name: "x",
			}, op, &ast.BasicLit{
				Kind:  token.INT,
				Value: "2",
			}, expected)
	}

	testBinaryBool := func(name string, x bool, op token.Token, expected any) {
		var xtree ast.Expr
		if x {
			xtree = &ast.Ident{
				Name: "true",
			}
		} else {
			xtree = &ast.Ident{
				Name: "false",
			}
		}

		testBinaryOp(name, 0, xtree,
			op, &ast.Ident{
				Name: "true",
			}, expected)
	}

	// Arithmetic
	testBinaryArithm("1 + 2", 1, token.ADD, 3)
	testBinaryArithm("1 - 2", 1, token.SUB, -1)
	testBinaryArithm("2 * 2", 2, token.MUL, 4)
	testBinaryArithm("6 / 2", 6, token.QUO, 3)
	testBinaryArithm("5 % 2", 5, token.REM, 1)
	// Equality
	testBinaryArithm("2 == 2", 2, token.EQL, true)
	testBinaryArithm("2 != 2", 2, token.NEQ, false)
	// Comparisons
	testBinaryArithm("2 < 2", 2, token.LSS, false)
	testBinaryArithm("2 <= 2", 2, token.LEQ, true)
	testBinaryArithm("2 > 2", 2, token.GTR, false)
	testBinaryArithm("2 >= 2", 2, token.GEQ, true)

	// Boolean
	// Equality
	testBinaryBool("true == true", true, token.EQL, true)
	testBinaryBool("true != true", true, token.NEQ, false)

	// Operations
	testBinaryBool("true && true", true, token.LAND, true)
	testBinaryBool("true || true", true, token.LOR, true)
}

func TestDeadlockShouldBeFound(t *testing.T) {
	t.Run("nil expression", func(t *testing.T) {
		require.False(t, ExpectedDeadlock{}.DeadlockShouldBeFound())
		require.False(t, ExpectedDeadlocks{}.DeadlockShouldBeFound())
	})

	t.Run("deadlock should be found (true)", func(t *testing.T) {
		dl := ExpectedDeadlock{
			Expression: &ast.Ident{
				Name: "true",
			},
		}
		require.True(t, dl.DeadlockShouldBeFound())
		dls := ExpectedDeadlocks{
			Deadlocks: []ExpectedDeadlock{dl},
		}
		require.True(t, dls.DeadlockShouldBeFound())
	})

	t.Run("deadlock should not be found (false)", func(t *testing.T) {
		dl := ExpectedDeadlock{
			Expression: &ast.Ident{
				Name: "false",
			},
		}
		require.False(t, dl.DeadlockShouldBeFound())
		dls := ExpectedDeadlocks{
			Deadlocks: []ExpectedDeadlock{dl},
		}
		require.False(t, dls.DeadlockShouldBeFound())
	})

	t.Run("deadlock should be found (const)", func(t *testing.T) {
		dl := ExpectedDeadlock{
			Expression: &ast.BasicLit{
				Kind:  token.INT,
				Value: "10",
			},
		}
		require.True(t, dl.DeadlockShouldBeFound())
		dls := ExpectedDeadlocks{
			Deadlocks: []ExpectedDeadlock{dl},
		}
		require.True(t, dls.DeadlockShouldBeFound())
	})

	t.Run("deadlock should not be found (0)", func(t *testing.T) {
		dl := ExpectedDeadlock{
			Expression: &ast.BasicLit{
				Kind:  token.INT,
				Value: "0",
			},
		}
		require.False(t, dl.DeadlockShouldBeFound())
		dls := ExpectedDeadlocks{
			Deadlocks: []ExpectedDeadlock{dl},
		}
		require.False(t, dls.DeadlockShouldBeFound())
	})
}

func TestCompareWithTraceValue(t *testing.T) {
	t.Run("nil expression", func(t *testing.T) {
		require.False(t, ExpectedDeadlock{}.CompareWithTraceValue(0))
		require.True(t, ExpectedDeadlock{}.CompareWithTraceValue(1))
	})

	t.Run("check always succeeds", func(t *testing.T) {
		dl := ExpectedDeadlock{
			Expression: &ast.Ident{
				Name: "true",
			},
		}
		require.True(t, dl.CompareWithTraceValue(1))
	})

	t.Run("check always fails", func(t *testing.T) {
		dl := ExpectedDeadlock{
			Expression: &ast.Ident{
				Name: "false",
			},
		}
		require.False(t, dl.CompareWithTraceValue(1))
	})

	t.Run("deadlock matches const", func(t *testing.T) {
		dl := ExpectedDeadlock{
			Expression: &ast.BasicLit{
				Kind:  token.INT,
				Value: "10",
			},
		}
		require.True(t, dl.CompareWithTraceValue(10))
		require.False(t, dl.CompareWithTraceValue(1))
	})
}

func TestCompareWithTrace(t *testing.T) {
	t.Run("no deadlocks", func(t *testing.T) {
		dls := ExpectedDeadlocks{}
		diff := dls.CompareWithTrace(Trace{})
		require.Zero(t, diff.CorrectDeadlockFound)
		require.Zero(t, diff.CorrectNoDeadlockFound)
		require.Empty(t, diff.Mismatches)
	})

	t.Run("expected deadlock mismatches", func(t *testing.T) {
		dl := ExpectedDeadlock{
			FunctionName: "main.main",
			Expression: &ast.BinaryExpr{
				X: &ast.Ident{
					Name: "x",
				},
				Op: token.GEQ,
				Y: &ast.BasicLit{
					Kind:  token.INT,
					Value: "10",
				},
			},
		}
		dls := ExpectedDeadlocks{
			Deadlocks: []ExpectedDeadlock{dl},
		}

		diff := dls.CompareWithTrace(Trace{})
		require.Zero(t, diff.CorrectDeadlockFound)
		require.Zero(t, diff.CorrectNoDeadlockFound)
		require.Len(t, diff.Mismatches, 1)
		require.EqualValues(t, diff.Mismatches[0], DeadlockMismatch{
			ExpectedDeadlock: dl,
		})
		require.Equal(t, diff.Mismatches[0].String(), "[Expected: x >= 10; Actual: 0] at - (main.main)")
	})

	t.Run("unexpected deadlock mismatches", func(t *testing.T) {
		dls := ExpectedDeadlocks{
			Deadlocks: []ExpectedDeadlock{},
		}

		diff := dls.CompareWithTrace(Trace{
			Deadlocks: []TraceDeadlock{{
				FunctionName: "main.main",
				Count:        1,
			}, {
				FunctionName: "main.main",
				Count:        1,
			}},
		})
		require.Zero(t, diff.CorrectDeadlockFound)
		require.Zero(t, diff.CorrectNoDeadlockFound)
		require.Len(t, diff.Mismatches, 1)
		require.EqualValues(t, diff.Mismatches[0], DeadlockMismatch{
			TraceDeadlock: TraceDeadlock{
				FunctionName: "main.main",
				Count:        2,
			},
			Unexpected: true,
		})
		require.Equal(t, diff.Mismatches[0].String(), "Unexpected DL: main.main (2)")
	})

	t.Run("Expected deadlock and trace, no mismatches", func(t *testing.T) {
		dl := ExpectedDeadlock{
			FunctionName: "main.main",
			Expression: &ast.BasicLit{
				Kind:  token.INT,
				Value: "10",
			},
		}
		dls := ExpectedDeadlocks{
			Deadlocks: []ExpectedDeadlock{dl},
		}

		trace := Trace{}
		for i := 0; i < 10; i++ {
			trace.Deadlocks = append(trace.Deadlocks, TraceDeadlock{
				FunctionName: "main.main",
			})
		}

		diff := dls.CompareWithTrace(trace)
		require.Equal(t, 1, diff.CorrectDeadlockFound)
		require.Empty(t, diff.Mismatches)

		r := TargetReport{
			ExpectedDeadlocks: dls,
			Trace:             trace,
		}
		r.CompareWithTrace()
		require.Equal(t, 1, diff.CorrectDeadlockFound)
		require.Empty(t, r.Diff.Mismatches)
	})
}

func TestTraceHasExceptions(t *testing.T) {
	t.Run("no exceptions", func(t *testing.T) {
		r := TargetReport{}
		msg, ok := r.TraceHasExceptions()
		require.False(t, ok)
		require.Empty(t, msg)
	})

	t.Run("exceptions - fatal", func(t *testing.T) {
		r := TargetReport{
			RawTrace: []byte(`
					Preceding information
					---------
					fatal error: trace fatal`),
		}
		msg, ok := r.TraceHasExceptions()
		require.True(t, ok)
		require.Equal(t, "trace fatal", msg)
	})

	t.Run("exceptions - panic", func(t *testing.T) {
		r := TargetReport{
			RawTrace: []byte(`
					Preceding information
					---------
					panic: trace panic`),
		}
		msg, ok := r.TraceHasExceptions()
		require.True(t, ok)
		require.Equal(t, "trace panic", msg)
	})
}
