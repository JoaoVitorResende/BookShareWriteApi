package exception

import (
	"errors"
	"fmt"
)

// ErrValidation é o erro "pai" de todas as regras de validação.
// Com errors.Is(err, ErrValidation) a camada HTTP sabe que é 400
// sem precisar listar cada erro.
var ErrValidation = errors.New("validation error")

var (
	ErrInvalidOwner     = fmt.Errorf("%w: owner id is required", ErrValidation)
	ErrEmptyName        = fmt.Errorf("%w: name can't be empty", ErrValidation)
	ErrEmptyAuthor      = fmt.Errorf("%w: author can't be empty", ErrValidation)
	ErrEmptyDescription = fmt.Errorf("%w: description can't be empty", ErrValidation)
	ErrPages            = fmt.Errorf("%w: must have at least 50 pages to be a book", ErrValidation)
)

var ErrBookAlreadyExists = errors.New("book already exists")
