package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/testTelegramApp/expenses-bot/internal/errorsx"
)

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	code := "internal"
	status := http.StatusInternalServerError
	msg := err.Error()
	switch {
	case errors.Is(err, errorsx.ErrNotFound):
		code = "not_found"
		status = http.StatusNotFound
	case errors.Is(err, errorsx.ErrForbidden):
		code = "forbidden"
		status = http.StatusForbidden
	case errors.Is(err, errorsx.ErrValidation):
		code = "validation"
		status = http.StatusBadRequest
	}
	writeJSON(w, status, ErrorBody{Code: code, Message: msg})
}
