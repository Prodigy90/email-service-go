# Development Roadmap

> Cross-repository development plan for the WasBot ecosystem
> Last updated: 2025-12-30

## System Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              USER JOURNEY                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  1. User clicks affiliate link (wasbot.app/?ref=abc123)                     │
│  2. Frontend stores ref_id in localStorage                                   │
│  3. User signs up → ref_id saved to user record                             │
│  4. User starts 7-day trial                                                  │
│  5. Trial emails sent (Day 0, 3, 5, 6, 7, 10)                               │
│  6. User subscribes via Paystack                                             │
│  7. Paystack webhook → Webhook Router → WasBot                              │
│  8. WasBot checks user's ref_id → pushes commission to Affiliate System     │
│  9. Affiliate receives commission                                            │
│                                                                              │
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
│                            WEBHOOK-ROUTER-GO                                  │
│  - Single entry point for all payment webhooks                               │
│  - Signature verification                                                     │
│  - Event normalization                                                        │
│  - Product routing                                                            │
└──────────────────────┬───────────────────────────────┬───────────────────────┘
                       │                               │
                       ▼                               ▼
        ┌──────────────────────────┐    ┌──────────────────────────┐
        │      WASBOT (k8s)        │    │   AFFILIATE-SYSTEM-GO    │
        │  - WhatsApp automation   │───▶│  - Affiliate management  │
        │  - User management       │    │  - Commission tracking   │
        │  - Trial system          │    │  - Payout processing     │
        │  - Subscription mgmt     │    │  - Referral links        │
        └──────────────┬───────────┘    └──────────────────────────┘
                       │
                       ▼
        ┌──────────────────────────┐
        │    EMAIL-SERVICE-GO      │
        │  - Transactional emails  │
        │  - Template management   │
        │  - Trial email sequences │
        └──────────────────────────┘
```

---

## Phase 1: Webhook Flow Fixes (Priority: HIGH)

### 1.1 Update WasBot Product Registration

**Repository:** `webhook-router-go`

**Current State:**
- WasBot monolith is registered as a **product in the database** (receives ALL payment events)
- Webhook-router is now purely a routing layer - no email sending, no internal service forwarding
- Products (WasBot) handle all their own business logic: emails, commission tracking, etc.

**How Products Work:**
- Products are **database entities**, NOT config file entries
- Register via API: `POST /api/products`
- Link to processor: `POST /api/products/{id}/processors`
- Add identifiers (matching rules) to route webhooks

**What WasBot k8s Needs:**
1. Update the existing product's `webhook_endpoint` to point to k8s (via API or direct DB update)
2. OR create a new product for k8s environment with proper identifiers

**API Example to Update Product Endpoint:**
```bash
# Update existing WasBot product endpoint
curl -X PUT https://webhook-router/api/products/{wasbot-product-id} \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json" \
  -d '{
    "webhook_endpoint": "https://wasbot-k8s.internal/api/v1/webhooks/payment",
    "auth_config": {
      "type": "hmac",
      "secret": "your-webhook-secret"
    }
  }'
```

**No Code Changes Required in webhook-router-go** - this is purely configuration/data.

**Note:** Products handle their own downstream logic (e.g., commission reversal on refunds).
The webhook-router simply routes events - it doesn't need to know about affiliate systems or other services.

### 1.2 Add Webhook Endpoint to WasBot (k8s version)

**Repository:** `whatsmeow-test`

**Current State:** Monolith receives webhooks, k8s version not connected

**Required Changes:**
- [ ] Create `/api/v1/webhooks/payment` endpoint
- [ ] Implement signature verification (HMAC)
- [ ] Handle normalized webhook events from webhook-router
- [ ] Process subscription lifecycle events
- [ ] Trigger commission push on successful payments

**Files to Create/Modify:**
- [ ] `cmd/api-server/handlers/webhook_handler.go` - New webhook handler
- [ ] `cmd/api-server/server.go` - Register webhook routes
- [ ] `internal/services/subscription_service.go` - Handle subscription events

### 1.3 Webhook Event Processing Flow

**Important:** Products (WasBot) handle ALL business logic including:
- Email notifications (payment success, failure, refunds)
- Subscription reminders (3-day, 1-day before renewal/expiration)
- Commission tracking and push to affiliate system

```go
// Pseudo-code for WasBot webhook handler
func HandlePaymentWebhook(event WebhookEvent) error {
    switch event.Type {
    case "subscription_created":
        // 1. Update user subscription status
        // 2. Send welcome/confirmation email
        // 3. Check for ref_id on user
        // 4. If ref_id exists, push commission to affiliate-system

    case "subscription_renewed":
        // 1. Extend subscription
        // 2. Send renewal confirmation email
        // 3. Push recurring commission if configured

    case "subscription_cancelled":
        // 1. Mark subscription as cancelled
        // 2. Send cancellation email
        // 3. Schedule access revocation

    case "invoice_created":
        // 1. Send 3-day reminder email
        // 2. Schedule 1-day reminder

    case "payment_success":
        // 1. Update payment status
        // 2. Send payment confirmation email

    case "payment_failed":
        // 1. Send failure notification email
        // 2. Trigger dunning sequence

    case "refund_processed":
        // 1. Update payment/subscription status
        // 2. Send refund notification email
        // 3. Reverse commission in affiliate-system if applicable
    }
}
```

---

## Phase 2: Affiliate Integration (Priority: HIGH)

### 2.1 Capture Referral ID in WasBot

**Repository:** `whatsmeow-test`

**Frontend Changes:**
```typescript
// On page load, check for ref parameter
const urlParams = new URLSearchParams(window.location.search);
const refId = urlParams.get('ref');
if (refId) {
    localStorage.setItem('wasbot_ref_id', refId);
}

