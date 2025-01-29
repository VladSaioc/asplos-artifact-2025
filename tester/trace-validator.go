package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Trace contains a fatal error
	fatalError = regexp.MustCompile(`fatal error:`)
	// Trace contains a panic
	panicError = regexp.MustCompile(`panic:`)
)

// Evaluate evaluates the expression `dl.Expression`, grounding variables
// with the value `n`. The function returns the result of the evaluation.
func (dl ExpectedDeadlock) evaluate(n int, expr ast.Expr) any {
	var eval func(ast.Expr) any
	eval = func(expr ast.Expr) any {
		switch expr := expr.(type) {
		case *ast.ParenExpr:
			return eval(expr.X)
		case *ast.BasicLit:
			if expr.Kind != token.INT {
				msg := fmt.Sprintf("Bad basic literal: %v of kind %T\n", AstString(dl.Expression), expr.Kind.String())
				panic(msg)
			}
			i, err := strconv.Atoi(expr.Value)
			if err == nil {
				return i
			}
		case *ast.BinaryExpr:
			x, xIsInt := eval(expr.X).(int)
			y, yIsInt := eval(expr.Y).(int)
			switch expr.Op {
			case token.ADD,
				token.SUB,
				token.MUL,
				token.QUO,
				token.REM:
				if xIsInt && yIsInt {
					switch expr.Op {
					case token.ADD:
						return x + y
					case token.SUB:
						return x - y
					case token.MUL:
						return x * y
					case token.QUO:
						return x / y
					case token.REM:
						return x % y
					}
				}
			case token.LSS,
				token.LEQ,
				token.GTR,
				token.GEQ:
				x, xIsInt := eval(expr.X).(int)
				y, yIsInt := eval(expr.Y).(int)
				if xIsInt && yIsInt {
					switch expr.Op {
					case token.LSS:
						return x < y
					case token.LEQ:
						return x <= y
					case token.GTR:
						return x > y
					case token.GEQ:
						return x >= y
					}
				}
			case token.EQL,
				token.NEQ:
				switch x := eval(expr.X).(type) {
				case int:
					if y, yIsInt := eval(expr.Y).(int); yIsInt {
						switch expr.Op {
						case token.EQL:
							return x == y
						case token.NEQ:
							return x != y
						}
					}
				case bool:
					if y, yIsBool := eval(expr.Y).(bool); yIsBool {
						switch expr.Op {
						case token.EQL:
							return x == y
						case token.NEQ:
							return x != y
						}
					}
				}
			case token.LAND, token.LOR:
				x, xIsBool := eval(expr.X).(bool)
				y, yIsBool := eval(expr.Y).(bool)
				if xIsBool && yIsBool {
					switch expr.Op {
					case token.LAND:
						return x && y
					case token.LOR:
						return x || y
					}
				}
			}
		case *ast.Ident:
			switch expr.Name {
			case "true":
				return true
			case "false":
				return false
			}
			expr.Name = "x"
			return n
		}
		msg := fmt.Sprintf("Bad expression: %v (%T)\n", AstString(dl.Expression), dl.Expression)
		panic(msg)
	}
	return eval(expr)
}

// DeadlockShouldBeFound returns `true` if the annotation signals that deadlocks
// are expected at this location, and `false` otherwise. The assumption is that,
// unless it is explicitly stated that deadlocks are not expected (by stating that
// the expression evaluates to `0` or `false`), deadlocks are expected.
func (dl ExpectedDeadlock) DeadlockShouldBeFound() bool {
	if dl.Expression == nil {
		return false
	}

	v := dl.evaluate(20, dl.Expression)
	switch v := v.(type) {
	case bool:
		return v
	case int:
		return v != 0
	}

	msg := fmt.Sprintf("Bad expression: %v; Unexpected value type: %v (%T)\n", AstString(dl.Expression), v, v)
	panic(msg)
}

// DeadlockShouldBeFound returns `true` if the annotation signals that at least one
// deadlock is expected in the file, and `false` otherwise. The assumption is that,
// unless it is explicitly stated that deadlocks are not expected (by stating that
// the expression evaluates to `0` or `false`), deadlocks are expected.
func (dl ExpectedDeadlocks) DeadlockShouldBeFound() bool {
	for _, dl := range dl.Deadlocks {
		if dl.DeadlockShouldBeFound() {
			return true
		}
	}
	return false
}

