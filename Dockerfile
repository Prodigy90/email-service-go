# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build API
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api

# Build Worker
RUN CGO_ENABLED=0 GOOS=linux go build -o /worker ./cmd/worker

# Unified image with both API and Worker
FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Copy both binaries
COPY --from=builder /api /app/api
COPY --from=builder /worker /app/worker

# Copy templates
COPY --from=builder /app/pkg/templates /app/pkg/templates

# Copy migrations
COPY --from=builder /app/migrations /app/migrations
ENV MIGRATIONS_DIR=/app/migrations

# Copy OpenAPI docs for Swagger UI
COPY --from=builder /app/docs /app/docs

EXPOSE 8083

CMD ["/app/api"]
