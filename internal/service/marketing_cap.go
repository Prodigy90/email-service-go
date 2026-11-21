package service

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// MarketingCapMode is how the daily marketing cap behaves
// (root repo .scratch/onboarding-tight/06-daily-cap.md).
type MarketingCapMode string

const (
	// CapOff: no lock, no count. Today's behaviour; the default, because
	// this service deploys on merge to main.
	CapOff MarketingCapMode = "off"
	// CapShadow: count under the lock; when the send would be refused, send
	// it anyway, log "would cap" and stamp metadata daily_cap=would_cap.
	CapShadow MarketingCapMode = "shadow"
	// CapEnforce: refuse with ErrMarketingCapped (HTTP 409).
	CapEnforce MarketingCapMode = "enforce"
)

// ParseMarketingCapMode reads MARKETING_CAP_MODE. Anything unrecognised is
// off, so a typo can never switch refusals on.
func ParseMarketingCapMode(s string) MarketingCapMode {
	switch MarketingCapMode(strings.ToLower(strings.TrimSpace(s))) {
	case CapShadow:
		return CapShadow
	case CapEnforce:
		return CapEnforce
	default:
		return CapOff
	}
}

// MarketingCapConfig configures the cap.
type MarketingCapConfig struct {
	Mode MarketingCapMode
	// PerDay is the most marketing emails one address gets per Lagos day.
	PerDay int
}

// ErrMarketingCapped is the refusal: the address already had its marketing
// email(s) today. Nothing was stored; retry from RetryAfter.
type ErrMarketingCapped struct {
	To             string
	MarketingToday int
	Cap            int
	RetryAfter     time.Time
}

func (e *ErrMarketingCapped) Error() string {
	return fmt.Sprintf("marketing_daily_cap: %s already had %d marketing email(s) today (Lagos), cap %d", e.To, e.MarketingToday, e.Cap)
}

// ErrMarketingCapUnavailable: the cap could not be checked (DB error), so an
// enforced marketing send holds rather than going out unchecked.
var ErrMarketingCapUnavailable = errors.New("marketing_cap_unavailable: could not read today's sends for this address")

// ErrInvalidRequest marks a request the caller must fix (HTTP 400).
var ErrInvalidRequest = errors.New("invalid request")

// NextDayStart is the Lagos midnight after now.
func NextDayStart(now time.Time) time.Time {
	return DayStart(now).AddDate(0, 0, 1)
}
