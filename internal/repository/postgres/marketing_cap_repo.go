package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/prodigy90/email-service-go/internal/domain"
)

// MarketingSQL is the predicate for "this emails row is a marketing send",
// with servicesParam the placeholder of a text[] of service templates. It is
// the one definition behind both the daily cap and /recipients/state's
// marketing counts, so the two can never disagree.
//
// A templated send is marketing unless its template is service. An
// untemplated (raw body) send is marketing only when it comes from the
// campaign server (source_service 'campaign' or a metadata campaign tag);
// other raw sends are the ads digest, support replies and ops notes. Keep
// in step with service.IsMarketingSend.
func MarketingSQL(servicesParam string) string {
	return `(NOT (COALESCE(template, '') = ANY(` + servicesParam + `))
	  AND (COALESCE(template, '') <> '' OR source_service = 'campaign' OR metadata->>'campaign' IS NOT NULL))`
}

// MarketingCap bounds one capped insert.
type MarketingCap struct {
	// DayStart is the start of the Lagos calendar day being counted.
	DayStart time.Time
	// Allow: the send is refused when the address already has this many
	// marketing sends since DayStart.
	Allow int
	// ServiceTemplates never count (see MarketingSQL).
	ServiceTemplates []string
	// Shadow: count and report, never refuse.
	Shadow bool
}

// CapResult says what the capped insert saw.
type CapResult struct {
	// MarketingToday is the address's marketing sends since DayStart,
	// before this one.
	MarketingToday int
	// Existed: the idempotency ID was already taken; the existing row is
	// returned and nothing was counted or inserted.
	Existed bool
	// WouldCap: shadow mode, and this send would have been refused.
	WouldCap bool
}

// ErrCapReached is returned (with the count in CapResult) when an enforced
// cap refuses the send. Nothing is written.
var ErrCapReached = errors.New("marketing daily cap reached")

// CreateUnderMarketingCap inserts a marketing email unless its address has
// already had cap.Allow marketing sends since cap.DayStart.
//
// The check and the insert are one transaction behind a per-address
// advisory lock, so two concurrent sends to one address serialize and the
// second sees the first's row. A transaction-scoped lock (not a session one)
// so it is released at commit or rollback and is safe behind PgBouncer's
// transaction pooling. The idempotency ID is looked up inside the lock
// first: a retry of an accepted send gets its row back and is never refused
// by its own earlier copy.
//
// The address is matched case-insensitively (lower(to_address), indexed by
// migration 005), and the lock key is lower-cased, so Foo@x and foo@x share
// one count and one lock.
func (r *EmailRepository) CreateUnderMarketingCap(ctx context.Context, email *domain.Email, cap MarketingCap) (*domain.Email, CapResult, error) {
	var res CapResult
	lower := strings.ToLower(strings.TrimSpace(email.To))
	services := cap.ServiceTemplates
	if services == nil {
		services = []string{}
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, res, fmt.Errorf("cap: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "mktcap:"+lower); err != nil {
		return nil, res, fmt.Errorf("cap: lock: %w", err)
	}

	if email.IdempotencyID != "" {
		var row emailRow
		err := tx.GetContext(ctx, &row, `SELECT id, to_address, from_address, subject, body, html_body, template, template_data, status, error_message, retry_count, max_retries, idempotency_id, source_service, metadata, headers, resend_email_id, sent_at, created_at, updated_at FROM emails WHERE idempotency_id = $1`, email.IdempotencyID)
		switch {
		case err == nil:
			res.Existed = true
			if err := tx.Commit(); err != nil {
				return nil, res, fmt.Errorf("cap: commit: %w", err)
			}
			return row.toDomain(), res, nil
		case !errors.Is(err, sql.ErrNoRows):
			return nil, res, fmt.Errorf("cap: idempotency lookup: %w", err)
		}
	}

	if err := tx.GetContext(ctx, &res.MarketingToday, `
		SELECT count(*) FROM emails
		WHERE lower(to_address) = $1 AND created_at >= $2 AND status <> 'failed'
		  AND `+MarketingSQL("$3"), lower, cap.DayStart, services); err != nil {
		return nil, res, fmt.Errorf("cap: count: %w", err)
	}

	if res.MarketingToday >= cap.Allow {
		if !cap.Shadow {
			return nil, res, ErrCapReached
		}
		res.WouldCap = true
		if email.Metadata == nil {
			email.Metadata = map[string]interface{}{}
		}
		email.Metadata["daily_cap"] = "would_cap"
	}

	metadata, _ := json.Marshal(email.Metadata)
	templateData, _ := json.Marshal(email.TemplateData)
	headers, _ := json.Marshal(email.Headers)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO emails (
			id, to_address, from_address, subject, body, html_body,
			template, template_data, status, max_retries, idempotency_id,
			source_service, metadata, headers, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (idempotency_id) DO NOTHING`,
		email.ID, email.To, email.From, email.Subject, email.Body, email.HTMLBody,
		email.Template, templateData, email.Status, email.MaxRetries, nullString(email.IdempotencyID),
		nullString(email.SourceService), metadata, headers, email.CreatedAt, email.UpdatedAt)
	if err != nil {
		return nil, res, fmt.Errorf("cap: insert: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		// The idempotency ID was taken between the lookup and the insert by
		// a send to a different address (same ID, other lock). Return it.
		var row emailRow
		if err := tx.GetContext(ctx, &row, `SELECT id, to_address, from_address, subject, body, html_body, template, template_data, status, error_message, retry_count, max_retries, idempotency_id, source_service, metadata, headers, resend_email_id, sent_at, created_at, updated_at FROM emails WHERE idempotency_id = $1`, email.IdempotencyID); err != nil {
			return nil, res, fmt.Errorf("cap: idempotency conflict: %w", err)
		}
		res.Existed, res.WouldCap = true, false
		if err := tx.Commit(); err != nil {
			return nil, res, fmt.Errorf("cap: commit: %w", err)
		}
		return row.toDomain(), res, nil
	}
	if err := tx.Commit(); err != nil {
		return nil, res, fmt.Errorf("cap: commit: %w", err)
	}
	return email, res, nil
}
