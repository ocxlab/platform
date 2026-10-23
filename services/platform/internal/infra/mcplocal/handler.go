// Package mcplocal owns the localhost development transport, not authorization
// policy. Each owner supplies its own typed principal, authenticator, and tools.
package mcplocal

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func Handler[P any](authenticate func(context.Context, string) (P, error), serverFor func(P) *mcp.Server, unauthenticated error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Request-ID", uuid.NewString())
		host := r.Host
		if parsed, _, err := net.SplitHostPort(host); err == nil {
			host = parsed
		}
		if host != "localhost" && host != "127.0.0.1" && host != "[::1]" && host != "::1" {
			boundaryError(w, 403, "forbidden")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			u, err := url.Parse(origin)
			if err != nil || u.Scheme != scheme || u.Host != r.Host || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
				boundaryError(w, 403, "forbidden")
				return
			}
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			boundaryError(w, 405, "method_not_allowed")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			boundaryError(w, 401, "unauthenticated")
			return
		}
		p, err := authenticate(ctx, strings.TrimPrefix(header, "Bearer "))
		if errors.Is(err, unauthenticated) {
			boundaryError(w, 401, "unauthenticated")
			return
		}
		if err != nil {
			boundaryError(w, 503, "unavailable")
			return
		}
		server := serverFor(p)
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: 262144, PropagateRequestCancellation: true})
		handler.ServeHTTP(privateResponse{w}, r.WithContext(ctx))
	})
}

type privateResponse struct{ http.ResponseWriter }

func (w privateResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w privateResponse) WriteHeader(status int) {
	w.Header().Set("Cache-Control", "no-store")
	w.ResponseWriter.WriteHeader(status)
}
func (w privateResponse) Write(data []byte) (int, error) {
	w.Header().Set("Cache-Control", "no-store")
	return w.ResponseWriter.Write(data)
}
func boundaryError(w http.ResponseWriter, status int, code string) {
	if status == 401 {
		w.Header().Set("WWW-Authenticate", `Bearer realm="local-mcp"`)
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "status": status, "title": http.StatusText(status), "code": code, "instance": "urn:uuid:" + w.Header().Get("X-Request-ID")})
}
