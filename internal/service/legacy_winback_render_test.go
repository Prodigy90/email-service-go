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
