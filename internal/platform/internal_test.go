package platform

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireInternal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := RequireInternal("tajna-tajna", logger, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"ispravan token", "tajna-tajna", http.StatusOK},
		{"pogrešan token", "druga-tajna", http.StatusUnauthorized},
		{"bez zaglavlja", "", http.StatusUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/internal/x", nil)
			if tc.header != "" {
				req.Header.Set(InternalTokenHeader, tc.header)
			}
			rec := httptest.NewRecorder()
			h(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status=%d, očekivano %d", rec.Code, tc.want)
			}
		})
	}
}
