package main

import (
	"fmt"
	"go/ast"
	"strconv"
	"strings"
)

type (
	funcLiteral struct {
		fun    int
		gowrap int
	}

	gowrap struct{}

	literalToken interface {
		IsFunc() bool
		IsGoWrap() bool
		IncFun()
		IncGoWrap()
	}

	functionLiteralNesting []literalToken
)

func (*funcLiteral) IsFunc() bool {
	return true
}

func (*funcLiteral) IsGoWrap() bool {
	return false
}

func (gowrap) IsFunc() bool {
	return false
}

func (gowrap) IsGoWrap() bool {
	return true
}

func (f *funcLiteral) IncFun() {
	f.fun++
}

func (f *funcLiteral) IncGoWrap() {
	f.gowrap++
}

func (gowrap) IncFun()    {}
func (gowrap) IncGoWrap() {}

func (n functionLiteralNesting) String() string {
	nesting := make([]string, 0, len(n))
	for _, l := range n {
		switch l := l.(type) {
		case *funcLiteral:
			if l == nil {
				nesting = append(nesting, "nil")
				continue
			}
			nesting = append(nesting, "func<"+strconv.Itoa(l.fun)+","+strconv.Itoa(l.gowrap)+">")
		case gowrap:
			nesting = append(nesting, "gowrap")
		}
	}
	return fmt.Sprintf("Nesting: %v", strings.Join(nesting, "."))
}

func (n functionLiteralNesting) IncFun() {
	for i := len(n) - 1; i >= 0; i-- {
		if f, ok := n[i].(*funcLiteral); ok {
			f.IncFun()
			return
		}
	}
}

func (n functionLiteralNesting) IncGoWrap() {
	for i := len(n) - 1; i >= 0; i-- {
		if f, ok := n[i].(*funcLiteral); ok {
			f.IncGoWrap()
			return
		}
	}
}

func (n functionLiteralNesting) PushGowrap(node ast.Node) functionLiteralNesting {
	n = append(n, gowrap{})
	return n
}

func (n functionLiteralNesting) PopGowrap() functionLiteralNesting {
	if len(n) == 0 {
		return n
	}

	if _, ok := n[len(n)-1].(gowrap); ok {
		n.IncGoWrap()
		n = n[:len(n)-1]
	}

	return n
}

func (n functionLiteralNesting) FunctionSuffix() (functionName string) {
	funs := make([]*funcLiteral, 0, len(n))
	for _, l := range n {
		if l.IsFunc() {
			funs = append(funs, l.(*funcLiteral))
		}
	}
	if len(funs) < 2 {
		return ""
	}

	functionName = ""
	if len(funs) > 1 && n[len(n)-2].IsGoWrap() {
		var start int
		if f := funs[0]; len(funs) > 2 {
			functionName += ".func" + strconv.Itoa(f.fun)
			start = 1
		}

		fs := funs[start : len(funs)-1]
		for i, f := range fs {
			if i == len(fs)-1 {
				functionName += ".gowrap" + strconv.Itoa(f.gowrap)
			} else {
				functionName += "." + strconv.Itoa(f.fun)
			}
		}

		return functionName
	}

	functionName += ".func" + strconv.Itoa(funs[0].fun)
	for _, f := range funs[1 : len(funs)-1] {
		functionName += "." + strconv.Itoa(f.fun)
	}

	return functionName
}
