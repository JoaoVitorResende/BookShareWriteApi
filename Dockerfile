# ---------- Etapa 1: build ----------
# Imagem com o Go completo, usada só para compilar.
FROM golang:1.26-alpine AS build

WORKDIR /src

# Copia só go.mod/go.sum primeiro e baixa as dependências.
# Enquanto eles não mudarem, o Docker reaproveita essa camada do cache
# e o build fica muito mais rápido.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 gera um binário estático (não depende de libs do sistema).
# -ldflags="-s -w" remove símbolos de debug e deixa o binário menor.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/write-api ./cmd/api

# ---------- Etapa 2: runtime ----------
# Imagem mínima: sem shell, sem gerenciador de pacotes, roda como usuário não-root.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/write-api /write-api

EXPOSE 8081

ENTRYPOINT ["/write-api"]