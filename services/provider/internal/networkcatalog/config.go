package networkcatalog

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// PublisherConfig is mounted explicitly on the worker, never returned by an API.
// Enrollment and key rotation remain provider operations, not caller parameters.
type PublisherConfig struct {
	Endpoint   string    `json:"endpoint"`
	Bindings   []Binding `json:"bindings"`
	PrivateKey string    `json:"private_key"`
}

func LoadPublisher(path string, local bool) (Publisher, error) {
	f, err := os.Open(path)
	if err != nil {
		return Publisher{}, errors.New("publisher configuration unavailable")
	}
	defer f.Close()
	var c PublisherConfig
	decoder := json.NewDecoder(io.LimitReader(f, 256*1024+1))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&c) != nil || decoder.Decode(new(any)) != io.EOF {
		return Publisher{}, errors.New("invalid publisher configuration")
	}
	endpoint, err := url.Parse(c.Endpoint)
	if err != nil || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Scheme != "https" && !(local && endpoint.Scheme == "http")) || len(c.Bindings) < 1 || len(c.Bindings) > 100 {
		return Publisher{}, errors.New("invalid publisher enrollment")
	}
	key, err := base64.StdEncoding.DecodeString(c.PrivateKey)
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return Publisher{}, errors.New("invalid publisher key")
	}
	p := Publisher{Endpoint: c.Endpoint, Bindings: map[uuid.UUID]Binding{}, PrivateKey: ed25519.PrivateKey(key), Client: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	public := base64.StdEncoding.EncodeToString(p.PrivateKey.Public().(ed25519.PublicKey))
	for _, b := range c.Bindings {
		if b.Validate(local) != nil || b.PublicKey != public {
			return Publisher{}, errors.New("publisher enrollment key mismatch")
		}
		if _, exists := p.Bindings[b.StoreID]; exists {
			return Publisher{}, errors.New("duplicate publisher store")
		}
		p.Bindings[b.StoreID] = b
	}
	return p, nil
}
