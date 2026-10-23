package database

import "testing"

func TestPoolBudget(t *testing.T) {
	for _, test := range []struct {
		open, idle string
		valid      bool
	}{{"", "", true}, {"2", "1", true}, {"64", "0", true}, {"0", "0", false}, {"65", "0", false}, {"2", "3", false}, {"4", "-1", false}, {"one", "1", false}} {
		_, err := PoolFromEnv(func(name string) string {
			if name == "OCP_DB_MAX_OPEN" {
				return test.open
			}
			return test.idle
		})
		if (err == nil) != test.valid {
			t.Fatalf("pool %+v: %v", test, err)
		}
	}
}
