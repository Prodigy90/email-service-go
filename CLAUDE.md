# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Git Commit Rules

When making git commits:
- Do NOT include the `Co-Authored-By` line
- Do NOT include the "Generated with Claude Code" line
- Just use a clean commit message with no attribution footer

## Build & Development Commands

```bash
# Build both binaries
make build                    # Outputs: bin/api, bin/worker

# Run locally (requires Postgres + Redis running)
make run-api                  # Start API server
make run-worker               # Start async worker

# Hot reload development (requires `make tools` first)
make dev-api                  # API with air hot reload
make dev-worker               # Worker with air hot reload

# Testing
make test                     # Run all tests: go test -v ./...
make test-coverage            # Generate HTML coverage report
go test -v ./internal/service/...  # Run tests for specific package

# Linting
make lint                     # Run golangci-lint

# Database migrations
make migrate-up               # Apply migrations (needs DATABASE_URL)
make migrate-down             # Rollback one migration
make migrate-reset            # Drop all and re-apply

# Docker
make docker-up                # Start all services (Postgres, Redis, Mailhog, API, Worker)
make docker-down              # Stop services
make docker-reset             # Full reset with volume cleanup
make docker-logs              # Stream logs

# Install dev tools
make tools                    # Installs air, golangci-lint, migrate
```

## Architecture Overview

This is a transactional email microservice with async processing:

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   HTTP API  │────▶│ Email Service│────▶│  PostgreSQL │
│  (Gin)      │     │              │     │  (Storage)  │
└─────────────┘     └──────┬───────┘     └─────────────┘
                          │
                          ▼
                   ┌──────────────┐     ┌─────────────┐
                   │  Asynq Queue │────▶│   Worker    │────▶ Email Provider
                   │  (Redis)     │     │             │      (SMTP/Resend)
                   └──────────────┘     └─────────────┘
```

**Request Flow:**
1. API receives email request → validates → stores in Postgres → enqueues to Redis → returns 202
2. Worker dequeues task → sends via SMTP or Resend API → updates status in DB

## Key Directories

- `cmd/api/` - API server entrypoint
- `cmd/worker/` - Async worker entrypoint
- `internal/api/handlers/` - HTTP request handlers
- `internal/api/middleware/` - Auth (API key), CORS, logging middleware
- `internal/service/` - Business logic (email orchestration, SMTP/Resend providers, template rendering)
- `internal/repository/postgres/` - Database operations
- `internal/worker/tasks/` - Asynq task definitions
- `pkg/client/` - Go SDK for consuming services
- `pkg/templates/` - Email template definitions (YAML)
- `migrations/` - PostgreSQL schema migrations

## Provider Pattern

Email sending uses a provider interface (`internal/service/sender.go`):
- `SMTPClient` - Traditional SMTP (AWS SES, etc.)
- `ResendClient` - Resend API
- Selected via `EMAIL_PROVIDER` env var ("smtp" or "resend")

## Template System

Templates defined in `pkg/templates/templates.yaml`. Each template has:
- Plain text body
- HTML body with styling
- Variable placeholders using Go's `text/template` syntax

To add a template: edit `templates.yaml` and add subject/body/html_body with `{{.VarName}}` placeholders.

## Environment Configuration

Copy `.env.example` to `.env`. Key variables:
- `DATABASE_URL` - Postgres connection string
- `REDIS_URL` - Redis for task queue
- `API_KEY` - Authentication key for API requests
- `EMAIL_PROVIDER` - "smtp" or "resend"
- `SMTP_*` or `RESEND_*` - Provider-specific config

## Local Development Ports

| Service    | Port  |
|------------|-------|
| API        | 8082  |
| PostgreSQL | 55433 |
| Redis      | 16380 |
| Mailhog UI | 8025  |
| Mailhog SMTP| 1025 |

## API Authentication

All `/api/v1/*` endpoints require `X-API-Key` header (or `Authorization: Bearer <key>`).
Health endpoints `/healthz` and `/readyz` are public.
