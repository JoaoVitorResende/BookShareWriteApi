package httpapi

import (
	"BookShareWriteApi/internal/application"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// Header que o gateway preenche com o "sub" do JWT já validado.
const userIDHeader = "X-User-ID"

const maxBodyBytes = 1 << 20 // 1 MB

// RegisterBookExecutor permite testar o handler com um mock do use case.
type RegisterBookExecutor interface {
	Execute(ctx context.Context, input application.RegisterBookInput) (application.RegisterBookOutput, error)
}

type registerBookRequest struct {
	Name        string `json:"name"`
	Author      string `json:"author"`
	Description string `json:"description"`
	Pages       int    `json:"pages"`
}

type registerBookResponse struct {
	ID string `json:"id"`
}

type RegisterHandler struct {
	useCase RegisterBookExecutor
}

func NewRegisterHandler(uc RegisterBookExecutor) *RegisterHandler {
	return &RegisterHandler{useCase: uc}
}

func (h *RegisterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ownerID, err := uuid.Parse(strings.TrimSpace(r.Header.Get(userIDHeader)))
	if err != nil || ownerID == uuid.Nil {
		http.Error(w, "missing or invalid user id", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var req registerBookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	out, err := h.useCase.Execute(r.Context(), application.RegisterBookInput{
		OwnerID:     ownerID,
		Name:        req.Name,
		Author:      req.Author,
		Description: req.Description,
		Pages:       req.Pages,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(registerBookResponse{ID: out.BookID.String()}); err != nil {
		log.Printf("encode response: %v", err)
	}
}
