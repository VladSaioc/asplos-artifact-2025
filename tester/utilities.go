package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"regexp"
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
