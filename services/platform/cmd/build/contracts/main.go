// contracts validates local modular sources and produces a self-contained bundle.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func main() {
	source := flag.String("source", "../../contracts/openapi/v1/openapi.yaml", "source entry document")
	output := flag.String("output", "../../contracts/build/provider.openapi.json", "derived bundle")
	flag.Parse()
	if err := run(*source, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(source, output string) error {
	absolute, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	root := filepath.Dir(absolute)
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	// Contract generation must not fetch remote references or read outside v1.
	loader.ReadFromURIFunc = func(_ *openapi3.Loader, location *url.URL) ([]byte, error) {
		if location.Scheme != "" || location.Host != "" {
			return nil, fmt.Errorf("remote reference prohibited: %s", location)
		}
		path, err := filepath.Abs(location.Path)
		if err != nil {
			return nil, err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("reference escapes contract root")
		}
		realPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, err
		}
		relative, err = filepath.Rel(realRoot, realPath)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("reference symlink escapes contract root")
		}
		return os.ReadFile(path)
	}
	doc, err := loader.LoadFromFile(absolute)
	if err != nil {
		return err
	}
	if err := doc.Validate(context.Background()); err != nil {
		return err
	}
	// The upstream default resolver can loop on unmatched absolute root paths.
	// A deterministic, collision-resistant local-reference name avoids that path
	// heuristic and keeps bundles independent of checkout location.
	names := map[string]string{}
	for name, ref := range doc.Components.Schemas {
		if ref.RefPath() != nil {
			if previous, found := names[ref.RefPath().String()]; found {
				return fmt.Errorf("schema aliases %s and %s refer to the same source; keep one canonical name", previous, name)
			}
			names[ref.RefPath().String()] = name
		}
	}
	// Root aliases are materialized before their references are internalized, so
	// reusing a public schema name cannot accidentally create a self-reference.
	for _, ref := range doc.Components.Schemas {
		ref.Ref = ""
	}
	for _, ref := range doc.Components.SecuritySchemes {
		ref.Ref = ""
	}
	for name, ref := range doc.Components.Parameters {
		if ref.RefPath() != nil {
			names[ref.RefPath().String()] = name
		}
		ref.Ref = ""
	}
	for name, ref := range doc.Components.Responses {
		if ref.RefPath() != nil {
			names[ref.RefPath().String()] = name
		}
		ref.Ref = ""
	}
	for name, ref := range doc.Components.Headers {
		if ref.RefPath() != nil {
			names[ref.RefPath().String()] = name
		}
		ref.Ref = ""
	}

	doc.InternalizeRefs(context.Background(), func(doc *openapi3.T, ref openapi3.ComponentRef) string {
		if ref.RefPath() != nil {
			if name, known := names[ref.RefPath().String()]; known {
				return name
			}
		}
		if name, found := openapi3.ReferencesComponentInRootDocument(doc, ref); found {
			parts := strings.Split(name, "/")
			return parts[len(parts)-1]
		}
		location := ref.RefPath()
		relative, _ := filepath.Rel(root, location.Path)
		identity := filepath.ToSlash(relative) + "#" + location.Fragment
		words := regexp.MustCompile(`[^a-zA-Z0-9]+`).Split(identity, -1)
		var name strings.Builder
		for _, word := range words {
			if word != "" {
				name.WriteString(strings.ToUpper(word[:1]) + word[1:])
			}
		}
		digest := sha256.Sum256([]byte(identity))
		return fmt.Sprintf("%s%x", name.String(), digest[:4])
	})
	if err := doc.Validate(context.Background()); err != nil {
		return err
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	// Validate serialized output as a new consumer would, not just the resolver's
	// in-memory graph. This catches alias/self-reference mistakes before codegen.
	bundled, err := openapi3.NewLoader().LoadFromData(body)
	if err != nil {
		return err
	}
	if err := bundled.Validate(context.Background()); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(output, append(body, '\n'), 0644); err != nil {
		return err
	}
	fmt.Println("Validated modular contract and wrote", output)
	return nil
}
