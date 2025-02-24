package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
)

type ErrorCode string

const (
	ErrNotFound   ErrorCode = "NOT_FOUND"
	ErrForbidden  ErrorCode = "FORBIDDEN"
	ErrValidation ErrorCode = "VALIDATION"
)

type AppError struct {
	Code    ErrorCode
	Message string
	Err     error
}

type ErrorResponse struct {
	Error string    `json:"error"`
	Code  ErrorCode `json:"code"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) StatusCode() int {
	switch e.Code {
	case ErrNotFound:
		return http.StatusNotFound
	case ErrForbidden:
		return http.StatusForbidden
	case ErrValidation:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

func HandleError(w http.ResponseWriter, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		status := appErr.StatusCode()
		response := ErrorResponse{
			Error: appErr.Message,
			Code:  appErr.Code,
		}
		WriteJSON(w, status, response)
		return
	}

	log.Printf("Internal error: %v", err)
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}
