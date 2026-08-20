package service

import (
	"strings"
	"testing"
)

// The A1 winback renders once per cohort. Each cohort's fill values come from
// docs/templates/legacy-winback-emails-2026-08.md and were verified against the
// prod legacy_users table on 2026-08-19. What these cases pin:
//   - no template field leaks (`<no value>`) in subject, text or HTML
//   - the cohort's own numbers appear where the copy quotes them
//   - the frozen pricing facts survive rendering (8,000 / 5,600 / 30%)
//   - no em-dash anywhere, per the house email rules
func TestLegacyWinbackA1RendersPerCohort(t *testing.T) {
	ts := newTestTemplateService(t)

	cohorts := []struct {
		name string
		data map[string]interface{}
	}{
		{"ultimate-organizer", map[string]interface{}{
			"OldPlan": "Ultimate Organizer", "OldAmount": "₦20,000", "OldPeriod": "year", "FreeDays": "30",
		}},
		{"efficiency-explorer", map[string]interface{}{
			"OldPlan": "Efficiency Explorer", "OldAmount": "₦8,000", "OldPeriod": "quarter", "FreeDays": "14",
		}},
		{"essential-saver", map[string]interface{}{
			"OldPlan": "Essential Saver", "OldAmount": "₦3,000", "OldPeriod": "month", "FreeDays": "7",
		}},
		{"telegram-accept-bot", map[string]interface{}{
			"OldPlan": "Telegram Accept Bot", "OldAmount": "₦30,000", "OldPeriod": "year", "FreeDays": "7",
		}},
	}

	for _, c := range cohorts {
		t.Run(c.name, func(t *testing.T) {
			data := map[string]interface{}{"UnsubscribeURL": "https://wasbot.app/u/x"}
			for k, v := range c.data {
				data[k] = v
			}
			subject, body, htmlBody, err := ts.Render("legacy_winback_a1", data)
			if err != nil {
				t.Fatalf("render failed: %v", err)
			}
			for surface, s := range map[string]string{"subject": subject, "body": body, "html": htmlBody} {
				if strings.Contains(s, "<no value>") {
					t.Errorf("%s leaks <no value>", surface)
				}
				if strings.Contains(s, "—") {
					t.Errorf("%s contains an em-dash", surface)
				}
			}
			for _, want := range []string{
				c.data["OldPlan"].(string), c.data["OldAmount"].(string),
				c.data["FreeDays"].(string) + " ", // "30 days", "7 free days"
				"₦8,000", "₦5,600", "30%",
				"https://www.wasbot.app/signup",
				"August 31st",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("text body missing %q", want)
				}
				if !strings.Contains(htmlBody, want) {
					t.Errorf("html body missing %q", want)
				}
			}
			if !strings.Contains(subject, "We rebuilt WASBOT") {
				t.Errorf("unexpected subject %q", subject)
			}
			// The casing rule: the raw plan strings in legacy_users carry a
			// "WASBOT - " prefix. The CSV must strip it; the template must never
			// end up with the double form.
			if strings.Contains(body, "WASBOT - ") {
				t.Errorf("body contains unsanitised plan prefix")
			}
		})
	}
}

// A2-A4 (the founder's storified drip) only consume FreeDays. Each case pins the
// facts its copy quotes plus the three fixes approved 2026-08-20: the discount-lock
// clause in A2, the softened multi-account bullet, and the "TVs" typo fix in A3.
func TestLegacyWinbackA2ToA4Render(t *testing.T) {
	ts := newTestTemplateService(t)

	emails := []struct {
		template string
		want     []string
		absent   []string
	}{
		{"legacy_winback_a2",
			[]string{
				"That is why I built WASBOT",
				"Add more WhatsApp numbers to your plan whenever you need them",
				"locks in for life once you make your first payment",
				"THIS SAME EMAIL ADDRESS",
				"https://www.wasbot.app/signup",
				"https://www.youtube.com/@wasbot_app",
				"August 31st",
				"30 free days",
			},
			[]string{"Link multiple WhatsApp numbers under one plan", "permanent 30% Legacy Believer discount across all plans. No card"},
		},
		{"legacy_winback_a3",
			[]string{
				"₦2,000", "₦3,000",
				"₦8,000", "₦5,600", "₦28,800",
				"₦20,000", "₦14,000", "₦72,000",
				"₦50,000", "₦35,000", "₦180,000",
				"Pay once for any plan before your free days run out",
				"THIS SAME EMAIL ADDRESS",
				"https://www.wasbot.app/signup",
				"August 31st",
				"marketers & TVs",
			},
			[]string{"TV’s", "TV's"},
		},
		{"legacy_winback_a4",
			[]string{
				"August 31st",
				"30% off any plan for life",
				"pay once before your free days finish",
				"THIS SAME EMAIL ADDRESS",
				"https://www.wasbot.app/signup",
				"30 free",
			},
			nil,
		},
	}

	for _, e := range emails {
		t.Run(e.template, func(t *testing.T) {
			data := map[string]interface{}{
				"FreeDays":       "30",
				"UnsubscribeURL": "https://wasbot.app/u/x",
			}
			subject, body, htmlBody, err := ts.Render(e.template, data)
			if err != nil {
				t.Fatalf("render failed: %v", err)
			}
			for surface, s := range map[string]string{"subject": subject, "body": body, "html": htmlBody} {
				if strings.Contains(s, "<no value>") {
					t.Errorf("%s leaks <no value>", surface)
				}
				if strings.Contains(s, "—") {
					t.Errorf("%s contains an em-dash", surface)
				}
			}
			for _, want := range e.want {
				htmlWant := strings.ReplaceAll(want, "&", "&amp;")
				if !strings.Contains(body, want) {
					t.Errorf("text body missing %q", want)
				}
				if !strings.Contains(htmlBody, want) && !strings.Contains(htmlBody, htmlWant) {
					t.Errorf("html body missing %q", want)
				}
			}
			for _, bad := range e.absent {
				if strings.Contains(body, bad) || strings.Contains(htmlBody, bad) {
					t.Errorf("stale copy present: %q", bad)
				}
			}
		})
	}
}

