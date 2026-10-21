package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/google/uuid"
	"ocp.local/platform/internal/offering"
	"ocp.local/platform/internal/offering/food"
	"ocp.local/platform/internal/persistence"
)

type principalKey struct{}
type Server struct {
	db     *sql.DB
	models *offering.Registry
}

// NewHandler installs the generated routing and contract validation around the
// application operations. Administrative grants are never accepted as DB roles.
func NewHandler(db *sql.DB) (http.Handler, error) {
	spec, err := GetSwagger()
	if err != nil {
		return nil, err
	}
	router, err := legacy.NewRouter(spec)
	if err != nil {
		return nil, err
	}
	server := &Server{db: db, models: offering.NewRegistry(map[offering.Model]offering.Validator{
		food.Model: food.Validator(spec.Components.Schemas["FoodMenu"].Value),
	})}
	generated := HandlerWithOptions(server, StdHTTPServerOptions{ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
		problem(w, 400, "invalid_request", "Request does not match the API contract.", nil)
	}})
	scopes := map[string]string{}
	known := map[string]bool{"store:write": true, "store:read": true, "offering:read": true, "offering:write": true}
	for _, path := range spec.Paths.Map() {
		for _, operation := range path.Operations() {
			if operation.OperationID == "GetLiveness" {
				continue
			}
			scope, ok := operation.Extensions["x-required-scope"].(string)
			if !ok || !known[scope] {
				return nil, errors.New("protected operation has no recognized action scope")
			}
			scopes[operation.OperationID] = scope
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", uuid.NewString())
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		w.Header().Set("Cache-Control", "no-store")
		route, pathParams, err := router.FindRoute(r)
		if err != nil {
			problem(w, 404, "not_found", "API operation not found.", nil)
			return
		}
		if route.Operation.OperationID != "GetLiveness" {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				problem(w, 401, "unauthenticated", "A valid bearer credential is required.", nil)
				return
			}
			principal, err := persistence.Authenticate(r.Context(), db, strings.TrimPrefix(header, "Bearer "))
			if errors.Is(err, persistence.ErrUnauthenticated) {
				problem(w, 401, "unauthenticated", "A valid bearer credential is required.", nil)
				return
			}
			if err != nil {
				problem(w, 503, "unavailable", "Credential service is temporarily unavailable.", nil)
				return
			}
			merchant, err := uuid.Parse(pathParams["merchant_id"])
			if err != nil {
				problem(w, 400, "invalid_request", "Merchant identifier must be a UUID.", nil)
				return
			}
			if principal.MerchantID != merchant || !principal.Allows(scopes[route.Operation.OperationID]) {
				problem(w, 403, "forbidden", "Credential does not authorize this merchant and action.", nil)
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), principalKey{}, principal))
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, 262144)
			data, err := io.ReadAll(r.Body)
			if err != nil {
				var tooLarge *http.MaxBytesError
				if errors.As(err, &tooLarge) {
					problem(w, 413, "body_too_large", "Request body exceeds 262144 bytes.", nil)
				} else {
					problem(w, 400, "invalid_request", "Cannot read request body.", nil)
				}
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(data))
		}
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || media != "application/json" {
				problem(w, 415, "unsupported_media_type", "Request body must use application/json.", nil)
				return
			}
		}
		input := &openapi3filter.RequestValidationInput{Request: r, PathParams: pathParams, Route: route,
			Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}}
		if err := openapi3filter.ValidateRequest(r.Context(), input); err != nil {
			problem(w, 400, "invalid_request", "Request does not match the API contract or size limit.", nil)
			return
		}
		generated.ServeHTTP(w, r)
	}), nil
}

func principal(r *http.Request) persistence.Principal {
	return r.Context().Value(principalKey{}).(persistence.Principal)
}
func decode(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("expected one JSON document")
	}
	return nil
}
func problem(w http.ResponseWriter, status int, code, detail string, current *int64) {
	body := map[string]any{"type": "about:blank", "title": http.StatusText(status), "status": status, "code": code, "detail": detail, "instance": "urn:uuid:" + w.Header().Get("X-Request-ID")}
	if current != nil {
		body["current_revision"] = *current
	}
	if status == 401 {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func write(w http.ResponseWriter, value any, err error) {
	if err != nil {
		var conflict *persistence.RevisionConflict
		switch {
		case errors.Is(err, persistence.ErrNotFound):
			problem(w, 404, "not_found", "Resource not found.", nil)
		case errors.Is(err, persistence.ErrReplayConflict):
			problem(w, 409, "idempotency_conflict", "This key was used with different request content.", nil)
		case errors.As(err, &conflict):
			problem(w, 409, "revision_conflict", "Offering revision has changed.", &conflict.Current)
		default:
			problem(w, 503, "unavailable", "Storage is temporarily unavailable.", nil)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if result, ok := value.(persistence.Result); ok {
		w.WriteHeader(result.Status)
		_, _ = w.Write(result.Body)
		return
	}
	_ = json.NewEncoder(w).Encode(value)
}
func (s *Server) GetLiveness(w http.ResponseWriter, r *http.Request) {
	write(w, map[string]string{"status": "ok", "service": "ocp-api"}, nil)
}
func (s *Server) CreateStore(w http.ResponseWriter, r *http.Request, merchant uuid.UUID, params CreateStoreParams) {
	var body CreateStoreJSONRequestBody
	if decode(r, &body) != nil || strings.TrimSpace(body.DisplayName) == "" {
		problem(w, 400, "invalid_request", "A nonempty display name is required.", nil)
		return
	}
	canonical, _ := json.Marshal(body)
	result, err := persistence.Mutate(r.Context(), s.db, principal(r), "createStore", merchant.String(), params.IdempotencyKey, canonical, func(tx *sql.Tx) (any, error) {
		return persistence.CreateStore(r.Context(), tx, merchant, body.DisplayName)
	})
	write(w, result, err)
}
func (s *Server) GetStore(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID) {
	value, err := persistence.GetStore(r.Context(), s.db, merchant, store)
	write(w, value, err)
}
func (s *Server) GetOffering(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID) {
	value, err := persistence.GetOffering(r.Context(), s.db, merchant, store, 0)
	write(w, value, err)
}
func (s *Server) GetOfferingRevision(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID, revision int64) {
	value, err := persistence.GetOffering(r.Context(), s.db, merchant, store, revision)
	write(w, value, err)
}
func (s *Server) WriteOffering(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID, params WriteOfferingParams) {
	var body WriteOfferingJSONRequestBody
	if decode(r, &body) != nil {
		problem(w, 400, "invalid_request", "A complete offering request is required.", nil)
		return
	}
	model := offering.Model{ID: body.Model.Id, Version: body.Model.Version}
	document, _ := json.Marshal(body.Document)
	canonicalDoc, err := s.models.Validate(model, document)
	if err != nil {
		code := "invalid_offering"
		if errors.Is(err, offering.ErrUnsupported) {
			code = "unsupported_model"
		}
		problem(w, 422, code, "Offering model or document is not supported.", nil)
		return
	}
	input := struct {
		Base     int64           `json:"base_revision"`
		Model    offering.Model  `json:"model"`
		Document json.RawMessage `json:"document"`
	}{body.BaseRevision, model, canonicalDoc}
	canonical, _ := json.Marshal(input)
	result, err := persistence.Mutate(r.Context(), s.db, principal(r), "writeOffering", store.String(), params.IdempotencyKey, canonical, func(tx *sql.Tx) (any, error) {
		return persistence.AppendOffering(r.Context(), tx, merchant, store, body.BaseRevision, model, canonicalDoc)
	})
	write(w, result, err)
}
