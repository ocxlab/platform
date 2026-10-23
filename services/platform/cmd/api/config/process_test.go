package runtimeconfig

import "testing"

func TestProcessSelection(t *testing.T) {
	for _, test := range []struct {
		service, role, domains string
		want                   Process
	}{
		{"", "public", "", Process{"public", "combined", ""}},
		{"", "internal", "lodging", Process{"internal", "combined", "lodging"}},
		{"", "worker", "food", Process{"worker", "combined", "food"}},
		{"combined", "public", "none", Process{"public", "combined", "none"}},
		{"core", "public", "", Process{"public", "core", ""}},
		{"food", "public", "", Process{"public", "food", "food"}},
		{"shopping", "public", "", Process{"internal", "shopping", "shopping"}},
		{"lodging", "public", "lodging", Process{"internal", "lodging", "lodging"}},
	} {
		t.Run(test.service+"/"+test.role+"/"+test.domains, func(t *testing.T) {
			got, err := Resolve(test.service, test.role, test.domains)
			if err != nil || got != test.want {
				t.Fatalf("got %+v, %v; want %+v", got, err, test.want)
			}
		})
	}
}

func TestRejectContradictoryProcessSelection(t *testing.T) {
	for _, input := range [][3]string{{"other", "public", ""}, {"core", "internal", ""}, {"", "unknown", ""}, {"food", "public", "none"}, {"shopping", "public", "lodging"}, {"lodging", "public", "shopping,lodging"}} {
		if _, err := Resolve(input[0], input[1], input[2]); err == nil {
			t.Fatalf("accepted contradictory selection: %v", input)
		}
	}
}
