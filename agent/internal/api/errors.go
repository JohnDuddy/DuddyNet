package api

import (
	"encoding/json"
	"net/http"

	"github.com/duddynet/agent/internal/models"
)

// writeJSON writes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a standard APIError envelope.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, models.APIError{
		Error:   code,
		Message: message,
		Status:  status,
	})
}

// Common error helpers.
func errUnauthorized(w http.ResponseWriter, msg string) {
	if msg == "" {
		msg = "missing or invalid token"
	}
	writeError(w, http.StatusUnauthorized, "unauthorized", msg)
}

func errForbidden(w http.ResponseWriter, msg string) {
	if msg == "" {
		msg = "token lacks required scope"
	}
	writeError(w, http.StatusForbidden, "forbidden", msg)
}

func errNotFound(w http.ResponseWriter, msg string) {
	if msg == "" {
		msg = "resource not found"
	}
	writeError(w, http.StatusNotFound, "not_found", msg)
}

func errBadRequest(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusBadRequest, "bad_request", msg)
}

func errInternal(w http.ResponseWriter, msg string) {
	if msg == "" {
		msg = "internal error"
	}
	writeError(w, http.StatusInternalServerError, "internal", msg)
}

func errTooManyRequests(w http.ResponseWriter, msg string) {
	if msg == "" {
		msg = "rate limited"
	}
	writeError(w, http.StatusTooManyRequests, "rate_limited", msg)
}
