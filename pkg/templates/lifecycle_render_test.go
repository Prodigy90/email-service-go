package templates_test

import (
	"strings"
	"testing"

	"github.com/prodigy90/email-service-go/internal/domain"
)

// The trial lifecycle emails are personalized from live user activity, so the
// data map varies per reader: a never-linked user has no stats, a churned
// reader has no discount code, and a stats-query failure sends none of the
// numbers at all. Every one of those must still produce a sane email.
//
// These states mirror exactly what wasbot-backend's SequenceEmailCron
// (buildTemplateData) attaches. Two invariants it guarantees, which the states
// below encode:
//
//	Code + CodePct    — attached together, only while the trial window is open
//	the six stat keys — attached together, or not at all on a query failure
//
// If that grouping ever changes on the backend, these states must change with
// it, and the {{if}} guards in onboarding.yaml need re-checking.

// lifecycleTemplates are the templates the cron can send for a trial reader.
var lifecycleTemplates = []string{
	"onboarding_day0", "onboarding_day1", "onboarding_day2", "onboarding_day3",
	"onboarding_day4", "onboarding_day5", "onboarding_day6", "onboarding_day7",
	"onboarding_unlinked_day1", "onboarding_unlinked_day2", "onboarding_unlinked_day3",
	"onboarding_unlinked_day4", "onboarding_unlinked_day5", "onboarding_unlinked_day6",
	"onboarding_unlinked_day7",
	"post_trial_day8", "post_trial_day9", "post_trial_unlinked",
}

func baseLifecycleData() map[string]interface{} {
	return map[string]interface{}{
		"Name":             "Ada",
		"DashboardURL":     "https://www.wasbot.app/dashboard",
		"ConnectURL":       "https://www.wasbot.app/dashboard",
		"PricingURL":       "https://www.wasbot.app/pricing",
		"UpgradeURL":       "https://www.wasbot.app/billing",
		"BlogURL":          "https://www.wasbot.app/blog",
		"WhatsAppGroupURL": "https://chat.whatsapp.com/example",
		"SocialProofText":  "Join 400+ businesses already automating their WhatsApp with WASBOT",
		"StatusURL":        "https://www.wasbot.app/status",
		"ContactsURL":      "https://www.wasbot.app/contacts",
		"GroupsURL":        "https://www.wasbot.app/groups",
		"BroadcastsURL":    "https://www.wasbot.app/broadcasts",
		"AutoresponderURL": "https://www.wasbot.app/autoresponder",
		"SequencesURL":     "https://www.wasbot.app/sequences",
	}
}

func withKeys(base map[string]interface{}, extra map[string]interface{}) map[string]interface{} {
	for k, v := range extra {
		base[k] = v
	}
	return base
}

func lifecycleStates() []struct {
	name string
	data map[string]interface{}
} {
	trialOffer := map[string]interface{}{
		"Code": "ADA10", "CodePct": 10, "TrialEnd": "Friday, 21 August", "HoursLeft": 30,
	}
	fullStats := map[string]interface{}{
		"ContactsSaved": 3412, "StatusesPosted": 18, "StatusRecipients": 41200,
		"StatusViews": 9633, "GroupSends": 22, "SequencesActive": 3,
		"HasActivity": true, "TimeSavedHours": 27,
	}
	zeroStats := map[string]interface{}{
		"ContactsSaved": 0, "StatusesPosted": 0, "StatusRecipients": 0,
		"StatusViews": 0, "GroupSends": 0, "SequencesActive": 0,
	}

	return []struct {
		name string
		data map[string]interface{}
	}{
		// Signed up, never linked: code is live, every stat is zero.
		{"unlinked", withKeys(withKeys(baseLifecycleData(), trialOffer), zeroStats)},
		// Active trial user with real numbers.
		{"active", withKeys(withKeys(baseLifecycleData(), trialOffer), fullStats)},
		// Churned/post-trial: stats but NO code. Zero grace means every offer
		// paragraph must disappear rather than render an empty code.
		{"no-offer", withKeys(baseLifecycleData(), fullStats)},
		// Stats query failed: the cron attaches no numbers at all.
		{"stats-unavailable", withKeys(baseLifecycleData(), trialOffer)},
	}
}

