package main

import (
	"bytes"
	"cmp"
	"go/ast"
	"go/format"
	"go/token"
	"math"
	"regexp"
	"slices"
)

// Generic parameter string representation.
// Covers all non-nested matching pairs of square brackets with at
// least one containing character.
//
// Is used to remove generic parameters from type strings.
var genericParam = regexp.MustCompile(`\[[^\[\]]+\]`)

func AstString(n ast.Node) string {
	exprstr := bytes.NewBuffer(nil)
	format.Node(exprstr, token.NewFileSet(), n)
	return exprstr.String()
}

// isFunctionLiteral checks if the given node is a function literal.
func isFunctionLiteral(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.FuncLit:
		return n != nil
	case *ast.ParenExpr:
		return n != nil && isFunctionLiteral(n.X)
	}
	return false
}

// isBuiltinFunction checks if the given node is a built-in function.
//
// The built-in functions are: make, new, len, cap, copy, delete, append, panic, recover, print, println, close.
func isBuiltinFunction(node ast.Node) bool {
	builtinName := map[string]struct{}{
		"make":    {},
		"new":     {},
		"len":     {},
		"cap":     {},
		"copy":    {},
		"delete":  {},
		"append":  {},
		"panic":   {},
		"recover": {},
		"print":   {},
		"println": {},
		"close":   {},
	}

	switch n := node.(type) {
	case *ast.Ident:
		if n == nil {
			return false
		}
		_, ok := builtinName[n.Name]
		return ok
	case *ast.ParenExpr:
		return n != nil && isBuiltinFunction(n.X)
	}
	return false
}

// IsCallWithParameters checks if the given node is a function call with parameters.
// Calls with parameters are calls where the function is a simple identifier with at
// least one argument, or a selector or index expression.
func IsCallWithParameters(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.CallExpr:
		var isIndexExpr func(ast.Expr) bool
		isIndexExpr = func(expr ast.Expr) bool {
			switch n := expr.(type) {
			case *ast.SelectorExpr:
				return true
			case *ast.IndexExpr:
				return true
			case *ast.ParenExpr:
				return isIndexExpr(n.X)
			}
			return false
		}

		return n != nil && (isIndexExpr(n.Fun) || len(n.Args) > 0)
	case *ast.ParenExpr:
		return n != nil && IsCallWithParameters(n.X)
	}
	return false
}

// ReceiverToString takes a method receiver type definition and constructs a string representation of it
// that conforms to its representation in goroutine traces emitted by the runtime.
//
// Non-empty strings are prefixed with a dot, because the actual trace will always qualify the
// receiver name with the package first.
func ReceiverToString(n ast.Node) string {
	var eval func(ast.Node) string
	eval = func(n ast.Node) string {
		switch n := n.(type) {
		case *ast.FieldList:
			if n != nil && len(n.List) > 0 {
				// Method receiver types are always single-element field lists.
				return eval(n.List[0])
			}
		case *ast.Field:
			if n != nil {
				return eval(n.Type)
			}
		case *ast.IndexExpr:
			return eval(n.X) + "[T]"
		case *ast.StarExpr:
			if n != nil {
				return "(*" + eval(n.X) + ")"
			}
		case *ast.Ident:
			if n != nil {
				return n.Name
			}
		case *ast.ParenExpr:
			if n != nil {
				return eval(n.X)
			}
		}
		return ""
	}

	str := eval(n)
	if str == "" {
		return ""
	}

	return "." + genericParam.ReplaceAllString(str, "")
}

// P1 gets the 1st percentile in a slice of orderable values.
func P1[T cmp.Ordered](xs []T) (x T) {
	if len(xs) == 0 {
		return
	}
	slices.Sort(xs)
	return xs[len(xs)/100]
}

// P1 gets the 25th percentile in a slice of orderable values.
func P25[T cmp.Ordered](xs []T) (x T) {
	if len(xs) == 0 {
		return
	}
	slices.Sort(xs)
	return xs[len(xs)/4]
}

// P50 gets the median value in a slice of orderable values.
func P50[T cmp.Ordered](xs []T) (x T) {
	if len(xs) == 0 {
		return
	}
	slices.Sort(xs)
	return xs[len(xs)/2]
}

// P50 gets the median value in a slice of orderable values.
func P75[T cmp.Ordered](xs []T) (x T) {
	if len(xs) == 0 {
		return
	}
	slices.Sort(xs)
	return xs[len(xs)*3/4]
}

// P99 gets the 99th percentile in a slice of orderable values.
func P99[T cmp.Ordered](xs []T) (x T) {
	if len(xs) == 0 {
		return
	}
	slices.Sort(xs)
	return xs[len(xs)*99/100]
}

type BoxMetrics[T cmp.Ordered] struct {
	// Smallest outlier
	Min T
	// Values below the 1'st percentile.
	SmallOutliers []T
	// P1 to P25 percentile values.
	Q0 T
	// P25 to P50 percentile values.
	Q1 T
	// P50 percentile value.
	Q2 T
	// P50 to P75 percentile values.
	Q3 T
	// P75 to P99 percentile values.
	Q4 T
	// Values above the 99th percentile.
	LargeOutliers []T
	// Biggest outlier
	Max T
}

// BoxPlotMetrics computes the box plot metrics for a slice of orderable values.
func BoxPlotMetrics[T cmp.Ordered](xs []T) (m BoxMetrics[T]) {
	if len(xs) == 0 {
		return
	}

	slices.Sort(xs)
	m.Min = xs[0]
	m.Q0 = P1(xs)
	m.Q1 = P25(xs)
	m.Q2 = P50(xs)
	m.Q3 = P75(xs)
	m.Q4 = P99(xs)
	m.Max = xs[len(xs)-1]

	for _, x := range xs {
		if x < m.Q0 {
			m.SmallOutliers = append(m.SmallOutliers, x)
		} else if x > m.Q4 {
			m.LargeOutliers = append(m.LargeOutliers, x)
		}
	}

	return
}

// NormalizeSlowdown takes the baseline and Golf marking time values,
// computes the slowdown, and normalizes the result to in the decimal logarithmic scale.
func NormalizeSlowdown(baseline, golf float64) float64 {
	if golf == 0 {
		return 0
	}
	sign, ratio := -1.0, (baseline-golf)/golf*100
	if golf > baseline {
		sign = 1.0
		ratio = (golf - baseline) / baseline * 100
	}

	var value float64
	if ratio >= 10.00 {
		value = math.Log10(ratio)
	} else {
		value = ratio / 10
	}

	return sign * value
}
