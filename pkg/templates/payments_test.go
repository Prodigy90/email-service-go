package templates_test

import (
	"strings"
	"testing"

	"github.com/prodigy90/email-service-go/internal/domain"
	"github.com/prodigy90/email-service-go/internal/service"
	"github.com/rs/zerolog"
)

// templatesDir returns the absolute path to the pkg/templates directory.
// Since this test file lives inside pkg/templates/, "." is the directory itself.
func templatesDir() string {
	return "."
}

func newTestTemplateService(t *testing.T) *service.TemplateService {
	t.Helper()
	ts, err := service.NewTemplateService(templatesDir(), zerolog.Nop())
	if err != nil {
		t.Fatalf("failed to create template service: %v", err)
	}
	return ts
}

// --- Escalating dunning template existence tests ---

func TestPaymentFailedTemplateExists(t *testing.T) {
	ts := newTestTemplateService(t)
	tmpl, ok := ts.Get(domain.TemplatePaymentFailed)
	if !ok {
		t.Fatal("payment_failed template not found")
	}
	if tmpl.Subject == "" {
		t.Error("expected non-empty subject")
	}
	if tmpl.Body == "" {
		t.Error("expected non-empty body")
	}
	if tmpl.HTMLBody == "" {
		t.Error("expected non-empty HTML body")
	}
}

func TestPaymentFailedAttempt2TemplateExists(t *testing.T) {
	ts := newTestTemplateService(t)
	tmpl, ok := ts.Get(domain.TemplatePaymentFailedAttempt2)
	if !ok {
		t.Fatal("payment_failed_attempt2 template not found")
	}
	if tmpl.Subject == "" {
		t.Error("expected non-empty subject")
	}
	if tmpl.Body == "" {
		t.Error("expected non-empty body")
	}
	if tmpl.HTMLBody == "" {
		t.Error("expected non-empty HTML body")
	}
}

func TestPaymentFailedFinalTemplateExists(t *testing.T) {
	ts := newTestTemplateService(t)
	tmpl, ok := ts.Get(domain.TemplatePaymentFailedFinal)
	if !ok {
		t.Fatal("payment_failed_final template not found")
	}
	if tmpl.Subject == "" {
		t.Error("expected non-empty subject")
	}
	if tmpl.Body == "" {
		t.Error("expected non-empty body")
	}
	if tmpl.HTMLBody == "" {
		t.Error("expected non-empty HTML body")
	}
}

// --- Escalating dunning template rendering tests ---

