package httpapi

import "testing"

func TestValidateJSONText(t *testing.T) {
	for _, input := range []string{`{"name":"Döner العربية"}`, `{"name":"literal \\u0000"}`, `{"price":1e999}`, `{}`} {
		if err := validateJSONText([]byte(input)); err != nil {
			t.Errorf("valid input %q: %v", input, err)
		}
	}
	for _, input := range []string{`{"name":"bad\u0000name"}`, `{"document":{"nested":["\u0000"]}}`, `{"\u0000":"key"}`, `{"broken":`} {
		if err := validateJSONText([]byte(input)); err == nil {
			t.Errorf("accepted invalid input %q", input)
		}
	}
}
