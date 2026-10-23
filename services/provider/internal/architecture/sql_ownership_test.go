package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// Application SQL belongs in owner-specific query inputs. Only connection/role
// setup and the administrative local-data bootstrap keep direct database calls.
func TestApplicationSQLUsesGeneratedQueries(t *testing.T) {
	err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && (entry.Name() == "sqlgen" || entry.Name() == "database" || entry.Name() == "devsetup") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, method := range directSQLMethods(file) {
			t.Errorf("%s calls %s directly; add a query to its owner's persistence/queries", path, method)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func directSQLMethods(file *ast.File) []string {
	var methods []string
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch selector.Sel.Name {
		case "ExecContext", "QueryContext", "QueryRowContext", "PrepareContext":
			methods = append(methods, selector.Sel.Name)
		}
		return true
	})
	return methods
}

func TestDirectSQLGuardDetectsVariableStatements(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "example.go", `package example
func run() { db.ExecContext(ctx, statement); tx.QueryRowContext(ctx, statement); tx.QueryContext(ctx, statement); db.PrepareContext(ctx, statement) }`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if methods := directSQLMethods(file); len(methods) != 4 {
		t.Fatalf("guard missed direct SQL calls: %v", methods)
	}
}
