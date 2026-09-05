package templates_test

import (
	"strings"
	"testing"
)

// Regression: campaign_update's HTML branch used to print .Closing AND an
// unconditional "Talk soon, The WASBOT Team", so any campaign that set a Closing
// went out with two sign-offs stacked on each other (caught in the 2026-08-16
// week-with-WASBOT self-test). The plaintext branch was already correct
// ({{if .Closing}}...{{else}}...{{end}}); only the HTML diverged.
func TestCampaignUpdateRendersExactlyOneSignoff(t *testing.T) {
	ts := newTestTemplateService(t)

	const teamSignoff = "The WASBOT Team"

	t.Run("closing set suppresses the default team sign-off", func(t *testing.T) {
		_, body, htmlBody, err := ts.Render("campaign_update", map[string]interface{}{
			"Name":    "Victor",
			"Subject": "s",
			"Intro":   "i",
			"Body":    "b",
			"Closing": "Any questions, just reply to this email.",
			"CTAText": "Go",
			"CTAURL":  "https://www.wasbot.app/billing",
		})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if n := strings.Count(htmlBody, teamSignoff); n != 0 {
			t.Errorf("html: want 0 %q when Closing is set, got %d", teamSignoff, n)
		}
		if n := strings.Count(body, teamSignoff); n != 0 {
			t.Errorf("text: want 0 %q when Closing is set, got %d", teamSignoff, n)
		}
		if !strings.Contains(htmlBody, "just reply to this email") {
			t.Error("html: Closing text missing")
		}
	})

	t.Run("no closing falls back to exactly one team sign-off", func(t *testing.T) {
		_, body, htmlBody, err := ts.Render("campaign_update", map[string]interface{}{
			"Name":    "Victor",
			"Subject": "s",
			"Intro":   "i",
			"Body":    "b",
			"CTAText": "Go",
			"CTAURL":  "https://www.wasbot.app/billing",
		})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if n := strings.Count(htmlBody, teamSignoff); n != 1 {
			t.Errorf("html: want exactly 1 %q fallback, got %d", teamSignoff, n)
		}
		if n := strings.Count(body, teamSignoff); n != 1 {
			t.Errorf("text: want exactly 1 %q fallback, got %d", teamSignoff, n)
		}
	})
}
