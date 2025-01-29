package main

import (
	"go/ast"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpectedDeadlockString(t *testing.T) {
	require.Equal(t, "Function: foo; Validation expression: x", ExpectedDeadlock{
		FunctionName: "foo",
		Expression:   &ast.Ident{Name: "x"},
	}.String())
}

func TestExpectedDeadlocksString(t *testing.T) {
	dls := ExpectedDeadlocks{
		Target: "foo",
		Deadlocks: []ExpectedDeadlock{{
			FunctionName: "foo",
			Expression:   &ast.Ident{Name: "x"}},
		},
	}

	require.Equal(t, `Target: foo; Config: GODEBUG=gctrace=1;
Deadlocks: [
	Function: foo; Validation expression: x
]`, dls.String())
}

func TestGetFunctionName(t *testing.T) {
	vis := &expectedDeadlockVisitor{
		Package: &ast.Package{
			Name: "foo",
		},
	}

	require.Equal(t, "foo.init", vis.GetFunctionName())

	vis.FunctionName = "bar"
	require.Equal(t, "foo.bar", vis.GetFunctionName())
}

func TestGetDeadlockExpectations(t *testing.T) {
	exp, err := getDeadlockExpectations("testdata/deadlock_expectations.json", Config{})
	require.Zero(t, exp)
	require.Error(t, err)
}
