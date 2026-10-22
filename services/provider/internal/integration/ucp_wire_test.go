package integration

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"ocp.local/platform/internal/sessionkit"
)

var upstreamSchemas struct {
	sync.Mutex
	compiled  map[string]*jsonschema.Resolved
	documents map[string]*jsonschema.Schema
}

// This optional test fetcher is not a production profile/schema fetcher. Its
// origin, source commit, and schema directory are hard-coded and caller input
// never chooses an HTTP destination. Ordinary test runs make no network calls.
func upstreamSchema(t *testing.T, domain string) *jsonschema.Resolved {
	t.Helper()
	upstreamSchemas.Lock()
	defer upstreamSchemas.Unlock()
	if upstreamSchemas.compiled == nil {
		upstreamSchemas.compiled = map[string]*jsonschema.Resolved{}
		upstreamSchemas.documents = map[string]*jsonschema.Schema{}
	}
	if s := upstreamSchemas.compiled[domain]; s != nil {
		return s
	}
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	loader := func(uri *url.URL) (*jsonschema.Schema, error) {
		if uri.Scheme != "https" || uri.Host != "ucp.dev" || !strings.HasPrefix(uri.Path, "/schemas/") || strings.Contains(uri.Path, "..") || uri.User != nil || uri.RawQuery != "" {
			return nil, fmt.Errorf("unexpected schema reference")
		}
		key := uri.Path
		if doc := upstreamSchemas.documents[key]; doc != nil {
			return doc, nil
		}
		if len(upstreamSchemas.documents) >= 150 {
			return nil, fmt.Errorf("schema budget exceeded")
		}
		location := "https://raw.githubusercontent.com/Universal-Commerce-Protocol/ucp/" + sessionkit.SourcePin + "/source" + key
		response, err := client.Get(location)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return nil, fmt.Errorf("schema fetch status %d for %s", response.StatusCode, key)
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 1048577))
		if err != nil || len(body) > 1048576 {
			return nil, fmt.Errorf("schema size/read failure")
		}
		sum := sha256.Sum256(body)
		t.Logf("UCP source %s sha256=%x", key, sum)
		var doc jsonschema.Schema
		if err = json.Unmarshal(body, &doc); err != nil {
			return nil, err
		}
		upstreamSchemas.documents[key] = &doc
		return &doc, nil
	}
	leaf := "checkout"
	if domain == "lodging" {
		leaf = "booking"
	}
	uri, _ := url.Parse("https://ucp.dev/schemas/" + domain + "/" + leaf + ".json")
	root, err := loader(uri)
	must(t, err)
	resolved, err := root.Resolve(&jsonschema.ResolveOptions{BaseURI: uri.String(), Loader: loader})
	must(t, err)
	upstreamSchemas.compiled[domain] = resolved
	return resolved
}
func validateUpstreamResponse(t *testing.T, domain string, raw []byte) {
	t.Helper()
	if os.Getenv("OCP_TEST_UCP_SOURCE") != "true" {
		return
	}
	schema := upstreamSchema(t, domain)
	var value any
	must(t, json.Unmarshal(raw, &value))
	if err := schema.Validate(value); err != nil {
		t.Fatalf("pinned upstream %s response validation: %v\n%s", domain, err, raw)
	}
	// A negative control establishes that validation is checking the root shape.
	object := value.(map[string]any)
	delete(object, "currency")
	if schema.Validate(object) == nil {
		t.Fatal("upstream required-field negative control failed")
	}
}
