package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Source ownership is checked without inspecting or depending on generated Go.
func TestDomainDependencyDirection(t *testing.T) {
	for _, owner := range []string{"core", "food", "shopping", "lodging", "payment", "discovery", "infra"} {
		err := filepath.WalkDir(filepath.Join("..", "..", "internal", owner), func(path string, entry fs.DirEntry, walkErr error) error {
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
				if forbiddenDependency(owner, dependency) {
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

func forbiddenDependency(owner, dependency string) bool {
	const module = "ocp.local/platform/"
	if !strings.HasPrefix(dependency, module) {
		return false
	}
	const root = module + "internal/"
	if !strings.HasPrefix(dependency, root) {
		// Executable transport composition and development/test helpers may
		// use business modules, never the other way around.
		return true
	}
	dependency = strings.TrimPrefix(dependency, root)
	// A domain may call an owned application interface, but must never bypass
	// it by importing another owner's generated persistence implementation.
	parts := strings.Split(dependency, "/")
	if len(parts) > 1 && parts[0] != owner && slices.Contains(parts[1:], "sqlgen") {
		return true
	}
	// Telemetry consumes shared route/error classifications, not core behavior.
	if owner == "infra" && !strings.HasPrefix(dependency, "infra/") && dependency != "core/capability" && dependency != "core/problems" {
		return true
	}
	for _, domain := range []string{"food", "shopping", "lodging"} {
		if owner != domain && (dependency == domain || strings.HasPrefix(dependency, domain+"/")) {
			return true
		}
	}
	return false
}

func TestInternalDirectoryOwners(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "..", "internal"))
	if err != nil {
		t.Fatal(err)
	}
	var owners []string
	for _, entry := range entries {
		if entry.IsDir() {
			owners = append(owners, entry.Name())
		}
	}
	expected := []string{"core", "discovery", "food", "infra", "lodging", "payment", "shopping"}
	if !slices.Equal(owners, expected) {
		t.Fatalf("internal owners = %v, want %v; nest new packages beneath their owner", owners, expected)
	}
}

func TestCommandEntryPointLayout(t *testing.T) {
	root := filepath.Join("..", "..", "cmd")
	var commands []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && entry.Name() == "main.go" {
			relative, err := filepath.Rel(root, filepath.Dir(path))
			if err != nil {
				return err
			}
			commands = append(commands, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{
		"api", "build/contracts", "dev/account", "dev/customer-mcp", "dev/provision", "dev/provision-internal",
		"dev/seed-demo", "dev/seed-demo-media", "ops/migrate",
		"test/browser-fixture", "test/check-local", "test/migration-replay", "test/network-lab",
	}
	if !slices.Equal(commands, expected) {
		t.Fatalf("command entry points = %v, want %v; group commands by purpose", commands, expected)
	}
}

func TestDependencyGuardRejectsReverseAndForeignImports(t *testing.T) {
	for _, tc := range []struct {
		owner, dependency string
		forbidden         bool
	}{
		{"core", "ocp.local/platform/internal/food/offering", true},
		{"food", "ocp.local/platform/internal/shopping", true},
		{"food", "ocp.local/platform/cmd/api/transport/httpapi", true},
		{"core", "ocp.local/platform/tests/devsetup", true},
		{"infra", "ocp.local/platform/internal/core/persistence", true},
		{"infra", "ocp.local/platform/internal/core/capability", false},
		{"food", "ocp.local/platform/internal/core/offering", false},
		{"discovery", "ocp.local/platform/internal/core/persistence", false},
		{"food", "ocp.local/platform/internal/core/persistence/sqlgen", true},
		{"shopping", "ocp.local/platform/internal/payment/persistence/sqlgen", true},
		{"core", "ocp.local/platform/internal/discovery/persistence/sqlgen", true},
		{"payment", "ocp.local/platform/internal/payment/persistence/sqlgen", false},
	} {
		if got := forbiddenDependency(tc.owner, tc.dependency); got != tc.forbidden {
			t.Errorf("%s -> %s: forbidden = %v, want %v", tc.owner, tc.dependency, got, tc.forbidden)
		}
	}
}
