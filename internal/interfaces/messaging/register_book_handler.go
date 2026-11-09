package messaging

import (
	"BookShareWriteApi/internal/application"
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
)

type registerBookRequest struct {
	OwnerID     string `json:"owner_id"`
	Name        string `json:"name"`
	Author      string `json:"author"`
	Description string `json:"description"`
	Pages       int    `json:"pages"`
}

type RegisterBookExecutor interface {
	Execute(ctx context.Context, input application.RegisterBookInput) (application.RegisterBookOutput, error)
}

type RegisterBookHandler struct {
	useCase RegisterBookExecutor
}

func NewRegisterBookHandler(uc RegisterBookExecutor) *RegisterBookHandler {
	return &RegisterBookHandler{useCase: uc}
}

func (h *RegisterBookHandler) Handle(ctx context.Context, body []byte) Response {
	var req registerBookRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return fail("INVALID_INPUT", "invalid body")
	}

	ownerID, err := uuid.Parse(strings.TrimSpace(req.OwnerID))
	if err != nil || ownerID == uuid.Nil {
		return fail("UNAUTHORIZED", "missing or invalid owner id")
	}

	out, err := h.useCase.Execute(ctx, application.RegisterBookInput{
		OwnerID:     ownerID,
		Name:        req.Name,
		Author:      req.Author,
		Description: req.Description,
		Pages:       req.Pages,
	})
	if err != nil {
		return failFrom(err)
	}

	return Response{OK: true, Data: map[string]string{"id": out.BookID.String()}}
}