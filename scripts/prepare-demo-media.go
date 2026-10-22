// This asset-preparation utility uses only the Go standard library. It is not
// part of the provider runtime or a general merchant-upload processing service.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	if err := prepare(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func prepare() error {
	const root = "/workspace"
	out := filepath.Join(root, "assets", "demo", "food")
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	var records []map[string]any
	for _, id := range []string{"doener", "pizza-margherita", "falafel"} {
		raw, err := os.ReadFile(filepath.Join(root, "working", "demo-media", id+".png"))
		if err != nil {
			return err
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			return err
		}
		var encoded bytes.Buffer
		if err := jpeg.Encode(&encoded, img, &jpeg.Options{Quality: 82}); err != nil {
			return err
		}
		data := encoded.Bytes()
		if len(data) > 512*1024 {
			return fmt.Errorf("%s exceeds the demo asset size limit", id)
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(data))
		name := id + ".jpg"
		if err := os.WriteFile(filepath.Join(out, name), data, 0644); err != nil {
			return err
		}
		records = append(records, map[string]any{
			"item_id": id, "file": name, "content_type": "image/jpeg",
			"bytes": len(data), "width": img.Bounds().Dx(), "height": img.Bounds().Dy(),
			"sha256": hash, "source_png_sha256": fmt.Sprintf("%x", sha256.Sum256(raw)),
			"object_key": "demo/food/" + hash + ".jpg", "synthetic_demo": true,
		})
		fmt.Printf("%s: %d bytes\n", name, len(data))
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "manifest.json"), append(data, '\n'), 0644)
}
