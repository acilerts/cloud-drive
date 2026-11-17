package main

import (
	"testing"
	"time"
)

func TestTooNew(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		changedAt time.Time
		minAge    time.Duration
		want      bool
	}{
		{"promenjena maločas", now.Add(-time.Minute), 24 * time.Hour, true},
		{"stara dva dana", now.Add(-48 * time.Hour), 24 * time.Hour, false},
		{"tačno na granici (malo preko)", now.Add(-24*time.Hour - time.Second), 24 * time.Hour, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tooNew(tc.changedAt, tc.minAge); got != tc.want {
				t.Errorf("tooNew=%v, očekivano %v", got, tc.want)
			}
		})
	}
}

func TestCheckNewPassword(t *testing.T) {
	if errs := checkNewPassword("Abcdef1!xy", "Abcdef1!xy", "p", "r"); len(errs) != 0 {
		t.Errorf("očekivano bez grešaka, dobijeno %v", errs)
	}
	errs := checkNewPassword("abc", "abd", "p", "r")
	if errs["p"] == "" || errs["r"] == "" {
		t.Errorf("očekivane greške za oba polja, dobijeno %v", errs)
	}
}
