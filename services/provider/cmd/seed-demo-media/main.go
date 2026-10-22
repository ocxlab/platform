// seed-demo-media targets only the fictional restaurant in local Compose.
// It never resets the database or republishes somebody's unpublished edits.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/commerce"
	"ocp.local/platform/internal/devfixtures"
	"ocp.local/platform/internal/devsetup"
	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/media"
	"ocp.local/platform/internal/offering"
	"ocp.local/platform/internal/offering/food"
	"ocp.local/platform/internal/persistence"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "demo photo attachment failed:", err)
		os.Exit(1)
	}
}
func run() (resultErr error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	admin, err := persistence.Open(ctx, "postgres://ocp_dev:ocp_local_only@db:5432/ocp_dev?sslmode=disable")
	if err != nil {
		return err
	}
	defer admin.Close()
	store, err := devsetup.EnsureRestaurant(ctx, admin)
	if err != nil {
		return err
	}
	db, err := persistence.Open(ctx, "postgres://ocp_app:ocp_app_local_only@db:5432/ocp_dev?sslmode=disable")
	if err != nil {
		return err
	}
	defer db.Close()
	latest, err := persistence.GetOffering(ctx, db, store.MerchantID, store.ID, 0)
	if err != nil {
		return err
	}
	publication, err := persistence.GetPublication(ctx, db, store.MerchantID, store.ID)
	if err != nil {
		return err
	}
	if latest.Model != food.Model {
		return errors.New("demo store no longer uses the expected model; left untouched")
	}
	var doc map[string]any
	if err = json.Unmarshal(latest.Document, &doc); err != nil {
		return err
	}
	fixtureMenu, products, err := devfixtures.Restaurant()
	if err != nil {
		return err
	}
	expected := map[string]string{}
	for _, p := range products {
		expected[p.ID] = p.Name
	}
	config := map[string]string{"OCP_MEDIA_ENABLED": "true", "OCP_MEDIA_INTERNAL_ENDPOINT": "http://object-store:9000", "OCP_MEDIA_PUBLIC_ENDPOINT": "http://localhost:18900", "OCP_MEDIA_REGION": "us-east-1", "OCP_MEDIA_BUCKET": "ocp-demo-media", "OCP_MEDIA_ACCESS_KEY": "ocp_local_media", "OCP_MEDIA_SECRET_KEY": "ocp_local_media_only_123"}
	storage, err := media.FromEnv(func(k string) string { return config[k] })
	if err != nil {
		return err
	}
	principal, _, err := persistence.ProvisionMerchantGrant(ctx, admin, store.MerchantID, []string{"offering:read", "offering:write"}, time.Now().Add(time.Hour))
	if err != nil {
		return err
	}
	// Keep only the attributable audit identity after this explicit bootstrap.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(cleanupCtx, `UPDATE auth.grants SET revoked=true WHERE id=$1`, principal.ID); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("revoke temporary demo grant: %w", err))
		}
	}()
	transport := &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr != "localhost:18900" {
			return nil, errors.New("unexpected upload destination")
		}
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, "object-store:9000")
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	operation := commerce.Media{DB: db, Storage: storage}
	changed := 0
	categorized := attachCategories(doc, fixtureMenu)
	translated := devfixtures.TranslateDemoNames(doc)
	meals := devfixtures.AddDemoMeals(doc)
	for _, entry := range doc["items"].([]any) {
		item := entry.(map[string]any)
		id, _ := item["id"].(string)
		name, _ := item["name"].(string)
		// Do not put an outdated fixture picture on a renamed/replaced user dish.
		if expected[id] != name || name == "" || item["image_id"] != nil {
			continue
		}
		raw, err := os.ReadFile("/workspace/assets/demo/food/" + id + ".jpg")
		if err != nil {
			return err
		}
		hash := sha256.Sum256(raw)
		sum := base64.StdEncoding.EncodeToString(hash[:])
		// Reuse a previously completed asset after an interrupted seed, independent
		// of this run's temporary credential and request replay scope.
		tx, err := persistence.TenantTransaction(ctx, db, store.MerchantID)
		if err != nil {
			return err
		}
		var existing uuid.UUID
		lookupErr := tx.QueryRowContext(ctx, `SELECT id FROM commerce.media_assets WHERE merchant_id=$1 AND store_id=$2 AND checksum=$3 AND state='ready' ORDER BY created_at,id LIMIT 1`, store.MerchantID, store.ID, sum).Scan(&existing)
		tx.Rollback()
		if lookupErr == nil {
			item["image_id"] = existing.String()
			changed++
			continue
		}
		if !errors.Is(lookupErr, sql.ErrNoRows) {
			return lookupErr
		}
		result, err := operation.Start(ctx, principal, store.MerchantID, store.ID, "image/jpeg", sum, int64(len(raw)), "demo-photo-start-"+id)
		if err != nil {
			return err
		}
		var a persistence.MediaAsset
		if err = json.Unmarshal(result.Body, &a); err != nil {
			return err
		}
		grant, err := operation.Grant(ctx, principal, store.MerchantID, store.ID, a.ID)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, "PUT", grant.URL, bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header = grant.SignedHeader.Clone()
		response, err := client.Do(req)
		if err != nil {
			return errors.New("direct demo photo upload failed")
		}
		_ = response.Body.Close() // No response payload is used; this is not a buffered output stream.
		if response.StatusCode != 200 {
			return fmt.Errorf("direct upload failed for %s: %d", id, response.StatusCode)
		}
		if _, err = operation.Complete(ctx, principal, store.MerchantID, store.ID, a.ID, "demo-photo-complete-"+id); err != nil {
			return err
		}
		item["image_id"] = a.ID.String()
		changed++
		fmt.Println("Prepared photo:", id)
	}
	if changed == 0 && !categorized && !translated && !meals {
		fmt.Println("No missing matching demo photos or categories; existing edits preserved.")
		return nil
	}
	spec, err := httpapi.GetSwagger()
	if err != nil {
		return err
	}
	models := offering.NewRegistry(map[offering.Model]offering.Validator{food.Model: food.Validator(spec.Components.Schemas["FoodMenu"].Value)}, map[offering.Model]offering.Validator{food.Model: food.Project})
	document, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	result, err := (commerce.Offerings{DB: db, Models: models}).Write(ctx, principal, store.MerchantID, store.ID, latest.Revision, food.Model, document, "demo-attach-"+uuid.NewString())
	if err != nil {
		return err
	}
	var revision persistence.Revision
	if err = json.Unmarshal(result.Body, &revision); err != nil {
		return err
	}
	if publication.Revision != nil && *publication.Revision == latest.Revision && !publication.Suspended {
		_, err = (commerce.Publications{DB: db, Models: models}).Change(ctx, principal, store.MerchantID, store.ID, publication.Generation, &revision.Revision, "demo-publish-"+uuid.NewString(), uuid.New())
		if err != nil {
			return err
		}
		fmt.Printf("Attached and published %d photos (categories added: %t; translations added: %t; meal deals added: %t) in revision %d. Other menu content preserved.\n", changed, categorized, translated, meals, revision.Revision)
	} else {
		fmt.Printf("Attached %d photos to private revision %d. Existing publication preserved; review and publish in the workspace.\n", changed, revision.Revision)
	}
	return nil
}

// Only enrich an old, uncategorized demo. Preserve custom category choices,
// renamed dishes, removed products, prices, image references, and unpublished edits.
func attachCategories(doc map[string]any, fixture devfixtures.Menu) bool {
	if _, exists := doc["categories"]; exists {
		return false
	}
	expected := map[string]devfixtures.Item{}
	for _, item := range fixture.Items {
		expected[item.ID] = item
	}
	used := map[string]bool{}
	for _, entry := range doc["items"].([]any) {
		item := entry.(map[string]any)
		id, _ := item["id"].(string)
		original, ok := expected[id]
		if ok && original.Name == item["name"] && item["category_id"] == nil {
			item["category_id"] = original.CategoryID
			used[original.CategoryID] = true
		}
	}
	if len(used) == 0 {
		return false
	}
	categories := []devfixtures.Category{}
	for _, category := range fixture.Categories {
		if used[category.ID] {
			categories = append(categories, category)
		}
	}
	// Keep the enrichment tree consistently JSON-shaped so translation enrichment
	// also visits categories inserted during this very first seed.
	encoded, _ := json.Marshal(categories)
	var entries []any
	_ = json.Unmarshal(encoded, &entries)
	doc["categories"] = entries
	return true
}
