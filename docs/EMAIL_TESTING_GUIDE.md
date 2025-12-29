# Email Testing Guide

This guide covers how to set up and test the email service with AWS SES.

## AWS SES Setup

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

## Configuration

### Environment Variables

```env
# AWS SES Configuration
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key

# Email Service Settings
FROM_EMAIL=noreply@yourdomain.com
FROM_NAME=Your App Name

# Optional: SMTP Configuration (alternative to API)
SMTP_HOST=email-smtp.us-east-1.amazonaws.com
SMTP_PORT=587
SMTP_USERNAME=your-smtp-username
SMTP_PASSWORD=your-smtp-password
```

### Domain Verification (Recommended)

For production, verify your domain:

1. Go to SES → Verified Identities
2. Click "Create identity" → "Domain"
3. Enter your domain
4. Add the provided DNS records:
   - 3 CNAME records for DKIM
   - 1 TXT record for domain verification
5. Wait for DNS propagation (up to 72 hours)

## Testing Locally

### 1. Start the Email Service

```bash
cd email-service-go
go run cmd/api/main.go
```

### 2. Send a Test Email

```bash
# Using curl
curl -X POST http://localhost:8082/api/v1/send \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "to": "verified-email@example.com",
    "template": "subscription_reminder_3d",
    "template_data": {
      "CustomerName": "John",
      "PlanName": "Pro Plan",
      "RenewalDate": "January 15, 2025",
      "Amount": "5000.00",
      "Currency": "₦",
      "ProfileURL": "https://yourapp.com/profile"
    },
    "source_service": "test"
  }'
```

### 3. Check Email Status

```bash
curl http://localhost:8082/api/v1/status/{email_id} \
  -H "X-API-Key: your-api-key"
```

## Testing Email Templates

### Available Templates

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

### Test Each Template

```bash
# List all templates
curl http://localhost:8082/api/v1/templates \
  -H "X-API-Key: your-api-key"
```

## Integration Testing

### Test Webhook → Email Flow

1. Start all services:
```bash
# Terminal 1: Email service
cd email-service-go && go run cmd/api/main.go

# Terminal 2: Webhook router worker
cd webhook-router-go && go run cmd/worker/main.go

# Terminal 3: Webhook router API
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
        "email": "verified-email@example.com",
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
        "email": "verified-email@example.com",
        "first_name": "Jane"
      },
      "metadata": {
        "referrer": "landing-page"
      }
    }
  }'
```

## Troubleshooting

### Email Not Sending

1. **Check SES Sandbox Mode**: Ensure recipient email is verified
2. **Check Bounce Rate**: High bounce rates can suspend your account
3. **Check Logs**: Review email-service logs for errors
4. **Verify Credentials**: Ensure AWS credentials are correct

### Email in Spam

1. Set up SPF record for your domain
2. Enable DKIM signing in SES
3. Set up DMARC record
4. Use consistent From address

### Common Errors

| Error | Solution |
|-------|----------|
| `Email address not verified` | Verify email in SES console |
| `Access Denied` | Check IAM permissions |
| `Throttling` | Request higher sending limits |
| `Template not found` | Check template name spelling |

## Production Checklist

- [ ] Domain verified in SES
- [ ] DKIM enabled
- [ ] SPF record configured
- [ ] Production access approved
- [ ] Bounce/complaint handling configured
- [ ] Sending limits appropriate for your volume
- [ ] Environment variables secured (not in code)
- [ ] Monitoring/alerting set up for email failures
