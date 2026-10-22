// network-lab is a private, synthetic native-runtime acceptance harness. It does
// not provision a public registry, expose production credentials, or mock ONIX.
package main

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/capability"
	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/networkcatalog"
	"ocp.local/platform/internal/offering"
	"ocp.local/platform/internal/offering/food"
	"ocp.local/platform/internal/persistence"
	"ocp.local/platform/internal/work"
)

const root = "/workspace/working/native-onix"
const adminDSN = "postgres://ocp_network:ocp_network_local_only@network-db:5432/ocp_network?sslmode=disable"

type peer struct {
	Name       string                 `json:"name"`
	Binding    networkcatalog.Binding `json:"binding"`
	PrivateKey string                 `json:"private_key"`
	Token      string                 `json:"token"`
}
type state struct {
	Peers    []peer              `json:"peers"`
	Registry []map[string]string `json:"registry"`
}

func main() {
	mode := flag.String("mode", "serve", "init or serve")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	switch *mode {
	case "init":
		err = initialize(ctx)
	case "serve":
		err = serve(ctx)
	default:
		err = errors.New("unknown lab mode")
	}
	if err != nil {
		log.Fatal(err)
	}
}
func databaseURL(name string, app bool) string {
	u, _ := url.Parse(adminDSN)
	u.Path = "/" + name
	if app {
		u.User = url.UserPassword("ocp_app", "ocp_app_local_only")
	}
	return u.String()
}
func writeJSON(name string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, name), raw, 0600)
}
func load() (state, error) {
	var s state
	raw, err := os.ReadFile(filepath.Join(root, "state.json"))
	if err == nil {
		err = json.Unmarshal(raw, &s)
	}
	return s, err
}

