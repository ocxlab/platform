package media

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials"
)

func TestPresignedUpload(t *testing.T) {
	s, err := NewSigner("http://localhost:18900", "us-east-1", "ocp-demo-media", credentials.NewStaticCredentialsProvider("local", "local-secret", ""))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("demo"))
	checksum := base64.StdEncoding.EncodeToString(sum[:])
	p, err := s.Upload(context.Background(), "merchant-1/images/item-1.jpg", "image/jpeg", checksum, 4, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(p.URL)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "localhost:18900" || u.Path != "/ocp-demo-media/merchant-1/images/item-1.jpg" || p.Method != "PUT" || u.Query().Get("X-Amz-Expires") != "300" {
		t.Fatalf("unexpected signing target: %s %s", p.Method, u.Host)
	}
	if !strings.Contains(u.Query().Get("X-Amz-SignedHeaders"), "content-type") {
		t.Fatalf("content type not signed: %s", u.Query().Get("X-Amz-SignedHeaders"))
	}
	if !strings.Contains(u.Query().Get("X-Amz-SignedHeaders"), "content-length") {
		t.Fatal("content length not signed")
	}
	if p.SignedHeader.Get("Content-Type") != "image/jpeg" {
		t.Fatal("missing required content type")
	}
	if p.SignedHeader.Get("X-Amz-Checksum-Sha256") != checksum && u.Query().Get("X-Amz-Checksum-Sha256") != checksum {
		t.Fatal("missing signed checksum")
	}
	for _, key := range []string{"", "../other", "m/../other", "/absolute", "m//a", "m/a?x", "m/a\\b"} {
		if _, err := s.Upload(context.Background(), key, "image/jpeg", checksum, 4, time.Minute); err == nil {
			t.Errorf("accepted invalid key %q", key)
		}
	}
	for _, typ := range []string{"image/svg+xml", "text/html", "image/gif"} {
		if _, err := s.Upload(context.Background(), "m/a", typ, checksum, 4, time.Minute); err == nil {
			t.Errorf("accepted type %q", typ)
		}
	}
	for _, size := range []int64{0, -1, MaxUploadBytes + 1} {
		if _, err := s.Upload(context.Background(), "m/a", "image/jpeg", checksum, size, time.Minute); err == nil {
			t.Errorf("accepted size %d", size)
		}
	}
	for _, ttl := range []time.Duration{0, 16 * time.Minute} {
		if _, err := s.Download(context.Background(), "m/a", ttl); err == nil {
			t.Errorf("accepted TTL %s", ttl)
		}
	}
	if _, err := s.Upload(context.Background(), "m/a", "image/jpeg", "bad-checksum", 4, time.Minute); err == nil {
		t.Fatal("accepted invalid checksum")
	}
}

func TestInvalidSigningConfiguration(t *testing.T) {
	creds := credentials.NewStaticCredentialsProvider("local", "local-secret", "")
	for _, endpoint := range []string{"", "file:///tmp", "https://user:secret@host", "https://host/path", "https://host?query=1", "https://host#fragment"} {
		if _, err := NewSigner(endpoint, "auto", "media", creds); err == nil {
			t.Errorf("accepted endpoint %q", endpoint)
		}
	}
	if _, err := NewSigner("https://host", "auto", "media", nil); err == nil {
		t.Fatal("accepted missing credentials")
	}
}
