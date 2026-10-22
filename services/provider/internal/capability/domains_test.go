package capability

import (
	"reflect"
	"testing"
)

func TestDomainAdmission(t *testing.T) {
	installed := []Domain{Food, Shopping, Lodging}
	for _, tc := range []struct {
		raw  string
		want []Domain
	}{
		{"", []Domain{Food}}, {"none", []Domain{}},
		{"lodging,food", []Domain{Food, Lodging}}, {" shopping ", []Domain{Shopping}},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := Parse(tc.raw, installed, []Domain{Food})
			if err != nil || !reflect.DeepEqual(got.Enabled(), tc.want) {
				t.Fatalf("got %v, %v; want %v", got.Enabled(), err, tc.want)
			}
			for _, d := range installed {
				want := false
				for _, allowed := range tc.want {
					if allowed == d {
						want = true
					}
				}
				if got.Allows(d) != want {
					t.Fatalf("admission for %s", d)
				}
			}
			copy := got.Enabled()
			if len(copy) > 0 {
				copy[0] = Domain("changed")
				if reflect.DeepEqual(copy, got.Enabled()) {
					t.Fatal("mutable admission leaked")
				}
			}
			if got.Allows("unknown") {
				t.Fatal("unknown domain admitted")
			}
		})
	}
}

func TestInvalidDomainConfiguration(t *testing.T) {
	for _, raw := range []string{"food,food", "FOOD", "food,", ",food", " ", "none,food", "shopping", "unknown"} {
		if _, err := Parse(raw, []Domain{Food}, []Domain{Food}); err != ErrConfiguration {
			t.Fatalf("accepted %q: %v", raw, err)
		}
	}
	for _, installed := range [][]Domain{{Food, Food}, {Domain("unknown")}} {
		if _, err := Parse("none", installed, nil); err != ErrConfiguration {
			t.Fatal("invalid installed set")
		}
	}
	if _, err := Parse("", []Domain{Food}, []Domain{Lodging}); err != ErrConfiguration {
		t.Fatal("uninstalled default")
	}
	if (Domains{}).Allows(Food) {
		t.Fatal("zero value allows a domain")
	}
}
