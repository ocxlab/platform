// Package lodging owns property rates, stay selection, and dated allocations.
package lodging

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/mail"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/google/uuid"
	"ocp.local/platform/internal/capability"
	"ocp.local/platform/internal/lodging/persistence/sqlgen"
	"ocp.local/platform/internal/payment"
	"ocp.local/platform/internal/sessionkit"
)

type Service struct{}

func (Service) Name() capability.Domain { return capability.Lodging }

type Rate struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Price int64  `json:"nightly_price_minor"`
}
type Unit struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Capacity int    `json:"capacity"`
	Rates    []Rate `json:"rates"`
}
type Property struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}
type Catalog struct {
	Currency         string   `json:"currency"`
	PaymentCondition string   `json:"payment_condition"`
	TimeZone         string   `json:"time_zone"`
	Property         Property `json:"property"`
	Units            []Unit   `json:"units"`
}
type Reference struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
}
type Dates struct {
	Start string `json:"start_date"`
	End   string `json:"end_date"`
}
type Occupancy struct {
	Total     int   `json:"total"`
	Adults    *int  `json:"adults,omitempty"`
	Children  *int  `json:"children,omitempty"`
	ChildAges []int `json:"child_ages,omitempty"`
}
type Stay struct {
	ID        string             `json:"id,omitempty"`
	Dates     Dates              `json:"stay_dates"`
	Unit      Reference          `json:"accommodation_type"`
	Rate      Reference          `json:"rate_plan"`
	Occupancy Occupancy          `json:"occupancy"`
	Totals    []sessionkit.Total `json:"totals,omitempty"`
}
type Booker struct {
	First string `json:"first_name,omitempty"`
	Last  string `json:"last_name,omitempty"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone_number,omitempty"`
}
type Request struct {
	Property      Property `json:"property"`
	Stays         []Stay   `json:"stays"`
	Booker        *Booker  `json:"booker,omitempty"`
	TravelPurpose string   `json:"travel_purpose,omitempty"`
}
type Confirmation struct {
	ID uuid.UUID `json:"id"`
}
type Session struct {
	sessionkit.Base
	Property     Property      `json:"property"`
	Stays        []Stay        `json:"stays"`
	Booker       *Booker       `json:"booker,omitempty"`
	Confirmation *Confirmation `json:"confirmation,omitempty"`
}

