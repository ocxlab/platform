package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"ocp.local/platform/internal/networkcatalog"
	"ocp.local/platform/internal/persistence"
)

// Real native ONIX services run in the opt-in isolated Docker composition. This
// test is intentionally separate from same-author HTTP/schema fixture tests.
func TestNativeONIXFoodJourney(t *testing.T) {
	if os.Getenv("OCP_TEST_NATIVE_NETWORK") != "true" {
		t.Skip("requires task test:network")
	}
	var state struct {
		Peers []struct {
			Name    string                 `json:"name"`
			Binding networkcatalog.Binding `json:"binding"`
			Token   string                 `json:"token"`
		} `json:"peers"`
	}
	raw, err := os.ReadFile("/workspace/working/native-onix/state.json")
	must(t, err)
	must(t, json.Unmarshal(raw, &state))
	if len(state.Peers) != 2 {
		t.Fatal("two provisioned providers required")
	}
	client := &http.Client{Timeout: 5 * time.Second}
	request := func(method, endpoint, token string, body any, want int) []byte {
		t.Helper()
		var encoded []byte
		if body != nil {
			encoded, err = json.Marshal(body)
			must(t, err)
		}
		req, err := http.NewRequest(method, endpoint, bytes.NewReader(encoded))
		must(t, err)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "native-network-"+strings.ReplaceAll(time.Now().Format("150405.000000000"), ".", ""))
		}
		response, err := client.Do(req)
		must(t, err)
		defer response.Body.Close()
		result, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		must(t, err)
		if response.StatusCode != want {
			t.Fatalf("%s %s: wanted%d got%d %s", method, endpoint, want, response.StatusCode, result)
		}
		return result
	}
	wait := func(label string, check func() bool) {
		t.Helper()
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) {
			if check() {
				t.Log(label)
				return
			}
			time.Sleep(300 * time.Millisecond)
		}
		t.Fatalf("timed out: %s", label)
	}
	wait("owned provider endpoints ready", func() bool {
		for _, p := range state.Peers {
			response, err := client.Get(strings.TrimSuffix(p.Binding.APIBase, "/api/v1") + "/health/live")
			if err != nil {
				return false
			}
			response.Body.Close()
			if response.StatusCode != 200 {
				return false
			}
		}
		return true
	})
	search := func() []networkcatalog.Result {
		t.Helper()
		var page struct {
			Items []networkcatalog.Result `json:"items"`
		}
		must(t, json.Unmarshal(request("GET", "http://network-lab:8090/search", "", nil, 200), &page))
		return page.Items
	}
	var publications [2]persistence.Publication
	var revisions [2]int64
	for n, p := range state.Peers {
		private := fmt.Sprintf("%s/merchants/%s/stores/%s", p.Binding.APIBase, p.Binding.MerchantID, p.Binding.StoreID)
		must(t, json.Unmarshal(request("GET", private+"/publication", p.Token, nil, 200), &publications[n]))
		// Use the immutable latest revision counter, including earlier retained runs.
		req, _ := http.NewRequest("GET", private+"/offering", nil)
		req.Header.Set("Authorization", "Bearer "+p.Token)
		response, err := client.Do(req)
		must(t, err)
		var current persistence.Revision
		if response.StatusCode == 200 {
			must(t, json.NewDecoder(response.Body).Decode(&current))
		} else if response.StatusCode != 404 {
			t.Fatal("unexpected offering read")
		}
		response.Body.Close()
		body := map[string]any{"base_revision": current.Revision, "model": map[string]string{"id": "ocp.food.menu", "version": "1.0.0"}, "document": menu(t)}
		var revision persistence.Revision
		must(t, json.Unmarshal(request("PUT", private+"/offering", p.Token, body, 201), &revision))
		revisions[n] = revision.Revision
		must(t, json.Unmarshal(request("PUT", private+"/publication", p.Token, map[string]any{"expected_generation": publications[n].Generation, "revision": revision.Revision}, 201), &publications[n]))
	}
	wait("both native providers crawled into owned discovery", func() bool {
		rows := search()
		if len(rows) != 2 {
			return false
		}
		for _, row := range rows {
			found := false
			for n, p := range state.Peers {
				if row.Binding == p.Binding && row.Generation == publications[n].Generation {
					found = true
				}
			}
			if !found {
				return false
			}
		}
		return true
	})
	for _, row := range search() {
		endpoint := fmt.Sprintf("%s/public/merchants/%s/stores/%s/food/quotes", row.APIBase, row.MerchantID, row.StoreID)
		var quote persistence.FoodQuote
		var selectionBody any
		must(t, json.Unmarshal([]byte(selection(row.Generation)), &selectionBody))
		must(t, json.Unmarshal(request("POST", endpoint, "", selectionBody, 201), &quote))
		if quote.Total != 1400 || quote.StoreID != row.StoreID {
			t.Fatal("discovery did not lead to authoritative current quote")
		}
	}
	p := state.Peers[0]
	private := fmt.Sprintf("%s/merchants/%s/stores/%s", p.Binding.APIBase, p.Binding.MerchantID, p.Binding.StoreID)
	doc := menu(t)
	doc["items"].([]any)[0].(map[string]any)["variants"].([]any)[0].(map[string]any)["price_minor"] = 900
	var revision persistence.Revision
	must(t, json.Unmarshal(request("PUT", private+"/offering", p.Token, map[string]any{"base_revision": revisions[0], "model": map[string]string{"id": "ocp.food.menu", "version": "1.0.0"}, "document": doc}, 201), &revision))
	oldGeneration := publications[0].Generation
	must(t, json.Unmarshal(request("PUT", private+"/publication", p.Token, map[string]any{"expected_generation": oldGeneration, "revision": revision.Revision}, 201), &publications[0]))
	wait("menu update reached discovery through native crawl", func() bool {
		for _, row := range search() {
			if row.StoreID == p.Binding.StoreID {
				return row.Generation == publications[0].Generation
			}
		}
		return false
	})
	public := strings.Replace(private, "/merchants/", "/public/merchants/", 1)
	var selected any
	must(t, json.Unmarshal([]byte(selection(oldGeneration)), &selected))
	request("POST", public+"/food/quotes", "", selected, 409)
	must(t, json.Unmarshal([]byte(selection(publications[0].Generation)), &selected))
	var quote persistence.FoodQuote
	must(t, json.Unmarshal(request("POST", public+"/food/quotes", "", selected, 201), &quote))
	if quote.Total != 1800 {
		t.Fatal("updated authoritative price not applied")
	}
	wrong := fmt.Sprintf("%s/public/merchants/%s/stores/%s/offering", state.Peers[1].Binding.APIBase, p.Binding.MerchantID, p.Binding.StoreID)
	request("GET", wrong, "", nil, 404)
	must(t, json.Unmarshal(request("PUT", private+"/publication", p.Token, map[string]any{"expected_generation": publications[0].Generation}, 201), &publications[0]))
	wait("withdrawal tombstone removed store from discovery", func() bool {
		rows := search()
		return len(rows) == 1 && rows[0].StoreID == state.Peers[1].Binding.StoreID
	})
	request("POST", public+"/food/quotes", "", selected, 404)
	t.Log("native publish/update/crawl/search/withdraw + current quote, stale generation, and wrong provider binding checks passed; synthetic static enrollment only")
}
