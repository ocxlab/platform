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
	"ocp.local/platform/internal/orderupdates"
	"ocp.local/platform/internal/persistence"
	"ocp.local/platform/internal/problems"
)

type principalKey struct{}
type Server struct {
	db                  *sql.DB
	models              *offering.Registry
	developmentPayments bool
	updates             *orderupdates.Hub
}

type Option func(*Server)

func WithOrderUpdates(hub *orderupdates.Hub) Option { return func(s *Server) { s.updates = hub } }

// Disabled by default. Enable only in explicit local/test composition.
func WithDevelopmentPayments() Option { return func(s *Server) { s.developmentPayments = true } }

// NewHandler installs the generated routing and contract validation around the
// application operations. Administrative grants are never accepted as DB roles.
func NewHandler(db *sql.DB, options ...Option) (http.Handler, error) {
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
	}, map[offering.Model]offering.Validator{food.Model: food.Project})}
	for _, option := range options {
		option(server)
	}
	generated := HandlerWithOptions(server, StdHTTPServerOptions{ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
		problem(w, 400, "invalid_request", "Request does not match the API contract.", nil)
	}})
	scopes := map[string]string{}
	known := map[string]bool{"store:write": true, "store:read": true, "offering:read": true, "offering:write": true, "publication:read": true, "publication:manage": true, "quote:create": true, "order:create": true, "order:read": true, "order:cancel": true, "order:manage": true, "order:inspect": true}
	public := map[string]bool{"GetPublicFoodOrdering": true, "GetLiveness": true, "GetPublicOffering": true, "CreateFoodQuote": true, "CreateGuestSession": true, "ListPublicStores": true, "GetPublicStoreByHandle": true}
	guest := map[string]bool{"CreateGuestFoodQuote": true, "PlaceFoodOrder": true, "GetGuestFoodOrder": true, "GetGuestFoodQuote": true, "CancelGuestFoodOrder": true, "ChangeDevelopmentPayment": true, "GetGuestFoodOrderByReference": true, "WatchGuestFoodOrder": true}
	provider := map[string]bool{"AdminGetPublication": true, "AdminSuspendPublication": true, "AdminGetFoodOrder": true, "AdminListStores": true, "AdminListFoodOrders": true}
	for _, path := range spec.Paths.Map() {
		for _, operation := range path.Operations() {
			expectedAccess := "merchant"
			if guest[operation.OperationID] {
				expectedAccess = "guest"
			}
			if public[operation.OperationID] {
				expectedAccess = "public"
			}
			if provider[operation.OperationID] {
				expectedAccess = "provider"
			}
			if operation.Extensions["x-access"] != expectedAccess {
				return nil, errors.New("operation must explicitly declare its recognized credential audience")
			}
			if public[operation.OperationID] {
				if operation.Security == nil || len(*operation.Security) != 0 {
					return nil, errors.New("public operation must explicitly disable bearer security")
				}
				continue
			}
			effective := spec.Security
			if operation.Security != nil {
				effective = *operation.Security
			}
			scheme := "BearerAuth"
			if guest[operation.OperationID] {
				scheme = "GuestBearerAuth"
			}
			if provider[operation.OperationID] {
				scheme = "ProviderBearerAuth"
			}
			if len(effective) != 1 || len(effective[0]) != 1 {
				return nil, errors.New("operation has ambiguous credential security")
			}
			if _, ok := effective[0][scheme]; !ok {
				return nil, errors.New("operation security does not match its declared audience")
			}
			scope, ok := operation.Extensions["x-required-scope"].(string)
			if !ok || !known[scope] {
				return nil, errors.New("protected operation has no recognized action scope")
			}
			scopes[operation.OperationID] = scope
			if operation.Security != nil && len(*operation.Security) == 0 {
				return nil, errors.New("protected operation cannot declare anonymous security")
			}
			if provider[operation.OperationID] && operation.Extensions["x-access"] != "provider" {
				return nil, errors.New("provider operation missing audience")
			}
			if provider[operation.OperationID] {
				if operation.Security == nil || len(*operation.Security) != 1 || len((*operation.Security)[0]) != 1 {
					return nil, errors.New("provider operation must declare its credential audience")
				}
				if _, ok := (*operation.Security)[0]["ProviderBearerAuth"]; !ok {
					return nil, errors.New("provider operation has wrong credential audience")
				}
			}
			if !provider[operation.OperationID] && (strings.HasPrefix(scope, "publication:") || scope == "order:inspect") {
				return nil, errors.New("provider scope on merchant operation")
			}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", uuid.NewString())
		w.Header().Set("Cache-Control", "no-store")
		route, pathParams, err := router.FindRoute(r)
		if err != nil {
			problem(w, 404, "not_found", "API operation not found.", nil)
			return
		}
		timeout := 10 * time.Second
		if route.Operation.OperationID == "WatchGuestFoodOrder" || route.Operation.OperationID == "WatchMerchantFoodOrders" {
			timeout = 2 * time.Minute
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		r = r.WithContext(ctx)
		if !public[route.Operation.OperationID] {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				problem(w, 401, "unauthenticated", "A valid bearer credential is required.", nil)
				return
			}
			authenticate := persistence.Authenticate
			if guest[route.Operation.OperationID] {
				authenticate = persistence.AuthenticateGuest
			}
			if provider[route.Operation.OperationID] {
				authenticate = persistence.AuthenticateProvider
			}
			authContext, authCancel := context.WithTimeout(r.Context(), 5*time.Second)
			principal, err := authenticate(authContext, db, strings.TrimPrefix(header, "Bearer "))
			authCancel()
			if errors.Is(err, persistence.ErrUnauthenticated) {
				problem(w, 401, "unauthenticated", "A valid bearer credential is required.", nil)
				return
			}
			if err != nil {
				problem(w, 503, "unavailable", "Credential service is temporarily unavailable.", nil)
				return
			}
			merchant := uuid.Nil
			if value, ok := pathParams["merchant_id"]; ok {
				merchant, err = uuid.Parse(value)
			} else if !provider[route.Operation.OperationID] {
				err = errors.New("merchant scope required")
			}
			if err != nil {
				problem(w, 400, "invalid_request", "Merchant identifier must be a UUID.", nil)
				return
			}
			if (!principal.Provider && principal.MerchantID != merchant) || !principal.Allows(scopes[route.Operation.OperationID]) {
				problem(w, 403, "forbidden", "Credential does not authorize this merchant and action.", nil)
				return
			}
			if principal.Guest {
				store, err := uuid.Parse(pathParams["store_id"])
				if err != nil {
					problem(w, 400, "invalid_request", "Store identifier must be a UUID.", nil)
					return
				}
				if principal.StoreID != store {
					problem(w, 403, "forbidden", "Guest credential does not authorize this store.", nil)
					return
				}
			}
			if principal.Provider {
				principal.MerchantID = merchant
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
	definition, ok := problems.Lookup(code)
	if !ok || definition.Status != status {
		status = 503
		code = "unavailable"
		detail = "The service could not complete this request."
	}
	// RFC 9457 section 4.2.1: about:blank uses the HTTP status phrase as title.
	// The additive code extension carries our more specific failure category.
	body := map[string]any{"type": "about:blank", "title": http.StatusText(status), "status": status, "code": code, "detail": detail, "instance": "urn:uuid:" + w.Header().Get("X-Request-ID")}
	if current != nil {
		field := "current_revision"
		if code == "publication_conflict" {
			field = "current_generation"
		}
		if code == "order_revision_conflict" {
			field = "current_order_revision"
		}
		body[field] = *current
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
		var publicationConflict *persistence.PublicationConflict
		var orderConflict *persistence.OrderRevisionConflict
		var orderingConflict *persistence.OrderingRevisionConflict
		switch {
		case errors.As(err, &orderingConflict):
			problem(w, 409, "ordering_revision_conflict", "Ordering settings have changed. Read their current revision before retrying.", &orderingConflict.Current)
		case errors.Is(err, persistence.ErrOrderingPaused):
			problem(w, 409, "ordering_paused", "This store has paused new orders. Existing orders remain available.", nil)
		case errors.Is(err, persistence.ErrPublicStoreCursor):
			problem(w, 400, "invalid_request", "The store continuation cursor is invalid.", nil)
		case errors.Is(err, persistence.ErrOrderCursor):
			problem(w, 400, "invalid_request", "The order continuation cursor is invalid.", nil)
		case errors.As(err, &orderConflict):
			problem(w, 409, "order_revision_conflict", "Order progress has changed. Read its current revision before retrying.", &orderConflict.Current)
		case errors.Is(err, persistence.ErrOrderTransition):
			problem(w, 409, "order_transition_conflict", "This action is not available for the current order state.", nil)
		case errors.Is(err, persistence.ErrUnauthenticated):
			problem(w, 401, "unauthenticated", "A valid bearer credential is required.", nil)
		case errors.Is(err, persistence.ErrQuoteExpired):
			problem(w, 409, "quote_expired", "This quote has expired. Request fresh terms before placing an order.", nil)
		case errors.Is(err, persistence.ErrQuoteUsed):
			problem(w, 409, "quote_already_placed", "This quote already has an order. Retry with the original placement key or inspect the existing order.", nil)
		case errors.Is(err, persistence.ErrNotFound):
			problem(w, 404, "not_found", "Resource not found.", nil)
		case errors.Is(err, persistence.ErrReplayConflict):
			problem(w, 409, "idempotency_conflict", "This key was used with different request content.", nil)
		case errors.As(err, &conflict):
			problem(w, 409, "revision_conflict", "Offering revision has changed.", &conflict.Current)
		case errors.As(err, &publicationConflict):
			problem(w, 409, "publication_conflict", "Publication generation has changed. Refresh the public offering or private publication state.", &publicationConflict.Current)
		case errors.Is(err, food.ErrSelection):
			problem(w, 422, "invalid_selection", "Select available items and supported variants/options within the published limits.", nil)
		case errors.Is(err, persistence.ErrQuoteCapacity):
			w.Header().Set("Retry-After", "60")
			problem(w, 429, "quote_capacity", "This store has reached its temporary quote capacity. Please try again later.", nil)
		case errors.Is(err, persistence.ErrGuestCapacity):
			w.Header().Set("Retry-After", "60")
			problem(w, 429, "guest_capacity", "This store has reached its temporary guest checkout capacity. Please try again later.", nil)
		case errors.Is(err, offering.ErrUnsupported):
			problem(w, 422, "unsupported_model", "No installed implementation supports this operation for the offering model.", nil)
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
