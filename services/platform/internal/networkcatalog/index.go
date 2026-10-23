package networkcatalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/networkcatalog/persistence/sqlgen"
	"ocp.local/platform/internal/offering"
)

type Index struct {
	DB       *sql.DB
	Bindings map[uuid.UUID]Binding
	Models   *offering.Registry
}

func (i Index) Accept(ctx context.Context, c Catalog) error {
	if len(c.Resources) != 1 {
		return errors.New("complete catalog required")
	}
	b, ok := i.Bindings[c.Resources[0].Attributes.OCP.StoreID]
	if !ok {
		return errors.New("store not enrolled")
	}
	p, err := Verify(c, b)
	if err != nil {
		return err
	}
	if !i.Models.Supports(p.Model) {
		return offering.ErrUnsupported
	}
	if p.Revision == nil || *p.Revision < 1 || p.DisplayName == "" || len(p.DisplayName) > 800 {
		return errors.New("invalid offering identity")
	}
	switch p.Action {
	case "publish":
	case "restore":
		if !p.Visible {
			return errors.New("invalid restored visibility")
		}
	case "withdraw", "suspend":
		if p.Visible {
			return errors.New("invalid hidden visibility")
		}
	default:
		return errors.New("unknown publication action")
	}
	if p.Visible {
		if i.Models == nil {
			return errors.New("offering model not installed")
		}
		if _, err = i.Models.Validate(p.Model, p.Document); err != nil {
			return err
		}
		projected, err := i.Models.Project(p.Model, p.Document)
		if err != nil {
			return err
		}
		// Only model-owned public fields may cross this boundary.
		original, _ := canonical(p.Document)
		safe, _ := canonical(projected)
		if !bytes.Equal(original, safe) {
			return errors.New("unapproved public offering fields")
		}
	} else if !bytes.Equal(bytes.TrimSpace(p.Document), []byte("null")) {
		return errors.New("hidden catalog contains document")
	}
	body, err := signingBytes(p)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(body)
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	err = sqlgen.New(i.DB).AcceptCatalog(ctx, sqlgen.AcceptCatalogParams{Provider: b.ProviderID, Merchant: b.MerchantID, Store: b.StoreID, Endpoint: b.APIBase, Key: b.PublicKey, Version: p.Generation, Hash: hash[:], Document: raw})
	return err
}

func canonical(raw []byte) ([]byte, error) {
	var v any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

type Result struct {
	Binding
	Generation  int64           `json:"generation"`
	Model       offering.Model  `json:"model"`
	DisplayName string          `json:"display_name"`
	Document    json.RawMessage `json:"document"`
	ObservedAt  time.Time       `json:"observed_at"`
}

func (i Index) Search(ctx context.Context, query string) ([]Result, error) {
	if len(query) > 160 {
		return nil, errors.New("query too long")
	}
	rows, err := sqlgen.New(i.DB).SearchCatalogs(ctx, strings.TrimSpace(query))
	if err != nil {
		return nil, err
	}
	results := []Result{}
	for _, row := range rows {
		var p Projection
		if err = json.Unmarshal(row.Body, &p); err != nil {
			return nil, err
		}
		b, ok := i.Bindings[p.StoreID]
		if !ok || p.Binding != b {
			continue
		}
		results = append(results, Result{b, p.Generation, p.Model, p.DisplayName, p.Document, row.ObservedAt})
	}
	return results, nil
}

// PushHandler belongs only on the isolated crawler ingress, not public API mux.
// Projection signatures, enrollment, and schema checks still apply to every push.
func (i Index) PushHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var value struct {
			Context struct {
				Action string `json:"action"`
			} `json:"context"`
			Message struct {
				Catalogs   []Catalog `json:"catalogs"`
				Directives []struct {
					ID   string `json:"catalogId"`
					Mode string `json:"updateMode"`
					Type string `json:"catalogType"`
				} `json:"publishDirectives"`
			} `json:"message"`
		}
		if json.NewDecoder(r.Body).Decode(&value) != nil || value.Context.Action != "catalog/push" || len(value.Message.Catalogs) != 1 || len(value.Message.Directives) != 1 || value.Message.Directives[0].ID != value.Message.Catalogs[0].ID || value.Message.Directives[0].Mode != "FULL" || value.Message.Directives[0].Type != "REGULAR" {
			w.WriteHeader(422)
			return
		}
		if err := i.Accept(r.Context(), value.Message.Catalogs[0]); err != nil {
			w.WriteHeader(422)
			return
		}
		_, _ = w.Write([]byte(`{"message":{"ack":{"status":"ACK"}}}`))
	})
}
