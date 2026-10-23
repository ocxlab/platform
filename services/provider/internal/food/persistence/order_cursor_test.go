package persistence

import (
	"encoding/base64"
	"errors"
	"github.com/google/uuid"
	"strings"
	"testing"
	"time"
)

func TestOrderCursor(t *testing.T) {
	at := time.Date(2026, 10, 21, 12, 13, 14, 123456000, time.UTC)
	id := uuid.New()
	cursor := orderCursor(FoodOrder{ID: id, CreatedAt: at})
	gotAt, gotID, err := parseOrderCursor(cursor)
	if err != nil || !gotAt.Equal(at) || gotID != id {
		t.Fatal("cursor lost order position")
	}
	for _, value := range []string{"", "invalid!", strings.Repeat("A", 129), base64.RawURLEncoding.EncodeToString([]byte("wrong|" + id.String())), base64.RawURLEncoding.EncodeToString([]byte(at.Format(time.RFC3339Nano) + "|wrong"))} {
		if _, _, err := parseOrderCursor(value); !errors.Is(err, ErrOrderCursor) {
			t.Fatalf("invalid cursor accepted: %q", value)
		}
	}
}