// Lapsed-believer templates (L1/L2) carry no per-row vars: 14 days and 20% are
// frozen copy backed by the LapsedBeliever* constants in wasbot-backend. The
// rewritten legacy_provisioned welcome renders per-pct (30 owed-days / 20 lapsed).
func TestLapsedWinbackAndProvisionedRender(t *testing.T) {
	ts := newTestTemplateService(t)

	for _, tmpl := range []string{"legacy_winback_lapsed_l1", "legacy_winback_lapsed_l2"} {
		t.Run(tmpl, func(t *testing.T) {
			subject, body, htmlBody, err := ts.Render(tmpl, map[string]interface{}{
				"UnsubscribeURL": "https://wasbot.app/u/x",
			})
			if err != nil {
				t.Fatalf("render failed: %v", err)
			}
			for surface, s := range map[string]string{"subject": subject, "body": body, "html": htmlBody} {
				if strings.Contains(s, "<no value>") {
					t.Errorf("%s leaks <no value>", surface)
				}
				if strings.Contains(s, "—") {
					t.Errorf("%s contains an em-dash", surface)
				}
			}
			for _, want := range []string{
				"14 free days", "20% discount",
				"THIS SAME EMAIL ADDRESS",
				"https://www.wasbot.app/signup",
				"August 31st",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("text body missing %q", want)
				}
			}
		})
	}

	t.Run("legacy_winback_lapsed_l1 pricing math", func(t *testing.T) {
		_, body, htmlBody, err := ts.Render("legacy_winback_lapsed_l1", map[string]interface{}{
			"UnsubscribeURL": "https://wasbot.app/u/x",
		})
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}
		for _, want := range []string{"₦8,000", "₦6,400", "₦20,000", "₦16,000", "₦50,000", "₦40,000"} {
			if !strings.Contains(body, want) || !strings.Contains(htmlBody, want) {
				t.Errorf("missing 20%% pricing fact %q", want)
			}
		}
		// The 30% believer numbers must NOT appear in the 20% email.
		for _, bad := range []string{"₦5,600", "₦14,000", "₦35,000", "30%"} {
			if strings.Contains(body, bad) || strings.Contains(htmlBody, bad) {
				t.Errorf("30%% cohort fact %q leaked into lapsed email", bad)
			}
		}
	})

	for _, pct := range []string{"30", "20"} {
		t.Run("legacy_provisioned pct "+pct, func(t *testing.T) {
			subject, body, htmlBody, err := ts.Render("legacy_provisioned", map[string]interface{}{
				"Name": "Ada", "FreeDays": "14", "DiscountPct": pct,
				"DashboardURL": "https://wasbot.app/dashboard",
			})
			if err != nil {
				t.Fatalf("render failed: %v", err)
			}
			for surface, s := range map[string]string{"subject": subject, "body": body, "html": htmlBody} {
				if strings.Contains(s, "<no value>") {
					t.Errorf("%s leaks <no value>", surface)
				}
				if strings.Contains(s, "—") {
					t.Errorf("%s contains an em-dash", surface)
				}
			}
			for _, bad := range []string{"10% off", "March 31", "uses of"} {
				if strings.Contains(body, bad) || strings.Contains(htmlBody, bad) {
					t.Errorf("stale copy %q still renders", bad)
				}
			}
			if !strings.Contains(body, pct+"% discount") {
				t.Errorf("text body missing %q", pct+"% discount")
			}
			if !strings.Contains(htmlBody, pct+"% off any plan for life") {
				t.Errorf("html missing pct offer line")
			}
		})
	}
}
