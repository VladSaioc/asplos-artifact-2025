package main

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAstString(t *testing.T) {
	require.Equal(t, "x + y", AstString(&ast.BinaryExpr{
		X: &ast.Ident{
			Name: "x",
		},
		Op: token.ADD,
		Y: &ast.Ident{
			Name: "y",
		},
	}))
}

func TestIsFunctionLiteral(t *testing.T) {
	// func() : true
	require.True(t, isFunctionLiteral(&ast.FuncLit{}))
	// (func()) : false
	require.True(t, isFunctionLiteral(&ast.ParenExpr{
		X: &ast.FuncLit{},
	}))
	// x : fale
	require.False(t, isFunctionLiteral(&ast.Ident{}))
}

func TestIsBuil(t *testing.T) {
	// (*ast.Ident)(nil) : false
	require.False(t, isBuiltinFunction((*ast.Ident)(nil)))
	// foo : false
	require.False(t, isBuiltinFunction(&ast.Ident{
		Name: "foo",
	}))
	// nil : false
	require.False(t, isBuiltinFunction(nil))

	// make : true
	require.True(t, isBuiltinFunction(&ast.Ident{
		Name: "make",
	}))
	// (make) : true
	require.True(t, isBuiltinFunction(&ast.ParenExpr{
		X: &ast.Ident{
			Name: "make",
		},
	}))
}

func TestIsCallWithParam(t *testing.T) {
	// nil : false
	require.False(t, IsCallWithParameters(nil))
	// foo() : false
	require.False(t, IsCallWithParameters(&ast.CallExpr{
		Fun: &ast.Ident{
			Name: "foo",
		},
	}))

	// foo(x) : true
	require.True(t, IsCallWithParameters(&ast.CallExpr{
		Fun: &ast.Ident{
			Name: "foo",
		},
		Args: []ast.Expr{&ast.Ident{Name: "x"}},
	}))

	// f._(x) : true
	require.True(t, IsCallWithParameters(&ast.CallExpr{
		Fun:  &ast.SelectorExpr{},
		Args: []ast.Expr{&ast.Ident{Name: "x"}},
	}))

	// f[...](y) : true
	require.True(t, IsCallWithParameters(&ast.CallExpr{
		Fun:  &ast.IndexExpr{},
		Args: []ast.Expr{&ast.Ident{Name: "x"}},
	}))

	// (f[...])(y) : true
	require.True(t, IsCallWithParameters(&ast.CallExpr{
		Fun:  &ast.ParenExpr{X: &ast.IndexExpr{}},
		Args: []ast.Expr{&ast.Ident{Name: "x"}},
	}))

	// (foo(x)) : true
	require.True(t, IsCallWithParameters(&ast.ParenExpr{
		X: &ast.CallExpr{
			Fun: &ast.Ident{
				Name: "foo",
			},
			Args: []ast.Expr{&ast.Ident{Name: "x"}},
		},
	}))
}

func TestReceiverToString(t *testing.T) {
	// nil : ""
	require.Equal(t, "", ReceiverToString(nil))
	// (*ast.Ident)(nil) : ""
	require.Equal(t, "", ReceiverToString((*ast.Ident)(nil)))
	// x : "x"
	require.Equal(t, ".x", ReceiverToString(&ast.Ident{Name: "x"}))
	// (x) : "x"
	require.Equal(t, ".x", ReceiverToString(&ast.ParenExpr{
		X: &ast.Ident{Name: "x"},
	}))
	// (*x) : "(*x)"
	require.Equal(t, ".(*x)", ReceiverToString(&ast.StarExpr{X: &ast.Ident{Name: "x"}}))
	// x[T] : ".x"
	require.Equal(t, ".x", ReceiverToString(&ast.IndexExpr{X: &ast.Ident{Name: "x"}}))
	// _, ... x : ".x"
	require.Equal(t, ".x", ReceiverToString(&ast.FieldList{
		List: []*ast.Field{{
			Type: &ast.IndexExpr{X: &ast.Ident{Name: "x"}}},
		}}))
}
