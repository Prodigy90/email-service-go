# Development Roadmap

> Cross-repository development plan for the WasBot ecosystem
> Last updated: 2025-12-30

## System Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              USER JOURNEY                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  1. User clicks affiliate link (wasbot.app/?ref=abc123)                     │
│  2. Frontend stores ref_id in localStorage                                  │
│  3. User signs up → ref_id saved to user record                             │
│  4. User starts 7-day trial                                                 │
│  5. Trial emails sent (Day 0, 3, 5, 6, 7, 10)                               │
│  6. User subscribes via Paystack                                            │
│  7. Paystack webhook → Webhook Router → WasBot                              │
│  8. WasBot handles event: updates DB, sends email, pushes commission        │
│  9. Affiliate receives commission in affiliate-system                       │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Repository Connections

```
                                    ┌──────────────┐
                                    │   Paystack   │
                                    └──────┬───────┘
                                           │ webhooks
                                           ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                            WEBHOOK-ROUTER-GO                                 │
│  - Single entry point for all payment webhooks                               │
│  - Signature verification                                                    │
│  - Event normalization (StandardWebhookResponse)                             │
│  - Product routing (database-driven)                                         │
│  - PURELY A ROUTING LAYER - no business logic                                │
└──────────────────────┬───────────────────────────────────────────────────────┘
                       │
                       ▼
        ┌──────────────────────────────────────────────────────────────────────┐
        │                         WASBOT (k8s)                                 │
        │  Receives ALL webhook events and handles business logic:             │
        │  - Update subscription status in DB                                  │
        │  - Send transactional emails via email-service                       │
        │  - Push commissions to affiliate-system                              │
        │  - Send WhatsApp notifications to user                               │
        │  - Handle tier changes (cache invalidation, pod rebuilds)            │
        └───────────────┬──────────────────────────┬───────────────────────────┘
                        │                          │
                        ▼                          ▼
         ┌──────────────────────────┐   ┌──────────────────────────┐
         │    EMAIL-SERVICE-GO      │   │   AFFILIATE-SYSTEM-GO    │
         │  - Transactional emails  │   │  - Commission tracking   │
         │  - Template management   │   │  - Commission reversal   │
         │  - Async queue (Redis)   │   │  - Payout processing     │
         └──────────────────────────┘   └──────────────────────────┘
```

---

## Current State Analysis

### What Already Exists

| Component | Location | Status |
|-----------|----------|--------|
| Webhook endpoint | `whatsmeow-test/cmd/api-server/handlers/webhook_handler.go` | ✅ Complete |
| HMAC verification | webhook_handler.go | ✅ Complete |
| Idempotency (trace_id) | webhook_handler.go + webhook_events table | ✅ Complete |
| Email client | `whatsmeow-test/internal/email/client.go` | ✅ Integrated |
| Subscription updates | webhook_handler.go → userSubRepo | ✅ Complete |
| WhatsApp notifications | NotificationDispatcher | ✅ Complete |
| Tier change handling | TierChangeService | ✅ Complete |

### What's Missing

| Component | Location | Status |
|-----------|----------|--------|
| `invoice_created` handler | webhook_handler.go | ✅ Implemented |
| `refund_*` handlers | webhook_handler.go | ✅ Implemented |
| Email on webhook events | webhook_handler.go | ✅ Complete (all events) |
| ref_id in user schema | migrations | ❌ Not implemented |
| Affiliate client | internal/services/ | ❌ Not implemented |
| Commission tracking | webhook_handler.go | ❌ Not implemented |

---

## Phase 1: Complete Webhook Event Handling (Priority: HIGH)

### 1.1 Add Missing Event Handlers

**Repository:** `whatsmeow-test`

**File:** `cmd/api-server/handlers/webhook_handler.go`

Add handlers for these events in `handleBusinessLogic()`:

```go
case "invoice_created":
    // Upcoming renewal notification
    // 1. Send 3-day reminder email
    // 2. Schedule 1-day reminder (or let cron handle it)

case "refund_pending":
    // Refund initiated
    // 1. Update payment status
    // 2. Send refund pending email

case "refund_processed":
    // Refund completed
    // 1. Update subscription status (if full refund → cancelled)
    // 2. Send refund processed email
    // 3. Reverse commission in affiliate-system
    // 4. Revoke access if applicable

case "refund_failed":
    // Refund failed
    // 1. Log the failure
    // 2. Send refund failed email
```

