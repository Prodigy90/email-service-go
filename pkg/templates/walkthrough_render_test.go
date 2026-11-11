package templates_test

import (
	"os"
	"strings"
	"testing"

	"github.com/prodigy90/email-service-go/internal/domain"
)

// vsl_walkthrough_link (item 136): the one email a /walkthrough lead gets. No
// name is known, so nothing may greet one; the button carries the tracked
// link; the copy is the founder-approved draft.
func TestVSLWalkthroughLink_Renders(t *testing.T) {
	ts := newTestTemplateService(t)
	const watch = "https://www.wasbot.app/walkthrough?ref=email_vsl1"
	const help = "https://www.wasbot.app/wa?text=Hi%20Victor%2C%20I%20watched%20the%20WASBOT%20walkthrough%20and%20have%20a%20question."
	subject, body, html, err := ts.RenderWithBranding("vsl_walkthrough_link",
		map[string]interface{}{"WatchURL": watch, "HelpURL": help}, domain.DefaultBranding())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	assertClean(t, "vsl_walkthrough_link", subject, body, html)
	// VSL_EMAIL_PREVIEW=<file.html> writes the render out for a visual check.
	if out := os.Getenv("VSL_EMAIL_PREVIEW"); out != "" {
		_ = os.WriteFile(out, []byte(html), 0o644)
	}

	if subject != "Your WASBOT walkthrough (16 minutes)" {
		t.Errorf("subject = %q", subject)
	}
	if !strings.Contains(body, watch) || !strings.Contains(html, `href="`+watch+`"`) {
		t.Error("missing the walkthrough link")
	}
	if !strings.Contains(body, help) || !strings.Contains(html, `href="`+help+`"`) {
		t.Error("missing the support link")
	}
	for _, line := range []string{
		"Here's the walkthrough you asked for.",
		"reply at 1am without touching their phone.",
		"Watch it when you're ready, it picks up where you stopped on the same device.",
		"Questions? Reply to this email or message us at",
	} {
		if !strings.Contains(body, line) || !strings.Contains(html, line) {
			t.Errorf("missing approved line %q", line)
		}
	}
	if !strings.Contains(body, "Victor, founder of WASBOT") || !strings.Contains(html, "<strong>Victor</strong>, founder of WASBOT") {
		t.Error("missing the sign-off")
	}
	if strings.Count(html, "Watch the walkthrough") != 1 {
		t.Errorf("want exactly one button, got %d", strings.Count(html, "Watch the walkthrough"))
	}
	for _, out := range []string{subject, body, html} {
		if strings.Contains(out, "<no value>") || strings.Contains(strings.ToLower(out), "hi ,") {
			t.Error("greets a missing name")
		}
	}
}
