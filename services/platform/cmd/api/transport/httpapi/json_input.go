package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// validateJSONText checks decoded keys and values, including opaque model documents.
// PostgreSQL text/jsonb cannot retain NUL; reject it before a storage operation.
func validateJSONText(data []byte) error {
	if !json.Valid(data) {
		return errors.New("expected valid JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if value, ok := token.(string); ok && strings.ContainsRune(value, 0) {
			return errors.New("JSON strings must not contain NUL")
		}
	}
}
