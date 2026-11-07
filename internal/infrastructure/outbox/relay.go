package outbox

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
)

// Event é uma linha da tabela outbox.
type Event struct {
	ID        uuid.UUID
	EventType string
	Payload   []byte
	CreatedAt time.Time
}

// Store lê os eventos pendentes e marca como publicados os que o handle aceitar.
type Store interface {
	ProcessPending(ctx context.Context, limit int, handle func(ctx context.Context, event Event) error) (int, error)
}

// Publisher envia um evento para o broker (Rabbit).
type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

// Relay é o worker que move os eventos da outbox para o Rabbit.
type Relay struct {
	store     Store
	publisher Publisher
	interval  time.Duration
	batchSize int
}

func NewRelay(store Store, publisher Publisher, interval time.Duration, batchSize int) *Relay {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 50
	}

	return &Relay{store: store, publisher: publisher, interval: interval, batchSize: batchSize}
}

// Run roda até o ctx ser cancelado (Ctrl+C / docker stop).
func (r *Relay) Run(ctx context.Context) {
	log.Printf("outbox relay started (every %s, batch %d)", r.interval, r.batchSize)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		r.drain(ctx)

		select {
		case <-ctx.Done():
			log.Println("outbox relay stopped")
			return
		case <-ticker.C:
		}
	}
}

// drain processa lotes até não sobrar pendência (ou dar erro).
// Se um lote veio cheio, provavelmente tem mais esperando.
func (r *Relay) drain(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		published, err := r.store.ProcessPending(ctx, r.batchSize, r.publisher.Publish)
		if published > 0 {
			log.Printf("outbox relay: published %d event(s)", published)
		}

		if err != nil {
			// Não para o worker: tenta de novo no próximo tick.
			log.Printf("outbox relay: %v", err)
			return
		}

		if published < r.batchSize {
			return
		}
	}
}
