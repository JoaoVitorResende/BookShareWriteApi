![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)

# BookShare Write API

Write service (the **command** side of CQRS) of the **BookShare** project. It registers books, enforces the business rules and publishes events so the read side (Book Read API) stays up to date.

## Architecture

```
Client
  │  HTTP + JWT
  ▼
Gateway (BookShareGateWay)
  │  validates the JWT and puts the owner_id (token sub) in the message
  │  RabbitMQ request/reply: queue book.register
  ▼
Write API (this service)
  │  use case -> domain -> repository
  ▼
PostgreSQL (books + outbox, in the same transaction)
  │
  │  outbox relay (every 2s)
  ▼
RabbitMQ: exchange bookshare.events (topic), routing key book.created
  │
  ▼
Book Read API (consumes the event and updates the read model)
```

### Key decisions

- **Clean Architecture:** `domain` and `application` know nothing about HTTP, RabbitMQ or PostgreSQL. The same use case serves both entry points (HTTP and queue).
- **Outbox pattern:** the book and the `book.created` event are written in the **same transaction**. A worker (relay) publishes pending events to RabbitMQ. This avoids the *dual write* problem: an event is never lost, even if RabbitMQ is down when the book is registered.
- **At-least-once delivery:** an event may be published more than once after a failure. Consumers must be idempotent (use the book `id`).
- **Uniqueness:** `UNIQUE (normalized_name, author)` in the database. The check in the use case is the fast path; the database is the guarantee against race conditions. Postgres error `23505` is mapped to `ErrBookAlreadyExists`.
- **Publisher confirms + persistent messages:** an event is only marked as published after RabbitMQ confirms it was stored.

## Project structure

```
cmd/api/                         main: configuration and wiring
db/migrations/                   migrations (golang-migrate)
internal/
  domain/                        Book entity and business rules
  application/                   RegisterBook use case
  exception/                     domain errors (ErrValidation as the parent error)
  infrastructure/
    postgres/                    book repository and outbox store (pgx)
    rabbitmq/                    event publisher
    outbox/                      relay (outbox worker)
  interfaces/
    httpapi/                     HTTP entry point (POST /books)
    messaging/                   RabbitMQ entry point (book.register queue, request/reply)
```

## Business rules

| Field | Rule |
|---|---|
| `owner_id` | required, valid UUID (comes from the JWT, through the gateway) |
| `name` | required |
| `author` | required |
| `description` | required |
| `pages` | at least 50 |
| `name` + `author` | must be unique (case-insensitive name) |

## Running

### Prerequisites

- Docker and Docker Compose
- The external network `bookshare-net`
- RabbitMQ running on `bookshare-net` with the name `rabbitmq` (started by the UserApi compose)

```bash
docker network create bookshare-net   # first time only
```

### Start

```bash
docker compose up -d --build
docker compose logs -f write-api
```

The compose file starts, in this order:

1. `books-write-db`: PostgreSQL 18 (port `5434` on the host)
2. `books-write-migrate`: runs the migrations and exits
3. `write-api`: the API (port `8081` on the host)

The API log should show:

```
outbox relay started (every 2s, batch 50)
consuming: book.register
book write api listening on :8081
```

### Running outside Docker (development)

```bash
export DATABASE_URL="postgres://postgres:postgres@localhost:5434/books_write?sslmode=disable"
export RABBITMQ_URL="amqp://guest:guest@localhost:5672/"
go run ./cmd/api
```

PowerShell:

```powershell
$env:DATABASE_URL="postgres://postgres:postgres@localhost:5434/books_write?sslmode=disable"
$env:RABBITMQ_URL="amqp://guest:guest@localhost:5672/"
go run ./cmd/api
```

### Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | yes | | PostgreSQL connection string |
| `RABBITMQ_URL` | yes | | RabbitMQ connection string |
| `HTTP_PORT` | no | `8081` | HTTP port |

## Registering a book

### 1. Through the gateway (real flow)

This is the path clients use. The gateway fills `owner_id` from the JWT; any `owner_id` sent in the body is ignored.

**PowerShell**

