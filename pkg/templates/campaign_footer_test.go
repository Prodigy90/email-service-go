package templates_test

import (
	"strings"
	"testing"
)

// Page leads (walkthrough sign-ups) have no WASBOT account, so a campaign sent to
// them sets FooterReason. Every other campaign keeps the account line.
func TestCampaignUpdateFooterReason(t *testing.T) {
	ts := newTestTemplateService(t)
	const accountLine = "You're receiving this because you have a WASBOT account."
	base := map[string]interface{}{"Name": "Ada", "Subject": "s", "Intro": "i", "Body": "b", "UnsubscribeURL": "https://u"}

	_, body, _, err := ts.Render("campaign_update", base)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(body, accountLine) {
		t.Fatalf("default footer missing:\n%s", body)
	}

	base["FooterReason"] = "You're receiving this because you asked for the WASBOT walkthrough video."
	_, body, _, err = ts.Render("campaign_update", base)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(body, accountLine) || !strings.Contains(body, "asked for the WASBOT walkthrough video") {
		t.Fatalf("FooterReason not used:\n%s", body)
	}
}
