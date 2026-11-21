package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
)

// RecipientSend is one accepted send, as the emails table records it.
type RecipientSend struct {
	At            time.Time `json:"at"`
	Template      *string   `json:"template"`
	Campaign      *string   `json:"campaign"`
	Sequence      bool      `json:"sequence"`
	SourceService *string   `json:"source_service"`
	Status        string    `json:"status"`
	Service       bool      `json:"service"`
}

// RecipientSuppression is the suppressions row for an address.
type RecipientSuppression struct {
	Reason      string    `json:"reason"`
	Source      string    `json:"source"`
	CampaignTag *string   `json:"campaign_tag"`
	At          time.Time `json:"at"`
}

// RecipientState is what the email service knows about one address.
type RecipientState struct {
	Suppression       *RecipientSuppression
	SendsToday        int
	MarketingToday    int
	Sends24h          int
	EverBounced       bool
	EverComplained    bool
	LastSend          *RecipientSend
	LastMarketingSend *RecipientSend
}

// RecipientStateQuery bounds one batch read.
type RecipientStateQuery struct {
	// Emails are matched exactly against emails.to_address and
	// lower(suppressions.email); callers pass them lower-cased.
	Emails []string
	// DayStart is the start of "today" (the caller picks the zone).
	DayStart time.Time
	// Since24h is now minus 24 hours.
	Since24h time.Time
	// ServiceTemplates are templates that never count as marketing.
	ServiceTemplates []string
}

// RecipientStateRepository reads per-address send and suppression state.
type RecipientStateRepository struct {
	db *sqlx.DB
}

// NewRecipientStateRepository creates the repository.
func NewRecipientStateRepository(db *sqlx.DB) *RecipientStateRepository {
	return &RecipientStateRepository{db: db}
}

// countedStatuses are the statuses of a send that was accepted for delivery.
// 'failed' (gave up after retries, never left) does not count. Suppressed
// sends never get a row at all.
const countedStatuses = `status <> 'failed'`

// Get returns state for each address that has any; an address with no rows
// anywhere is absent from the map (the caller fills an empty state).
//
// emails.to_address is matched with "= ANY" so the btree on to_address is
// used: the table is large (message bodies), and a lower() on the column
// would scan it. A row stored in mixed case is therefore missed. The
// campaign server lower-cases its lists; the backend sends the address as
// user_auth holds it, and prod's pg_stats sample of both columns (11-21)
// held no mixed-case value. A lower(to_address) index would close the gap.
func (r *RecipientStateRepository) Get(ctx context.Context, q RecipientStateQuery) (map[string]*RecipientState, error) {
	out := map[string]*RecipientState{}
	if len(q.Emails) == 0 {
		return out, nil
	}
	get := func(e string) *RecipientState {
		s, ok := out[e]
		if !ok {
			s = &RecipientState{}
			out[e] = s
		}
		return s
	}
	emails := q.Emails
	service := q.ServiceTemplates
	if service == nil {
		service = []string{}
	}

	// Suppressions: one row per address (unique index; ON CONFLICT DO
	// NOTHING keeps the first reason). The table is tiny, so lower() on the
	// column is fine, and it catches bounce rows the webhook stored in the
	// case it was sent with.
	{
		rows, err := r.db.QueryContext(ctx, `
			SELECT DISTINCT ON (lower(email)) lower(email), reason, source, campaign_tag, created_at
			FROM suppressions
			WHERE lower(email) = ANY($1)
			ORDER BY lower(email), created_at`, emails)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var e string
			var s RecipientSuppression
			var tag sql.NullString
			if err := rows.Scan(&e, &s.Reason, &s.Source, &tag, &s.At); err != nil {
				rows.Close()
				return nil, err
			}
			if tag.Valid {
				s.CampaignTag = &tag.String
			}
			get(e).Suppression = &s
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// Counts and bounce history over every row of each address.
	{
		rows, err := r.db.QueryContext(ctx, `
			SELECT to_address,
			  count(*) FILTER (WHERE `+countedStatuses+` AND created_at >= $2),
			  count(*) FILTER (WHERE `+countedStatuses+` AND created_at >= $2
			                   AND NOT (COALESCE(template, '') = ANY($4))),
			  count(*) FILTER (WHERE `+countedStatuses+` AND created_at >= $3),
			  bool_or(status = 'bounced'),
			  bool_or(status = 'complaint')
			FROM emails
			WHERE to_address = ANY($1)
			GROUP BY to_address`, emails, q.DayStart, q.Since24h, service)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var e string
			var today, marketing, last24 int
			var bounced, complained bool
			if err := rows.Scan(&e, &today, &marketing, &last24, &bounced, &complained); err != nil {
				rows.Close()
				return nil, err
			}
			s := get(e)
			s.SendsToday, s.MarketingToday, s.Sends24h = today, marketing, last24
			s.EverBounced, s.EverComplained = bounced, complained
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// Last send, and last marketing send.
	for _, marketingOnly := range []bool{false, true} {
		rows, err := r.db.QueryContext(ctx, `
			SELECT DISTINCT ON (to_address) to_address, created_at, template,
			  metadata->>'campaign', COALESCE(metadata->>'sequence', '') = 'true',
			  source_service, status, COALESCE(template, '') = ANY($2)
			FROM emails
			WHERE to_address = ANY($1) AND `+countedStatuses+`
			  AND (NOT $3 OR NOT (COALESCE(template, '') = ANY($2)))
			ORDER BY to_address, created_at DESC`, emails, service, marketingOnly)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var e string
			var s RecipientSend
			var tmpl, camp, src sql.NullString
			if err := rows.Scan(&e, &s.At, &tmpl, &camp, &s.Sequence, &src, &s.Status, &s.Service); err != nil {
				rows.Close()
				return nil, err
			}
			s.Template, s.Campaign, s.SourceService = nullable(tmpl), nullable(camp), nullable(src)
			if marketingOnly {
				get(e).LastMarketingSend = &s
			} else {
				get(e).LastSend = &s
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func nullable(s sql.NullString) *string {
	if !s.Valid || s.String == "" {
		return nil
	}
	v := s.String
	return &v
}