```powershell
# login
$login = Invoke-RestMethod -Method Post -Uri http://localhost:8080/auth/login `
  -ContentType "application/json" `
  -Body '{"email":"test@bookshare.com","password":"Password123"}'

# register the book
Invoke-RestMethod -Method Post -Uri http://localhost:8080/books `
  -Headers @{ Authorization = "Bearer $($login.token)" } `
  -ContentType "application/json" `
  -Body '{"name":"Dune","author":"Frank Herbert","description":"Science fiction","pages":600}'
```

**bash / curl**

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@bookshare.com","password":"Password123"}' | jq -r .token)

curl -X POST http://localhost:8080/books \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Dune","author":"Frank Herbert","description":"Science fiction","pages":600}'
```

Response (`201 Created`):

```json
{"id": "51e664a5-79bb-4c59-9002-ff0b2020e5a2"}
```

### 2. Directly on the Write API over HTTP (development)

Without the gateway, the owner is sent in the `X-User-ID` header (in production, only the gateway should set it).

**PowerShell**

```powershell
$body = @{
    name        = "The Hobbit"
    author      = "J.R.R. Tolkien"
    description = "Fantasy"
    pages       = 300
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri http://localhost:8081/books `
  -Headers @{ "X-User-ID" = "3f1c2b9e-8a4d-4c1e-9f2a-1b2c3d4e5f60" } `
  -ContentType "application/json; charset=utf-8" `
  -Body ([System.Text.Encoding]::UTF8.GetBytes($body))
```

**bash / curl**

```bash
curl -X POST http://localhost:8081/books \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 3f1c2b9e-8a4d-4c1e-9f2a-1b2c3d4e5f60" \
  -d '{"name":"The Hobbit","author":"J.R.R. Tolkien","description":"Fantasy","pages":300}'
```

### 3. Through the book.register queue (RabbitMQ management UI)

Useful to test the messaging entry point without the gateway.

1. Create a `test.reply` queue (Durable).
2. Go to **Queues > book.register > Publish message**:
   - **Properties:** `reply_to` = `test.reply`
   - **Payload:**
     ```json
     {"owner_id":"3f1c2b9e-8a4d-4c1e-9f2a-1b2c3d4e5f60","name":"Neuromancer","author":"William Gibson","description":"Cyberpunk","pages":270}
     ```
3. In **Queues > test.reply > Get messages**, the reply shows up as an envelope:
   ```json
   {"ok": true, "data": {"id": "..."}}
   ```

## Error responses

| Case | HTTP | Envelope code (queue) |
|---|---|---|
| invalid body | 400 | `INVALID_INPUT` |
| business rule violated (e.g. fewer than 50 pages) | 400 | `INVALID_INPUT` |
| missing or invalid `owner_id` / `X-User-ID` | 401 | `UNAUTHORIZED` |
| book with the same name and author | 409 | `BOOK_ALREADY_EXISTS` |
| unexpected error | 500 | `INTERNAL` |

Error example over the queue:

```json
{"ok": false, "error": {"code": "BOOK_ALREADY_EXISTS", "message": "book already exists"}}
```

## Published event

**Exchange:** `bookshare.events` (topic, durable)
**Routing key:** `book.created`

```json
{
  "id": "51e664a5-79bb-4c59-9002-ff0b2020e5a2",
  "owner_id": "3f1c2b9e-8a4d-4c1e-9f2a-1b2c3d4e5f60",
  "name": "Dune",
  "author": "Frank Herbert",
  "description": "Science fiction",
  "pages": 600,
  "created_at": "2026-11-07T12:22:18.0983904Z"
}
```

To listen to the event, bind a durable queue to the `bookshare.events` exchange with the `book.created` routing key.

## Checking the database

```bash
docker exec -it books-write-db psql -U postgres -d books_write \
  -c "SELECT name, author, owner_id FROM books;"

docker exec -it books-write-db psql -U postgres -d books_write \
  -c "SELECT event_type, published_at FROM outbox;"
```

A filled `published_at` means the event has already been published to RabbitMQ.

## Tech stack

- Go 1.26
- chi (HTTP router)
- pgx v5 (PostgreSQL)
- amqp091-go (RabbitMQ)
- golang-migrate (migrations)
- Docker / Docker Compose