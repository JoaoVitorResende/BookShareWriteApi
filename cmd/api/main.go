package main

import (
	"BookShareWriteApi/internal/application"
	"BookShareWriteApi/internal/infrastructure/outbox"
	"BookShareWriteApi/internal/infrastructure/postgres"
	"BookShareWriteApi/internal/infrastructure/rabbitmq"
	"BookShareWriteApi/internal/interfaces/httpapi"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultHTTPPort = "8081"
	eventsExchange  = "bookshare.events"

	outboxInterval  = 2 * time.Second
	outboxBatchSize = 50
)

func main() {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	rabbitURL := strings.TrimSpace(os.Getenv("RABBITMQ_URL"))
	if rabbitURL == "" {
		log.Fatal("RABBITMQ_URL is required")
	}

	port := strings.TrimSpace(os.Getenv("HTTP_PORT"))
	if port == "" {
		port = defaultHTTPPort
	}

	// ctx é cancelado quando chega Ctrl+C (SIGINT) ou docker stop (SIGTERM).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := newPool(ctx, databaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	publisher, err := rabbitmq.NewPublisher(rabbitURL, eventsExchange)
	if err != nil {
		log.Fatalf("rabbitmq: %v", err)
	}
	defer publisher.Close()

	// Composição: infraestrutura -> use case -> HTTP.
	bookRepository := postgres.NewBookRepository(pool)
	registerBook := application.NewRegisterBookUseCase(bookRepository)
	router := httpapi.NewRouter(registerBook)

	// Worker do outbox: roda em paralelo ao HTTP e para junto com o ctx.
	relay := outbox.NewRelay(postgres.NewOutboxStore(pool), publisher, outboxInterval, outboxBatchSize)
	relayDone := make(chan struct{})
	go func() {
		relay.Run(ctx)
		close(relayDone)
	}()

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// O servidor roda numa goroutine para o main poder esperar o sinal de parada.
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("book write api listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		log.Printf("http server: %v", err)
	case <-ctx.Done():
		log.Println("shutting down...")
	}

	// Graceful shutdown: para de aceitar conexões e espera as requisições em andamento terminarem.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}

	// Garante que o relay terminou o lote atual antes de fechar Rabbit e banco (defers).
	stop()
	<-relayDone
}

// newPool cria o pool e faz um Ping para falhar logo se o banco estiver fora.
func newPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
