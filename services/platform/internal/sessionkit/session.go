// Package sessionkit shares UCP transport records, not domain allocation rules.
package sessionkit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/capability"
)

const Version = "2026-08-25"
const SourcePin = "d13cc61455d2a90f7e6038df07fdbe72dcccdcc7"

var ErrInput = errors.New("invalid internal commerce request")
var ErrConflict = errors.New("internal commerce conflict")
var ErrNotFound = errors.New("internal commerce resource not found")

type Principal struct {
	ID, MerchantID, StoreID, CustomerID uuid.UUID
	Domain                              capability.Domain
	Profile                             string
}
type Total struct {
	Type   string `json:"type"`
	Amount int64  `json:"amount"`
}
type Metadata struct {
	Version         string              `json:"version"`
	Capabilities    map[string][]Entity `json:"capabilities"`
	PaymentHandlers map[string][]Entity `json:"payment_handlers"`
}
type Entity struct {
	ID        string `json:"id,omitempty"`
	Version   string `json:"version"`
	Schema    string `json:"schema,omitempty"`
	Spec      string `json:"spec,omitempty"`
	Transport string `json:"transport,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
}

func Capability(domain capability.Domain) string {
	if domain == capability.Shopping {
		return "dev.ucp.shopping.checkout"
	}
	if domain == capability.Lodging {
		return "dev.ucp.lodging.booking"
	}
	return ""
}
func Meta(domain capability.Domain) Metadata {
	// Handler details are resolved through the business profile. Session metadata
	// identifies the negotiated instance without inventing a global schema host.
	return Metadata{Version, map[string][]Entity{Capability(domain): {{Version: Version}}}, map[string][]Entity{PayLaterHandler: {{ID: PayLaterID, Version: Version}}}}
}
func Totals(amount int64) []Total { return []Total{{"subtotal", amount}, {"total", amount}} }

type Message struct {
	Type     string `json:"type"`
	Code     string `json:"code"`
	Content  string `json:"content"`
	Severity string `json:"severity,omitempty"`
}
type Link struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}
type Base struct {
	UCP       Metadata  `json:"ucp"`
	ID        uuid.UUID `json:"id"`
	Status    string    `json:"status"`
	Currency  string    `json:"currency"`
	Totals    []Total   `json:"totals"`
	Links     []Link    `json:"links"`
	Messages  []Message `json:"messages,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
}

func NewBase(domain capability.Domain, now time.Time) Base {
	return Base{UCP: Meta(domain), ID: uuid.New(), Status: "ready_for_complete", Currency: "EUR", Totals: Totals(0), Links: []Link{}, ExpiresAt: now.Add(15 * time.Minute)}
}
func (b *Base) Block(code, content string) {
	b.Status = "incomplete"
	b.Messages = []Message{{Type: "error", Code: code, Content: content, Severity: "recoverable"}}
}

type Reply struct {
	Status int
	Body   json.RawMessage
}

// Domain implementations own their tables, pricing, allocation and confirmation.
// Execute runs inside the caller's tenant/replay transaction.
type Domain interface {
	Name() capability.Domain
	Catalog(context.Context, *sql.Tx, Principal, json.RawMessage) (json.RawMessage, error)
	Execute(context.Context, *sql.Tx, Principal, string, uuid.UUID, json.RawMessage) (Reply, error)
}
