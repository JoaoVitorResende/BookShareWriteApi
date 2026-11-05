package httpapi

import (
	"BookShareWriteApi/internal/application"
	"encoding/json"
	"net/http"

	"github.com/shopspring/decimal"
)

type registerBook struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Pages       int             `json:"pages"`
	Price       decimal.Decimal `json:"price"`
}

type RegisterHandler struct {
	useCase *application.RegisterBookUseCase
}

func NewRegisterHandler(uc *application.RegisterBookUseCase) *RegisterHandler {
	return &RegisterHandler{useCase: uc}
}

func (h *RegisterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	var req registerBook

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	out, err := h.useCase.Execute(r.Context(), application.RegisterBookInput{
		Name: req.Name, Description: req.Description, Pages: req.Pages, Price: req.Price,
	})

	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": out.BookID.String()})
}