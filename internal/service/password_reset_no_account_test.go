package service

import (
	"strings"
	"testing"

	"github.com/prodigy90/email-service-go/internal/domain"
)

// The no-account email goes to whoever typed the address into the reset
// form, so it must never carry a reset link or anything account-specific,
// and it is a transactional reply (no unsubscribe headers).
func TestPasswordResetNoAccount_Render(t *testing.T) {
	ts := newTestTemplateService(t)

	data := map[string]interface{}{
		"AppName":   "WASBOT",
		"Email":     "someone@example.com",
		"SignupURL": "https://www.wasbot.app/signup",
	}
	subject, body, htmlBody, err := ts.Render(domain.TemplatePasswordResetNoAccount, data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	if subject != "There's no WASBOT account for this email" {
		t.Errorf("unexpected subject %q", subject)
	}
	for name, s := range map[string]string{"body": body, "html": htmlBody} {
		if !strings.Contains(s, "someone@example.com") {
			t.Errorf("%s: missing the requested address", name)
		}
		if !strings.Contains(s, "https://www.wasbot.app/signup") {
			t.Errorf("%s: missing the signup link", name)
		}
		if strings.Contains(s, "ResetURL") || strings.Contains(strings.ToLower(s), "reset-password/") {
			t.Errorf("%s: must not contain a reset link", name)
		}
		if strings.Contains(s, "<no value>") {
			t.Errorf("%s: unrendered template variable", name)
		}
	}
	// Copy rule: no em-dashes in what the reader sees (the CSS comment in
	// the shared chassis is not visible copy).
	if strings.Contains(body, "—") {
		t.Errorf("plain body contains an em-dash")
	}
	if !isTransactionalTemplate(domain.TemplatePasswordResetNoAccount) {
		t.Errorf("no-account email must be transactional (no List-Unsubscribe)")
	}
}
