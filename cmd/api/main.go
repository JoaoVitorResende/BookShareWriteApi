package main

import (
	"BookShareWriteApi/internal/application"
	"BookShareWriteApi/internal/infrastructure/outbox"
	"BookShareWriteApi/internal/infrastructure/postgres"
	"BookShareWriteApi/internal/infrastructure/rabbitmq"
	"BookShareWriteApi/internal/interfaces/httpapi"
	"BookShareWriteApi/internal/interfaces/messaging"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	defaultHTTPPort = "8081"
	eventsExchange  = "bookshare.events"

	outboxInterval  = 2 * time.Second
	outboxBatchSize = 50

	shutdownTimeout = 10 * time.Second
)

// config reúne tudo que vem das variáveis de ambiente.
type config struct {
	databaseURL string
	rabbitURL   string
	httpPort    string
}

// main só faz três coisas: lê a config, prepara o sinal de parada e chama run.
func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	// ctx é cancelado quando chega Ctrl+C (SIGINT) ou docker stop (SIGTERM).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// run devolve erro em vez de chamar log.Fatal lá dentro:
	// log.Fatal encerra o programa na hora e pula os defers (banco e Rabbit não fechariam).
	if err := run(ctx, stop, cfg); err != nil {
		log.Fatal(err)
	}

	log.Println("bye")
}

// loadConfig lê e valida as variáveis de ambiente.
func loadConfig() (config, error) {
	cfg := config{
		databaseURL: strings.TrimSpace(os.Getenv("DATABASE_URL")),
		rabbitURL:   strings.TrimSpace(os.Getenv("RABBITMQ_URL")),
		httpPort:    strings.TrimSpace(os.Getenv("HTTP_PORT")),
	}

	if cfg.databaseURL == "" {
		return config{}, errors.New("DATABASE_URL is required")
	}

	if cfg.rabbitURL == "" {
		return config{}, errors.New("RABBITMQ_URL is required")
	}

	if cfg.httpPort == "" {
		cfg.httpPort = defaultHTTPPort
	}

	return cfg, nil
}

// run monta a aplicação, sobe tudo e espera o sinal de parada.
// A ordem dos defers garante que banco e Rabbit fecham por último.
func run(ctx context.Context, stop context.CancelFunc, cfg config) error {
	// 1. Infraestrutura
	pool, err := newPool(ctx, cfg.databaseURL)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()

	publisher, err := rabbitmq.NewPublisher(cfg.rabbitURL, eventsExchange)
	if err != nil {
		return fmt.Errorf("rabbitmq publisher: %w", err)
	}
	defer publisher.Close()

	consumerCh, closeConsumer, err := newConsumerChannel(cfg.rabbitURL)
	if err != nil {
		return fmt.Errorf("rabbitmq consumer: %w", err)
	}
	defer closeConsumer()

	// 2. Use cases (o mesmo use case atende HTTP e Rabbit)
	registerBook := application.NewRegisterBookUseCase(postgres.NewBookRepository(pool))

	// 3. Workers e entradas, cada um na sua goroutine
	relayDone := startOutboxRelay(ctx, pool, publisher)
	consumerDone := startConsumer(ctx, stop, consumerCh, registerBook)
	server, serverErr := startHTTPServer(cfg.httpPort, registerBook)

	// 4. Espera: Ctrl+C, docker stop, ou o HTTP falhar
	waitForShutdown(ctx, serverErr)

	// 5. Desligamento na ordem certa
	shutdownHTTPServer(server)
	stop() // avisa relay e consumer para pararem (caso o motivo tenha sido erro no HTTP)
	<-relayDone
	<-consumerDone

	return nil
}

// newPool cria o pool do Postgres e faz um Ping para falhar logo se o banco estiver fora.
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

// newConsumerChannel abre uma conexão própria para o consumer, separada da do
// publisher do outbox, para um não travar o outro.
// Devolve uma função que fecha canal e conexão, para usar com defer.
func newConsumerChannel(rabbitURL string) (*amqp.Channel, func(), error) {
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		return nil, nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, err
	}

	closeAll := func() {
		ch.Close()
		conn.Close()
	}

	return ch, closeAll, nil
}

// startOutboxRelay sobe o worker que move eventos da tabela outbox para o Rabbit.
// O channel devolvido fecha quando o relay terminou o lote atual e parou.
func startOutboxRelay(ctx context.Context, pool *pgxpool.Pool, publisher *rabbitmq.Publisher) <-chan struct{} {
	relay := outbox.NewRelay(postgres.NewOutboxStore(pool), publisher, outboxInterval, outboxBatchSize)

	done := make(chan struct{})
	go func() {
		defer close(done)
		relay.Run(ctx)
	}()

	return done
}

// startConsumer sobe o consumer do Rabbit (request/reply) com as filas registradas.
// Se ele morrer (ex.: Rabbit caiu), chama stop e derruba a API inteira:
// melhor cair e o Docker reiniciar do que ficar respondendo HTTP e ignorando a fila.
func startConsumer(ctx context.Context, stop context.CancelFunc, ch *amqp.Channel, registerBook *application.RegisterBookUseCase) <-chan struct{} {
	consumer := messaging.NewConsumer(ch)
	consumer.Register(messaging.QueueRegisterBook, messaging.NewRegisterBookHandler(registerBook))

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := consumer.Run(ctx); err != nil {
			log.Printf("consumer: %v", err)
			stop()
		}
	}()

	return done
}

// startHTTPServer sobe o servidor HTTP numa goroutine.
// Erros de execução (ex.: porta ocupada) chegam pelo channel devolvido.
func startHTTPServer(port string, registerBook *application.RegisterBookUseCase) (*http.Server, <-chan error) {
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewRouter(registerBook),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("book write api listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	return server, serverErr
}

// waitForShutdown bloqueia até chegar o sinal de parada ou o HTTP falhar.
func waitForShutdown(ctx context.Context, serverErr <-chan error) {
	select {
	case err := <-serverErr:
		log.Printf("http server: %v", err)
	case <-ctx.Done():
		log.Println("shutting down...")
	}
}

// shutdownHTTPServer para de aceitar conexões e espera as requisições em andamento
// terminarem (até shutdownTimeout).
func shutdownHTTPServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("http shutdown: %v", err)
	}
}