func TestPaymentFailedAttempt2_Renders(t *testing.T) {
	ts := newTestTemplateService(t)

	data := map[string]interface{}{
		"CustomerName":  "Chinaza",
		"PlanName":      "Premium",
		"UpdateCardURL": "https://wasbot.app/billing",
	}

	subject, body, htmlBody, err := ts.Render(domain.TemplatePaymentFailedAttempt2, data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	// Subject should mention second attempt
	if !strings.Contains(subject, "Second") {
		t.Errorf("expected subject to contain 'Second', got %q", subject)
	}

	// Body checks
	if !strings.Contains(body, "Chinaza") {
		t.Error("expected body to contain customer name 'Chinaza'")
	}
	if !strings.Contains(body, "Premium") {
		t.Error("expected body to contain plan name 'Premium'")
	}
	if !strings.Contains(body, "https://wasbot.app/billing") {
		t.Error("expected body to contain UpdateCardURL")
	}

	// HTML body should also render
	if htmlBody == "" {
		t.Error("expected non-empty HTML body")
	}
	if !strings.Contains(htmlBody, "Chinaza") {
		t.Error("expected HTML body to contain customer name")
	}
}

func TestPaymentFailedFinal_Renders(t *testing.T) {
	ts := newTestTemplateService(t)

	data := map[string]interface{}{
		"CustomerName": "Emeka",
		"PlanName":     "Business",
		"DashboardURL": "https://wasbot.app/dashboard",
	}

	subject, body, htmlBody, err := ts.Render(domain.TemplatePaymentFailedFinal, data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	// Subject should indicate finality / suspension
	if !strings.Contains(subject, "Final") && !strings.Contains(subject, "Suspended") {
		t.Errorf("expected subject to mention 'Final' or 'Suspended', got %q", subject)
	}

	// Body checks
	if !strings.Contains(body, "Emeka") {
		t.Error("expected body to contain customer name 'Emeka'")
	}
	if !strings.Contains(body, "Business") {
		t.Error("expected body to contain plan name 'Business'")
	}
	if !strings.Contains(body, "https://wasbot.app/dashboard") {
		t.Error("expected body to contain DashboardURL")
	}

	// HTML body
	if htmlBody == "" {
		t.Error("expected non-empty HTML body")
	}
	if !strings.Contains(htmlBody, "Emeka") {
		t.Error("expected HTML body to contain customer name")
	}
	// The final template should mention disconnection consequences
	if !strings.Contains(htmlBody, "disconnected") {
		t.Error("expected HTML body to mention sessions being disconnected")
	}
}

// --- Addon breakdown in subscription reminder tests ---

func TestSubscriptionReminder3d_WithAddons(t *testing.T) {
	ts := newTestTemplateService(t)

	data := map[string]interface{}{
		"CustomerName": "Adaeze",
		"PlanName":     "Premium",
		"RenewalDate":  "April 1, 2026",
		"Amount":       "₦20,000",
		"AddonCount":   3,
		"AddonAmount":  "₦15,000",
		"TotalAmount":  "₦35,000",
		"ProfileURL":   "https://wasbot.app/billing",
	}

	_, body, htmlBody, err := ts.Render(domain.TemplateSubscriptionReminder3d, data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	// Plain text body should show addon breakdown
	if !strings.Contains(body, "₦15,000") {
		t.Error("expected body to contain addon amount '₦15,000'")
	}
	if !strings.Contains(body, "₦35,000") {
		t.Error("expected body to contain total amount '₦35,000'")
	}
	if !strings.Contains(body, "3 addon sessions") {
		t.Error("expected body to contain '3 addon sessions'")
	}

	// HTML body should also show addon breakdown
	if !strings.Contains(htmlBody, "₦35,000") {
		t.Error("expected HTML body to contain total amount")
	}
	if !strings.Contains(htmlBody, "addon") {
		t.Error("expected HTML body to contain 'addon' reference")
	}
}

func TestSubscriptionReminder3d_WithoutAddons(t *testing.T) {
	ts := newTestTemplateService(t)

	data := map[string]interface{}{
		"CustomerName": "Tunde",
		"PlanName":     "Basic",
		"RenewalDate":  "April 1, 2026",
		"Amount":       "₦10,000",
		"AddonCount":   0,
		"AddonAmount":  "",
		"TotalAmount":  "",
		"ProfileURL":   "https://wasbot.app/billing",
	}

	_, body, htmlBody, err := ts.Render(domain.TemplateSubscriptionReminder3d, data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	// Plain text body should show simple amount, no addon breakdown
	if !strings.Contains(body, "₦10,000") {
		t.Error("expected body to contain amount '₦10,000'")
	}
	if strings.Contains(body, "addon session") {
		t.Error("expected body NOT to contain 'addon session' when AddonCount is 0")
	}

	// HTML body should not show addon breakdown
	if strings.Contains(htmlBody, "addon session") {
		t.Error("expected HTML body NOT to contain 'addon session' when AddonCount is 0")
	}
}

func TestSubscriptionReminder1d_WithAddons(t *testing.T) {
	ts := newTestTemplateService(t)

	data := map[string]interface{}{
		"CustomerName": "Ngozi",
		"PlanName":     "Premium",
		"Amount":       "₦20,000",
		"AddonCount":   2,
		"AddonAmount":  "₦10,000",
		"TotalAmount":  "₦30,000",
		"ProfileURL":   "https://wasbot.app/billing",
	}

	_, body, htmlBody, err := ts.Render(domain.TemplateSubscriptionReminder1d, data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	// Plain text body should show addon breakdown
	if !strings.Contains(body, "₦10,000") {
		t.Error("expected body to contain addon amount '₦10,000'")
	}
	if !strings.Contains(body, "₦30,000") {
		t.Error("expected body to contain total amount '₦30,000'")
	}
	if !strings.Contains(body, "addon session") {
		t.Error("expected body to contain 'addon session' text")
	}

	// HTML body
	if !strings.Contains(htmlBody, "₦30,000") {
		t.Error("expected HTML body to contain total amount")
	}
}

// --- Consequences text in expiring templates ---

func TestSubscriptionExpiring3d_HasConsequences(t *testing.T) {
	ts := newTestTemplateService(t)

	data := map[string]interface{}{
		"CustomerName": "Kemi",
		"PlanName":     "Business",
		"ExpiryDate":   "April 1, 2026",
		"ProfileURL":   "https://wasbot.app/billing",
	}

	_, body, htmlBody, err := ts.Render(domain.TemplateSubscriptionExpiring3d, data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	// Check that consequence items appear in plain text body
	consequences := []string{
		"WhatsApp sessions will be disconnected",
		"Auto-save contacts will stop",
		"Status broadcasting will be disabled",
		"Group messaging will stop",
		"re-link your WhatsApp after renewing",
	}
	for _, c := range consequences {
		if !strings.Contains(body, c) {
			t.Errorf("expected body to contain consequence %q", c)
		}
	}

	// Same consequences should appear in HTML body
	for _, c := range consequences {
		if !strings.Contains(htmlBody, c) {
			t.Errorf("expected HTML body to contain consequence %q", c)
		}
	}
}

func TestSubscriptionExpiring1d_HasConsequences(t *testing.T) {
	ts := newTestTemplateService(t)

	data := map[string]interface{}{
		"CustomerName": "Femi",
		"PlanName":     "Premium",
		"ExpiryDate":   "March 30, 2026",
		"ProfileURL":   "https://wasbot.app/billing",
	}

	_, body, htmlBody, err := ts.Render(domain.TemplateSubscriptionExpiring1d, data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	// Check that consequence items appear in plain text body
	consequences := []string{
		"WhatsApp sessions will be disconnected",
		"Auto-save contacts will stop",
		"Status broadcasting will be disabled",
		"Group messaging will stop",
		"re-link your WhatsApp after renewing",
	}
	for _, c := range consequences {
		if !strings.Contains(body, c) {
			t.Errorf("expected body to contain consequence %q", c)
		}
	}

	// Same consequences should appear in HTML body
	for _, c := range consequences {
		if !strings.Contains(htmlBody, c) {
			t.Errorf("expected HTML body to contain consequence %q", c)
		}
	}
}

// --- Abandoned checkout (migration 159) ---

func TestCheckoutAbandonedTemplateExists(t *testing.T) {
	ts := newTestTemplateService(t)
	tmpl, ok := ts.Get(domain.TemplateCheckoutAbandoned)
	if !ok {
		t.Fatal("checkout_abandoned template not found")
	}
	if tmpl.Subject == "" || tmpl.Body == "" || tmpl.HTMLBody == "" {
		t.Error("expected subject, body and HTML body to all be non-empty")
	}
}

// The whole point of moving this off hand-rolled HTML: it has to come out
// wearing the same shell as every other WASBOT email — branded header, logo,
// footer — not a bare white card.
func TestCheckoutAbandonedRendersHouseShell(t *testing.T) {
	ts := newTestTemplateService(t)
	_, _, html, err := ts.Render(domain.TemplateCheckoutAbandoned, map[string]interface{}{
		"CustomerName": "Victor",
		"PlanName":     "Premium",
		"PriceLine":    "Premium is ₦20,000.",
		"CheckoutURL":  "https://www.wasbot.app/pricing",
	})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	for _, want := range []string{
		"WASBOT",                    // branded header/footer
		"Technologies",              // footer company block
		"All rights reserved",       // footer legal
		"Quick one, Victor.",        // greeting
		"Premium is ₦20,000.",       // price line
		"Finish setting up Premium", // CTA
		"https://www.wasbot.app/pricing",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
	// No unresolved placeholders should survive into a sent email.
	if strings.Contains(html, "{{") {
		t.Error("rendered HTML still contains template placeholders")
	}
}

// Recurring checkouts init from a plan code and carry no amount, so the price
// line must disappear rather than render an empty paragraph.
func TestCheckoutAbandonedOmitsEmptyPrice(t *testing.T) {
	ts := newTestTemplateService(t)
	_, text, html, err := ts.Render(domain.TemplateCheckoutAbandoned, map[string]interface{}{
		"CustomerName": "Victor",
		"PlanName":     "Premium",
		"PriceLine":    "",
		"CheckoutURL":  "https://www.wasbot.app/pricing",
	})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if strings.Contains(html, `<p style="margin: 0 0 20px 0;"></p>`) {
		t.Error("empty price rendered as a blank paragraph")
	}
	if !strings.Contains(html, "Finish setting up Premium") {
		t.Error("CTA missing when price line is absent")
	}
	if strings.Contains(text, "\n\n\n") {
		t.Error("plain-text body has a gap where the price line was")
	}
}

// The preheader is the snippet Gmail shows beside the subject in the inbox
// list. This template was generated from payment_failed, whose fallback text is
// "Payment Failed" — inheriting that would have previewed an abandoned-checkout
// nudge as a failed payment.
func TestCheckoutAbandonedPreheaderIsNotInherited(t *testing.T) {
	ts := newTestTemplateService(t)
	_, _, html, err := ts.Render(domain.TemplateCheckoutAbandoned, map[string]interface{}{
		"CustomerName": "Victor", "PlanName": "Premium", "CheckoutURL": "https://x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "Payment Failed") {
		t.Error(`preheader still falls back to "Payment Failed"`)
	}
}

// Plenty of accounts carry no display name; "Quick one, ." reads worse than no
// greeting at all.
func TestCheckoutAbandonedHandlesMissingName(t *testing.T) {
	ts := newTestTemplateService(t)
	_, text, html, err := ts.Render(domain.TemplateCheckoutAbandoned, map[string]interface{}{
		"CustomerName": "", "PlanName": "Premium", "CheckoutURL": "https://x",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{text, html} {
		if strings.Contains(body, "Quick one, .") {
			t.Error("empty name produced a dangling greeting")
		}
		if !strings.Contains(body, "Quick one.") {
			t.Error("expected the nameless greeting")
		}
	}
}

// House voice: no em-dashes in the prose (see the copy guide in CLAUDE.md).
func TestCheckoutAbandonedHasNoEmDashesInCopy(t *testing.T) {
	ts := newTestTemplateService(t)
	_, text, _, err := ts.Render(domain.TemplateCheckoutAbandoned, map[string]interface{}{
		"CustomerName": "Victor", "PlanName": "Premium",
		"PriceLine": "Premium is ₦20,000.", "CheckoutURL": "https://x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "—") {
		t.Errorf("em-dash in copy: %q", text)
	}
}
