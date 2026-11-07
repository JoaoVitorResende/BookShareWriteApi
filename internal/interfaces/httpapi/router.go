package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter(register RegisterBookExecutor) http.Handler {
	r := chi.NewRouter()
	r.Method(http.MethodPost, "/books", NewRegisterHandler(register))
	return r
}
