package main

import (
	"strings"
	"testing"
)

func validReq() createRequest {
	return createRequest{
		UserID:  "64b7f0c2a1b2c3d4e5f60718",
		Kind:    "share",
		Title:   "Deljenje",
		Message: "Ana je podelila datoteku sa vama",
	}
}

func TestCreateRequestValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*createRequest)
		field  string
	}{
		{"ispravan ulaz", func(r *createRequest) {}, ""},
		{"los userId", func(r *createRequest) { r.UserID = "abc" }, "userId"},
		{"userId sa velikim slovima", func(r *createRequest) { r.UserID = strings.ToUpper(r.UserID) }, "userId"},
		{"nepoznat tip", func(r *createRequest) { r.Kind = "hack" }, "type"},
		{"prazan naslov", func(r *createRequest) { r.Title = "" }, "title"},
		{"predugacak naslov", func(r *createRequest) { r.Title = strings.Repeat("a", 101) }, "title"},
		{"novi red u naslovu", func(r *createRequest) { r.Title = "a\nb" }, "title"},
		{"novi red u poruci je dozvoljen", func(r *createRequest) { r.Message = "a\nb" }, ""},
		{"predugacka poruka", func(r *createRequest) { r.Message = strings.Repeat("a", 501) }, "message"},
		{"predugacak resourceId", func(r *createRequest) { r.ResourceID = strings.Repeat("a", 65) }, "resourceId"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := validReq()
			tc.mutate(&r)
			errs := r.validate()
			if tc.field == "" && len(errs) != 0 {
				t.Fatalf("ocekivano bez gresaka, dobijeno %v", errs)
			}
			if tc.field != "" && errs[tc.field] == "" {
				t.Fatalf("ocekivana greska za polje %q, dobijeno %v", tc.field, errs)
			}
		})
	}
}
