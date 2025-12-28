# Build stage
FROM golang:1.23-alpine AS builder

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

# API image
FROM alpine:3.19 AS api

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /api /app/api
COPY --from=builder /app/pkg/templates /app/pkg/templates

EXPOSE 8082

CMD ["/app/api"]

# Worker image
FROM alpine:3.19 AS worker

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /worker /app/worker
COPY --from=builder /app/pkg/templates /app/pkg/templates

CMD ["/app/worker"]