// On signup, include ref_id
const signup = async (userData) => {
    const refId = localStorage.getItem('wasbot_ref_id');
    return fetch('/api/v1/auth/register', {
        body: JSON.stringify({
            ...userData,
            ref_id: refId
        })
    });
};
```

**Backend Changes:**
- [ ] Add `ref_id` column to users table (Migration 018)
- [ ] Update signup handler to accept and store ref_id
- [ ] Add index on ref_id for lookups

**Files to Modify:**
- [ ] `migrations/018_add_referral_tracking.sql`
- [ ] `internal/database/postgres/user_repository.go`
- [ ] `cmd/api-server/handlers/auth_handler.go`

### 2.2 Commission Push to Affiliate System

**Repository:** `whatsmeow-test`

**New Service: Commission Client**

```go
// internal/services/affiliate_client.go
type AffiliateClient struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

type CommissionRequest struct {
    ReferralLinkID string  `json:"referral_link_id"`
    Amount         float64 `json:"amount"`
    Currency       string  `json:"currency"`
    TransactionID  string  `json:"transaction_id"`
    CustomerEmail  string  `json:"customer_email"`
    ProductID      string  `json:"product_id"`
    EventType      string  `json:"event_type"` // initial, renewal
}

func (c *AffiliateClient) PushCommission(ctx context.Context, req CommissionRequest) error {
    // POST to affiliate-system /api/v1/commissions/record
}
```

**Files to Create:**
- [ ] `internal/services/affiliate_client.go`
- [ ] `internal/services/affiliate_client_test.go`

### 2.3 Add Commission Endpoint to Affiliate System

**Repository:** `affiliate-system-go`

**Current State:** Expects products to push commission data, endpoint may need creation

**Required Endpoint:**
```
POST /api/v1/commissions/record
Authorization: X-API-Key: {product_api_key}

