package httpapi

import (
	"BookShareWriteApi/internal/exception"
	"errors"
	"log"
	"net/http"
)

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, exception.ErrBookAlreadyExists):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, exception.ErrValidation):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		log.Printf("internal error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