**Checklist:**
- [x] Add `invoice_created` case - send renewal reminder email
- [x] Add `refund_pending` case - send refund pending email
- [x] Add `refund_processed` case - send email, reverse commission, update status
- [x] Add `refund_failed` case - send failure notification email

### 1.2 Wire Up Email Sending on All Events

**Repository:** `whatsmeow-test`

**Current State:** Email client exists at `internal/email/client.go` with methods:
- `SendWelcome()`
- `SendTrialExpiring()`
- `SendSubscriptionCancelled()`
- `SendRefundProcessed()`
- `SendPaymentSuccess()`
- `SendPaymentFailed()`

**Required Changes:**

Add email client to WebhookHandler and call appropriate methods:

```go
// In WebhookHandler struct
emailClient *email.Client

// In handleBusinessLogic switch cases:
case "subscription_created":
    // ... existing logic ...
    // Add: Send welcome/confirmation email
    go h.emailClient.SendPaymentSuccess(ctx, p.Customer.Email, customerName, planName, amount, currency)

case "subscription_renewed":
    // ... existing logic ...
    // Add: Send renewal confirmation email
    go h.emailClient.SendPaymentSuccess(ctx, p.Customer.Email, customerName, planName, amount, currency)

case "subscription_cancelled":
    // ... existing logic ...
    // Add: already sends email via notificationDispatcher, add email too
    go h.emailClient.SendSubscriptionCancelled(ctx, p.Customer.Email, customerName, planName, expiryDate)

case "payment_failed":
    // ... existing logic ...
    // Add: Send payment failed email
    go h.emailClient.SendPaymentFailed(ctx, p.Customer.Email, customerName, planName, retryURL)

case "invoice_created":
    // NEW: Send renewal reminder
    go h.emailClient.SendSubscriptionReminder(ctx, p.Customer.Email, customerName, planName, renewalDate, amount, currency)

case "refund_processed":
    // NEW: Send refund confirmation
    go h.emailClient.SendRefundProcessed(ctx, p.Customer.Email, customerName, amount, currency, transactionID)
```

**Files to Modify:**
- [x] `cmd/api-server/handlers/webhook_handler.go` - Add emailClient field, inject in constructor
- [x] `cmd/api-server/server.go` - Pass emailClient to NewWebhookHandler()
- [x] `internal/email/client.go` - Add `SendSubscriptionReminder()` method if missing

---

## Phase 2: Affiliate Integration (Priority: HIGH)

### 2.1 Add Referral Tracking to User Schema

**Repository:** `whatsmeow-test`

**New Migration:** `migrations/019_add_referral_tracking.sql`

```sql
-- Add ref_id to track which affiliate referred this user
ALTER TABLE user_subscriptions
ADD COLUMN IF NOT EXISTS ref_id VARCHAR(255);

-- Index for lookups
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_ref_id
ON user_subscriptions(ref_id) WHERE ref_id IS NOT NULL;

-- Track commission status per transaction
ALTER TABLE user_subscriptions
ADD COLUMN IF NOT EXISTS last_commission_txn_id VARCHAR(255);
```

**Files to Create/Modify:**
- [ ] `migrations/019_add_referral_tracking.up.sql`
- [ ] `migrations/019_add_referral_tracking.down.sql`
- [ ] `internal/database/postgres/user_subscription_repository.go` - Add ref_id to UpsertFromWebhook

### 2.2 Capture Referral ID on Signup

**Repository:** `whatsmeow-test`

**Frontend Changes (Next.js rebuild):**

```typescript
// On page load
const urlParams = new URLSearchParams(window.location.search);
const refId = urlParams.get("ref");
if (refId) {
  localStorage.setItem("wasbot_ref_id", refId);
}

// On signup API call
const signup = async (userData) => {
  const refId = localStorage.getItem("wasbot_ref_id");
  return fetch("/api/v1/auth/register", {
    body: JSON.stringify({ ...userData, ref_id: refId })
  });
};
```

**Backend Changes:**

- [ ] `cmd/api-server/handlers/auth_handler.go` - Accept ref_id in signup request
- [ ] `internal/database/postgres/user_repository.go` - Store ref_id on user creation
- [ ] Pass ref_id to Paystack checkout as metadata (so it comes back in webhooks)