func initialize(ctx context.Context) error {
	if _, err := os.Stat(filepath.Join(root, "state.json")); err == nil {
		for _, name := range []string{"ocp_network", "provider_a", "provider_b"} {
			db, err := persistence.Open(ctx, databaseURL(name, false))
			if err != nil {
				return err
			}
			err = persistence.Migrate(ctx, db)
			db.Close()
			if err != nil {
				return err
			}
		}
		fmt.Println("Retained local network state and applied pending migrations")
		s, err := load()
		if err != nil {
			return err
		}
		return writeWorkerConfigs(s)
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	admin, err := persistence.Open(ctx, adminDSN)
	if err != nil {
		return err
	}
	defer admin.Close()
	for _, name := range []string{"provider_a", "provider_b", "catalog"} {
		var exists bool
		if err = admin.QueryRowContext(ctx, `SELECT EXISTS(SELECT FROM pg_database WHERE datname=$1)`, name).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			if _, err = admin.ExecContext(ctx, `CREATE DATABASE `+name); err != nil {
				return err
			}
		}
	}
	if err = persistence.Migrate(ctx, admin); err != nil {
		return err
	}
	var s state
	registry := map[string]any{"id": "registry", "config": map[string]string{"url": "http://network-lab:8090", "retry_max": "0"}}
	for n, name := range []string{"provider-a", "provider-b"} {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		encryption, err := ecdh.X25519().GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		dbName := strings.ReplaceAll(name, "-", "_")
		db, err := persistence.Open(ctx, databaseURL(dbName, false))
		if err != nil {
			return err
		}
		if err = persistence.Migrate(ctx, db); err != nil {
			db.Close()
			return err
		}
		principal, token, err := persistence.Provision(ctx, db, name+" restaurant", []string{"store:read", "store:write", "offering:read", "offering:write", "order:read", "order:manage"}, time.Now().Add(30*24*time.Hour))
		if err != nil {
			db.Close()
			return err
		}
		tx, err := persistence.TenantTransaction(ctx, db, principal.MerchantID)
		if err != nil {
			db.Close()
			return err
		}
		store, err := persistence.CreateStore(ctx, tx, principal.MerchantID, name+" restaurant")
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		db.Close()
		if err != nil {
			return err
		}
		p := peer{Name: name, Binding: networkcatalog.Binding{ProviderID: name + ".lab", MerchantID: principal.MerchantID, StoreID: store.ID, APIBase: fmt.Sprintf("http://network-lab:%d/api/v1", 8091+n), PublicKey: base64.StdEncoding.EncodeToString(public)}, PrivateKey: base64.StdEncoding.EncodeToString(private), Token: token}
		s.Peers = append(s.Peers, p)
		if _, err = admin.ExecContext(ctx, `INSERT INTO discovery.bindings(store_id,merchant_id,provider_id,api_base,public_key) VALUES($1,$2,$3,$4,$5)`, p.Binding.StoreID, p.Binding.MerchantID, p.Binding.ProviderID, p.Binding.APIBase, p.Binding.PublicKey); err != nil {
			return err
		}
		key := map[string]string{"subscriberId": p.Binding.ProviderID, "keyId": "lab-key", "signingPrivateKey": base64.StdEncoding.EncodeToString(private.Seed()), "signingPublicKey": p.Binding.PublicKey, "encrPrivateKey": base64.StdEncoding.EncodeToString(encryption.Bytes()), "encrPublicKey": base64.StdEncoding.EncodeToString(encryption.PublicKey().Bytes())}
		s.Registry = append(s.Registry, map[string]string{"subscriber_id": p.Binding.ProviderID, "url": "http://onix-" + name + ":8080", "type": "BPP", "key_id": "lab-key", "signing_public_key": p.Binding.PublicKey, "encr_public_key": key["encrPublicKey"], "status": "SUBSCRIBED"})
		if err = os.MkdirAll(filepath.Join(root, name), 0700); err != nil {
			return err
		}
		plugins := map[string]any{"registry": registry, "keyManager": map[string]any{"id": "simplekeymanager", "config": key}, "catalogBlobStore": map[string]any{"id": "localcatalogblobstore", "config": map[string]string{"root": root + "/" + name}}, "catalogPublisher": map[string]any{"id": "catalogpublisher", "config": map[string]string{"catalogBaseURL": "http://network-lab:8090/" + name, "nextUpdateIn": "24h", "gzip": "false", "allowPrivateHosts": "true"}}}
		config := common(name)
		config["modules"] = []any{map[string]any{"name": "publish", "path": "/catalog/publish", "handler": map[string]any{"type": "catalogPublish", "role": "bpp", "plugins": plugins}}}
		if err = writeJSON(name+".json", config); err != nil {
			return err
		}
	}
	crawler := common("crawler")
	crawler["plugins"] = map[string]any{"registry": registry, "crawler": map[string]any{"id": "catalogcrawler", "config": map[string]string{"dbDsn": databaseURL("catalog", false), "staticIndexUrls": "http://network-lab:8090/provider-a/index/becknCatalogs.index.json,http://network-lab:8090/provider-b/index/becknCatalogs.index.json", "discoveryPushUrl": "http://network-lab:8090/internal/catalog-push", "participantId": "crawler.lab", "bppUri": "http://onix-crawler:8080", "indexIntervalSeconds": "2", "catalogIntervalSeconds": "2", "maxAttempts": "3", "allowPrivateHosts": "true", "maxPushBytes": "1048576"}}}
	crawler["modules"] = []any{map[string]any{"name": "crawl", "path": "/crawl/", "handler": map[string]any{"type": "catalogCrawl", "authDisabled": true}}}
	if err = writeJSON("crawler.json", crawler); err != nil {
		return err
	}
	if err = writeJSON("state.json", s); err != nil {
		return err
	}
	if err = writeWorkerConfigs(s); err != nil {
		return err
	}
	fmt.Println("Created synthetic merchants, separate provider databases, enrolled keys, and native ONIX configuration; secrets stay in ignored working/native-onix")
	return nil
}
func writeWorkerConfigs(s state) error {
	for _, p := range s.Peers {
		if err := writeJSON(p.Name+"-worker.json", networkcatalog.PublisherConfig{Endpoint: "http://onix-" + p.Name + ":8080/catalog/publish", Bindings: []networkcatalog.Binding{p.Binding}, PrivateKey: p.PrivateKey}); err != nil {
			return err
		}
		// The scratch runtime runs as 65532. Keep private mode 0600, granting
		// ownership only to that worker rather than making keys world-readable.
		if err := os.Chown(filepath.Join(root, p.Name+"-worker.json"), 65532, 65532); err != nil {
			return err
		}
	}
	return nil
}
func common(name string) map[string]any {
	return map[string]any{"appName": name, "log": map[string]any{"level": "info", "destinations": []any{map[string]string{"type": "stdout"}}}, "http": map[string]any{"port": "8080", "timeout": map[string]int{"read": 30, "write": 30, "idle": 30}}, "pluginManager": map[string]any{"root": "/app/plugins", "becknConstants": map[string]bool{"disableRemoteRefresh": true}}}
}