func validID(s string) bool {
	return len(s) > 0 && len(s) <= 80 && !strings.ContainsAny(s, " /\\\n\r\t")
}
func validate(c Catalog) error {
	if c.Currency != "EUR" || c.PaymentCondition != "pay_later" || !validID(c.Property.ID) || strings.TrimSpace(c.Property.Name) == "" || len(c.Property.Name) > 200 || len(c.Units) < 1 || len(c.Units) > 200 || c.TimeZone == "" || c.TimeZone == "Local" {
		return sessionkit.ErrInput
	}
	if _, err := time.LoadLocation(c.TimeZone); err != nil {
		return sessionkit.ErrInput
	}
	seen := map[string]bool{}
	for _, u := range c.Units {
		if !validID(u.ID) || seen[u.ID] || strings.TrimSpace(u.Title) == "" || len(u.Title) > 200 || u.Capacity < 1 || u.Capacity > 20 || len(u.Rates) < 1 || len(u.Rates) > 20 {
			return sessionkit.ErrInput
		}
		seen[u.ID] = true
		rates := map[string]bool{}
		for _, r := range u.Rates {
			if !validID(r.ID) || rates[r.ID] || strings.TrimSpace(r.Title) == "" || len(r.Title) > 200 || r.Price < 0 || r.Price > 100000000 {
				return sessionkit.ErrInput
			}
			rates[r.ID] = true
		}
	}
	return nil
}
func (Service) Catalog(ctx context.Context, tx *sql.Tx, p sessionkit.Principal, raw json.RawMessage) (json.RawMessage, error) {
	var w sessionkit.CatalogWrite
	var c Catalog
	if sessionkit.Decode(raw, &w) != nil || sessionkit.Decode(w.Document, &c) != nil || validate(c) != nil || w.BaseRevision < 0 {
		return nil, sessionkit.ErrInput
	}
	queries := sqlgen.New(tx)
	if err := queries.LockPropertyKey(ctx, "lodging/property/"+p.MerchantID.String()+"/"+p.StoreID.String()); err != nil {
		return nil, err
	}
	rev, err := queries.LockPropertyRevision(ctx, sqlgen.LockPropertyRevisionParams{MerchantID: p.MerchantID, StoreID: p.StoreID})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if rev != w.BaseRevision {
		return nil, sessionkit.ErrConflict
	}
	err = queries.SaveProperty(ctx, sqlgen.SavePropertyParams{MerchantID: p.MerchantID, StoreID: p.StoreID, Revision: rev + 1, Document: sessionkit.JSON(c)})
	if err != nil {
		return nil, err
	}
	for _, u := range c.Units {
		if err = queries.InsertUnit(ctx, sqlgen.InsertUnitParams{MerchantID: p.MerchantID, StoreID: p.StoreID, UnitID: u.ID}); err != nil {
			return nil, err
		}
	}
	return sessionkit.JSON(map[string]any{"revision": rev + 1, "document": c}), nil
}
func contact(b *Booker) bool {
	if b == nil || strings.TrimSpace(b.First) == "" || strings.TrimSpace(b.Last) == "" || len(b.First) > 100 || len(b.Last) > 100 {
		return false
	}
	if b.Email != "" {
		a, err := mail.ParseAddress(b.Email)
		return err == nil && a.Address == b.Email && len(b.Email) <= 254
	}
	return len(b.Phone) >= 6 && len(b.Phone) <= 32
}
func price(ctx context.Context, tx *sql.Tx, p sessionkit.Principal, req Request, now time.Time) (Catalog, []Stay, int64, int64, error) {
	var c Catalog
	row, err := sqlgen.New(tx).ReadProperty(ctx, sqlgen.ReadPropertyParams{MerchantID: p.MerchantID, StoreID: p.StoreID})
	if errors.Is(err, sql.ErrNoRows) {
		return c, nil, 0, 0, sessionkit.ErrNotFound
	}
	if err != nil {
		return c, nil, 0, 0, err
	}
	if json.Unmarshal(row.Document, &c) != nil {
		return c, nil, 0, 0, sessionkit.ErrInput
	}
	if req.Property.ID != c.Property.ID || req.Property.Name != "" || len(req.Stays) < 1 || len(req.Stays) > 8 {
		return c, nil, 0, 0, sessionkit.ErrInput
	}
	zone, _ := time.LoadLocation(c.TimeZone)
	today := now.In(zone).Format(time.DateOnly)
	var total int64
	stays := []Stay{}
	for _, chosen := range req.Stays {
		start, e1 := time.Parse(time.DateOnly, chosen.Dates.Start)
		end, e2 := time.Parse(time.DateOnly, chosen.Dates.End)
		nights := int64(end.Sub(start) / (24 * time.Hour))
		if e1 != nil || e2 != nil || start.Format(time.DateOnly) != chosen.Dates.Start || end.Format(time.DateOnly) != chosen.Dates.End || nights < 1 || nights > 30 || chosen.Dates.Start < today || chosen.Unit.Title != "" || chosen.Rate.Title != "" || len(chosen.Totals) > 0 {
			return c, nil, 0, 0, sessionkit.ErrInput
		}
		var unit *Unit
		var rate *Rate
		for i := range c.Units {
			if c.Units[i].ID == chosen.Unit.ID {
				unit = &c.Units[i]
				for j := range unit.Rates {
					if unit.Rates[j].ID == chosen.Rate.ID {
						rate = &unit.Rates[j]
					}
				}
			}
		}
		o := chosen.Occupancy
		if unit == nil || rate == nil || o.Total < 1 || o.Total > unit.Capacity {
			return c, nil, 0, 0, sessionkit.ErrInput
		}
		adults, children := 0, 0
		if o.Adults != nil {
			adults = *o.Adults
		}
		if o.Children != nil {
			children = *o.Children
		}
		if adults < 0 || children < 0 || (o.Adults != nil || o.Children != nil) && adults+children != o.Total || o.ChildAges != nil && len(o.ChildAges) != children {
			return c, nil, 0, 0, sessionkit.ErrInput
		}
		for _, age := range o.ChildAges {
			if age < 0 || age > 17 {
				return c, nil, 0, 0, sessionkit.ErrInput
			}
		}
		for _, previous := range stays {
			if previous.Unit.ID == chosen.Unit.ID && previous.Dates.Start < chosen.Dates.End && previous.Dates.End > chosen.Dates.Start {
				return c, nil, 0, 0, sessionkit.ErrInput
			}
		}
		cost := rate.Price * nights
		if cost > 100000000-total {
			return c, nil, 0, 0, sessionkit.ErrInput
		}
		total += cost
		chosen.ID = uuid.NewString()
		chosen.Unit.Title = unit.Title
		chosen.Rate.Title = rate.Title
		chosen.Totals = sessionkit.Totals(cost)
		stays = append(stays, chosen)
	}
	return c, stays, total, row.Revision, nil
}
func (Service) Execute(ctx context.Context, tx *sql.Tx, p sessionkit.Principal, action string, id uuid.UUID, raw json.RawMessage) (sessionkit.Reply, error) {
	var s Session
	var revision int64
	var requestRaw []byte
	queries := sqlgen.New(tx)
	now, err := queries.DatabaseTime(ctx)
	if err != nil {
		return sessionkit.Reply{}, err
	}
	if action != "create" {
		row, err := queries.LockSession(ctx, sqlgen.LockSessionParams{MerchantID: p.MerchantID, StoreID: p.StoreID, CustomerID: p.CustomerID, ID: id})
		if errors.Is(err, sql.ErrNoRows) {
			return sessionkit.Reply{}, sessionkit.ErrNotFound
		}
		if err != nil {
			return sessionkit.Reply{}, err
		}
		revision, requestRaw = row.Revision, row.Request
		if json.Unmarshal(row.Body, &s) != nil {
			return sessionkit.Reply{}, sessionkit.ErrInput
		}
		if action == "get" || s.Status == "completed" || s.Status == "canceled" {
			return sessionkit.Reply{Status: 200, Body: row.Body}, nil
		}
		if !s.ExpiresAt.After(now) {
			s.Block("session_expired", "Create a new booking session.")
			return save(ctx, tx, p, s, revision, requestRaw, 200)
		}
	}
	if action == "create" || action == "update" {
		var req Request
		if sessionkit.Decode(raw, &req) != nil {
			return sessionkit.Reply{}, sessionkit.ErrInput
		}
		c, stays, total, rev, err := price(ctx, tx, p, req, now)
		if err != nil {
			return sessionkit.Reply{}, err
		}
		if action == "create" {
			s.Base = sessionkit.NewBase(capability.Lodging, now)
		}
		s.Status = "ready_for_complete"
		s.Messages = nil
		s.Property = c.Property
		s.Stays = stays
		s.Booker = req.Booker
		s.Totals = sessionkit.Totals(total)
		if !contact(req.Booker) {
			s.Block("booker_required", "Provide the booker's legal name and a contact channel.")
		}
		status := 200
		if action == "create" {
			status = 201
		}
		return save(ctx, tx, p, s, rev, sessionkit.JSON(req), status)
	}
	if action == "cancel" {
		s.Status = "canceled"
		s.Messages = nil
		return save(ctx, tx, p, s, revision, requestRaw, 200)
	}
	if action != "complete" || sessionkit.ValidatePayLater(raw) != nil {
		return sessionkit.Reply{}, sessionkit.ErrInput
	}
	if !contact(s.Booker) {
		s.Block("booker_required", "Provide the booker's legal name and a contact channel.")
		return save(ctx, tx, p, s, revision, requestRaw, 200)
	}
	var req Request
	if json.Unmarshal(requestRaw, &req) != nil {
		return sessionkit.Reply{}, sessionkit.ErrInput
	}
	_, _, _, current, err := price(ctx, tx, p, req, now)
	if err != nil {
		return sessionkit.Reply{}, err
	}
	if current != revision {
		s.Block("rates_changed", "Update booking to review current rates.")
		return save(ctx, tx, p, s, revision, requestRaw, 200)
	}
	// Serialize each physical unit before checking overlaps. The SQL exclusion
	// constraint independently rejects overlaps, including direct SQL writes.
	units := []string{}
	seen := map[string]bool{}
	for _, stay := range s.Stays {
		if !seen[stay.Unit.ID] {
			units = append(units, stay.Unit.ID)
			seen[stay.Unit.ID] = true
		}
	}
	sort.Strings(units)
	for _, unit := range units {
		if err := queries.LockUnitKey(ctx, "lodging/unit/"+p.MerchantID.String()+"/"+p.StoreID.String()+"/"+unit); err != nil {
			return sessionkit.Reply{}, err
		}
		if _, err := queries.ReadUnit(ctx, sqlgen.ReadUnitParams{MerchantID: p.MerchantID, StoreID: p.StoreID, UnitID: unit}); err != nil {
			return sessionkit.Reply{}, err
		}
	}
	for _, stay := range s.Stays {
		occupied, err := queries.StayOccupied(ctx, sqlgen.StayOccupiedParams{MerchantID: p.MerchantID, StoreID: p.StoreID, UnitID: stay.Unit.ID, CheckIn: stay.Dates.Start, CheckOut: stay.Dates.End})
		if err != nil {
			return sessionkit.Reply{}, err
		}
		if occupied {
			s.Block("stay_unavailable", "The accommodation is no longer available for these dates.")
			return save(ctx, tx, p, s, revision, requestRaw, 200)
		}
	}
	booking := uuid.New()
	subject := payment.Subject{Domain: capability.Lodging, MerchantID: p.MerchantID, StoreID: p.StoreID, ID: booking}
	amount := s.Totals[len(s.Totals)-1].Amount
	if err := payment.RecordPayLater(ctx, tx, payment.Terms{Subject: subject, AmountMinor: amount, Currency: "EUR"}); err != nil {
		return sessionkit.Reply{}, err
	}
	s.Status = "completed"
	s.Messages = nil
	s.Confirmation = &Confirmation{booking}
	terms := sessionkit.JSON(map[string]any{"session": s, "payment_condition": "pay_later", "payment_subject": subject})
	if err := queries.InsertBooking(ctx, sqlgen.InsertBookingParams{MerchantID: p.MerchantID, StoreID: p.StoreID, ID: booking, SessionID: s.ID, Terms: terms}); err != nil {
		return sessionkit.Reply{}, err
	}
	for _, stay := range s.Stays {
		if err := queries.InsertAllocation(ctx, sqlgen.InsertAllocationParams{MerchantID: p.MerchantID, StoreID: p.StoreID, UnitID: stay.Unit.ID, BookingID: booking, CheckIn: stay.Dates.Start, CheckOut: stay.Dates.End}); err != nil {
			return sessionkit.Reply{}, err
		}
	}
	return save(ctx, tx, p, s, revision, requestRaw, 200)
}
func save(ctx context.Context, tx *sql.Tx, p sessionkit.Principal, s Session, rev int64, request []byte, status int) (sessionkit.Reply, error) {
	body := sessionkit.JSON(s)
	err := sqlgen.New(tx).SaveSession(ctx, sqlgen.SaveSessionParams{MerchantID: p.MerchantID, StoreID: p.StoreID, CustomerID: p.CustomerID, ID: s.ID, Revision: rev, Body: body, Request: request})
	return sessionkit.Reply{Status: status, Body: body}, err
}