// CompareWithActual compares the expected deadlock count with the actual
// number of deadlocks in the trace. If the expected deadlock count is
// nil, the function returns true if the actual deadlock count is greater
// than zero. Otherwise, it returns true if the actual deadlock count is
// equal to the expected deadlock count.
func (dl ExpectedDeadlock) CompareWithTraceValue(n int) bool {
	if dl.Expression == nil {
		return n > 0
	}

	v := dl.evaluate(n, dl.Expression)
	switch v := v.(type) {
	case bool:
		return v
	case int:
		return v == n
	}

	msg := fmt.Sprintf("Bad expression: %v; Unexpected value type: %v (%T)\n", AstString(dl.Expression), v, v)
	panic(msg)
}

// DeadlockMismatch represents a mismatch between the expected and actual deadlocks.
type DeadlockMismatch struct {
	ExpectedDeadlock
	TraceDeadlock
	Unexpected bool
}

// DeadlockDifferential indexes the number of correct annotation hits, as well as
// all the mismatches.
type DeadlockDifferential struct {
	CorrectDeadlockFound   int
	CorrectNoDeadlockFound int
	Mismatches             []DeadlockMismatch
}

func (dm DeadlockMismatch) String() string {
	if dm.Unexpected {
		return fmt.Sprintf("Unexpected DL: %v (%v)", dm.TraceDeadlock.FunctionName, dm.TraceDeadlock.Count)
	}
	return fmt.Sprintf("[Expected: %v; Actual: %v] at %v (%v)",
		AstString(dm.ExpectedDeadlock.Expression),
		dm.Count, dm.Position, dm.ExpectedDeadlock.FunctionName)
}

// CompareWithTrace compares the expected deadlocks with the actual deadlocks reported
// by the GC. It ensures that all the expected deadlocks (or absence) noted via annotations are found
// in the trace, and that no unexpected deadlocks are reported.
//
// It produces a set of mismatches for each distinct function name.
func (dls ExpectedDeadlocks) CompareWithTrace(t Trace) (diff DeadlockDifferential) {
	diff.Mismatches = make([]DeadlockMismatch, 0, len(dls.Deadlocks)+len(t.Deadlocks))

	visitedFunctions := make(map[string]struct{})
	for _, dl := range dls.Deadlocks {
		dlsAtF := t.DeadlocksAtFunction(dl.FunctionName)
		visitedFunctions[dl.FunctionName] = struct{}{}
		if dl.CompareWithTraceValue(dlsAtF) {
			if dl.DeadlockShouldBeFound() {
				diff.CorrectDeadlockFound++
			} else {
				diff.CorrectNoDeadlockFound++
			}
			continue
		}
		diff.Mismatches = append(diff.Mismatches, DeadlockMismatch{
			ExpectedDeadlock: dl,
			TraceDeadlock: TraceDeadlock{
				Count: dlsAtF,
			},
		})
	}

	unexpectedVisited := make(map[string]int)
	for _, deadlock := range t.Deadlocks {
		if _, ok := visitedFunctions[deadlock.FunctionName]; !ok {
			if num, ok := unexpectedVisited[deadlock.FunctionName]; !ok {
				unexpectedVisited[deadlock.FunctionName] = 1
			} else {
				unexpectedVisited[deadlock.FunctionName] = num + 1
			}
		}
	}
	for fn, count := range unexpectedVisited {
		diff.Mismatches = append(diff.Mismatches, DeadlockMismatch{
			TraceDeadlock: TraceDeadlock{
				FunctionName: fn,
				Count:        count,
			},
			Unexpected: true,
		})
	}

	return
}

func (r *TargetReport) TraceHasExceptions() (string, bool) {
	trace := string(r.RawTrace)
	for _, line := range strings.Split(trace, "\n") {
		switch {
		case fatalError.MatchString(line):
			if splits := fatalError.Split(line, -1); len(splits) > 1 {
				return strings.TrimSpace(splits[1]), true
			}
		case panicError.MatchString(line):
			if splits := panicError.Split(line, -1); len(splits) > 1 {
				return strings.TrimSpace(splits[1]), true
			}
		}
	}
	return "", false
}