{
    "referral_link_id": "abc123",
    "amount": 5000,
    "currency": "NGN",
    "transaction_id": "txn_xxx",
    "customer_email": "user@example.com",
    "product_id": "wasbot-prod",
    "event_type": "initial"
}
```

**Files to Verify/Create:**
- [ ] `internal/handlers/commission_handler.go`
- [ ] `internal/services/commission_service.go`
- [ ] Verify product API key authentication

### 2.4 Referral Link Resolution

**Repository:** `affiliate-system-go`

The affiliate system needs to:
1. Receive referral_link_id from WasBot
2. Look up which affiliate owns that link
3. Calculate commission based on product config
4. Credit affiliate's balance

**Verify these exist:**
- [ ] Referral link → Affiliate lookup
- [ ] Product commission percentage config
- [ ] Commission calculation logic
- [ ] Affiliate balance crediting

---

## Phase 3: Trial Email Sequence (Priority: MEDIUM)

### 3.1 Create Trial Email Templates

**Repository:** `email-service-go`

**Templates to Create:**

| Template ID | Trigger | Subject |
|-------------|---------|---------|
| `trial_started` | Day 0 | Welcome to WasBot! Your 7-day trial has begun |
| `trial_day3_engagement` | Day 3 | How's your WasBot experience? |
| `trial_day5_reminder` | Day 5 | 2 days left on your trial |
| `trial_day6_urgent` | Day 6 | Last day tomorrow! |
| `trial_expired` | Day 7 | Your trial has ended |
| `trial_day10_followup` | Day 10 | We miss you! Special offer inside |

**Files to Create:**
- [ ] `templates/trial_started.html`
- [ ] `templates/trial_day3_engagement.html`
- [ ] `templates/trial_day5_reminder.html`
- [ ] `templates/trial_day6_urgent.html`
- [ ] `templates/trial_expired.html`
- [ ] `templates/trial_day10_followup.html`
- [ ] `migrations/xxx_seed_trial_templates.sql`

### 3.2 Implement Trial Email Scheduler

**Repository:** `whatsmeow-test`

**Current State:** Has 7-day trial expiration cron

**Required Changes:**
- [ ] Create trial email job scheduler
- [ ] Track which emails have been sent per user
- [ ] Skip emails if user already converted

**New Table:**
```sql
CREATE TABLE trial_emails_sent (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    template_id VARCHAR(50) NOT NULL,
    sent_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, template_id)
);
```

**Scheduler Logic:**
```go
// Run daily at 9 AM
func ProcessTrialEmails(ctx context.Context) error {
    users := repo.GetActiveTrialUsers(ctx)

    for _, user := range users {
        daysSinceStart := time.Since(user.TrialStartedAt).Hours() / 24

        switch {
        case daysSinceStart >= 10:
            sendIfNotSent(user, "trial_day10_followup")
        case daysSinceStart >= 7:
            sendIfNotSent(user, "trial_expired")
        case daysSinceStart >= 6:
            sendIfNotSent(user, "trial_day6_urgent")
        case daysSinceStart >= 5:
            sendIfNotSent(user, "trial_day5_reminder")
        case daysSinceStart >= 3:
            sendIfNotSent(user, "trial_day3_engagement")
        case daysSinceStart >= 0:
            sendIfNotSent(user, "trial_started")
        }
    }
}
```

---

## Phase 4: Frontend Rebuild (Priority: MEDIUM)

### 4.1 Technology Stack

```
Framework:     Next.js 15 (App Router)
UI Library:    shadcn/ui
Animations:    Framer Motion
Styling:       Tailwind CSS
Auth:          Better Auth
State:         Zustand or TanStack Query
Testing:       Playwright
```

### 4.2 Authentication Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     AUTHENTICATION FLOW                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐         ┌──────────────┐                      │
│  │   Next.js    │◀───────▶│  Better Auth │                      │
│  │   Frontend   │         │   (Server)   │                      │
│  └──────┬───────┘         └──────┬───────┘                      │
│         │                        │                               │
│         │ JWT Token              │ Issues JWT                    │
│         │                        │                               │
│         ▼                        ▼                               │
│  ┌──────────────┐         ┌──────────────┐                      │
│  │   WasBot     │         │  PostgreSQL  │                      │
│  │   Go API     │         │  (Sessions)  │                      │
│  └──────────────┘         └──────────────┘                      │
│         │                                                        │
│         │ Verify JWT signature                                   │
│         │ using shared secret                                    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Better Auth Setup (Next.js):**
```typescript
// lib/auth.ts
import { betterAuth } from "better-auth";
import { Pool } from "pg";

