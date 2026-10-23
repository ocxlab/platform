package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModularBundleIsDeterministic(t *testing.T) {
	source := "../../../../contracts/openapi/v1/openapi.yaml"
	dir := t.TempDir()
	one := filepath.Join(dir, "one.json")
	two := filepath.Join(dir, "two.json")
	if err := run(source, one); err != nil {
		t.Fatal(err)
	}
	if err := run(source, two); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(one)
	b, _ := os.ReadFile(two)
	if string(a) != string(b) {
		t.Fatal("nondeterministic bundle")
	}
	if strings.Contains(string(a), `"$ref": "paths/`) {
		t.Fatal("external reference remains")
	}
}
func TestUnsafeReferencesRejected(t *testing.T) {
	for _, reference := range []string{"https://example.invalid/schema.yaml", "../outside.yaml"} {
		t.Run(reference, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "api.yaml")
			body := "openapi: 3.0.3\ninfo: {title: Test, version: 1.0.0}\npaths: {}\ncomponents:\n  schemas:\n    Unsafe:\n      $ref: '" + reference + "'\n"
			if err := os.WriteFile(source, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if err := run(source, filepath.Join(dir, "out.json")); err == nil {
				t.Fatal("unsafe reference accepted")
			}
		})
	}
}
