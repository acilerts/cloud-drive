package main

import (
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

type RegisterRequest struct {
	FirstName      string `json:"firstName"`
	LastName       string `json:"lastName"`
	Email          string `json:"email"`
	Password       string `json:"password"`
	PasswordRepeat string `json:"passwordRepeat"`
}

func (r *RegisterRequest) Normalize() {
	r.FirstName = strings.TrimSpace(r.FirstName)
	r.LastName = strings.TrimSpace(r.LastName)
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
}

func (r RegisterRequest) Validate() map[string]string {
	errs := map[string]string{}
	if m := validateName(r.FirstName); m != "" {
		errs["firstName"] = m
	}
	if m := validateName(r.LastName); m != "" {
		errs["lastName"] = m
	}
	if m := validateEmail(r.Email); m != "" {
		errs["email"] = m
	}
	if m := validatePassword(r.Password); m != "" {
		errs["password"] = m
	}
	if r.Password != r.PasswordRepeat {
		errs["passwordRepeat"] = "Lozinke se ne poklapaju"
	}
	return errs
}

func validateName(s string) string {
	n := utf8.RuneCountInString(s)
	if n < 1 || n > 50 {
		return "Duzina mora biti od 1 do 50 karaktera"
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && r != ' ' && r != '-' && r != '\'' {
			return "Dozvoljena su samo slova, razmak, crtica i apostrof"
		}
	}

	return ""
}

func validateEmail(s string) string {
	if s == "" || len(s) > 254 {
		return "Neispravna imejl adresa"
	}

	addr, err := mail.ParseAddress(s)

	if err != nil || addr.Address != s {
		return "Neispravna imejl adresa"
	}

	return ""
}

func validatePassword(p string) string {
	if utf8.RuneCountInString(p) < 10 {
		return "Lozinka mora imati najmanje 10 karaktera"
	}

	if len(p) > 72 {
		return "Lozinka moze imati najvise 72 bajta"
	}

	var upper, lower, digit, other bool

	for _, r := range p {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case unicode.IsDigit(r):
			digit = true
		default:
			other = true
		}
	}

	if !(upper && lower && digit && other) {
		return "Lozinka mora sadrzati veliko slovo, malo slovo, cifru i specijalni karakter"
	}

	return ""
}
