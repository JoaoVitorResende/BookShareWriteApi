package httpapi

import (
	"BookShareWriteApi/internal/application"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter(register *application.RegisterBookUseCase) http.Handler {
	r := chi.NewRouter()
	r.Method(http.MethodPost, "/books", NewRegisterHandler(register))
	return r
}