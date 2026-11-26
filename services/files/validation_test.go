package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsValidItemName(t *testing.T) {
	valid := []string{"Dokumenti", "projekat_2026.pdf", "Moji fajlovi (1)"}
	for _, name := range valid {
		if !isValidItemName(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	invalid := []string{"", "   ", ".", "..", "<script>alert(1)</script>", "a/b", `a\b`, "bad:name", strings.Repeat("a", 121)}
	for _, name := range invalid {
		if isValidItemName(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}

func TestIsAllowedFileType(t *testing.T) {
	if !isAllowedFileType("notes.txt", "text/plain; charset=utf-8", 12) {
		t.Error("expected .txt with text/plain to be allowed")
	}
	if !isAllowedFileType("doc.pdf", "application/pdf", 100) {
		t.Error("expected .pdf with application/pdf to be allowed")
	}
	if isAllowedFileType("malware.exe", "application/octet-stream", 100) {
		t.Error("expected .exe to be rejected")
	}
	if isAllowedFileType("xss.html", "text/html; charset=utf-8", 50) {
		t.Error("expected .html to be rejected")
	}
	if isAllowedFileType("fake.png", "text/html; charset=utf-8", 50) {
		t.Error("expected spoofed .png with text/html content to be rejected")
	}
}

func TestUploadRejectsDisallowedFileType(t *testing.T) {
	handler, token := createTestHandler(t)
	mux := http.NewServeMux()
	handler.Routes(mux)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "script.html")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("<html><script>alert(1)</script></html>"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/files", &body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for disallowed file type, got %d", rec.Code)
	}
}

func TestUploadRejectsIntegrityMismatch(t *testing.T) {
	handler, token := createTestHandler(t)
	mux := http.NewServeMux()
	handler.Routes(mux)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("sadrzaj fajla"))
	_ = writer.WriteField("sha256", "0000000000000000000000000000000000000000000000000000000000000000")
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/files", &body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for sha256 mismatch, got %d", rec.Code)
	}
}

func TestComputeSHA256(t *testing.T) {
	content := []byte("cloud drive test")
	sum := sha256.Sum256(content)
	want := hex.EncodeToString(sum[:])

	got, err := computeSHA256(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
