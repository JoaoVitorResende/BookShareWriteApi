package rabbitmq

import (
	"BookShareWriteApi/internal/infrastructure/outbox"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const publishTimeout = 5 * time.Second

var ErrNotConfirmed = errors.New("message not confirmed by broker")

// Publisher publica eventos num exchange do tipo topic.
// A routing key é o tipo do evento (ex.: book.created), então a Book read api
// só precisa ligar uma fila nesse exchange com a chave que quer ouvir.
type Publisher struct {
	url      string
	exchange string

	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
}

func NewPublisher(url, exchange string) (*Publisher, error) {
	p := &Publisher{url: url, exchange: exchange}

	if err := p.connect(); err != nil {
		return nil, err
	}

	return p, nil
}

// connect deve ser chamado com p.mu travado (ou antes de o Publisher ser compartilhado).
func (p *Publisher) connect() error {
	conn, err := amqp.Dial(p.url)
	if err != nil {
		return fmt.Errorf("dial rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("open channel: %w", err)
	}

	// Publisher confirms: o Rabbit avisa quando realmente guardou a mensagem.
	// Sem isso, você marcaria o evento como publicado sem ter certeza.
	if err := ch.Confirm(false); err != nil {
		conn.Close()
		return fmt.Errorf("enable confirms: %w", err)
	}

	// durable=true: o exchange sobrevive a um restart do Rabbit.
	if err := ch.ExchangeDeclare(p.exchange, "topic", true, false, false, false, nil); err != nil {
		conn.Close()
		return fmt.Errorf("declare exchange: %w", err)
	}

	p.conn = conn
	p.ch = ch
	return nil
}

func (p *Publisher) Publish(ctx context.Context, event outbox.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Reconecta se o Rabbit caiu desde a última publicação.
	if p.ch == nil || p.ch.IsClosed() {
		p.closeLocked()
		if err := p.connect(); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()

	confirmation, err := p.ch.PublishWithDeferredConfirmWithContext(ctx, p.exchange, event.EventType, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent, // a mensagem sobrevive a um restart do Rabbit
		MessageId:    event.ID.String(),
		Type:         event.EventType,
		Timestamp:    event.CreatedAt,
		Body:         event.Payload,
	})
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	confirmed, err := confirmation.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait confirm: %w", err)
	}

	if !confirmed {
		return ErrNotConfirmed
	}

	return nil
}

func (p *Publisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked()
}

func (p *Publisher) closeLocked() {
	if p.ch != nil {
		p.ch.Close()
		p.ch = nil
	}
	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
	}
}
