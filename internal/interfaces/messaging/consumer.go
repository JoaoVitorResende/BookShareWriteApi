package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

const QueueRegisterBook = "book.register"

type Handler interface {
	Handle(ctx context.Context, body []byte) Response
}

type Consumer struct {
	ch     *amqp.Channel
	routes map[string]Handler
}

func NewConsumer(ch *amqp.Channel) *Consumer {
	return &Consumer{ch: ch, routes: make(map[string]Handler)}
}

func (c *Consumer) Register(queue string, h Handler) {
	c.routes[queue] = h
}

func (c *Consumer) Run(ctx context.Context) error {
	if err := c.ch.Qos(10, 0, false); err != nil {
		return err
	}

	errCh := make(chan error, len(c.routes))

	for queue, h := range c.routes {
		if _, err := c.ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
			return err
		}
		msgs, err := c.ch.Consume(queue, "", false, false, false, false, nil)
		if err != nil {
			return err
		}

		log.Println("consuming:", queue)
		go func(msgs <-chan amqp.Delivery, h Handler) {
			errCh <- c.serve(ctx, msgs, h)
		}(msgs, h)
	}

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func (c *Consumer) serve(ctx context.Context, msgs <-chan amqp.Delivery, h Handler) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-msgs:
			if !ok {
				return errors.New("delivery channel closed")
			}
			resp := h.Handle(ctx, d.Body)
			c.reply(ctx, d, resp)
			_ = d.Ack(false)
		}
	}
}

func (c *Consumer) reply(ctx context.Context, d amqp.Delivery, resp Response) {
	if d.ReplyTo == "" {
		return
	}

	body, err := json.Marshal(resp)
	if err != nil {
		log.Printf("marshal reply: %v", err)
		return
	}

	err = c.ch.PublishWithContext(ctx, "", d.ReplyTo, false, false, amqp.Publishing{
		ContentType:   "application/json",
		CorrelationId: d.CorrelationId,
		Body:          body,
	})
	if err != nil {
		log.Printf("publish reply: %v", err)
	}
}