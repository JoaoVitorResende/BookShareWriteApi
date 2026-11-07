package postgres

import (
	"BookShareWriteApi/internal/domain"
	"BookShareWriteApi/internal/exception"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Código do Postgres para violação de UNIQUE.
const uniqueViolationCode = "23505"

const eventBookCreated = "book.created"

// bookCreatedEvent é o que vai para a outbox e, depois, para o Rabbit.
// A Book read api consome esse formato.
type bookCreatedEvent struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id"`
	Name        string    `json:"name"`
	Author      string    `json:"author"`
	Description string    `json:"description"`
	Pages       int       `json:"pages"`
	CreatedAt   time.Time `json:"created_at"`
}

type BookRepository struct {
	pool *pgxpool.Pool
}

func NewBookRepository(pool *pgxpool.Pool) *BookRepository {
	return &BookRepository{pool: pool}
}

func (r *BookRepository) ExistsByNameAndAuthor(ctx context.Context, normalizedName, author string) (bool, error) {
	const query = `SELECT EXISTS (SELECT 1 FROM books WHERE normalized_name = $1 AND author = $2)`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, normalizedName, author).Scan(&exists); err != nil {
		return false, fmt.Errorf("check book exists: %w", err)
	}

	return exists, nil
}

// Save grava o livro e o evento na mesma transação (outbox pattern):
// ou os dois entram, ou nenhum entra.
func (r *BookRepository) Save(ctx context.Context, book domain.Book) error {
	payload, err := json.Marshal(bookCreatedEvent{
		ID:          book.ID().String(),
		OwnerID:     book.OwnerID().String(),
		Name:        book.Name(),
		Author:      book.Author(),
		Description: book.Description(),
		Pages:       book.Pages(),
		CreatedAt:   book.CreatedAt(),
	})
	if err != nil {
		return fmt.Errorf("marshal book event: %w", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	// Se o Commit já aconteceu, o Rollback não faz nada.
	// Se algo falhou antes, ele desfaz tudo.
	defer tx.Rollback(ctx)

	const insertBook = `
		INSERT INTO books (id, owner_id, name, normalized_name, author, description, pages, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err = tx.Exec(ctx, insertBook,
		book.ID(), book.OwnerID(), book.Name(), book.NormalizedName(),
		book.Author(), book.Description(), book.Pages(), book.CreatedAt(),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return exception.ErrBookAlreadyExists
		}
		return fmt.Errorf("insert book: %w", err)
	}

	const insertOutbox = `
		INSERT INTO outbox (id, event_type, payload, created_at)
		VALUES ($1, $2, $3, $4)`

	if _, err := tx.Exec(ctx, insertOutbox, uuid.New(), eventBookCreated, payload, book.CreatedAt()); err != nil {
		return fmt.Errorf("insert outbox: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}
