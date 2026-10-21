package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestInvalidProblemMappingFailsClosed(t *testing.T) {
	for _, test := range []struct {
		code   string
		status int
	}{{"not_registered", 400}, {"quote_expired", 500}} {
		response := httptest.NewRecorder()
		problem(response, test.status, test.code, "private internal error", nil)
		var body map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != 503 || body["code"] != "unavailable" || body["detail"] == "private internal error" {
			t.Fatal("invalid mapping was exposed")
		}
	}
}
