package ucpinternal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"ocp.local/platform/internal/database"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/google/uuid"
	"ocp.local/platform/internal/capability"
	"ocp.local/platform/internal/lodging"
	"ocp.local/platform/internal/problems"
	"ocp.local/platform/internal/sessionkit"
	"ocp.local/platform/internal/shopping"
	"ocp.local/platform/internal/telemetry"
	"ocp.local/platform/internal/ucpcontract"
)

type Config struct {
	Domains     string
	Endpoint    string
	Concurrency int
	Observer    *telemetry.Observer
}
type server struct {
	db       *sql.DB
	endpoint string
	domains  map[capability.Domain]sessionkit.Domain
	slots    map[capability.Domain]chan struct{}
	observer *telemetry.Observer
	router   routers.Router
}

func NewHandler(db *sql.DB, c Config) (http.Handler, error) {
	parsed, err := url.Parse(c.Endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" || parsed.RawPath != "" {
		return nil, sessionkit.ErrInput
	}
	if c.Concurrency < 1 || c.Concurrency > 20 {
		return nil, sessionkit.ErrInput
	}
	enabled, err := capability.Parse(c.Domains, []capability.Domain{capability.Shopping, capability.Lodging}, []capability.Domain{capability.Shopping, capability.Lodging})
	if err != nil {
		return nil, err
	}
	spec, err := ucpcontract.GetSwagger()
	if err != nil {
		return nil, err
	}
	spec.Servers = nil
	router, err := legacy.NewRouter(spec)
	if err != nil {
		return nil, err
	}
	s := &server{db: db, endpoint: c.Endpoint, domains: map[capability.Domain]sessionkit.Domain{}, slots: map[capability.Domain]chan struct{}{}, observer: c.Observer, router: router}
	for _, d := range enabled.Enabled() {
		if d == capability.Shopping {
			s.domains[d] = shopping.Service{OrderBaseURL: c.Endpoint + "/shopping"}
		} else {
			s.domains[d] = lodging.Service{}
		}
		s.slots[d] = make(chan struct{}, c.Concurrency)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.observer == nil {
			s.serve(w, r)
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		domain := capability.Domain(parts[0])
		action, _, route, known := operation(r, parts, domain)
		if !known || s.domains[domain] == nil {
			action, route = "other", "/_OTHER"
			domain = ""
		}
		for _, fixed := range []string{"/health/live", "/.well-known/ucp", "/schemas/internal/pay-later.json"} {
			if r.URL.Path == fixed {
				action, route, domain = "profile", fixed, ""
				break
			}
		}
		// Instrument admission failures too, using templates rather than raw IDs.
		s.observer.HTTP(w, r, route, "ucp_"+action, s.serve, domain)
	}), nil
}
func failure(w http.ResponseWriter, status int, code string) {
	definition, ok := problems.Lookup(code)
	if !ok || definition.Status != status {
		status = 503
		code = "unavailable"
		definition, _ = problems.Lookup(code)
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": definition.Title, "status": status, "code": code, "detail": definition.Title, "instance": "urn:uuid:" + w.Header().Get("Request-Id")})
}
func respond(w http.ResponseWriter, r sessionkit.Reply) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.Status)
	_, _ = w.Write(r.Body)
}
func (s *server) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Request-Id", uuid.NewString())
	if r.URL.Path == "/health/live" && r.Method == "GET" {
		respond(w, sessionkit.Reply{Status: 200, Body: []byte(`{"status":"live"}`)})
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	domain := capability.Domain(parts[0])
	adapter := s.domains[domain]
	profileRoute := r.URL.Path == "/.well-known/ucp" && r.Method == "GET"
	schemaRoute := r.URL.Path == "/schemas/internal/pay-later.json" && r.Method == "GET"
	if adapter == nil && !profileRoute && !schemaRoute {
		failure(w, 404, "not_found")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	p, profile, management, err := authenticate(ctx, s.db, r.Header.Get("Authorization"))
	if err != nil {
		if errors.Is(err, sessionkit.ErrNotFound) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="ocp-internal"`)
			failure(w, 401, "unauthenticated")
		} else {
			failure(w, 503, "unavailable")
		}
		return
	}
	if s.domains[p.Domain] == nil {
		failure(w, 404, "not_found")
		return
	}
	if profileRoute {
		business := ControlledProfile(p.Domain)
		business.UCP.PaymentHandlers[sessionkit.PayLaterHandler][0].Schema = s.endpoint + "/schemas/internal/pay-later.json"
		business.UCP.PaymentHandlers[sessionkit.PayLaterHandler][0].Spec = s.endpoint + "/schemas/internal/pay-later.json"
		business.UCP.Services["dev.ucp."+string(p.Domain)] = []sessionkit.Entity{{Version: sessionkit.Version, Transport: "rest", Endpoint: s.endpoint + "/" + string(p.Domain), Schema: "https://ucp.dev/services/" + string(p.Domain) + "/rest.openapi.json", Spec: "https://ucp.dev/specification/" + string(p.Domain)}}
		respond(w, sessionkit.Reply{Status: 200, Body: sessionkit.JSON(business)})
		return
	}
	if schemaRoute {
		respond(w, sessionkit.Reply{Status: 200, Body: []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","title":"Internal pay later instrument","type":"object","required":["id","handler_id","type"],"properties":{"id":{"const":"pay-later"},"handler_id":{"const":"ocp-pay-later"},"type":{"const":"pay_later"}}}`)})
		return
	}
	if p.Domain != domain {
		failure(w, 404, "not_found")
		return
	}
	header := agentHeader.FindStringSubmatch(r.Header.Get("UCP-Agent"))
	if len(header) != 2 || !validProfileURI(header[1]) || header[1] != p.Profile {
		failure(w, 400, "invalid_request")
		return
	}
	// Signature authentication is deliberately not selected in this bearer cohort.
	if r.Header.Get("Signature") != "" || r.Header.Get("Signature-Input") != "" || r.Header.Get("Content-Digest") != "" || r.Header.Get("X-API-Key") != "" {
		failure(w, 400, "invalid_request")
		return
	}
	requestID, err := uuid.Parse(r.Header.Get("Request-Id"))
	if err != nil {
		failure(w, 400, "invalid_request")
		return
	}
	w.Header().Set("Request-Id", requestID.String())
	if accept := r.Header.Get("Accept"); accept != "" && accept != "application/json" && accept != "*/*" {
		failure(w, 406, "unsupported_accept")
		return
	}
	if len(profile.UCP.Capabilities[sessionkit.Capability(domain)]) == 0 {
		respond(w, sessionkit.Reply{Status: 200, Body: sessionkit.JSON(map[string]any{"ucp": map[string]any{"version": sessionkit.Version, "status": "error"}, "messages": []sessionkit.Message{{Type: "error", Code: "capability_not_supported", Content: "No common capability is configured.", Severity: "unrecoverable"}}})})
		return
	}
	select {
	case s.slots[domain] <- struct{}{}:
		defer func() { <-s.slots[domain] }()
	default:
		w.Header().Set("Retry-After", "1")
		failure(w, 429, "internal_capacity")
		return
	}
	// Derive only bounded route/operation labels; identifiers and bodies stay out
	// of telemetry. The domain adapter below remains the authoritative operation.
	action, id, _, ok := operation(r, parts, domain)
	if !ok {
		failure(w, 404, "not_found")
		return
	}
	s.execute(w, r, p, adapter, management, action, id)
}
func operation(r *http.Request, parts []string, d capability.Domain) (string, uuid.UUID, string, bool) {
	if d == capability.Shopping && len(parts) == 3 && parts[1] == "orders" && r.Method == "GET" {
		id, err := uuid.Parse(parts[2])
		return "order", id, "/shopping/orders/{id}", err == nil
	}
	root := "checkout-sessions"
	if d == capability.Lodging {
		root = "booking-sessions"
	}
	prefix := "/" + string(d) + "/" + root
	if len(parts) == 2 && parts[1] == "catalog" && r.Method == "PUT" {
		return "catalog", uuid.Nil, "/" + string(d) + "/catalog", true
	}
	if len(parts) < 2 || parts[1] != root {
		return "", uuid.Nil, "", false
	}
	if len(parts) == 2 && r.Method == "POST" {
		return "create", uuid.Nil, prefix, true
	}
	if len(parts) < 3 {
		return "", uuid.Nil, "", false
	}
	id, err := uuid.Parse(parts[2])
	if err != nil {
		return "", uuid.Nil, "", false
	}
	if len(parts) == 3 {
		if r.Method == "GET" {
			return "get", id, prefix + "/{id}", true
		}
		if r.Method == "PUT" {
			return "update", id, prefix + "/{id}", true
		}
	}
	if len(parts) == 4 && r.Method == "POST" && (parts[3] == "complete" || parts[3] == "cancel") {
		return parts[3], id, prefix + "/{id}/" + parts[3], true
	}
	return "", uuid.Nil, "", false
}
func (s *server) execute(w http.ResponseWriter, r *http.Request, p sessionkit.Principal, adapter sessionkit.Domain, management bool, action string, id uuid.UUID) {
	if (action == "catalog") != management {
		failure(w, 403, "forbidden")
		return
	}
	var raw []byte
	if action != "get" && action != "order" {
		kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || kind != "application/json" {
			failure(w, 415, "unsupported_media_type")
			return
		}
		raw, err = io.ReadAll(http.MaxBytesReader(w, r.Body, 256*1024))
		if err != nil {
			failure(w, 413, "body_too_large")
			return
		}
		if action == "cancel" && len(raw) == 0 {
			raw = []byte(`{}`)
		}
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil || obj == nil {
			failure(w, 400, "invalid_request")
			return
		}
		raw, err = sessionkit.Canonical(raw)
		if err != nil {
			failure(w, 400, "invalid_request")
			return
		}
		if action == "cancel" && len(obj) != 0 {
			failure(w, 400, "invalid_request")
			return
		}
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	r.ContentLength = int64(len(raw))
	route, params, err := s.router.FindRoute(r)
	if err != nil {
		failure(w, 404, "not_found")
		return
	}
	if openapi3filter.ValidateRequest(r.Context(), &openapi3filter.RequestValidationInput{Request: r, PathParams: params, Route: route, Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}}) != nil {
		failure(w, 400, "invalid_request")
		return
	}
	result, err := s.transaction(r.Context(), p, adapter, action, id, r.Header.Get("Idempotency-Key"), raw)
	switch {
	case err == nil:
		respond(w, result)
	case errors.Is(err, sessionkit.ErrInput):
		failure(w, 400, "invalid_request")
	case errors.Is(err, sessionkit.ErrNotFound):
		failure(w, 404, "not_found")
	case errors.Is(err, sessionkit.ErrConflict):
		failure(w, 409, "idempotency_conflict")
	default:
		failure(w, 503, "unavailable")
	}
}
func (s *server) transaction(ctx context.Context, p sessionkit.Principal, adapter sessionkit.Domain, action string, id uuid.UUID, key string, raw []byte) (sessionkit.Reply, error) {
	tx, err := database.TenantTransaction(ctx, s.db, p.MerchantID)
	if err != nil {
		return sessionkit.Reply{}, err
	}
	defer tx.Rollback()
	var digest [32]byte
	var replayKey uuid.UUID
	if action != "get" && action != "order" {
		replayKey, err = uuid.Parse(key)
		if err != nil {
			return sessionkit.Reply{}, sessionkit.ErrInput
		}
		digest = sha256.Sum256(raw)
		lock := sessionkit.JSON([]string{p.ID.String(), string(p.Domain), action, id.String(), replayKey.String()})
		if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, string(lock)); err != nil {
			return sessionkit.Reply{}, err
		}
		var stored []byte
		var previous sessionkit.Reply
		err = tx.QueryRowContext(ctx, `SELECT digest,response_status,response_body FROM commerce_internal.replays WHERE merchant_id=$1 AND grant_id=$2 AND domain=$3 AND operation=$4 AND resource=$5 AND replay_key=$6`, p.MerchantID, p.ID, p.Domain, action, id.String(), replayKey).Scan(&stored, &previous.Status, &previous.Body)
		if err == nil {
			if !bytes.Equal(stored, digest[:]) {
				return sessionkit.Reply{}, sessionkit.ErrConflict
			}
			return previous, tx.Commit()
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return sessionkit.Reply{}, err
		}
	}
	var result sessionkit.Reply
	if action == "catalog" {
		result.Status = 200
		result.Body, err = adapter.Catalog(ctx, tx, p, raw)
	} else {
		result, err = adapter.Execute(ctx, tx, p, action, id, raw)
	}
	if err != nil {
		return result, err
	}
	if action != "get" && action != "order" {
		_, err = tx.ExecContext(ctx, `INSERT INTO commerce_internal.replays(merchant_id,grant_id,domain,operation,resource,replay_key,digest,response_status,response_body) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, p.MerchantID, p.ID, p.Domain, action, id.String(), replayKey, digest[:], result.Status, []byte(result.Body))
		if err != nil {
			return result, err
		}
	}
	return result, tx.Commit()
}
