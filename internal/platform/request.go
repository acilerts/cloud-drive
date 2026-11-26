package platform

import (
	"crypto/ed25519"
	"encoding/json"
	"log/slog"
	"net/http"
)

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func Authenticate(w http.ResponseWriter, r *http.Request, pub ed25519.PublicKey, logger *slog.Logger) (*Claims, bool) {
	raw, ok := BearerToken(r)
	if !ok {
		logger.Warn("access denied", "reason", "missing_token", "path", r.URL.Path)
		WriteError(w, http.StatusUnauthorized, "authentication required")
		return nil, false
	}
	claims, err := VerifyToken(pub, raw)
	if err != nil {
		logger.Warn("access denied", "reason", "invalid_or_expired_token", "path", r.URL.Path)
		WriteError(w, http.StatusUnauthorized, "invalid or expired token")
		return nil, false
	}
	return claims, true
}
