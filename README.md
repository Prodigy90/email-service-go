# Email Service

A standalone email microservice for sending transactional emails across multiple services.

## Features

- REST API for sending single and bulk emails
- Template system with HTML support
- Async processing via Asynq (Redis)
- SMTP provider abstraction (AWS SES, SendGrid, Mailgun, etc.)
- Idempotency support
- Delivery status tracking
- Client package for Go services

## Quick Start

```bash
# Start all services (Postgres, Redis, Mailhog, API, Worker)
docker-compose up -d

# View Mailhog UI (local email testing)
open http://localhost:8025
```

## API Endpoints

All endpoints require `X-API-Key` header.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/send` | Send single email |
| POST | `/api/v1/send/bulk` | Send to multiple recipients |
| GET | `/api/v1/status/:id` | Get email status |
| GET | `/api/v1/templates` | List available templates |
| GET | `/healthz` | Liveness check |
| GET | `/readyz` | Readiness check |

## Sending Emails

### Using Template

```bash
curl -X POST http://localhost:8083/api/v1/send \
  -H "Content-Type: application/json" \
  -H "X-API-Key: dev-api-key" \
  -d '{
    "to": "user@example.com",
    "template": "payout_approved",
    "template_data": {
      "Name": "John",
      "Amount": "5000",
      "Currency": "₦"
    },
    "source_service": "affiliate-system"
  }'
```

### Using Raw Content

```bash
curl -X POST http://localhost:8083/api/v1/send \
  -H "Content-Type: application/json" \
  -H "X-API-Key: dev-api-key" \
  -d '{
    "to": "user@example.com",
    "subject": "Hello!",
    "body": "This is a test email.",
    "html_body": "<p>This is a <strong>test</strong> email.</p>"
  }'
```

## Using the Go Client

```go
import "github.com/prodigy90/email-service-go/pkg/client"

// Create client
emailClient := client.New("http://email-service:8083", "your-api-key")

// Send email
resp, err := emailClient.Send(ctx, &client.SendRequest{
    To:       "user@example.com",
    Template: client.TemplatePayoutApproved,
    TemplateData: map[string]interface{}{
        "Name":     "John",
        "Amount":   "5000",
        "Currency": "₦",
    },
    SourceService: "affiliate-system",
})
```

## Available Templates

| Template | Description | Variables |
|----------|-------------|-----------|
| `payout_approved` | Payout approved notification | Name, Amount, Currency |
| `payout_rejected` | Payout rejected notification | Name, Amount, Currency, Reason |
| `payout_processed` | Payout completed notification | Name, Amount, Currency, Reference |
| `commission_earned` | Commission earned notification | Name, Amount, Currency, ProductName |
| `payment_success` | Payment confirmation | CustomerName, Amount, Currency, ProductName, TransactionID |
| `payment_failed` | Payment failed notification | CustomerName, Amount, Currency, ProductName |
| `subscription_renewed` | Subscription renewal confirmation | CustomerName, PlanName, Amount, Currency, NextBillingDate |
| `subscription_expiring` | Subscription expiring reminder | CustomerName, PlanName, Days |
| `subscription_cancelled` | Subscription cancelled notification | CustomerName, PlanName, ExpiryDate |
| `welcome` | Welcome email | Name, AppName |
| `trial_expiring` | Trial expiring reminder | Name, AppName, Days, UpgradeURL |
| `account_upgraded` | Account upgraded notification | Name, PlanName, Features |

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `API_PORT` | API server port | 8083 |
| `DATABASE_URL` | PostgreSQL connection string | - |
| `REDIS_URL` | Redis connection string | - |
| `API_KEY` | API authentication key | - |
| `SMTP_HOST` | SMTP server host | localhost |
| `SMTP_PORT` | SMTP server port | 1025 |
| `SMTP_USERNAME` | SMTP username | - |
| `SMTP_PASSWORD` | SMTP password | - |
| `SMTP_FROM_ADDRESS` | Default from address | - |
| `SMTP_FROM_NAME` | Default from name | - |

## AWS SES Setup

1. Verify your domain in AWS SES console
2. Request production access (to send to unverified addresses)
3. Create SMTP credentials in SES console
4. Configure environment variables:

```bash
SMTP_HOST=email-smtp.us-east-1.amazonaws.com
SMTP_PORT=587
SMTP_USERNAME=your-ses-smtp-username
SMTP_PASSWORD=your-ses-smtp-password
SMTP_FROM_ADDRESS=noreply@yourdomain.com
```

## Development

```bash
# Run locally
make run-api
make run-worker

# Run tests
make test

# Lint
make lint
```

## Ports

| Service | Port |
|---------|------|
| API | 8083 |
| PostgreSQL | 55433 |
| Redis | 16380 |
| Mailhog SMTP | 1025 |
| Mailhog Web | 8025 |
