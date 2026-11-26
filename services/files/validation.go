package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxItemNameLen = 120

var allowedExtensions = map[string]bool{
	".txt":  true,
	".pdf":  true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
	".csv":  true,
	".json": true,
	".md":   true,
	".docx": true,
	".xlsx": true,
	".zip":  true,
}

var allowedMIMETypes = map[string]bool{
	"text/plain":      true,
	"application/pdf": true,
	"image/png":       true,
	"image/jpeg":      true,
	"image/gif":       true,
	"image/webp":      true,
	"application/zip": true,
}

func isValidItemName(name string) bool {
	name = strings.TrimSpace(name)
	n := utf8.RuneCountInString(name)
	if n < 1 || n > maxItemNameLen {
		return false
	}
	if name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			return false
		}
	}
	return true
}

func isAllowedFileType(filename, detectedContentType string, size int64) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	if !allowedExtensions[ext] {
		return false
	}
	if size == 0 {
		return true
	}

	mimeType := strings.ToLower(strings.TrimSpace(detectedContentType))
	if base, _, ok := strings.Cut(mimeType, ";"); ok {
		mimeType = strings.TrimSpace(base)
	}
	return allowedMIMETypes[mimeType]
}

func computeSHA256(rs io.ReadSeeker) (string, error) {
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.Copy(h, rs); err != nil {
		return "", err
	}
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
