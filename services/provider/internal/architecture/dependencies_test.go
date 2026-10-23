package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Source ownership is checked without inspecting or depending on generated Go.
func TestDomainDependencyDirection(t *testing.T) {
	for _, owner := range []string{"core", "food", "shopping", "lodging", "payment", "database"} {
		err := filepath.WalkDir(filepath.Join("..", owner), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() && entry.Name() == "sqlgen" {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, item := range file.Imports {
				dependency, err := strconv.Unquote(item.Path.Value)
				if err != nil {
					return err
				}
				const root = "ocp.local/platform/internal/"
				if !strings.HasPrefix(dependency, root) {
					continue
				}
				dependency = strings.TrimPrefix(dependency, root)
				forbidden := owner == "database"
				for _, domain := range []string{"food", "shopping", "lodging"} {
					if owner != domain && (dependency == domain || strings.HasPrefix(dependency, domain+"/")) {
						forbidden = true
					}
				}
				if strings.HasPrefix(dependency, "httpapi") || strings.HasPrefix(dependency, "ucpinternal") || strings.HasPrefix(dependency, "persistence/") {
					forbidden = true
				}
				if forbidden {
					t.Errorf("%s imports foreign domain or transport %s", path, dependency)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
