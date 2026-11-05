package domain

import (
	"BookShareWriteApi/internal/exception"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Book struct {
	id          uuid.UUID
	name        string
	description string
	pages       int
	price       decimal.Decimal
}

func validateBookItem(name, description string, pages int, price decimal.Decimal) (error){
	 if name == "" {
		return exception.ErrEmptyName
	}

	if description == "" {
		return exception.ErrEmptyDescription
	}

	if pages <= 49 {
		return exception.ErrPages
	}

	if price.IsNegative(){
		return exception.ErrPrices
	}

    return nil
}

func NewBook(name, description string, pages int, price decimal.Decimal) (Book, error) {

	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	err:= validateBookItem(name, description, pages, price)

	if err != nil{
		return Book{}, err
	}

	return Book{id: uuid.New(), name:name, description: description, pages: pages, price: price}, nil
}

func NormalizeBookName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func (b Book) ID() uuid.UUID          { return b.id }
func (b Book) Name() string           { return b.name }
func (b Book) Description() string    { return b.description }
func (b Book) Pages() int             { return b.pages }
func (b Book) Price() decimal.Decimal { return b.price }