func TestLifecycleTemplates_RenderCleanlyInEveryUserState(t *testing.T) {
	ts := newTestTemplateService(t)
	branding := domain.DefaultBranding()

	for _, name := range lifecycleTemplates {
		for _, st := range lifecycleStates() {
			subject, body, html, err := ts.RenderWithBranding(name, st.data, branding)
			if err != nil {
				t.Errorf("%s [%s]: render failed: %v", name, st.name, err)
				continue
			}
			for part, out := range map[string]string{"subject": subject, "body": body, "html": html} {
				// A missing field renders as "<no value>" — the exact way a
				// personalized email embarrasses us in front of a customer.
				if strings.Contains(out, "<no value>") {
					t.Errorf("%s [%s] %s: leaked <no value>", name, st.name, part)
				}
			}
			// Every email must carry a working CTA, and never a bare scheme
			// left behind by an empty URL.
			if !strings.Contains(body, "https://") {
				t.Errorf("%s [%s]: body has no CTA link", name, st.name)
			}
			for _, line := range strings.Split(body, "\n") {
				if strings.HasSuffix(strings.TrimSpace(line), "https://") {
					t.Errorf("%s [%s]: truncated URL: %q", name, st.name, line)
				}
			}
		}
	}
}

// The winback is the one template with its own field contract: the cron
// attaches DiscountPercent/DiscountCode only when it sends this template.
func TestWinbackTemplate_RendersWithItsOwnFields(t *testing.T) {
	ts := newTestTemplateService(t)
	data := withKeys(baseLifecycleData(), map[string]interface{}{
		"DiscountPercent": 20,
		"DiscountCode":    "WINBACK20",
	})

	subject, body, html, err := ts.RenderWithBranding("post_trial_day10", data, domain.DefaultBranding())
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	for part, out := range map[string]string{"subject": subject, "body": body, "html": html} {
		if strings.Contains(out, "<no value>") {
			t.Errorf("%s: leaked <no value>", part)
		}
	}
	if !strings.Contains(body, "WINBACK20") {
		t.Error("winback body does not carry the code")
	}
	// The forever price lock is earned during the trial and never resurrected
	// by a winback. Promising it here would make the trial deadline a lie.
	if strings.Contains(strings.ToLower(body), "locked forever") {
		t.Error("winback copy promises the price lock, which only the trial earns")
	}
}

// Pre-payment lifecycle copy must not quote absolute prices: the backend
// cannot know a trial user's currency (geo is resolved client-side, and no
// currency is stored until a subscription exists), so a naira figure would
// misprice every international reader.
func TestLifecycleTemplates_QuoteNoAbsolutePrices(t *testing.T) {
	ts := newTestTemplateService(t)
	branding := domain.DefaultBranding()
	banned := []string{"₦", "NGN", "$9", "$25", "$59"}

	for _, name := range append(lifecycleTemplates, "post_trial_day10") {
		data := withKeys(baseLifecycleData(), map[string]interface{}{
			"Code": "ADA10", "CodePct": 10, "TrialEnd": "Friday, 21 August", "HoursLeft": 30,
			"DiscountPercent": 20, "DiscountCode": "WINBACK20",
		})
		_, body, html, err := ts.RenderWithBranding(name, data, branding)
		if err != nil {
			t.Fatalf("%s: render failed: %v", name, err)
		}
		for _, token := range banned {
			if strings.Contains(body, token) || strings.Contains(html, token) {
				t.Errorf("%s: quotes an absolute price (%q); percentages and a pricing link only", name, token)
			}
		}
	}
}

// Plaintext and HTML must carry exactly one sign-off each. The two branches of
// a template are written and edited separately, so they drift: campaign_update
// shipped with two stacked sign-offs in HTML (guarded in plaintext, not in
// HTML) and the whole lifecycle set shipped with a sign-off in plaintext and
// none in HTML, which is what almost every reader sees.
func TestLifecycleTemplates_HaveExactlyOneSignoffPerBranch(t *testing.T) {
	ts := newTestTemplateService(t)
	branding := domain.DefaultBranding()

	for _, name := range append(lifecycleTemplates, "post_trial_day10") {
		for _, st := range lifecycleStates() {
			_, body, html, err := ts.RenderWithBranding(name, st.data, branding)
			if err != nil {
				t.Fatalf("%s [%s]: render failed: %v", name, st.name, err)
			}
			if got := strings.Count(body, "The WASBOT Team"); got != 1 {
				t.Errorf("%s [%s]: plaintext has %d sign-offs, want 1", name, st.name, got)
			}
			// The chassis footer names the company but never signs off, so any
			// count other than 1 here is the template's own doing.
			if got := strings.Count(html, "The WASBOT Team"); got != 1 {
				t.Errorf("%s [%s]: HTML has %d sign-offs, want 1", name, st.name, got)
			}
		}
	}
}
