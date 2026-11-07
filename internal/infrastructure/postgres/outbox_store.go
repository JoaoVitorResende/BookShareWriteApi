package postgres

import (
	"BookShareWriteApi/internal/infrastructure/outbox"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxStore struct {
	pool *pgxpool.Pool
}

func NewOutboxStore(pool *pgxpool.Pool) *OutboxStore {
	return &OutboxStore{pool: pool}
}

// ProcessPending pega até `limit` eventos pendentes, chama handle para cada um
// (na ordem em que foram criados) e marca como publicados os que deram certo.
//
// FOR UPDATE SKIP LOCKED: trava as linhas lidas. Se você subir duas instâncias
// da API, cada uma pega eventos diferentes em vez de publicar o mesmo duas vezes.
func (s *OutboxStore) ProcessPending(ctx context.Context, limit int, handle func(ctx context.Context, event outbox.Event) error) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const selectPending = `
		SELECT id, event_type, payload, created_at
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED`

	rows, err := tx.Query(ctx, selectPending, limit)
	if err != nil {
		return 0, fmt.Errorf("select pending: %w", err)
	}

	var events []outbox.Event
	for rows.Next() {
		var e outbox.Event
		if err := rows.Scan(&e.ID, &e.EventType, &e.Payload, &e.CreatedAt); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}
	rows.Close()

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read pending: %w", err)
	}

	if len(events) == 0 {
		return 0, nil
	}

	// Publica em ordem e para no primeiro erro, para não embaralhar a sequência.
	publishedIDs := make([]string, 0, len(events))
	var handleErr error

	for _, e := range events {
		if err := handle(ctx, e); err != nil {
			handleErr = fmt.Errorf("publish event %s: %w", e.ID, err)
			break
		}
		publishedIDs = append(publishedIDs, e.ID.String())
	}

	if len(publishedIDs) == 0 {
		return 0, handleErr
	}

	const markPublished = `UPDATE outbox SET published_at = now() WHERE id = ANY($1::uuid[])`
	if _, err := tx.Exec(ctx, markPublished, publishedIDs); err != nil {
		return 0, fmt.Errorf("mark published: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return len(publishedIDs), handleErr
}
