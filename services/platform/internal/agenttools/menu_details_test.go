package agenttools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"ocp.local/platform/internal/commerce"
	"ocp.local/platform/internal/core/persistence"
)

func TestFilterMenuItems(t *testing.T) {
	items := []json.RawMessage{
		json.RawMessage(`{"id":"dish-1","name":"Döner mit Käse"}`),
		json.RawMessage(`{"id":"dish-2","name":"بيتزا"}`),
		json.RawMessage(`{"id":"dish-3","name":"Pizza"}`),
		json.RawMessage(`{"id":"dish-4","name":"Pizza"}`),
	}
	for _, tc := range []struct {
		query string
		count int
	}{
		{"", 4}, {"   ", 4}, {" KÄSE ", 1}, {"بيتزا", 1}, {"DISH-2", 1},
		{"pizza", 2}, {"%", 0}, {"*", 0}, {"not present", 0},
	} {
		t.Run(tc.query, func(t *testing.T) {
			result, err := filterMenuItems(items, tc.query)
			if err != nil || len(result) != tc.count {
				t.Fatalf("matches=%d err=%v", len(result), err)
			}
		})
	}
	if _, err := filterMenuItems(items, strings.Repeat("ش", 101)); !errors.Is(err, commerce.ErrInvalid) {
		t.Fatal("unbounded query")
	}
	if _, err := filterMenuItems([]json.RawMessage{json.RawMessage(`{broken`)}, "dish"); err == nil {
		t.Fatal("ignored corrupt item")
	}
}

func TestItemReadRejectsWithoutDatabaseAccess(t *testing.T) {
	p := persistence.Principal{ID: uuid.New(), MerchantID: uuid.New(), Scopes: []string{"offering:read"}}
	for _, tc := range []struct {
		p    persistence.Principal
		in   itemInput
		want error
	}{
		{persistence.Principal{}, itemInput{Item: "dish"}, commerce.ErrForbidden},
		{p, itemInput{}, commerce.ErrInvalid},
		{p, itemInput{Item: "dish", Revision: -1}, commerce.ErrInvalid},
		{p, itemInput{Item: strings.Repeat("a", 65)}, commerce.ErrInvalid},
	} {
		_, _, _, err := (Merchant{}).loadMenuItem(context.Background(), tc.p, tc.in)
		if !errors.Is(err, tc.want) {
			t.Fatalf("error=%v want=%v", err, tc.want)
		}
	}
}
