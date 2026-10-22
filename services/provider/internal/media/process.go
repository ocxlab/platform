package media

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

var ErrInvalid = errors.New("invalid image upload")

// Derivative validates bounded input before allocating decoded pixels, strips
// source metadata by re-encoding, and fits the image within 960 pixels.
// It is not an antivirus service or an ingredient/allergen verification.
func Derivative(raw []byte, mediaType, checksum string, size int64) ([]byte, int, int, error) {
	if size < 1 || size > MaxUploadBytes || int64(len(raw)) != size {
		return nil, 0, 0, ErrInvalid
	}
	sum := sha256.Sum256(raw)
	if base64.StdEncoding.EncodeToString(sum[:]) != checksum {
		return nil, 0, 0, ErrInvalid
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	expected := map[string]string{"jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp"}
	if err != nil || expected[format] != mediaType || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
		return nil, 0, 0, ErrInvalid
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, 0, ErrInvalid
	}
	w, h := cfg.Width, cfg.Height
	if w > 960 || h > 960 {
		if w >= h {
			h = max(1, h*960/w)
			w = 960
		} else {
			w = max(1, w*960/h)
			h = 960
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	var result bytes.Buffer
	if err := jpeg.Encode(&result, dst, &jpeg.Options{Quality: 80}); err != nil {
		return nil, 0, 0, err
	}
	return result.Bytes(), w, h, nil
}
