package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestMerchantMCPRequiresExplicitLocalEnablement(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []Option
		host    string
		status  int
	}{
		{"disabled", nil, "localhost:18180", 404},
		{"local_auth_required", []Option{WithLocalMerchantMCP()}, "localhost:18180", 401},
		{"foreign_host_denied", []Option{WithLocalMerchantMCP()}, "public.example", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := NewHandler(nil, tc.options...)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("POST", "http://localhost:18180/mcp/merchant", nil)
			r.Host = tc.host
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.status || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}
