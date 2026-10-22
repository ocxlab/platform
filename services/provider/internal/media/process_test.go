package media

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func checksum(raw []byte) string {
	s := sha256.Sum256(raw)
	return base64.StdEncoding.EncodeToString(s[:])
}
func TestDerivative(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1920, 960))
	src.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()
	data, w, h, err := Derivative(raw, "image/png", checksum(raw), int64(len(raw)))
	if err != nil || w != 960 || h != 480 {
		t.Fatalf("dimensions %dx%d: %v", w, h, err)
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != "jpeg" || img.Bounds().Dx() != 960 {
		t.Fatal("not JPEG", err)
	}
	// Transparent pixels are composited onto white, not black.
	r, g, b, _ := img.At(100, 100).RGBA()
	if r < 64000 || g < 64000 || b < 64000 {
		t.Fatal("wrong transparency background")
	}
	for _, tc := range []struct {
		name, kind, sum string
		size            int64
		raw             []byte
	}{
		{"size", "image/png", checksum(raw), int64(len(raw) + 1), raw},
		{"checksum", "image/png", checksum([]byte("different")), int64(len(raw)), raw},
		{"mime", "image/jpeg", checksum(raw), int64(len(raw)), raw},
		{"svg", "image/svg+xml", checksum([]byte("<svg/>")), 6, []byte("<svg/>")},
		{"empty", "image/png", checksum(nil), 0, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, _, err := Derivative(tc.raw, tc.kind, tc.sum, tc.size); err != ErrInvalid {
				t.Fatalf("accepted invalid input: %v", err)
			}
		})
	}
	// Forge a valid PNG header with an excessive pixel budget; reject it before
	// decoding any pixels or accepting the forged image's body.
	huge := bytes.Clone(raw)
	binary.BigEndian.PutUint32(huge[16:20], 8000)
	binary.BigEndian.PutUint32(huge[20:24], 8000)
	binary.BigEndian.PutUint32(huge[29:33], crc32.ChecksumIEEE(huge[12:29]))
	if _, _, _, err := Derivative(huge, "image/png", checksum(huge), int64(len(huge))); err != ErrInvalid {
		t.Fatal("pixel budget not checked", err)
	}
}
func TestStorageConfiguration(t *testing.T) {
	if storage, err := FromEnv(func(string) string { return "" }); err != nil || storage != nil {
		t.Fatal("disabled storage should be absent")
	}
	if _, err := FromEnv(func(key string) string {
		if key == "OCP_MEDIA_ENABLED" {
			return "true"
		}
		return ""
	}); err == nil {
		t.Fatal("missing credentials accepted")
	}
}
