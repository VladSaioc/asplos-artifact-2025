package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

type ExpectedDeadlock struct {
	token.Position

	// FunctionName is the name of the function expected to cause a deadlock.
	// It follows the naming convention of named functions in Go traces.
	FunctionName string

	// Expression quantifies the number of deadlocks expected to occur in the
	// trace. The expression may evaluate to either a numeric constant, in
	// which case it is the exact number of deadlocks expected, or it may
	// be a boolean expression that the number of expected deadlocks must
	// satisfy.
	Expression ast.Expr
}

type ExpectedDeadlocks struct {
	// Config is the configuration used to parse the test case.
	Config
	// Target is the package path of the test case.
	Target string

	// FileSet allows using positional information in the AST.
	*token.FileSet

	// Deadlocks is a set of expected deadlocks that the trace must produce.
	Deadlocks []ExpectedDeadlock
}
type expectedDeadlockVisitor struct {
	*token.FileSet
	*ast.Package

	ast.CommentMap
	ExpectedDeadlocks

	FunctionName           string
	Receiver               ast.Node
	FunctionLiteralNesting functionLiteralNesting
}

func (dl ExpectedDeadlock) String() string {
	return fmt.Sprintf("Function: %s; Validation expression: %s", dl.FunctionName, AstString(dl.Expression))
}
func (dls ExpectedDeadlocks) String() string {
	dlsStr := make([]string, 0, len(dls.Deadlocks))
	for _, dl := range dls.Deadlocks {
		dlsStr = append(dlsStr, dl.String())
	}

	return fmt.Sprintf("Target: %s; Config: %v;\nDeadlocks: [\n\t%v\n]",
		dls.Target, dls.Config,
		strings.Join(dlsStr, "\n\t"))
}

// PosString returns the position string of the expected deadlock.
func (dl ExpectedDeadlock) PosString() string {
	return fmt.Sprintf("%d:%d", dl.Line, dl.Column)
}

// GetFunctionName constructs a
func (v *expectedDeadlockVisitor) GetFunctionName() string {
	functionName := v.Package.Name + ReceiverToString(v.Receiver)
	if v.FunctionName == "" {
		functionName += ".init"
	} else {
		functionName += "." + v.FunctionName
	}
	return functionName + v.FunctionLiteralNesting.FunctionSuffix()
}

// Visit allows expectedDeadlockVisitor to satisfy the ast.Visitor interface.
func (v *expectedDeadlockVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	functionScope := func(n ...ast.Node) {
		v.FunctionLiteralNesting = append(v.FunctionLiteralNesting, &funcLiteral{
			fun:    1,
			gowrap: 1,
		})
		for _, node := range n {
			ast.Walk(v, node)
		}
		v.FunctionLiteralNesting = v.FunctionLiteralNesting[:len(v.FunctionLiteralNesting)-1]
	}

	addExpectedDeadlock := func() {
		comments, ok := v.CommentMap[node]
		if !ok {
			return
		}
		for _, cg := range comments {
			for _, line := range strings.Split(cg.Text(), "\n") {
				if !strings.Contains(line, "deadlocks:") {
					continue
				}
				expression := strings.TrimSpace(strings.TrimPrefix(line, "deadlocks:"))
				dl := ExpectedDeadlock{
					Position:     v.FileSet.Position(node.Pos()),
					FunctionName: v.GetFunctionName(),
				}
				if expression != "" {
					if expr, err := parser.ParseExpr(expression); err != nil {
						fmt.Printf("Failed to parse expression %s: %v\n", expression, err)
					} else {
						dl.Expression = expr
					}
				}
				v.Deadlocks = append(v.Deadlocks, dl)
			}
		}
	}

	switch n := node.(type) {
	case *ast.GoStmt:
		switch {
		case isFunctionLiteral(n.Call.Fun) &&
			IsCallWithParameters(n.Call):
			v.FunctionLiteralNesting = v.FunctionLiteralNesting.PushGowrap(n)
			ast.Walk(v, n.Call.Fun)
			v.FunctionLiteralNesting = v.FunctionLiteralNesting.PopGowrap()
			for _, arg := range n.Call.Args {
				ast.Walk(v, arg)
			}
			return nil
		case isBuiltinFunction(n.Call.Fun),
			!isFunctionLiteral(n.Call) &&
				IsCallWithParameters(n.Call):
			ast.Walk(v, n.Call.Fun)
			for _, arg := range n.Call.Args {
				ast.Walk(v, arg)
			}
			// Add a mock function literal token to the nesting to use when adding a deadlock annotation
			// for a builtin function.
			// Current builtin functions do not have deadlocking behaviour.
			v.FunctionLiteralNesting = append(v.FunctionLiteralNesting.PushGowrap(n), &funcLiteral{})
			addExpectedDeadlock()
			v.FunctionLiteralNesting = v.FunctionLiteralNesting[:len(v.FunctionLiteralNesting)-1].PopGowrap()
			return nil
		}
	case *ast.DeferStmt:
		switch {
		case isFunctionLiteral(n.Call.Fun) &&
			IsCallWithParameters(n.Call):
			v.FunctionLiteralNesting = v.FunctionLiteralNesting.PushGowrap(n)
			ast.Walk(v, n.Call.Fun)
			v.FunctionLiteralNesting = v.FunctionLiteralNesting.PopGowrap()
			for _, arg := range n.Call.Args {
				ast.Walk(v, arg)
			}
			return nil
		case isBuiltinFunction(n.Call.Fun),
			!isFunctionLiteral(n.Call) &&
				IsCallWithParameters(n.Call):
			ast.Walk(v, n.Call.Fun)
			for _, arg := range n.Call.Args {
				ast.Walk(v, arg)
			}
			addExpectedDeadlock()
			v.FunctionLiteralNesting = v.FunctionLiteralNesting.PushGowrap(n)
			v.FunctionLiteralNesting = v.FunctionLiteralNesting.PopGowrap()
			return nil
		}
	case *ast.FuncDecl:
		v.FunctionName = n.Name.Name
		v.Receiver = n.Recv
		functionScope(n.Body)
		v.FunctionName = ""
		return nil
	case *ast.FuncLit:
		functionScope(n.Type, n.Body)
		v.FunctionLiteralNesting.IncFun()
		return nil
	}
	addExpectedDeadlock()

	return v
}

// getDeadlockExpectations constructs a mapping of deadlock expectations for
// the given target. The target is the path of the test case package.
func getDeadlockExpectations(target string, c Config) (ExpectedDeadlocks, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, target, nil, parser.ParseComments)
	if err != nil {
		return ExpectedDeadlocks{}, fmt.Errorf("failed to parse directory: %v", err)
	}

	expectedDeadlocks := ExpectedDeadlocks{
		Config:    c,
		Target:    target,
		FileSet:   fset,
		Deadlocks: make([]ExpectedDeadlock, 0, len(pkgs)),
	}

	for _, p := range pkgs {
		comments := make([]*ast.CommentGroup, 0, len(p.Files))
		for _, f := range p.Files {
			comments = append(comments, f.Comments...)
		}

		vis := &expectedDeadlockVisitor{
			FileSet:    fset,
			Package:    p,
			CommentMap: ast.NewCommentMap(fset, p, comments),
		}
		ast.Walk(vis, p)
		expectedDeadlocks.Deadlocks = append(expectedDeadlocks.Deadlocks, vis.Deadlocks...)
	}

	return expectedDeadlocks, nil
}
