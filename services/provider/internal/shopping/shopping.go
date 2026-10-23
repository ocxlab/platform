// Package shopping owns catalog, stock, checkout, and retail order decisions.
package shopping

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"ocp.local/platform/internal/capability"
	"ocp.local/platform/internal/payment"
	"ocp.local/platform/internal/sessionkit"
	"ocp.local/platform/internal/shopping/persistence/sqlgen"
)

type Service struct{ OrderBaseURL string }

func (Service) Name() capability.Domain { return capability.Shopping }

type Variant struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Price int64  `json:"price_minor"`
	Stock int64  `json:"stock"`
}
type Product struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Variants []Variant `json:"variants"`
}
type Catalog struct {
	Currency         string    `json:"currency"`
	PaymentCondition string    `json:"payment_condition"`
	Products         []Product `json:"products"`
}
type Item struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
	Price *int64 `json:"price,omitempty"`
}
type Line struct {
	ID       string             `json:"id,omitempty"`
	Item     Item               `json:"item"`
	Quantity int64              `json:"quantity"`
	Totals   []sessionkit.Total `json:"totals,omitempty"`
}
type Buyer struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Email     string `json:"email,omitempty"`
}
type Request struct {
	Lines []Line `json:"line_items"`
	Buyer *Buyer `json:"buyer,omitempty"`
}
type Confirmation struct {
	ID        uuid.UUID `json:"id"`
	Permalink string    `json:"permalink_url"`
}
type Session struct {
	sessionkit.Base
	Lines []Line        `json:"line_items"`
	Buyer *Buyer        `json:"buyer,omitempty"`
	Order *Confirmation `json:"order,omitempty"`
}

