package domain

import (
	"BookShareWriteApi/internal/exception"
	"strings"
	"time"

	"github.com/google/uuid"
)

const MinPages = 50

type Book struct {
	id             uuid.UUID
	ownerID        uuid.UUID
	name           string
	normalizedName string
	author         string
	description    string
	pages          int
	createdAt      time.Time
}

func validateBook(ownerID uuid.UUID, name, author, description string, pages int) error {
	if ownerID == uuid.Nil {
		return exception.ErrInvalidOwner
	}

	if name == "" {
		return exception.ErrEmptyName
	}

	if author == "" {
		return exception.ErrEmptyAuthor
	}

	if description == "" {
		return exception.ErrEmptyDescription
	}

	if pages < MinPages {
		return exception.ErrPages
	}

	return nil
}

func NewBook(ownerID uuid.UUID, name, author, description string, pages int) (Book, error) {
	name = strings.TrimSpace(name)
	author = strings.TrimSpace(author)
	description = strings.TrimSpace(description)

	if err := validateBook(ownerID, name, author, description, pages); err != nil {
		return Book{}, err
	}

	return Book{
		id:             uuid.New(),
		ownerID:        ownerID,
		name:           name,
		normalizedName: NormalizeBookName(name),
		author:         author,
		description:    description,
		pages:          pages,
		createdAt:      time.Now().UTC(),
	}, nil
}

// RestoreBook reconstrói um livro que já está salvo no banco.
// Não gera novo id nem revalida, pois os dados já passaram pelo NewBook.
func RestoreBook(id, ownerID uuid.UUID, name, author, description string, pages int, createdAt time.Time) Book {
	return Book{
		id:             id,
		ownerID:        ownerID,
		name:           name,
		normalizedName: NormalizeBookName(name),
		author:         author,
		description:    description,
		pages:          pages,
		createdAt:      createdAt,
	}
}

func NormalizeBookName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func (b Book) ID() uuid.UUID          { return b.id }
func (b Book) OwnerID() uuid.UUID     { return b.ownerID }
func (b Book) Name() string           { return b.name }
func (b Book) NormalizedName() string { return b.normalizedName }
func (b Book) Author() string         { return b.author }
func (b Book) Description() string    { return b.description }
func (b Book) Pages() int             { return b.pages }
func (b Book) CreatedAt() time.Time   { return b.createdAt }
