// check-local checks the running scaffold, not a complete restaurant journey.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	fmt.Println("API process liveness passed; commerce readiness is not implemented")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