### 2.3 Create Affiliate Client Service

**Repository:** `whatsmeow-test`

**New File:** `internal/services/affiliate_client.go`

```go
package services

import (
    "bytes"
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type AffiliateClient struct {
    baseURL    string
    apiKey     string
    productID  string
    httpClient *http.Client
}

func NewAffiliateClient(baseURL, apiKey, productID string) *AffiliateClient {
    return &AffiliateClient{
        baseURL:    baseURL,
        apiKey:     apiKey,
        productID:  productID,
        httpClient: &http.Client{Timeout: 10 * time.Second},
    }
}

type CommissionRequest struct {
    RefID              string            `json:"ref_id"`
    ProductID          string            `json:"product_id"`
    CustomerIdentifier string            `json:"customer_identifier"`
    Payment            PaymentInfo       `json:"payment"`
    Subscription       SubscriptionInfo  `json:"subscription"`
    Trace              TraceInfo         `json:"trace"`
}

type PaymentInfo struct {
    TransactionID string    `json:"transaction_id"`
    Amount        int64     `json:"amount"`        // In smallest unit (kobo)
    Currency      string    `json:"currency"`
    PaidAt        time.Time `json:"paid_at"`
}

type SubscriptionInfo struct {
    PlanName string `json:"plan_name"`
    Interval string `json:"interval"`
    Type     string `json:"type"` // recurring, one_time
}

type TraceInfo struct {
    TraceID string `json:"trace_id"`
}

// TrackCommission records a commission for an affiliate
func (c *AffiliateClient) TrackCommission(ctx context.Context, req CommissionRequest) error {
    req.ProductID = c.productID
    req.CustomerIdentifier = hashEmail(req.CustomerIdentifier)

    body, _ := json.Marshal(req)
    httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/commissions/track", bytes.NewBuffer(body))
    if err != nil {
        return err
    }

    httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusAccepted {
        return fmt.Errorf("affiliate API returned %d", resp.StatusCode)
    }
    return nil
}

// RefundCommission reverses a commission when payment is refunded
func (c *AffiliateClient) RefundCommission(ctx context.Context, transactionID, reason string) error {
    body, _ := json.Marshal(map[string]string{
        "transaction_id": transactionID,
        "reason":         reason,
    })

    httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/commissions/refund", bytes.NewBuffer(body))
    if err != nil {
        return err
    }

    httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusAccepted {
        return fmt.Errorf("affiliate API returned %d", resp.StatusCode)
    }
    return nil
}

func hashEmail(email string) string {
    h := sha256.Sum256([]byte(email))
    return hex.EncodeToString(h[:])
}
```

**Files to Create:**
- [ ] `internal/services/affiliate_client.go`
- [ ] `internal/services/affiliate_client_test.go`

### 2.4 Integrate Commission Tracking into Webhook Handler

**Repository:** `whatsmeow-test`

**File:** `cmd/api-server/handlers/webhook_handler.go`

```go
// Add to WebhookHandler struct
affiliateClient *services.AffiliateClient

// In handleBusinessLogic, after successful subscription/payment:
case "subscription_created", "subscription_renewed", "payment_success":
    // ... existing logic ...

    // Push commission if user has ref_id
    if sub != nil && sub.RefID != "" {
        go func() {
            err := h.affiliateClient.TrackCommission(context.Background(), services.CommissionRequest{
                RefID: sub.RefID,
                Payment: services.PaymentInfo{
                    TransactionID: p.Payment.TransactionID,
                    Amount:        int64(p.Payment.Amount),
                    Currency:      p.Payment.Currency,
                    PaidAt:        time.Now(),
                },
                Subscription: services.SubscriptionInfo{
                    PlanName: p.Subscription.PlanName,
                    Interval: p.Subscription.Interval,
                    Type:     p.Subscription.Type,
                },
                Trace: services.TraceInfo{TraceID: p.TraceID},
            })
            if err != nil {
                h.logger.Warn().Err(err).Str("ref_id", sub.RefID).Msg("Failed to track commission")
            }
        }()
    }

case "refund_processed":
    // ... send email ...

    // Reverse commission
    if p.Payment.TransactionID != "" {
        go func() {
            err := h.affiliateClient.RefundCommission(context.Background(), p.Payment.TransactionID, "Customer refund")
            if err != nil {
                h.logger.Warn().Err(err).Str("txn_id", p.Payment.TransactionID).Msg("Failed to reverse commission")
            }
        }()
    }
```