export const auth = betterAuth({
    database: new Pool({
        connectionString: process.env.DATABASE_URL
    }),
    emailAndPassword: {
        enabled: true
    },
    socialProviders: {
        google: {
            clientId: process.env.GOOGLE_CLIENT_ID!,
            clientSecret: process.env.GOOGLE_CLIENT_SECRET!
        }
    },
    session: {
        strategy: "jwt", // Stateless for Go backend verification
        maxAge: 7 * 24 * 60 * 60 // 7 days
    }
});
```

**Go JWT Verification:**
```go
// WasBot verifies tokens using shared secret
func VerifyBetterAuthJWT(tokenString string, secret []byte) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
        return secret, nil
    })
    // ...
}
```

### 4.3 Page Structure

```
/                       → Landing page (marketing)
/login                  → Login page
/signup                 → Signup page (captures ref_id)
/dashboard              → Main dashboard
/dashboard/devices      → Manage WhatsApp devices
/dashboard/messages     → Message history
/dashboard/settings     → Account settings
/dashboard/billing      → Subscription & billing
/pricing                → Pricing page
```

### 4.4 Key Components

- [ ] `components/ui/*` - shadcn components
- [ ] `components/landing/*` - Marketing page components
- [ ] `components/dashboard/*` - Dashboard components
- [ ] `components/auth/*` - Auth forms
- [ ] `lib/api.ts` - API client for WasBot backend
- [ ] `lib/auth.ts` - Better Auth configuration

---

## Phase 5: Infrastructure Consolidation (Priority: LOW)

### 5.1 k3s Cluster Setup

```yaml
# Namespace per service
namespaces:
  - wasbot
  - affiliate-system
  - webhook-router
  - email-service
  - shared (PostgreSQL, Redis)
```

### 5.2 PostgreSQL with PgBouncer

```yaml
# Deploy single PostgreSQL instance with PgBouncer
apiVersion: v1
kind: ConfigMap
metadata:
  name: pgbouncer-config
  namespace: shared
data:
  pgbouncer.ini: |
    [databases]
    wasbot = host=postgresql port=5432 dbname=wasbot
    affiliate = host=postgresql port=5432 dbname=affiliate
    webhook = host=postgresql port=5432 dbname=webhook
    email = host=postgresql port=5432 dbname=email

    [pgbouncer]
    pool_mode = transaction
    max_client_conn = 1000
    default_pool_size = 20
```

### 5.3 Redis (No Pooling Needed)

Redis handles connection multiplexing internally. go-redis client manages a small connection pool automatically. Default settings are sufficient:

```go
// go-redis default pool settings (usually fine)
rdb := redis.NewClient(&redis.Options{
    Addr:         "redis:6379",
    PoolSize:     10 * runtime.NumCPU(),
    MinIdleConns: 5,
})
```

### 5.4 Internal Service Communication

```yaml
# Services communicate via k8s internal DNS
# Example: http://webhook-router.webhook-router.svc.cluster.local:8080

# Tailscale for external secure access
# All services expose Swagger only on Tailscale network (100.64.0.0/10)
```

---

## Phase 6: Paystack Discount Codes (Priority: LOW)

### 6.1 Implementation Options

**Option A: Paystack Coupons (Recommended)**
- Create coupons via Paystack Dashboard or API
- Apply at checkout using Paystack's coupon field
- Paystack handles validation and discount calculation

**Option B: Custom Discount System**
- Create discount_codes table in WasBot
- Validate before redirecting to Paystack
- Adjust plan/amount sent to Paystack
- More control but more complexity

### 6.2 Paystack Coupon API

```bash
# Create coupon
curl -X POST https://api.paystack.co/coupon \
  -H "Authorization: Bearer sk_xxx" \
  -d "name=LAUNCH50" \
  -d "percent_off=50" \
  -d "duration=once"
```

---

## Task Checklist

### Immediate (This Week)
- [ ] Create webhook endpoint in WasBot k8s (`/api/v1/webhooks/payment`)
- [ ] Update existing WasBot product in webhook-router DB to point to k8s endpoint
- [ ] Test webhook flow end-to-end

### Short Term (Next 2 Weeks)
- [ ] Add ref_id column to WasBot users table
- [ ] Implement referral capture in frontend
- [ ] Create affiliate client service
- [ ] Create/verify commission endpoint in affiliate-system
- [ ] Test affiliate flow end-to-end

### Medium Term
- [ ] Create trial email templates
- [ ] Implement trial email scheduler
- [ ] Set up Next.js frontend project
- [ ] Implement Better Auth
- [ ] Build core dashboard pages

### Long Term
- [ ] Migrate all services to k3s
- [ ] Set up PgBouncer
- [ ] Implement discount codes
- [ ] Performance optimization

---

## Environment Variables Reference

### webhook-router-go
```env
# Swagger access control
SWAGGER_ALLOWED_IPS=100.64.0.0/10

# Note: Webhook-router is purely a routing layer.
# Product configurations are stored in the DATABASE, not env vars.
# Products are registered via API: POST /api/products
# Products handle ALL their own business logic (emails, commissions, etc.)
```

### whatsmeow-test (WasBot)
```env
# Webhook verification (must match the auth_config.secret in product registration)
WEBHOOK_HMAC_SECRET=xxx

# Affiliate system integration (for pushing commissions)
AFFILIATE_SERVICE_URL=http://affiliate-system:8080
AFFILIATE_API_KEY=xxx

# Email service
EMAIL_SERVICE_URL=http://email-service:8080
EMAIL_SERVICE_API_KEY=xxx

# Swagger access control
SWAGGER_ALLOWED_IPS=100.64.0.0/10

# Auth (shared with Next.js frontend for JWT verification)
BETTER_AUTH_SECRET=xxx
```

### affiliate-system-go
```env
# For validating commission push requests from products (WasBot, etc.)
PRODUCT_API_KEY=xxx

# Swagger access control
SWAGGER_ALLOWED_IPS=100.64.0.0/10
```

### email-service-go
```env
SWAGGER_ALLOWED_IPS=100.64.0.0/10
```

---

## Notes

- All timestamps in UTC
- All services use structured logging (zerolog)
- API versioning: /api/v1/
- Health checks: /health
- Swagger UI: /swagger (IP restricted)
