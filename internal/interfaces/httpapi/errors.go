package httpapi

import (
	"BookShareWriteApi/internal/exception"
	"errors"
	"log"
	"net/http"
)

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, exception.ErrBookAlreadyExist):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, exception.ErrEmptyName),
		errors.Is(err, exception.ErrEmptyName),
		errors.Is(err, exception.ErrEmptyDescription),
		errors.Is(err, exception.ErrPages),
		errors.Is(err, exception.ErrPrices):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		log.Printf("internal error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}