# Stage 1: build production binary
FROM golang:1.25 AS build
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends git && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server ./cmd/api

# Stage 1.5: build swagger docs
FROM golang:1.25 AS swagger-builder
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends git && rm -rf /var/lib/apt/lists/*
RUN go install github.com/swaggo/swag/cmd/swag@latest
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN /go/bin/swag init -g cmd/api/main.go

# Stage 2: air binary for dev hot-reload
FROM golang:1.25 AS air-builder
RUN go install github.com/air-verse/air@v1.61.1

# Stage 3: dev (hot reload via air)
FROM golang:1.25 AS dev
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends git ffmpeg ca-certificates && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /app/tmp && chmod 777 /app/tmp
COPY --from=air-builder /go/bin/air /usr/local/bin/air
COPY go.mod go.sum ./
RUN go mod download
COPY --from=swagger-builder /app/docs ./docs
COPY .air.toml .
COPY . .
CMD ["air"]

# Stage 4: production — debian slim so ffmpeg can run alongside the static binary
FROM debian:bookworm-slim AS production
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg ca-certificates && \
    rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /app/server .
COPY --from=swagger-builder /app/docs ./docs
COPY --from=build /app/internal/email/templates ./internal/email/templates
EXPOSE 3000
CMD ["/app/server"]
