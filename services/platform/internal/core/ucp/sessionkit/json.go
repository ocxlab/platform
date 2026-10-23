package sessionkit

import (
	"bytes"
	"encoding/json"
	"io"
)

func Decode(raw []byte, into any) error {
	if len(raw) == 0 || len(raw) > 256*1024 {
		return ErrInput
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(into) != nil || d.Decode(new(any)) != io.EOF {
		return ErrInput
	}
	return nil
}
func JSON(value any) json.RawMessage { b, _ := json.Marshal(value); return b }

// Canonical recursively orders object keys while preserving JSON number text.
// It is a local replay fingerprint format, not a general signing standard.
func Canonical(raw []byte) (json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var object map[string]any
	if d.Decode(&object) != nil || object == nil || d.Decode(new(any)) != io.EOF {
		return nil, ErrInput
	}
	return json.Marshal(object)
}

type CatalogWrite struct {
	BaseRevision int64           `json:"base_revision"`
	Document     json.RawMessage `json:"document"`
}
