package platform

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
)

const InternalTokenHeader = "X-Internal-Token"

func RequireInternal(token string, logger *slog.Logger, next http.HandlerFunc) http.HandlerFunc {
	want := []byte(token)
	return func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get(InternalTokenHeader))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			logger.Warn("access denied", "reason", "bad_internal_token", "path", r.URL.Path)
			WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}