func validate(c Catalog) error {
	if c.Currency != "EUR" || c.PaymentCondition != "pay_later" || len(c.Products) < 1 || len(c.Products) > 500 {
		return sessionkit.ErrInput
	}
	products, skus := map[string]bool{}, map[string]bool{}
	for _, p := range c.Products {
		if !validID(p.ID) || strings.TrimSpace(p.Title) == "" || len(p.Title) > 200 || products[p.ID] || len(p.Variants) < 1 || len(p.Variants) > 50 {
			return sessionkit.ErrInput
		}
		products[p.ID] = true
		for _, v := range p.Variants {
			if !validID(v.ID) || strings.TrimSpace(v.Title) == "" || len(v.Title) > 200 || skus[v.ID] || v.Price < 0 || v.Price > 100000000 || v.Stock < 0 || v.Stock > 1000000 {
				return sessionkit.ErrInput
			}
			skus[v.ID] = true
		}
	}
	return nil
}
func validID(s string) bool {
	return len(s) > 0 && len(s) <= 80 && !strings.ContainsAny(s, " /\\\n\r\t")
}
func (Service) Catalog(ctx context.Context, tx *sql.Tx, p sessionkit.Principal, raw json.RawMessage) (json.RawMessage, error) {
	var write sessionkit.CatalogWrite
	var c Catalog
	if sessionkit.Decode(raw, &write) != nil || sessionkit.Decode(write.Document, &c) != nil || validate(c) != nil || write.BaseRevision < 0 {
		return nil, sessionkit.ErrInput
	}
	// Serialize first insert too; all catalog mutations use the same scoped lock.
	queries := sqlgen.New(tx)
	if err := queries.LockCatalogKey(ctx, "shopping/catalog/"+p.MerchantID.String()+"/"+p.StoreID.String()); err != nil {
		return nil, err
	}
	current, err := queries.LockCatalogRevision(ctx, sqlgen.LockCatalogRevisionParams{MerchantID: p.MerchantID, StoreID: p.StoreID})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if current != write.BaseRevision {
		return nil, sessionkit.ErrConflict
	}
	err = queries.SaveCatalog(ctx, sqlgen.SaveCatalogParams{MerchantID: p.MerchantID, StoreID: p.StoreID, Revision: current + 1, Document: sessionkit.JSON(c)})
	if err != nil {
		return nil, err
	}
	for _, product := range c.Products {
		for _, v := range product.Variants {
			// Stock is an explicit authoritative replacement, never inferred from menu use.
			err = queries.SaveStock(ctx, sqlgen.SaveStockParams{MerchantID: p.MerchantID, StoreID: p.StoreID, Sku: v.ID, Available: v.Stock})
			if err != nil {
				return nil, err
			}
		}
	}
	return sessionkit.JSON(map[string]any{"revision": current + 1, "document": c}), nil
}
func price(ctx context.Context, tx *sql.Tx, p sessionkit.Principal, req Request) ([]Line, int64, int64, error) {
	if len(req.Lines) < 1 || len(req.Lines) > 50 {
		return nil, 0, 0, sessionkit.ErrInput
	}
	row, err := sqlgen.New(tx).ReadCatalog(ctx, sqlgen.ReadCatalogParams{MerchantID: p.MerchantID, StoreID: p.StoreID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, 0, sessionkit.ErrNotFound
	}
	if err != nil {
		return nil, 0, 0, err
	}
	var c Catalog
	if json.Unmarshal(row.Document, &c) != nil {
		return nil, 0, 0, sessionkit.ErrInput
	}
	skus := map[string]Variant{}
	for _, product := range c.Products {
		for _, v := range product.Variants {
			skus[v.ID] = v
		}
	}
	var amount int64
	lines := []Line{}
	seen := map[string]bool{}
	for _, line := range req.Lines {
		v, ok := skus[line.Item.ID]
		if !ok || seen[v.ID] || line.Quantity < 1 || line.Quantity > 100 || line.Item.Title != "" || line.Item.Price != nil || len(line.Totals) > 0 {
			return nil, 0, 0, sessionkit.ErrInput
		}
		seen[v.ID] = true
		cost := v.Price * line.Quantity
		if cost > 100000000-amount {
			return nil, 0, 0, sessionkit.ErrInput
		}
		amount += cost
		unitPrice := v.Price
		lines = append(lines, Line{ID: fmt.Sprint(len(lines) + 1), Item: Item{ID: v.ID, Title: v.Title, Price: &unitPrice}, Quantity: line.Quantity, Totals: sessionkit.Totals(cost)})
	}
	return lines, amount, row.Revision, nil
}
func (svc Service) Execute(ctx context.Context, tx *sql.Tx, p sessionkit.Principal, action string, id uuid.UUID, raw json.RawMessage) (sessionkit.Reply, error) {
	queries := sqlgen.New(tx)
	if action == "order" {
		row, err := queries.ReadCustomerOrder(ctx, sqlgen.ReadCustomerOrderParams{MerchantID: p.MerchantID, StoreID: p.StoreID, CustomerID: p.CustomerID, ID: id})
		if errors.Is(err, sql.ErrNoRows) {
			err = sessionkit.ErrNotFound
		}
		if err != nil {
			return sessionkit.Reply{}, err
		}
		return sessionkit.Reply{Status: 200, Body: sessionkit.JSON(map[string]any{"id": id, "checkout_id": row.SessionID, "terms": row.Terms})}, nil
	}
	var state Session
	var revision int64
	var requestRaw []byte
	now, err := queries.DatabaseTime(ctx)
	if err != nil {
		return sessionkit.Reply{}, err
	}
	// Separate typed load keeps stored request distinct from authoritative response.
	if action != "create" {
		row, err := queries.LockSession(ctx, sqlgen.LockSessionParams{MerchantID: p.MerchantID, StoreID: p.StoreID, CustomerID: p.CustomerID, ID: id})
		if errors.Is(err, sql.ErrNoRows) {
			return sessionkit.Reply{}, sessionkit.ErrNotFound
		}
		if err != nil {
			return sessionkit.Reply{}, err
		}
		revision, requestRaw = row.Revision, row.Request
		// Retain the stored request separately from the authoritative response.
		if json.Unmarshal(row.Body, &state) != nil {
			return sessionkit.Reply{}, sessionkit.ErrInput
		}
		if action == "get" {
			return sessionkit.Reply{Status: 200, Body: row.Body}, nil
		}
		if state.Status == "completed" || state.Status == "canceled" {
			return sessionkit.Reply{Status: 200, Body: row.Body}, nil
		}
		if !state.ExpiresAt.After(now) {
			state.Block("session_expired", "Create a new checkout session.")
			return save(ctx, tx, p, state, revision, requestRaw, 200)
		}
	}
	if action == "create" || action == "update" {
		var req Request
		if sessionkit.Decode(raw, &req) != nil {
			return sessionkit.Reply{}, sessionkit.ErrInput
		}
		for _, line := range req.Lines {
			if action == "create" && line.ID != "" {
				return sessionkit.Reply{}, sessionkit.ErrInput
			}
		}
		lines, total, rev, err := price(ctx, tx, p, req)
		if err != nil {
			return sessionkit.Reply{}, err
		}
		if action == "create" {
			state.Base = sessionkit.NewBase(capability.Shopping, now)
		}
		state.Status = "ready_for_complete"
		state.Messages = nil
		state.Lines = lines
		state.Buyer = req.Buyer
		state.Totals = sessionkit.Totals(total)
		return save(ctx, tx, p, state, rev, sessionkit.JSON(req), map[bool]int{true: 201, false: 200}[action == "create"])
	}
	if action == "cancel" {
		state.Status = "canceled"
		state.Messages = nil
		return save(ctx, tx, p, state, revision, requestRaw, 200)
	}
	if action != "complete" {
		return sessionkit.Reply{}, sessionkit.ErrInput
	}
	if sessionkit.ValidatePayLater(raw) != nil {
		return sessionkit.Reply{}, sessionkit.ErrInput
	}
	var req Request
	if json.Unmarshal(requestRaw, &req) != nil {
		return sessionkit.Reply{}, sessionkit.ErrInput
	}
	_, _, current, err := price(ctx, tx, p, req)
	if err != nil {
		return sessionkit.Reply{}, err
	}
	if current != revision {
		state.Block("catalog_changed", "Update checkout to review current prices.")
		return save(ctx, tx, p, state, revision, requestRaw, 200)
	}
	// Lock all selected stock rows in deterministic order before validating any.
	lines := append([]Line(nil), state.Lines...)
	sort.Slice(lines, func(i, j int) bool { return lines[i].Item.ID < lines[j].Item.ID })
	for _, line := range lines {
		available, err := queries.LockStock(ctx, sqlgen.LockStockParams{MerchantID: p.MerchantID, StoreID: p.StoreID, Sku: line.Item.ID})
		if err != nil {
			return sessionkit.Reply{}, err
		}
		if available < line.Quantity {
			state.Block("out_of_stock", "An item is no longer available in the requested quantity.")
			return save(ctx, tx, p, state, revision, requestRaw, 200)
		}
	}
	order := uuid.New()
	amount := state.Totals[len(state.Totals)-1].Amount
	subject := payment.Subject{MerchantID: p.MerchantID, StoreID: p.StoreID, Domain: capability.Shopping, ID: order}
	if err := payment.RecordPayLater(ctx, tx, payment.Terms{Subject: subject, AmountMinor: amount, Currency: "EUR"}); err != nil {
		return sessionkit.Reply{}, err
	}
	for _, line := range lines {
		if err := queries.AllocateStock(ctx, sqlgen.AllocateStockParams{MerchantID: p.MerchantID, StoreID: p.StoreID, Sku: line.Item.ID, Available: line.Quantity}); err != nil {
			return sessionkit.Reply{}, err
		}
	}
	state.Status = "completed"
	state.Messages = nil
	state.Order = &Confirmation{order, strings.TrimSuffix(svc.OrderBaseURL, "/") + "/orders/" + order.String()}
	terms := sessionkit.JSON(map[string]any{"session": state, "payment_condition": "pay_later", "payment_subject": subject})
	if err := queries.InsertOrder(ctx, sqlgen.InsertOrderParams{MerchantID: p.MerchantID, StoreID: p.StoreID, ID: order, SessionID: state.ID, Terms: terms}); err != nil {
		return sessionkit.Reply{}, err
	}
	return save(ctx, tx, p, state, revision, requestRaw, 200)
}

func save(ctx context.Context, tx *sql.Tx, p sessionkit.Principal, s Session, rev int64, req []byte, status int) (sessionkit.Reply, error) {
	body := sessionkit.JSON(s)
	err := sqlgen.New(tx).SaveSession(ctx, sqlgen.SaveSessionParams{MerchantID: p.MerchantID, StoreID: p.StoreID, CustomerID: p.CustomerID, ID: s.ID, Revision: rev, Body: body, Request: req})
	return sessionkit.Reply{Status: status, Body: body}, err
}
