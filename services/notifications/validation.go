package main

import (
	"regexp"
	"unicode"
	"unicode/utf8"
)

var userIDPattern = regexp.MustCompile(`^[0-9a-f]{24}$`)

var allowedKInds = map[string]bool{
	"share":       true,
	"new_file":    true,
	"new_version": true,
	"info":        true,
}

type createRequest struct {
	UserID     string `json:"userId"`
	Kind       string `json:"type"`
	Title      string `json:"title"`
	Message    string `json:"message"`
	ResourceID string `json:"resourceId"`
}

func hasControlChars(s string, allowNewline bool) bool {
	for _, r := range s {
		if allowNewline && r == '\n' {
			continue
		}
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func (r createRequest) validate() map[string]string {
	errs := map[string]string{}

	if !userIDPattern.MatchString(r.UserID) {
		errs["userId"] = "Neispravan ID korisnika"
	}
	if !allowedKInds[r.Kind] {
		errs["type"] = "Nepoznat tip notifikacije"
	}
	if n := utf8.RuneCountInString(r.Title); n < 1 || n > 100 || hasControlChars(r.Title, false) {
		errs["title"] = "Naslov mora imati 1 do 100 karaktera bez kontrolnih znakova"
	}
	if n := utf8.RuneCountInString(r.Message); n < 1 || n > 500 || hasControlChars(r.Message, true) {
		errs["message"] = "Poruka mora imati 1 do 500 karaktera bez kontrolnih znakova"
	}
	if utf8.RuneCountInString(r.ResourceID) > 64 || hasControlChars(r.ResourceID, false) {
		errs["resourceId"] = "ID resursa moze imati najvise 64 karaktera"
	}
	return errs
}
