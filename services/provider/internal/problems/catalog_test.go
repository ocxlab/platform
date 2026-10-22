package problems

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
)

func TestErrorCallSitesAreRegistered(t *testing.T) {
	files, err := filepath.Glob("../httpapi/*.go")
	if err != nil {
		t.Fatal(err)
	}
	internal, err := filepath.Glob("../ucpinternal/*.go")
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, internal...)
	for _, file := range files {
		tree, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(tree, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, ok := call.Fun.(*ast.Ident)
			if !ok || (name.Name != "problem" && name.Name != "failure") || len(call.Args) < 3 {
				return true
			}
			code, ok := call.Args[2].(*ast.BasicLit)
			if !ok {
				return true
			}
			value, err := strconv.Unquote(code.Value)
			if err != nil {
				t.Error(err)
				return true
			}
			metadata, ok := Lookup(value)
			if !ok {
				t.Errorf("unregistered code %s in %s", value, file)
				return true
			}
			if status, ok := call.Args[1].(*ast.BasicLit); ok {
				number, _ := strconv.Atoi(status.Value)
				if number != metadata.Status {
					t.Errorf("wrong status for %s", value)
				}
			}
			return true
		})
	}
}
