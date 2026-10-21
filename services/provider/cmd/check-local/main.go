// check-local checks the running scaffold, not a complete restaurant journey.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	base := os.Getenv("OCP_SMOKE_URL")
	if base == "" {
		base = "http://127.0.0.1:18180"
	}
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get(base + "/health/live")
	if err != nil {
		fail(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		fail(fmt.Errorf("API liveness returned HTTP %d", res.StatusCode))
	}
	var body struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&body); err != nil {
		fail(err)
	}
	if body.Status != "ok" || body.Service != "ocp-api" {
		fail(fmt.Errorf("unexpected liveness response: %+v", body))
	}
	req, err := http.NewRequest("GET", base+"/api/v1/merchants/00000000-0000-0000-0000-000000000001/stores/00000000-0000-0000-0000-000000000002", nil)
	if err != nil {
		fail(err)
	}
	// A well-shaped but unissued credential must reach the restricted DB lookup.
	// This read-only negative control does not create merchant data or grants.
	req.Header.Set("Authorization", "Bearer ocp_"+strings.Repeat("x", 43))
	negative, err := client.Do(req)
	if err != nil {
		fail(err)
	}
	defer negative.Body.Close()
	if negative.StatusCode != 401 {
		fail(fmt.Errorf("credential lookup negative control returned HTTP %d", negative.StatusCode))
	}
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(io.LimitReader(negative.Body, 4096)).Decode(&problem); err != nil {
		fail(err)
	}
	if problem.Code != "unauthenticated" {
		fail(fmt.Errorf("unexpected authentication failure code"))
	}
	fmt.Println("API liveness and database-backed credential rejection passed; ordering/network/payment readiness is not implemented")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
