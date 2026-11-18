package main

import "testing"

func TestValidateRequestedQuota(t *testing.T) {
	const gib = int64(1) << 30
	tests := []struct {
		name      string
		requested int64
		ok        bool
	}{
		{"veca od trenutne", 2 * gib, true},
		{"jednaka trenutnoj", 1 * gib, false},
		{"manja od trenutne", gib / 2, false},
		{"nula", 0, false},
		{"negativna", -5, false},
		{"tacno maksiumum", 100 * gib, true},
		{"preko maksimuma", 100*gib + 1, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateRequestedQuota(tc.requested, 1*gib, 100*gib) == ""
			if got != tc.ok {
				t.Errorf("ok=%v, ocekivano %v", got, tc.ok)
			}
		})
	}
}
