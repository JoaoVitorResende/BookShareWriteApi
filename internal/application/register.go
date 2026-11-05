package application

import (
	"BookShareWriteApi/internal/domain"
	"BookShareWriteApi/internal/exception"
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type BookRepository interface {
	ExistsByName(ctx context.Context, name string) (bool,error)
	Save(ctx context.Context, book domain.Book) error
	FindByName(ctx context.Context, name string) (domain.Book, error)
}

type RegisterBookInput struct{
	Name        string
	Description string
	Pages       int
	Price       decimal.Decimal
}

type RegisterBookOutput struct {
	BookID uuid.UUID
}

type RegisterBookUseCase struct {
	books  BookRepository
}

func Register(books BookRepository) *RegisterBookUseCase{
	return &RegisterBookUseCase{books: books}
}

func (uc *RegisterBookUseCase) Execute(ctx context.Context, request RegisterBookInput) (RegisterBookOutput, error){
	book, err := domain.NewBook(request.Name, request.Description, request.Pages, request.Price)
	if err != nil {
		return RegisterBookOutput{}, err
	}

	exists, err := uc.books.ExistsByName(ctx, book.Name())

	if err != nil {
		return RegisterBookOutput{}, err
	}

	if exists {
		return RegisterBookOutput{}, exception.ErrBookAlreadyExist
	}

	if err := uc.books.Save(ctx, book); err != nil {
		return RegisterBookOutput{}, err
	}

	return RegisterBookOutput{BookID: book.ID()}, nil
}