package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/prodigy90/email-service-go/internal/repository/postgres"
)

// MaxRecipientStateBatch is the most addresses one state call takes.
const MaxRecipientStateBatch = 500

// lagos is West Africa Time. Africa/Lagos has no daylight saving, so a fixed
// zone is exact and does not depend on tzdata being in the image.
var lagos = time.FixedZone("Africa/Lagos", 3600)

// serviceTemplates are sends that never count toward a marketing cap: the
// transactional set (no unsubscribe header) plus the other mail the email map
// keeps outside the system (root repo .scratch/onboarding-tight/03-email-map.md,
// "Kept outside the system"): the day-0 welcome, the abandoned-checkout
// reminder, the second payment-failed notice, and the walkthrough link a
// lead asked for minutes earlier.
func serviceTemplates() []string {
	out := make([]string, 0, len(transactionalTemplates)+4)
	for t := range transactionalTemplates {
		out = append(out, t)
	}
	out = append(out, "onboarding_day0", "checkout_abandoned", "payment_failed_attempt2", "vsl_walkthrough_link")
	slices.Sort(out)
	return out
}

// IsServiceTemplate reports whether a template is service mail (cap-exempt).
func IsServiceTemplate(t string) bool {
	return slices.Contains(serviceTemplates(), t)
}

// RecipientStateReader is the read the service needs.
type RecipientStateReader interface {
	Get(ctx context.Context, q postgres.RecipientStateQuery) (map[string]*postgres.RecipientState, error)
}

// RecipientStateService answers "what has this address had from us?".
type RecipientStateService struct {
	repo RecipientStateReader
	now  func() time.Time
}

// NewRecipientStateService creates the service.
func NewRecipientStateService(repo RecipientStateReader) *RecipientStateService {
	return &RecipientStateService{repo: repo, now: time.Now}
}

// RecipientStateResponse is the body of POST /api/v1/recipients/state.
type RecipientStateResponse struct {
	// DayStart is the start of the Lagos calendar day "today" counts from.
	DayStart   time.Time        `json:"day_start"`
	Timezone   string           `json:"timezone"`
	Recipients []RecipientState `json:"recipients"`
}

// RecipientState is one address. Every requested address is returned, in
// request order (duplicates once); an address we never mailed has zero
// counts and null sends.
type RecipientState struct {
	Email string `json:"email"`
	// Suppressed: on the suppression list, so every send to it is dropped.
	Suppressed  bool                           `json:"suppressed"`
	Suppression *postgres.RecipientSuppression `json:"suppression"`
	// Unsubscribed, Bounced and Complained come from the suppression reason.
	// The list keeps one row per address (the first reason), so at most one
	// is true. Unsubscribes are global: the unsubscribe page records no list.
	Unsubscribed bool `json:"unsubscribed"`
	Bounced      bool `json:"bounced"`
	Complained   bool `json:"complained"`
	// EverBounced: any send to it was marked bounced by Resend, whether or not
	// the address is still suppressed (an admin can lift a suppression).
	EverBounced bool `json:"ever_bounced"`
	// EverComplained: any send to it drew a spam complaint.
	EverComplained bool `json:"ever_complained"`
	// SendsToday counts sends accepted since DayStart (status not failed);
	// MarketingToday leaves out service mail (see serviceTemplates).
	SendsToday     int `json:"sends_today"`
	MarketingToday int `json:"marketing_today"`
	// Sends24h counts accepted sends in the rolling last 24 hours.
	Sends24h          int                     `json:"sends_24h"`
	LastSend          *postgres.RecipientSend `json:"last_send"`
	LastMarketingSend *postgres.RecipientSend `json:"last_marketing_send"`
}

// NormalizeRecipient is how an address is matched: trimmed, lower-cased.
func NormalizeRecipient(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

// Get returns state for up to MaxRecipientStateBatch addresses.
func (s *RecipientStateService) Get(ctx context.Context, emails []string) (*RecipientStateResponse, error) {
	var order []string
	seen := map[string]bool{}
	for _, e := range emails {
		e = NormalizeRecipient(e)
		if e == "" || seen[e] {
			continue
		}
		seen[e] = true
		order = append(order, e)
	}
	if len(order) > MaxRecipientStateBatch {
		return nil, fmt.Errorf("at most %d addresses per call, got %d", MaxRecipientStateBatch, len(order))
	}
	now := s.now()
	day := DayStart(now)
	resp := &RecipientStateResponse{DayStart: day, Timezone: lagos.String(), Recipients: make([]RecipientState, 0, len(order))}
	if len(order) == 0 {
		return resp, nil
	}
	got, err := s.repo.Get(ctx, postgres.RecipientStateQuery{
		Emails:           order,
		DayStart:         day,
		Since24h:         now.Add(-24 * time.Hour),
		ServiceTemplates: serviceTemplates(),
	})
	if err != nil {
		return nil, err
	}
	for _, e := range order {
		r := RecipientState{Email: e}
		if st, ok := got[e]; ok {
			r.Suppression = st.Suppression
			if st.Suppression != nil {
				r.Suppressed = true
				r.Unsubscribed = st.Suppression.Reason == "unsubscribe"
				r.Bounced = st.Suppression.Reason == "bounce"
				r.Complained = st.Suppression.Reason == "complaint"
			}
			r.EverBounced = st.EverBounced
			r.EverComplained = st.EverComplained
			r.SendsToday, r.MarketingToday, r.Sends24h = st.SendsToday, st.MarketingToday, st.Sends24h
			r.LastSend, r.LastMarketingSend = st.LastSend, st.LastMarketingSend
		}
		resp.Recipients = append(resp.Recipients, r)
	}
	return resp, nil
}

// DayStart is midnight of now's Lagos calendar day.
func DayStart(now time.Time) time.Time {
	l := now.In(lagos)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, lagos)
}
