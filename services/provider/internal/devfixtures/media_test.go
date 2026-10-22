package devfixtures_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"ocp.local/platform/internal/devfixtures"
)

func TestEveryDemoProductHasAnOptimizedProvenancedPhoto(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "assets", "demo", "food")
	read := func(name string, into any) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(data, into); err != nil {
			t.Fatal(err)
		}
	}
	var manifest []struct {
		ID         string `json:"item_id"`
		File       string `json:"file"`
		Hash       string `json:"sha256"`
		SourceHash string `json:"source_png_sha256"`
		Bytes      int    `json:"bytes"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		Synthetic  bool   `json:"synthetic_demo"`
	}
	read("manifest.json", &manifest)
	prompts := map[string]string{}
	for _, name := range []string{"prompts.json", "additional-prompts.json"} {
		var batch struct {
			Prompts []struct {
				ID     string `json:"item_id"`
				Prompt string `json:"prompt"`
			} `json:"prompts"`
		}
		read(name, &batch)
		for _, p := range batch.Prompts {
			if prompts[p.ID] != "" {
				t.Fatal("duplicate prompt", p.ID)
			}
			prompts[p.ID] = p.Prompt
		}
	}
	var remaining []struct {
		ID     string `json:"id"`
		Prompt string `json:"prompt"`
	}
	read("remaining-prompts.json", &remaining)
	for _, p := range remaining {
		if prompts[p.ID] != "" {
			t.Fatal("duplicate prompt", p.ID)
		}
		prompts[p.ID] = p.Prompt
	}
	_, products, err := devfixtures.Restaurant()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest) != len(products) || len(prompts) != len(products) {
		t.Fatal("fixture, manifest, and prompt coverage differ")
	}
	seen := map[string]bool{}
	for _, a := range manifest {
		if seen[a.ID] || a.File != a.ID+".jpg" || !a.Synthetic || len(a.SourceHash) != 64 || prompts[a.ID] == "" {
			t.Fatal("invalid provenance", a.ID)
		}
		seen[a.ID] = true
		data, err := os.ReadFile(filepath.Join(root, a.File))
		if err != nil {
			t.Fatal(err)
		}
		config, err := jpeg.DecodeConfig(bytes.NewReader(data))
		if err != nil || len(data) != a.Bytes || len(data) > 512*1024 || fmt.Sprintf("%x", sha256.Sum256(data)) != a.Hash || config.Width != a.Width || config.Height != a.Height || config.Width > 768 || config.Height > 768 {
			t.Fatal("invalid optimized photo", a.ID)
		}
	}
	for _, p := range products {
		if !seen[p.ID] {
			t.Fatal("missing photo", p.ID)
		}
	}
}