func serve(ctx context.Context) error {
	s, err := load()
	if err != nil {
		return err
	}
	db, err := persistence.Open(ctx, databaseURL("ocp_network", true))
	if err != nil {
		return err
	}
	defer db.Close()
	if err = persistence.CheckRuntimeRole(ctx, db); err != nil {
		return err
	}
	spec, err := httpapi.GetSwagger()
	if err != nil {
		return err
	}
	models := offering.NewRegistry(map[offering.Model]offering.Validator{food.Model: food.Validator(spec.Components.Schemas["FoodMenu"].Value), food.ModelV2: food.ValidatorV2(spec.Components.Schemas["FoodMenuV2"].Value, spec.Components.Schemas["FoodMenu"].Value)}, map[offering.Model]offering.Validator{food.Model: food.Project, food.ModelV2: food.Project})
	bindings := map[uuid.UUID]networkcatalog.Binding{}
	for _, p := range s.Peers {
		bindings[p.Binding.StoreID] = p.Binding
	}
	index := networkcatalog.Index{DB: db, Bindings: bindings, Models: models}
	mux := http.NewServeMux()
	mux.Handle("POST /internal/catalog-push", index.PushHandler())
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"status":"ok"}`)) })
	mux.HandleFunc("POST /lookup", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var q map[string]any
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			w.WriteHeader(400)
			return
		}
		rows := []map[string]string{}
		for _, row := range s.Registry {
			match := true
			for k, v := range q {
				if text, ok := v.(string); ok && text != "" && row[k] != "" && row[k] != text {
					match = false
				}
			}
			if match {
				rows = append(rows, row)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rows)
	})
	mux.HandleFunc("GET /search", func(w http.ResponseWriter, r *http.Request) {
		rows, err := index.Search(r.Context(), r.URL.Query().Get("q"))
		if err != nil {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": rows})
	})
	for _, name := range []string{"provider-a", "provider-b"} {
		for _, kind := range []string{"index", "catalogs"} {
			prefix := "/" + name + "/" + kind + "/"
			files, err := os.OpenRoot(filepath.Join(root, name, kind))
			if err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					return err
				}
				if err = os.MkdirAll(filepath.Join(root, name, kind), 0700); err != nil {
					return err
				}
				files, err = os.OpenRoot(filepath.Join(root, name, kind))
				if err != nil {
					return err
				}
			}
			defer files.Close()
			mux.Handle("GET "+prefix, http.StripPrefix(prefix, http.FileServerFS(files.FS())))
		}
	}
	servers := []*http.Server{{Addr: ":8090", Handler: mux, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}}
	result := make(chan error, len(s.Peers)*2+1)
	for n, p := range s.Peers {
		app, err := persistence.Open(ctx, databaseURL(strings.ReplaceAll(p.Name, "-", "_"), true))
		if err != nil {
			return err
		}
		defer app.Close()
		if err = persistence.CheckRuntimeRole(ctx, app); err != nil {
			return err
		}
		handler, err := httpapi.NewHandler(app)
		if err != nil {
			return err
		}
		servers = append(servers, &http.Server{Addr: fmt.Sprintf(":%d", 8091+n), Handler: handler, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second})
		private, err := base64.StdEncoding.DecodeString(p.PrivateKey)
		if err != nil {
			return err
		}
		publisher := networkcatalog.Publisher{Endpoint: "http://onix-" + p.Name + ":8080/catalog/publish", Bindings: map[uuid.UUID]networkcatalog.Binding{p.Binding.StoreID: p.Binding}, PrivateKey: ed25519.PrivateKey(private)}
		worker := work.Worker{DB: app, Domains: []capability.Domain{capability.Food}, Handlers: map[string]work.Handler{"food/catalog.publish": publisher.Deliver}, Concurrency: 2}
		go func() { result <- worker.Run(ctx) }()
	}
	for _, server := range servers {
		go func() { result <- server.ListenAndServe() }()
	}
	select {
	case err = <-result:
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, server := range servers {
		_ = server.Shutdown(shutdown)
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