**Files to Modify:**
- [ ] `cmd/api-server/handlers/webhook_handler.go` - Add affiliateClient, integrate tracking
- [ ] `cmd/api-server/server.go` - Initialize and inject affiliateClient

---

## Phase 3: Trial Email Sequence (Priority: MEDIUM)

### 3.1 Trial Email Templates

**Repository:** `email-service-go`

Templates already exist:
- `trial_expiring` - Generic trial expiring template

**May need to add:**
- [ ] `trial_day3_engagement` - Day 3 engagement check
- [ ] `trial_day5_reminder` - Day 5 reminder
- [ ] `trial_day6_urgent` - Day 6 urgent reminder
- [ ] `trial_day10_followup` - Day 10 win-back

### 3.2 Trial Email Scheduler

**Repository:** `whatsmeow-test`

**Current State:** Has `SubscriptionExpirationCron` that runs daily

**Required Changes:**
- [ ] Extend cron to send trial emails based on trial_started_at
- [ ] Track sent emails to avoid duplicates (new table or Redis)
- [ ] Skip users who converted to paid

---

## Phase 4: Frontend Rebuild (Priority: MEDIUM)

### 4.1 Technology Stack

```
Framework:     Next.js 15 (App Router)
UI Library:    shadcn/ui
Animations:    Framer Motion
Styling:       Tailwind CSS
Auth:          Better Auth
State:         TanStack Query
```

### 4.2 Key Features

- [ ] Referral link capture on signup (`?ref=` parameter)
- [ ] Dashboard with subscription management
- [ ] WhatsApp device management
- [ ] Billing and payment history

---

## Phase 5: Infrastructure (Priority: LOW)

### 5.1 k3s Deployment

All services deployed to k3s cluster with:
- PgBouncer for PostgreSQL connection pooling
- Redis for queues (asynq)
- Tailscale for internal service mesh

---

## Environment Variables Reference

### whatsmeow-test (WasBot)

```env
# Webhook verification
WEBHOOK_HMAC_SECRET=xxx

# Email service
EMAIL_SERVICE_URL=http://email-service:8082
EMAIL_SERVICE_API_KEY=xxx

# Affiliate service
AFFILIATE_SERVICE_URL=http://affiliate-system:8080
AFFILIATE_API_KEY=xxx                    # Product API key from affiliate-system
AFFILIATE_PRODUCT_ID=wasbot              # Product identifier

# Swagger access control
SWAGGER_ALLOWED_IPS=100.64.0.0/10
```

### webhook-router-go

```env
# Swagger access control
SWAGGER_ALLOWED_IPS=100.64.0.0/10

# Products configured via database API, not env vars
```

### affiliate-system-go

```env
# Product API keys generated per-product via admin API
SWAGGER_ALLOWED_IPS=100.64.0.0/10
```

### email-service-go

```env
API_KEY=xxx
SWAGGER_ALLOWED_IPS=100.64.0.0/10
```

---

## Task Checklist

### Immediate (Phase 1) - ✅ COMPLETE

- [x] Add `invoice_created` handler to webhook_handler.go
- [x] Add `refund_pending`, `refund_processed`, `refund_failed` handlers
- [x] Wire up email sending on all webhook events
- [ ] Test webhook flow end-to-end

### Short Term (Phase 2)

- [ ] Create migration for ref_id column
- [ ] Update signup to capture ref_id
- [ ] Create affiliate_client.go service
- [ ] Integrate commission tracking into webhook handler
- [ ] Test affiliate commission flow end-to-end

### Medium Term (Phase 3-4)

- [ ] Implement trial email scheduler
- [ ] Start Next.js frontend rebuild
- [ ] Implement Better Auth integration

### Long Term (Phase 5)

- [ ] Deploy to k3s cluster
- [ ] Set up PgBouncer
- [ ] Configure Tailscale mesh

---

## Notes

- All timestamps in UTC
- All services use structured logging (zerolog in WasBot, slog in others)
- API versioning: /api/v1/
- Health checks: /health
- Swagger UI: /swagger (IP restricted to Tailscale network)
- Webhook router is PURELY a routing layer - all business logic lives in products (WasBot)
