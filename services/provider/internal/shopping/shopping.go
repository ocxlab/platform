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
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/capability"
	"ocp.local/platform/internal/payment"
	"ocp.local/platform/internal/sessionkit"
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
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "shopping/catalog/"+p.MerchantID.String()+"/"+p.StoreID.String()); err != nil {
		return nil, err
	}
	var current int64
	err := tx.QueryRowContext(ctx, `SELECT revision FROM shopping.catalogs WHERE merchant_id=$1 AND store_id=$2 FOR UPDATE`, p.MerchantID, p.StoreID).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if current != write.BaseRevision {
		return nil, sessionkit.ErrConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO shopping.catalogs(merchant_id,store_id,revision,document) VALUES($1,$2,$3,$4) ON CONFLICT(merchant_id,store_id) DO UPDATE SET revision=EXCLUDED.revision,document=EXCLUDED.document`, p.MerchantID, p.StoreID, current+1, string(sessionkit.JSON(c)))
	if err != nil {
		return nil, err
	}
	for _, product := range c.Products {
		for _, v := range product.Variants {
			// Stock is an explicit authoritative replacement, never inferred from menu use.
			_, err = tx.ExecContext(ctx, `INSERT INTO shopping.stock(merchant_id,store_id,sku,available) VALUES($1,$2,$3,$4) ON CONFLICT(merchant_id,store_id,sku) DO UPDATE SET available=EXCLUDED.available`, p.MerchantID, p.StoreID, v.ID, v.Stock)
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
	var doc []byte
	var revision int64
	err := tx.QueryRowContext(ctx, `SELECT revision,document FROM shopping.catalogs WHERE merchant_id=$1 AND store_id=$2 FOR SHARE`, p.MerchantID, p.StoreID).Scan(&revision, &doc)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, 0, sessionkit.ErrNotFound
	}
	if err != nil {
		return nil, 0, 0, err
	}
	var c Catalog
	if json.Unmarshal(doc, &c) != nil {
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
	return lines, amount, revision, nil
}
func (svc Service) Execute(ctx context.Context, tx *sql.Tx, p sessionkit.Principal, action string, id uuid.UUID, raw json.RawMessage) (sessionkit.Reply, error) {
	if action == "order" {
		var terms []byte
		var checkout uuid.UUID
		err := tx.QueryRowContext(ctx, `SELECT o.session_id,o.terms FROM shopping.orders o JOIN shopping.sessions s ON s.merchant_id=o.merchant_id AND s.store_id=o.store_id AND s.id=o.session_id WHERE o.merchant_id=$1 AND o.store_id=$2 AND s.customer_id=$3 AND o.id=$4`, p.MerchantID, p.StoreID, p.CustomerID, id).Scan(&checkout, &terms)
		if errors.Is(err, sql.ErrNoRows) {
			err = sessionkit.ErrNotFound
		}
		if err != nil {
			return sessionkit.Reply{}, err
		}
		return sessionkit.Reply{Status: 200, Body: sessionkit.JSON(map[string]any{"id": id, "checkout_id": checkout, "terms": json.RawMessage(terms)})}, nil
	}
	var state Session
	var revision int64
	var requestRaw []byte
	var now time.Time
	if err := tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return sessionkit.Reply{}, err
	}
	// Separate typed load keeps stored request distinct from authoritative response.
	if action != "create" {
		var body []byte
		err := tx.QueryRowContext(ctx, `SELECT revision,body,request FROM shopping.sessions WHERE merchant_id=$1 AND store_id=$2 AND customer_id=$3 AND id=$4 FOR UPDATE`, p.MerchantID, p.StoreID, p.CustomerID, id).Scan(&revision, &body, &requestRaw)
		if errors.Is(err, sql.ErrNoRows) {
			return sessionkit.Reply{}, sessionkit.ErrNotFound
		}
		if err != nil {
			return sessionkit.Reply{}, err
		}
		if json.Unmarshal(body, &state) != nil {
			return sessionkit.Reply{}, sessionkit.ErrInput
		}
		if action == "get" {
			return sessionkit.Reply{Status: 200, Body: body}, nil
		}
		if state.Status == "completed" || state.Status == "canceled" {
			return sessionkit.Reply{Status: 200, Body: body}, nil
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
		var available int64
		if err := tx.QueryRowContext(ctx, `SELECT available FROM shopping.stock WHERE merchant_id=$1 AND store_id=$2 AND sku=$3 FOR UPDATE`, p.MerchantID, p.StoreID, line.Item.ID).Scan(&available); err != nil {
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
		if _, err := tx.ExecContext(ctx, `UPDATE shopping.stock SET available=available-$4 WHERE merchant_id=$1 AND store_id=$2 AND sku=$3`, p.MerchantID, p.StoreID, line.Item.ID, line.Quantity); err != nil {
			return sessionkit.Reply{}, err
		}
	}
	state.Status = "completed"
	state.Messages = nil
	state.Order = &Confirmation{order, strings.TrimSuffix(svc.OrderBaseURL, "/") + "/orders/" + order.String()}
	terms := sessionkit.JSON(map[string]any{"session": state, "payment_condition": "pay_later", "payment_subject": subject})
	if _, err := tx.ExecContext(ctx, `INSERT INTO shopping.orders(merchant_id,store_id,id,session_id,terms) VALUES($1,$2,$3,$4,$5)`, p.MerchantID, p.StoreID, order, state.ID, string(terms)); err != nil {
		return sessionkit.Reply{}, err
	}
	return save(ctx, tx, p, state, revision, requestRaw, 200)
}

func save(ctx context.Context, tx *sql.Tx, p sessionkit.Principal, s Session, rev int64, req []byte, status int) (sessionkit.Reply, error) {
	body := sessionkit.JSON(s)
	_, err := tx.ExecContext(ctx, `INSERT INTO shopping.sessions(merchant_id,store_id,customer_id,id,revision,body,request) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(merchant_id,store_id,id) DO UPDATE SET revision=EXCLUDED.revision,body=EXCLUDED.body,request=EXCLUDED.request`, p.MerchantID, p.StoreID, p.CustomerID, s.ID, rev, string(body), string(req))
	return sessionkit.Reply{Status: status, Body: body}, err
}
