package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"file-storage-server/internal/repository"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

func DecodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("%w: %v", repository.ErrInvalidInput, err)
	}
	return nil
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func WriteError(w http.ResponseWriter, err error) {
	status := statusCode(err)
	message := repository.PublicMessage(err)
	if status == http.StatusInternalServerError {
		message = http.StatusText(status)
	}

	WriteJSON(w, status, ErrorResponse{Error: message})
}

func statusCode(err error) int {
	switch {
	case errors.Is(err, repository.ErrInvalidInput):
		return http.StatusUnprocessableEntity
	case errors.Is(err, repository.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, repository.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, repository.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, repository.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, repository.ErrLimitExceeded):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}
