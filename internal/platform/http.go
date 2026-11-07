package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

func WriteJSON(responseWriter http.ResponseWriter, status int, value any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)
	_ = json.NewEncoder(responseWriter).Encode(value)
}

func WriteError(responseWriter http.ResponseWriter, status int, message string) {
	WriteJSON(responseWriter, status, ErrorResponse{Error: message})
}

func HealthHandler(service string, check func(ctx context.Context) error) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		if check != nil {
			ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
			defer cancel()
			if err := check(ctx); err != nil {
				WriteJSON(responseWriter, http.StatusServiceUnavailable, map[string]string{"service": service, "status": "unavailable"})
				return
			}
		}
		WriteJSON(responseWriter, http.StatusOK, map[string]string{"service": service, "status": "ok"})
	}
}
