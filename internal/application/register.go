package application

import (
	"BookShareWriteApi/internal/domain"
	"BookShareWriteApi/internal/exception"
	"context"

	"github.com/google/uuid"
)

// BookRepository tem só o que este use case precisa (ISP).
type BookRepository interface {
	ExistsByNameAndAuthor(ctx context.Context, normalizedName, author string) (bool, error)
	// Save deve retornar exception.ErrBookAlreadyExists quando o UNIQUE do banco for violado.
	Save(ctx context.Context, book domain.Book) error
}

type RegisterBookInput struct {
	OwnerID     uuid.UUID
	Name        string
	Author      string
	Description string
	Pages       int
}

type RegisterBookOutput struct {
	BookID uuid.UUID
}

type RegisterBookUseCase struct {
	books BookRepository
}

func NewRegisterBookUseCase(books BookRepository) *RegisterBookUseCase {
	return &RegisterBookUseCase{books: books}
}

func (uc *RegisterBookUseCase) Execute(ctx context.Context, input RegisterBookInput) (RegisterBookOutput, error) {
	book, err := domain.NewBook(input.OwnerID, input.Name, input.Author, input.Description, input.Pages)
	if err != nil {
		return RegisterBookOutput{}, err
	}

	// Checagem rápida para dar um erro amigável.
	// Quem garante de verdade é o UNIQUE do banco (ver Save).
	exists, err := uc.books.ExistsByNameAndAuthor(ctx, book.NormalizedName(), book.Author())
	if err != nil {
		return RegisterBookOutput{}, err
	}

	if exists {
		return RegisterBookOutput{}, exception.ErrBookAlreadyExists
	}

	if err := uc.books.Save(ctx, book); err != nil {
		return RegisterBookOutput{}, err
	}

	return RegisterBookOutput{BookID: book.ID()}, nil
}
