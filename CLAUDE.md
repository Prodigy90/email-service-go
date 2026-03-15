# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Git Commit Rules

When making git commits:
- Do NOT include the `Co-Authored-By` line
- Do NOT include the "Generated with Claude Code" line
- Just use a clean commit message with no attribution footer

## Branding

The product name is **WASBOT** — always written in ALL CAPS. Never write it as "WasBot", "Wasbot", or "wasbot" in user-facing text, docs, or code comments.

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

Templates are defined in categorized YAML files under `pkg/templates/`:
- `auth.yaml` — email_verification, password_reset
- `welcome.yaml` — welcome, trial_day3/5/6/10, trial_expiring, account_upgraded
- `payments.yaml` — payment_success/failed, subscription_activated/renewed/cancelled/reminders
- `refunds.yaml` — refund_pending/processed/failed
- `affiliate.yaml` — payout_*, commission_*, access_revoked, affiliate_link_updated
- `migration.yaml` — migration campaign templates + legacy_provisioned
- `campaigns.yaml` — campaign_update, feature_announcement, testimonial_request

Each template has:
- `subject` — email subject line
- `body` — plain text body
- `html_body` — HTML body with styling
- `description` — template purpose (optional)
- `preview_text` — email preview text (optional)
- Variable placeholders using Go's `text/template` syntax: `{{.VarName}}`

To add a template: create or edit the appropriate YAML file in `pkg/templates/` and add the template with subject/body/html_body fields.
All `*.yaml` files in the directory are loaded automatically on startup.

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
| API        | 8083  |
| PostgreSQL | 55433 |
| Redis      | 16380 |
| Mailhog UI | 8025  |
| Mailhog SMTP| 1025 |

## API Authentication

All `/api/v1/*` endpoints require `X-API-Key` header (or `Authorization: Bearer <key>`).
Health endpoints `/healthz` and `/readyz` are public.
