package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prodigy90/email-service-go/internal/domain"
	"github.com/prodigy90/email-service-go/internal/repository/postgres"
)

func TestParseMarketingCapMode(t *testing.T) {
	for in, want := range map[string]MarketingCapMode{
		"": CapOff, "off": CapOff, "OFF": CapOff, "enforced": CapOff, "nonsense": CapOff,
		"shadow": CapShadow, " Shadow ": CapShadow, "enforce": CapEnforce, "ENFORCE": CapEnforce,
	} {
		if got := ParseMarketingCapMode(in); got != want {
			t.Errorf("ParseMarketingCapMode(%q) = %s, want %s", in, got, want)
		}
	}
}

// The founder's service list (11-21) plus the affiliate account mail.
func TestIsMarketingSend(t *testing.T) {
	service := []string{
		"password_reset", "password_reset_no_account", "email_verification", "payment_success",
		"payment_success_onetime", "payment_failed", "payment_failed_attempt2", "payment_failed_final",
		"subscription_expiring_3d", "subscription_expiring_1d", "subscription_reminder_3d", "subscription_reminder_1d",
		"onboarding_day0", "checkout_abandoned", "vsl_walkthrough_link", "refund_processed",
		"payout_approved", "payout_processed", "commission_earned", "commission_refunded", "affiliate_signup",
	}
	for _, tmpl := range service {
		if IsMarketingSend(tmpl, "wasbot", nil) {
			t.Errorf("%s classified as marketing", tmpl)
		}
	}
	marketing := []string{
		"campaign_update", "onboarding_day1", "onboarding_day7", "onboarding_unlinked_day3", "post_trial_day9",
		"post_trial_unlinked", "onboarding_paid_day1", "reengage_day0", "winback_day12", "legacy_winback_a1",
		"winback_ad_leads_d0", "stats_pages_launch",
	}
	for _, tmpl := range marketing {
		if !IsMarketingSend(tmpl, "wasbot", nil) {
			t.Errorf("%s classified as service", tmpl)
		}
	}
	if IsMarketingSend("", "support", nil) || IsMarketingSend("", "wasbot-backend", nil) || IsMarketingSend("", "ops", map[string]interface{}{"campaign": nil}) {
		t.Error("untemplated non-campaign send classified as marketing")
	}
	if !IsMarketingSend("", "campaign", nil) || !IsMarketingSend("", "x", map[string]interface{}{"campaign": "c"}) {
		t.Error("untemplated campaign send classified as service")
	}
}

type failingCapStore struct{}

func (failingCapStore) CreateUnderMarketingCap(context.Context, *domain.Email, postgres.MarketingCap) (*domain.Email, postgres.CapResult, error) {
	return nil, postgres.CapResult{}, errors.New("connection refused")
}

// When today's sends can't be read, an enforced marketing send holds (503),
// never goes out unchecked.
func TestMarketingCap_UnreadableHolds(t *testing.T) {
	svc := &EmailService{now: time.Now, capCfg: MarketingCapConfig{Mode: CapEnforce, PerDay: 1}, capRepo: failingCapStore{}}
	_, _, handled, err := svc.saveUnderCap(context.Background(), &domain.Email{To: "a@b.co", Template: "campaign_update"}, nil, time.Now())
	if !handled || !errors.Is(err, ErrMarketingCapUnavailable) {
		t.Fatalf("enforce + unreadable: handled=%v err=%v, want held with ErrMarketingCapUnavailable", handled, err)
	}
	svc.capCfg.Mode = CapShadow
	if _, _, handled, err := svc.saveUnderCap(context.Background(), &domain.Email{To: "a@b.co", Template: "campaign_update"}, nil, time.Now()); handled || err != nil {
		t.Fatalf("shadow + unreadable: handled=%v err=%v, want fall through to a normal send", handled, err)
	}
	// Service mail never touches the cap store.
	svc.capCfg.Mode = CapEnforce
	if _, _, handled, err := svc.saveUnderCap(context.Background(), &domain.Email{To: "a@b.co", Template: "password_reset"}, nil, time.Now()); handled || err != nil {
		t.Fatalf("service + unreadable: handled=%v err=%v, want untouched", handled, err)
	}
}

func TestNextDayStart_Lagos(t *testing.T) {
	n := time.Date(2026, 11, 21, 22, 59, 0, 0, time.UTC) // 23:59 Lagos
	if got, want := NextDayStart(n), time.Date(2026, 11, 21, 23, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("NextDayStart = %s, want %s", got.UTC(), want)
	}
}
