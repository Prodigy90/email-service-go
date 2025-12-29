# Email Testing Guide

This guide covers how to set up and test the email service. The service supports two providers:
- **Resend** (Recommended) - Simple API, minimal setup
- **SMTP** (AWS SES, etc.) - Traditional SMTP

## Quick Start with Resend (Recommended)

### 1. Create Resend Account

1. Go to [resend.com](https://resend.com) and sign up
2. Go to API Keys → Create API Key
3. Copy your API key

### 2. Configure Environment

```env
EMAIL_PROVIDER=resend
RESEND_API_KEY=re_xxxxxxxxxxxx
RESEND_FROM_ADDRESS=onboarding@resend.dev  # Use this for testing
RESEND_FROM_NAME=Your App Name
```

> **Note**: `onboarding@resend.dev` works immediately for testing. For production, verify your own domain.

### 3. Start the Service

```bash
# Terminal 1: Start API
cd email-service-go && go run cmd/api/main.go

# Terminal 2: Start worker
cd email-service-go && go run cmd/worker/main.go
```

### 4. Send a Test Email

```bash
curl -X POST http://localhost:8082/api/v1/send \
  -H "Content-Type: application/json" \
  -H "X-API-Key: dev-api-key" \
  -d '{
    "to": "your-email@example.com",
    "template": "payment_success",
    "template_data": {
      "CustomerName": "John",
      "Amount": "5,000.00",
      "Currency": "₦",
      "ProductName": "Pro Plan",
      "TransactionID": "TXN_123456"
    },
    "source_service": "test"
  }'
```

### 5. Verify Your Domain (Production)

For production, add your domain in Resend dashboard:

1. Go to Domains → Add Domain
2. Add DNS records (2 records: MX + TXT)
3. Wait for verification (usually < 5 minutes)
4. Update `RESEND_FROM_ADDRESS` to use your domain

---

## AWS SES Setup (Alternative)

If you prefer AWS SES, follow this section.

### 1. Create AWS Account & Access Keys

1. Go to [AWS Console](https://console.aws.amazon.com/)
2. Create an IAM user with `AmazonSESFullAccess` policy
3. Generate access keys (Access Key ID + Secret Access Key)

### 2. Verify Email Addresses (Sandbox Mode)

While in sandbox mode, you can only send emails to verified addresses:

```bash
# Using AWS CLI
aws ses verify-email-identity --email-address your-email@example.com
```

Or via AWS Console:
1. Go to SES → Verified Identities
2. Click "Create identity"
3. Choose "Email address"
4. Enter your email and click "Create identity"
5. Check your inbox and click the verification link

### 3. Request Production Access

To send to any email address:

1. Go to SES → Account Dashboard
2. Click "Request production access"
3. Fill out the form:
   - **Mail type**: Transactional
   - **Website URL**: Your app URL
   - **Use case**: Describe your use case (e.g., "Subscription reminders and payment notifications")
   - **Expected volume**: Estimate daily email volume

Approval typically takes 24-48 hours.

### 4. Configure Environment

```env
EMAIL_PROVIDER=smtp

# SMTP Configuration for AWS SES
SMTP_HOST=email-smtp.us-east-1.amazonaws.com
SMTP_PORT=587
SMTP_USERNAME=your-smtp-username
SMTP_PASSWORD=your-smtp-password
SMTP_FROM_ADDRESS=noreply@yourdomain.com
SMTP_FROM_NAME=Your App Name
```

### 5. Domain Verification (Recommended)

For production, verify your domain:

1. Go to SES → Verified Identities
2. Click "Create identity" → "Domain"
3. Enter your domain
4. Add the provided DNS records:
   - 3 CNAME records for DKIM
   - 1 TXT record for domain verification
5. Wait for DNS propagation (up to 72 hours)

---

## Configuration Reference

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `EMAIL_PROVIDER` | `resend` or `smtp` (default: smtp) | No |
| `RESEND_API_KEY` | Resend API key | If using Resend |
| `RESEND_FROM_ADDRESS` | Sender email address | If using Resend |
| `RESEND_FROM_NAME` | Sender display name | No |
| `SMTP_HOST` | SMTP server hostname | If using SMTP |
| `SMTP_PORT` | SMTP server port | If using SMTP |
| `SMTP_USERNAME` | SMTP username | If using SMTP |
| `SMTP_PASSWORD` | SMTP password | If using SMTP |
| `SMTP_FROM_ADDRESS` | Sender email address | If using SMTP |
| `SMTP_FROM_NAME` | Sender display name | No |

---

## Available Templates

| Template | Use Case | Required Variables |
|----------|----------|-------------------|
| `subscription_reminder_3d` | 3-day renewal reminder (recurring) | CustomerName, PlanName, RenewalDate, Amount, Currency, ProfileURL |
| `subscription_reminder_1d` | 1-day renewal reminder (recurring) | Same as above |
| `subscription_expiring_3d` | 3-day expiry reminder (one-off) | CustomerName, PlanName, ExpiryDate, ProfileURL |
| `subscription_expiring_1d` | 1-day expiry reminder (one-off) | Same as above |
| `payment_success` | Payment confirmation | CustomerName, Amount, Currency, ProductName, TransactionID |
| `payment_failed` | Payment failure notice | CustomerName, Amount, Currency, ProductName |
| `refund_pending` | Refund initiated | CustomerName, Amount, Currency, TransactionID |
| `refund_processed` | Refund completed | CustomerName, Amount, Currency, TransactionID |
| `refund_failed` | Refund failed | CustomerName, Amount, Currency, TransactionID, Reason |

### List All Templates

```bash
curl http://localhost:8082/api/v1/templates \
  -H "X-API-Key: dev-api-key"
```

---

## Integration Testing

### Test Webhook → Email Flow

1. Start all services:
```bash
# Terminal 1: Email service API
cd email-service-go && go run cmd/api/main.go

# Terminal 2: Email service worker
cd email-service-go && go run cmd/worker/main.go

# Terminal 3: Webhook router worker
cd webhook-router-go && go run cmd/worker/main.go

# Terminal 4: Webhook router API
cd webhook-router-go && go run cmd/api/main.go
```

2. Simulate a Paystack invoice.create webhook:
```bash
curl -X POST http://localhost:8080/api/v1/webhooks/paystack \
  -H "Content-Type: application/json" \
  -H "x-paystack-signature: test-signature" \
  -d '{
    "event": "invoice.create",
    "data": {
      "amount": 500000,
      "customer": {
        "email": "your-email@example.com",
        "first_name": "John",
        "last_name": "Doe"
      },
      "subscription": {
        "subscription_code": "SUB_test123"
      },
      "period_end": "2025-01-15T00:00:00.000Z"
    }
  }'
```

3. Check your email inbox for the reminder

### Test One-Off Expiration Flow

```bash
# Simulate a charge.success for one-off payment
curl -X POST http://localhost:8080/api/v1/webhooks/paystack \
  -H "Content-Type: application/json" \
  -H "x-paystack-signature: test-signature" \
  -d '{
    "event": "charge.success",
    "data": {
      "reference": "TXN_test123",
      "amount": 1000000,
      "created_at": "2025-01-01T00:00:00.000Z",
      "customer": {
        "email": "your-email@example.com",
        "first_name": "Jane"
      },
      "metadata": {
        "referrer": "landing-page"
      }
    }
  }'
```

---

## Troubleshooting

### Common Issues

| Issue | Solution |
|-------|----------|
| `RESEND_API_KEY is required` | Set the `RESEND_API_KEY` environment variable |
| `validation_error: Invalid to address` | Use a valid email format |
| `Email address not verified` (SES) | Verify email in SES console or use Resend |
| `Access Denied` (SES) | Check IAM permissions |
| `Template not found` | Check template name spelling |

### Check Email Status

```bash
curl http://localhost:8082/api/v1/status/{email_id} \
  -H "X-API-Key: dev-api-key"
```

### View Logs

Both API and worker services log email sending activity. Check console output for:
- `Email queued for delivery` - Email accepted and queued
- `Email sent successfully` - Delivered to provider
- `Failed to send email` - Error details

---

## Production Checklist

### Resend
- [ ] Domain verified
- [ ] API key stored securely (not in code)
- [ ] Monitoring set up in Resend dashboard

### AWS SES
- [ ] Domain verified in SES
- [ ] DKIM enabled
- [ ] SPF record configured
- [ ] Production access approved
- [ ] Bounce/complaint handling configured
- [ ] Sending limits appropriate for your volume
- [ ] Environment variables secured (not in code)
- [ ] Monitoring/alerting set up for email failures
