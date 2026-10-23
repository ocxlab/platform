package agenttools

import (
	"encoding/json"
	"github.com/google/uuid"
	"testing"
)

func TestPublicationReviewBindsExactSelection(t *testing.T) {
	store := uuid.New()
	doc := json.RawMessage(`{"items":[]}`)
	token := publicationReviewToken(store, 2, 4, doc)
	if len(token) != 64 || token != publicationReviewToken(store, 2, 4, doc) {
		t.Fatal("unstable digest")
	}
	for _, other := range []string{publicationReviewToken(uuid.New(), 2, 4, doc), publicationReviewToken(store, 3, 4, doc), publicationReviewToken(store, 2, 5, doc), publicationReviewToken(store, 2, 4, json.RawMessage(`{"items":[1]}`))} {
		if token == other {
			t.Fatal("unbound selection")
		}
	}
}
