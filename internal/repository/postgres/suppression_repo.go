package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Suppression represents a suppressed email address.
type Suppression struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Email       string    `db:"email" json:"email"`
	Reason      string    `db:"reason" json:"reason"`
	Source      string    `db:"source" json:"source"`
	CampaignTag *string   `db:"campaign_tag" json:"campaign_tag,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

// SuppressionRepository handles suppression list persistence.
type SuppressionRepository struct {
	db *sqlx.DB
}

// NewSuppressionRepository creates a new suppression repository.
func NewSuppressionRepository(db *sqlx.DB) *SuppressionRepository {
	return &SuppressionRepository{db: db}
}

// IsSuppressed checks if an email is in the suppression list.
func (r *SuppressionRepository) IsSuppressed(ctx context.Context, email string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM suppressions WHERE email = $1`
	if err := r.db.GetContext(ctx, &count, query, email); err != nil {
		return false, err
	}
	return count > 0, nil
}

// FilterSuppressed takes a list of emails and returns only those NOT in the suppression list.
func (r *SuppressionRepository) FilterSuppressed(ctx context.Context, emails []string) ([]string, error) {
	if len(emails) == 0 {
		return emails, nil
	}

	query, args, err := sqlx.In(`SELECT email FROM suppressions WHERE email IN (?)`, emails)
	if err != nil {
		return nil, err
	}
	query = r.db.Rebind(query)

	var suppressed []string
	if err := r.db.SelectContext(ctx, &suppressed, query, args...); err != nil {
		return nil, err
	}

	suppressedSet := make(map[string]bool, len(suppressed))
	for _, e := range suppressed {
		suppressedSet[e] = true
	}

	filtered := make([]string, 0, len(emails)-len(suppressed))
	for _, e := range emails {
		if !suppressedSet[e] {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// CheckSuppressed takes a list of emails and returns those that ARE in the suppression list.
func (r *SuppressionRepository) CheckSuppressed(ctx context.Context, emails []string) ([]string, error) {
	if len(emails) == 0 {
		return nil, nil
	}

	query, args, err := sqlx.In(`SELECT email FROM suppressions WHERE email IN (?)`, emails)
	if err != nil {
		return nil, err
	}
	query = r.db.Rebind(query)

	var suppressed []string
	if err := r.db.SelectContext(ctx, &suppressed, query, args...); err != nil {
		return nil, err
	}
	return suppressed, nil
}

// Add adds an email to the suppression list. If already suppressed, this is a no-op.
func (r *SuppressionRepository) Add(ctx context.Context, email, reason, source string, campaignTag *string) error {
	query := `
		INSERT INTO suppressions (email, reason, source, campaign_tag)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email) DO NOTHING`
	_, err := r.db.ExecContext(ctx, query, email, reason, source, campaignTag)
	return err
}

// Remove removes an email from the suppression list.
func (r *SuppressionRepository) Remove(ctx context.Context, email string) error {
	query := `DELETE FROM suppressions WHERE email = $1`
	_, err := r.db.ExecContext(ctx, query, email)
	return err
}

// List returns all suppressed emails with optional filtering by reason.
func (r *SuppressionRepository) List(ctx context.Context, reason string, limit, offset int) ([]Suppression, int, error) {
	var suppressions []Suppression
	var total int

	if reason != "" {
		countQuery := `SELECT COUNT(*) FROM suppressions WHERE reason = $1`
		if err := r.db.GetContext(ctx, &total, countQuery, reason); err != nil {
			return nil, 0, err
		}
		query := `SELECT * FROM suppressions WHERE reason = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		if err := r.db.SelectContext(ctx, &suppressions, query, reason, limit, offset); err != nil {
			return nil, 0, err
		}
	} else {
		countQuery := `SELECT COUNT(*) FROM suppressions`
		if err := r.db.GetContext(ctx, &total, countQuery); err != nil {
			return nil, 0, err
		}
		query := `SELECT * FROM suppressions ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		if err := r.db.SelectContext(ctx, &suppressions, query, limit, offset); err != nil {
			return nil, 0, err
		}
	}

	return suppressions, total, nil
}
