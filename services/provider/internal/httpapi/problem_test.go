package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
)

func TestProblemEnvelopeMatchesContract(t *testing.T) {
	spec, err := GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		status int
		code   string
	}{
		{400, "invalid_request"}, {401, "unauthenticated"}, {403, "forbidden"},
		{404, "not_found"}, {409, "ordering_paused"}, {409, "ordering_revision_conflict"},
		{413, "body_too_large"}, {415, "unsupported_media_type"}, {503, "unavailable"},
	} {
		t.Run(test.code, func(t *testing.T) {
			response := httptest.NewRecorder()
			id := uuid.NewString()
			response.Header().Set("X-Request-ID", id)
			problem(response, test.status, test.code, "Safe corrective guidance.", nil)
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if response.Code != test.status || body["status"] != float64(test.status) || body["code"] != test.code || body["instance"] != "urn:uuid:"+id || body["title"] != http.StatusText(test.status) {
				t.Fatal("inconsistent problem envelope")
			}
			if response.Header().Get("Content-Type") != "application/problem+json" || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing problem headers")
			}
			if test.status == 401 && response.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing authentication challenge")
			}
			if err := spec.Components.Schemas["Problem"].Value.VisitJSON(body, openapi3.VisitAsResponse()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

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
