package exception

import "errors"

var (
	ErrEmptyName        = errors.New("name can't be empty")
	ErrEmptyDescription = errors.New("description can't be empty")
	ErrPages            = errors.New("must be over 49 pages to be a book")
	ErrPrices           = errors.New("price can't be negative")
	ErrBookAlreadyExist = errors.New("Book already exists")
)
