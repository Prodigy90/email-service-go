package templates_test

import (
	"strings"
	"testing"

	"github.com/prodigy90/email-service-go/internal/domain"
)

// reengage_day0 opens the re-engagement series (churned paid users) in place
// of onboarding_day0, which since the welcome refresh reads as a first
// welcome. The cron sends it with the lifecycle data (stats attached as the
// six-field group, or none on a query failure) and never with trial or code
// fields: those are only attached while a trial is running.

const reengageHelpLink = "https://wa.me/2349018943768?text=Hi%20Victor%2C%20my%20WASBOT%20plan%20ended%20and%20I%20have%20a%20question."

func reengageStates() []renderState {
	fullStats := map[string]interface{}{
		"ContactsSaved": 3412, "StatusesPosted": 18, "StatusRecipients": 41200,
		"StatusViews": 9633, "GroupSends": 22, "SequencesActive": 3,
		"HasActivity": true, "TimeSavedHours": 27,
	}
	zeroStats := map[string]interface{}{
		"ContactsSaved": 0, "StatusesPosted": 0, "StatusRecipients": 0,
		"StatusViews": 0, "GroupSends": 0, "SequencesActive": 0,
	}
	contactsOnly := map[string]interface{}{
		"ContactsSaved": 40, "StatusesPosted": 0, "StatusRecipients": 0,
		"StatusViews": 0, "GroupSends": 0, "SequencesActive": 0, "HasActivity": true,
	}
	bare := map[string]interface{}{
		"Name": "there", "DashboardURL": "https://www.wasbot.app/dashboard",
		"BillingURL": "https://www.wasbot.app/billing",
	}
	withName := func(m map[string]interface{}) map[string]interface{} {
		return withKeys(m, map[string]interface{}{"FirstName": "Ada"})
	}
	return []renderState{
		{"churned-active-user", withName(withKeys(baseLifecycleData(), fullStats))},
		{"churned-contacts-only", withKeys(baseLifecycleData(), contactsOnly)},
		{"churned-no-activity", withKeys(baseLifecycleData(), zeroStats)},
		{"stats-unavailable", baseLifecycleData()},
		{"missing-optional", bare},
	}
}

func TestReengageDay0_RendersForEveryChurnedReader(t *testing.T) {
	ts := newTestTemplateService(t)
	branding := domain.DefaultBranding()
	for _, st := range reengageStates() {
		label := "reengage_day0 [" + st.name + "]"
		subject, body, html, err := ts.RenderWithBranding("reengage_day0", st.data, branding)
		if err != nil {
			t.Fatalf("%s: render failed: %v", label, err)
		}
		assertClean(t, label, subject, body, html)

		// A returning customer, not a newcomer: no welcome, no trial, no code.
		for part, out := range map[string]string{"subject": subject, "body": body, "html": html} {
			low := strings.ToLower(out)
			for _, banned := range []string{"welcome", "trial", "discount", "code "} {
				if strings.Contains(low, banned) {
					t.Errorf("%s %s: says %q to a churned payer", label, part, banned)
				}
			}
		}
		if !strings.Contains(body, reengageHelpLink) || !strings.Contains(html, reengageHelpLink) {
			t.Errorf("%s: missing the preset wa.me help link", label)
		}
		if !strings.Contains(body, "https://www.wasbot.app/billing") || !strings.Contains(html, `href="https://www.wasbot.app/billing"`) {
			t.Errorf("%s: no way back to a plan", label)
		}
		if n := countSignoffs(body, html); n != [2]int{1, 1} {
			t.Errorf("%s: sign-offs (body, html) = %v, want one each", label, n)
		}
		if strings.Contains(body, "Tomorrow:") || strings.Contains(html, "Tomorrow:") {
			t.Errorf("%s: a win-back email teases a next one", label)
		}

		// The recap only shows what really happened.
		hasActivity := st.data["HasActivity"] == true
		if got := strings.Contains(body, "while you were with us"); got != hasActivity {
			t.Errorf("%s: recap shown=%v, want %v", label, got, hasActivity)
		}
		if strings.Contains(body, "Saved 0") || strings.Contains(body, "Posted 0") || strings.Contains(body, "Sent 0") {
			t.Errorf("%s: recap quotes a zero", label)
		}
	}

	// Numbers are formatted the house way.
	_, body, html, err := ts.RenderWithBranding("reengage_day0", reengageStates()[0].data, branding)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Saved 3,412 contacts", "Posted 18 statuses, seen 9,633 times", "Sent 22 group blasts"} {
		if !strings.Contains(body, want) {
			t.Errorf("full recap missing %q", want)
		}
	}
	if !strings.Contains(html, "<strong>3,412</strong>") {
		t.Error("html recap missing the formatted contact count")
	}
}

// reengage_day1..7 are the rest of the come-back series. Each renders for every
// churned reader, says nothing a trial reader would hear, always offers the
// way back through /billing, and never teases a next email (a win-back is
// not onboarding).
func TestReengageSeries_RendersForEveryChurnedReader(t *testing.T) {
	ts := newTestTemplateService(t)
	branding := domain.DefaultBranding()
	names := []string{"reengage_day1", "reengage_day2", "reengage_day3", "reengage_day4", "reengage_day5", "reengage_day6", "reengage_day7"}
	for _, name := range names {
		for _, st := range reengageStates() {
			label := name + " [" + st.name + "]"
			subject, body, html, err := ts.RenderWithBranding(name, st.data, branding)
			if err != nil {
				t.Fatalf("%s: render failed: %v", label, err)
			}
			assertClean(t, label, subject, body, html)
			for part, out := range map[string]string{"subject": subject, "body": body, "html": html} {
				low := strings.ToLower(out)
				for _, banned := range []string{"welcome", "trial", "discount", "code ", "—"} {
					if strings.Contains(low, banned) {
						t.Errorf("%s %s: contains %q", label, part, banned)
					}
				}
			}
			if !strings.Contains(body, reengageHelpLink) || !strings.Contains(html, reengageHelpLink) {
				t.Errorf("%s: missing the preset wa.me help link", label)
			}
			if !strings.Contains(body, "https://www.wasbot.app/billing") || !strings.Contains(html, `href="https://www.wasbot.app/billing"`) {
				t.Errorf("%s: no way back to a plan", label)
			}
			if n := countSignoffs(body, html); n != [2]int{1, 1} {
				t.Errorf("%s: sign-offs (body, html) = %v, want one each", label, n)
			}
			if strings.Contains(body, "Tomorrow:") || strings.Contains(html, "Tomorrow:") {
				t.Errorf("%s: a win-back email teases a next one", label)
			}
			if strings.Contains(body, "pricing") || strings.Contains(html, "/pricing") {
				t.Errorf("%s: sends a lapsed payer to /pricing, not /billing", label)
			}
			for _, zero := range []string{" 0 contacts", " 0 statuses", " 0 group", "viewed 0", " 0 follow-up"} {
				if strings.Contains(body, zero) {
					t.Errorf("%s: quotes a zero (%q)", label, zero)
				}
			}
		}
	}

	_, body, _, err := ts.RenderWithBranding("reengage_day7", reengageStates()[0].data, branding)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Saved 3,412 contacts", "Posted 18 statuses, viewed 9,633 times", "Roughly 27 hours"} {
		if !strings.Contains(body, want) {
			t.Errorf("day7 recap missing %q", want)
		}
	}
}
