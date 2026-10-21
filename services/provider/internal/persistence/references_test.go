package persistence

import "testing"

func TestOrderReference(t *testing.T) {
	for number, want := range map[int64]string{1: "O-000001", 42: "O-000042", 999999: "O-999999", 1000000: "O-1000000"} {
		if got := orderReference(number); got != want {
			t.Fatalf("%d: got %q want %q", number, got, want)
		}
	}
}
