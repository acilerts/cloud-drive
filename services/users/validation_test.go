package main

import (
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name string
		pass string
		ok   bool
	}{
		{"jaka", "Abcdef1!xy", true},
		{"prekratka", "Ab1!", false},
		{"bez velikog slova", "abcdef1!xyz", false},
		{"bez cifre", "Abcdefg!xyz", false},
		{"bez specijalnog", "Abcdefg1xyz", false},
		{"predugačka za bcrypt", strings.Repeat("Aa1!", 19), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validatePassword(tc.pass) == ""
			if got != tc.ok {
				t.Errorf("validatePassword(%q) ok=%v, ocekivano %v", tc.pass, got, tc.ok)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		in string
		ok bool
	}{
		{"ana@example.com", true},
		{"", false},
		{"nije-imejl", false},
		{"Ana <ana@example.com>", false},
		{"ana@example.com\r\nBcc: x@y.com", false},
	}

	for _, tc := range tests {
		if got := validateEmail(tc.in) == ""; got != tc.ok {
			t.Errorf("validateEmail(%q) ok=%v, ocekivano %v", tc.in, got, tc.ok)
		}
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		in string
		ok bool
	}{
		{"Ana", true},
		{"Đorđe", true},
		{"Marija-Ana", true},
		{"O'Brien", true},
		{"", false},
		{"<script>", false},
		{"Ana123", false},
		{strings.Repeat("a", 51), false},
	}

	for _, tc := range tests {
		if got := validateName(tc.in) == ""; got != tc.ok {
			t.Errorf("validateName(%q) ok=%v, ocekivano %v", tc.in, got, tc.ok)
		}
	}
}
