package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// Run only through task media:check: this does not contact any cloud service.
func TestLocalPresigned(t *testing.T) {
	if os.Getenv("OCP_TEST_LOCAL_MEDIA") != "1" {
		t.Skip("run task media:check for local SILO tests")
	}
	signer, err := NewSigner("http://localhost:18900", "us-east-1", "ocp-demo-media", credentials.NewStaticCredentialsProvider("ocp_local_media", "ocp_local_media_only_123", ""))
	if err != nil {
		t.Fatal(err)
	}
	settings := map[string]string{
		"OCP_MEDIA_ENABLED": "true", "OCP_MEDIA_INTERNAL_ENDPOINT": "http://object-store:9000",
		"OCP_MEDIA_PUBLIC_ENDPOINT": "http://localhost:18900", "OCP_MEDIA_REGION": "us-east-1",
		"OCP_MEDIA_BUCKET": "ocp-demo-media", "OCP_MEDIA_ACCESS_KEY": "ocp_local_media",
		"OCP_MEDIA_SECRET_KEY": "ocp_local_media_only_123",
	}
	storage, err := FromEnv(func(key string) string { return settings[key] })
	if err != nil {
		t.Fatal(err)
	}
	// Emulate a browser's localhost URL inside the test container. Preserve the
	// signed Host; only route the TCP connection through Docker's internal DNS.
	transport := &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr != "localhost:18900" {
			return nil, fmt.Errorf("unexpected test target")
		}
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, "object-store:9000")
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	do := func(p *v4.PresignedHTTPRequest, body []byte, contentType string) (int, []byte, http.Header) {
		t.Helper()
		req, err := http.NewRequest(p.Method, p.URL, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header = p.SignedHeader.Clone()
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal("local media request failed:", err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(io.LimitReader(resp.Body, MaxUploadBytes+1))
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, data, resp.Header
	}
	for _, id := range []string{"doener", "pizza-margherita", "falafel"} {
		t.Run(id, func(t *testing.T) {
			data, err := os.ReadFile("/workspace/assets/demo/food/" + id + ".jpg")
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(data)
			key := fmt.Sprintf("demo/food/%x.jpg", hash)
			upload, err := signer.Upload(context.Background(), key, "image/jpeg", base64.StdEncoding.EncodeToString(hash[:]), int64(len(data)), 5*time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			status, _, _ := do(upload, data, "")
			if status != 200 {
				t.Fatalf("presigned PUT status %d", status)
			}
			read, err := signer.Download(context.Background(), key, 5*time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			status, received, headers := do(read, nil, "")
			if status != 200 || !bytes.Equal(data, received) || headers.Get("Content-Type") != "image/jpeg" {
				t.Fatal("presigned GET content mismatch")
			}
			final := "verified/" + key
			w, h, err := storage.Process(context.Background(), key, final, "image/jpeg", base64.StdEncoding.EncodeToString(hash[:]), int64(len(data)))
			if err != nil || w < 1 || h < 1 || w > 960 || h > 960 {
				t.Fatalf("stored derivative failed: %dx%d, %v", w, h, err)
			}
			derivative, err := storage.Download(context.Background(), final, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			status, encoded, headers := do(derivative, nil, "")
			decoded, format, err := image.DecodeConfig(bytes.NewReader(encoded))
			if status != 200 || err != nil || format != "jpeg" || decoded.Width != w || decoded.Height != h || headers.Get("Content-Type") != "image/jpeg" {
				t.Fatal("stored JPEG derivative did not match checked dimensions")
			}
			if _, _, err := storage.Process(context.Background(), key, final, "image/jpeg", base64.StdEncoding.EncodeToString(make([]byte, 32)), int64(len(data))); err != ErrInvalid {
				t.Fatal("processing accepted a mismatching declared checksum")
			}
			if status, _, _ := do(upload, data, "text/html"); status != 403 {
				t.Fatalf("changed signed type accepted: %d", status)
			}
			bad := bytes.Clone(data)
			bad[len(bad)-1] ^= 1
			if status, _, _ := do(upload, bad, ""); status < 400 || status >= 500 {
				t.Fatalf("body checksum not rejected: %d", status)
			}
			status, received, _ = do(read, nil, "")
			if status != 200 || !bytes.Equal(data, received) {
				t.Fatal("rejected overwrite changed stored content")
			}
			mutated := *read
			u, _ := url.Parse(read.URL)
			u.Path += "-other"
			mutated.URL = u.String()
			if status, _, _ := do(&mutated, nil, ""); status != 403 {
				t.Fatalf("changed key accepted: %d", status)
			}
			u.RawQuery = ""
			mutated.URL = u.String()
			if status, _, _ := do(&mutated, nil, ""); status != 403 {
				t.Fatalf("anonymous read accepted: %d", status)
			}
			expiring, err := signer.Download(context.Background(), key, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			time.Sleep(2 * time.Second)
			if status, _, _ := do(expiring, nil, ""); status != 403 {
				t.Fatalf("expired URL accepted: %d", status)
			}
			// Browser CORS preflight, not a claim that a real browser uploaded.
			preflight := &v4.PresignedHTTPRequest{URL: strings.Split(upload.URL, "?")[0], Method: "OPTIONS", SignedHeader: http.Header{
				"Origin": {"http://localhost:18181"}, "Access-Control-Request-Method": {"PUT"}, "Access-Control-Request-Headers": {"content-type,x-amz-checksum-sha256"},
			}}
			status, _, headers = do(preflight, nil, "")
			if status != 200 || headers.Get("Access-Control-Allow-Origin") != "http://localhost:18181" {
				t.Fatalf("local browser CORS preflight failed: %d", status)
			}
			preflight.SignedHeader.Set("Origin", "https://untrusted.example")
			status, _, headers = do(preflight, nil, "")
			if status != 403 || headers.Get("Access-Control-Allow-Origin") != "" {
				t.Fatalf("unapproved CORS origin accepted: %d", status)
			}
			t.Log("direct presigned upload/read, byte integrity, signature tampering, expiry, private access, and CORS checked")
		})
	}
}
