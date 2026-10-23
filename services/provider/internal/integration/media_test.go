package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"ocp.local/platform/internal/database"
	"sync/atomic"
	"testing"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/media"
)

type fakeMedia struct {
	signer    *media.Signer
	processed atomic.Int32
}

func (m *fakeMedia) Upload(ctx context.Context, key, kind, sum string, size int64, ttl time.Duration) (*v4.PresignedHTTPRequest, error) {
	return m.signer.Upload(ctx, key, kind, sum, size, ttl)
}
func (m *fakeMedia) Download(ctx context.Context, key string, ttl time.Duration) (*v4.PresignedHTTPRequest, error) {
	return m.signer.Download(ctx, key, ttl)
}
func (m *fakeMedia) Process(context.Context, string, string, string, string, int64) (int, int, error) {
	m.processed.Add(1)
	return 640, 480, nil
}

func TestMediaOwnershipReplayAndPublication(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	signer, err := media.NewSigner("http://storage.invalid", "local", "ocp-images", credentials.NewStaticCredentialsProvider("local", "local-secret", ""))
	must(t, err)
	storage := &fakeMedia{signer: signer}
	handler, err := httpapi.NewHandler(f.app, httpapi.WithMediaStorage(storage))
	must(t, err)
	f.server.Close()
	f.server = httptest.NewServer(handler)
	t.Cleanup(f.server.Close)
	var store persistence.Store
	must(t, json.Unmarshal(f.request(t, "POST", f.merchantPath, f.token, "media-store-create01", `{"display_name":"Media restaurant"}`, 201), &store))
	path := f.merchantPath + "/" + store.ID.String()
	declaration := `{"content_type":"image/jpeg","size_bytes":100,"sha256":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}`
	body := f.request(t, "POST", path+"/media", f.token, "media-upload-start01", declaration, 201)
	if !bytes.Equal(body, f.request(t, "POST", path+"/media", f.token, "media-upload-start01", declaration, 201)) {
		t.Fatal("allocation replay changed")
	}
	f.request(t, "POST", path+"/media", f.token, "media-upload-start01", `{"content_type":"image/png","size_bytes":100,"sha256":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}`, 409)
	var a persistence.MediaAsset
	must(t, json.Unmarshal(body, &a))
	if a.Width != nil || a.Height != nil || a.State != "pending" {
		t.Fatalf("pending asset lost nullable dimensions: %+v", a)
	}
	asset := path + "/media/" + a.ID.String()
	f.request(t, "GET", asset, "", "", "", 401)
	f.request(t, "GET", asset, f.token, "", "", 200)
	f.request(t, "POST", asset+"/upload-grant", f.token, "", "", 200)
	f.request(t, "POST", path+"/media/"+uuid.NewString()+"/upload-grant", f.token, "", "", 404)
	other, token, err := persistence.Provision(ctx, f.admin, "Other merchant", []string{"offering:read", "offering:write"}, time.Now().Add(time.Hour))
	must(t, err)
	f.request(t, "GET", asset, token, "", "", 403)
	f.request(t, "POST", asset+"/completion", token, "foreign-complete-01", "", 403)
	ownPath := "/api/v1/merchants/" + other.MerchantID.String() + "/stores/" + store.ID.String() + "/media/" + a.ID.String()
	f.request(t, "GET", ownPath, token, "", "", 404)
	_, readToken, err := persistence.ProvisionMerchantGrant(ctx, f.admin, f.principal.MerchantID, []string{"offering:read"}, time.Now().Add(time.Hour))
	must(t, err)
	f.request(t, "POST", asset+"/upload-grant", readToken, "", "", 403)
	doc := menu(t)
	item := doc["items"].([]any)[0].(map[string]any)
	item["image_id"] = a.ID.String()
	f.request(t, "PUT", path+"/offering", f.token, "image-pending-draft1", offeringBody(t, 0, doc, "ocp.food.menu"), 422)
	ready := f.request(t, "POST", asset+"/completion", f.token, "media-completion-001", "", 201)
	var completed persistence.MediaAsset
	must(t, json.Unmarshal(ready, &completed))
	if completed.Width == nil || completed.Height == nil || *completed.Width != 640 || *completed.Height != 480 || completed.State != "ready" {
		t.Fatalf("completed asset lost dimensions: %+v", completed)
	}
	if !bytes.Equal(ready, f.request(t, "POST", asset+"/completion", f.token, "media-completion-001", "", 201)) {
		t.Fatal("completion replay changed")
	}
	if storage.processed.Load() != 1 {
		t.Fatal("ready retry reprocessed bytes")
	}
	f.request(t, "POST", asset+"/upload-grant", f.token, "", "", 422)
	f.request(t, "PUT", path+"/offering", f.token, "image-ready-draft001", offeringBody(t, 0, doc, "ocp.food.menu"), 201)
	publicPath := "/api/v1/public/merchants/" + f.principal.MerchantID.String() + "/stores/" + store.ID.String() + "/media/" + a.ID.String() + "/image"
	f.request(t, "GET", publicPath, "", "", "", 404)
	// Same-tenant, different-store attachment is also denied.
	var second persistence.Store
	must(t, json.Unmarshal(f.request(t, "POST", f.merchantPath, f.token, "media-store-create02", `{"display_name":"Second store"}`, 201), &second))
	f.request(t, "PUT", f.merchantPath+"/"+second.ID.String()+"/offering", f.token, "image-cross-store01", offeringBody(t, 0, doc, "ocp.food.menu"), 422)
	tx, err := database.TenantTransaction(ctx, f.admin, f.principal.MerchantID)
	must(t, err)
	_, err = tx.Exec(`UPDATE commerce.stores SET published_revision=1,public_document=(SELECT document FROM commerce.offering_revisions WHERE merchant_id=$1 AND store_id=$2 AND revision=1),publication_generation=1 WHERE merchant_id=$1 AND id=$2`, f.principal.MerchantID, store.ID)
	must(t, err)
	must(t, tx.Commit())
	// Do not follow the presigned URL: this boundary test has a fake storage
	// processor. Local SILO tests separately exercise actual bytes and signatures.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(f.server.URL + publicPath)
	must(t, err)
	response.Body.Close()
	if response.StatusCode != 307 || response.Header.Get("Location") == "" || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("missing private short-lived redirect")
	}
	_, err = f.admin.Exec(`UPDATE commerce.stores SET publication_suspended=true WHERE merchant_id=$1 AND id=$2`, f.principal.MerchantID, store.ID)
	must(t, err)
	f.request(t, "GET", publicPath, "", "", "", 404)
	// RLS is real: a tenant transaction cannot enumerate the other tenant.
	tx, err = database.TenantTransaction(ctx, f.app, other.MerchantID)
	must(t, err)
	defer tx.Rollback()
	var count int
	must(t, tx.QueryRow(`SELECT count(*) FROM commerce.media_assets`).Scan(&count))
	if count != 0 {
		t.Fatal("media RLS leak")
	}
}
