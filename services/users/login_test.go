package main

import "testing"

func TestValidOTP(t *testing.T) {
	tests := []struct {
		in string
		ok bool
	}{
		{"123456", true},
		{"000000", true},
		{"12345", false},
		{"1234567", false},
		{"12345a", false},
		{"", false},
		{"１２３４５６", false},
	}
	for _, tc := range tests {
		if got := validOTP(tc.in); got != tc.ok {
			t.Errorf("validOTP(%q)=%v, očekivano %v", tc.in, got, tc.ok)
		}
	}
}

func TestNewOTP(t *testing.T) {
	for i := 0; i < 100; i++ {
		code, err := newOTP()
		if err != nil || !validOTP(code) {
			t.Fatalf("neispravan kod %q (err=%v)", code, err)
		}
	}
}
