package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/prodigy90/email-service-go/internal/domain"
	"github.com/rs/zerolog"
)

// templateDir returns the path to pkg/templates relative to the project root.
func templateDir(t *testing.T) string {
	t.Helper()
	// Walk up from the test file location to find the project root
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get caller info")
	}
	// filename is .../internal/service/template_test.go
	// project root is 3 levels up
	dir := filepath.Dir(filename)
	root := filepath.Join(dir, "..", "..")
	templatesDir := filepath.Join(root, "pkg", "templates")
	if _, err := os.Stat(templatesDir); err != nil {
		t.Fatalf("templates directory not found at %s: %v", templatesDir, err)
	}
	return templatesDir
}

func newTestTemplateService(t *testing.T) *TemplateService {
	t.Helper()
	ts, err := NewTemplateService(templateDir(t), zerolog.Nop())
	if err != nil {
		t.Fatalf("failed to create template service: %v", err)
	}
	return ts
}

func TestTemplateService_MigrationAnnouncementExists(t *testing.T) {
	ts := newTestTemplateService(t)
	tmpl, ok := ts.Get(domain.TemplateMigrationAnnouncement)
	if !ok {
		t.Fatal("migration_announcement template not found")
	}
	if tmpl.Name != "migration_announcement" {
		t.Errorf("expected name 'migration_announcement', got %q", tmpl.Name)
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

func TestTemplateService_MigrationFollowUpExists(t *testing.T) {
	ts := newTestTemplateService(t)
	tmpl, ok := ts.Get(domain.TemplateMigrationFollowUp)
	if !ok {
		t.Fatal("migration_follow_up template not found")
	}
	if tmpl.Name != "migration_follow_up" {
		t.Errorf("expected name 'migration_follow_up', got %q", tmpl.Name)
	}
}

func TestTemplateService_RenderMigrationAnnouncement(t *testing.T) {
	ts := newTestTemplateService(t)

	data := map[string]interface{}{
		"Name":         "John",
		"DashboardURL": "https://wasbot.app/dashboard",
		"RefParam":     "",
	}

	subject, body, htmlBody, err := ts.Render(domain.TemplateMigrationAnnouncement, data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	if !strings.Contains(subject, "WASBOT") {
		t.Errorf("expected subject to contain 'WASBOT', got %q", subject)
	}
	if !strings.Contains(body, "John") {
		t.Errorf("expected body to contain 'John', got body without it")
	}
	if !strings.Contains(body, "wasbot.ng") {
		t.Errorf("expected body to mention wasbot.ng migration")
	}
	if !strings.Contains(htmlBody, "https://wasbot.app/dashboard") {
		t.Errorf("expected HTML body to contain dashboard URL")
	}
}

func TestTemplateService_RenderMigrationFollowUp(t *testing.T) {
	ts := newTestTemplateService(t)

	data := map[string]interface{}{
		"Name":         "Jane",
		"DashboardURL": "https://wasbot.app/dashboard",
	}

	subject, body, _, err := ts.Render(domain.TemplateMigrationFollowUp, data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	if !strings.Contains(subject, "wasbot.ng") {
		t.Errorf("expected follow-up subject to mention wasbot.ng, got %q", subject)
	}
	if !strings.Contains(body, "Jane") {
		t.Errorf("expected body to contain 'Jane'")
	}
}

func TestTemplateService_ListIncludesMigrationTemplates(t *testing.T) {
	ts := newTestTemplateService(t)
	templates := ts.List()

	found := map[string]bool{
		"migration_announcement": false,
		"migration_follow_up":    false,
	}

	for _, tmpl := range templates {
		if _, ok := found[tmpl.Name]; ok {
			found[tmpl.Name] = true
		}
	}

	for name, ok := range found {
		if !ok {
			t.Errorf("template %q not found in List()", name)
		}
	}
}

func TestTemplateService_AllTemplatesLoad(t *testing.T) {
	ts := newTestTemplateService(t)
	templates := ts.List()

	// We expect at least 39 templates (original builtins) + extras from YAML
	if len(templates) < 39 {
		t.Errorf("expected at least 39 templates, got %d", len(templates))
	}
}

func TestTemplateService_AllDomainTemplatesPresent(t *testing.T) {
	ts := newTestTemplateService(t)

	// Verify all template constants from domain are loaded
	requiredTemplates := []string{
		domain.TemplatePayoutApproved,
		domain.TemplatePayoutRejected,
		domain.TemplatePayoutProcessed,
		domain.TemplateCommissionEarned,
		domain.TemplatePaymentSuccess,
		domain.TemplatePaymentFailed,
		domain.TemplatePaymentFailedAttempt2,
		domain.TemplatePaymentFailedFinal,
		domain.TemplateSubscriptionRenewed,
		domain.TemplateSubscriptionExpiring,
		domain.TemplateSubscriptionCancelled,
		domain.TemplatePaymentSuccessOnetime,
		domain.TemplateSubscriptionActivated,
		domain.TemplateSubscriptionActivatedOnetime,
		domain.TemplateWelcome,
		domain.TemplateTrialExpiring,
		domain.TemplateAccountUpgraded,
		domain.TemplateTrialDay3,
		domain.TemplateTrialDay5,
		domain.TemplateTrialDay6,
		domain.TemplateTrialDay10,
		domain.TemplateRefundPending,
		domain.TemplateRefundProcessed,
		domain.TemplateRefundFailed,
		domain.TemplateSubscriptionReminder3d,
		domain.TemplateSubscriptionReminder1d,
		domain.TemplateSubscriptionExpiring3d,
		domain.TemplateSubscriptionExpiring1d,
		domain.TemplateCommissionRefunded,
		domain.TemplateAccessRevoked,
		domain.TemplateEmailVerification,
		domain.TemplatePasswordReset,
		domain.TemplateAffiliateLinkUpdated,
		domain.TemplateMigrationAnnouncement,
		domain.TemplateMigrationFollowUp,
		domain.TemplateMigrationVerifyEmail,
		domain.TemplateMigrationUpgradeNudge,
		domain.TemplateMigrationFinalNotice,
		domain.TemplateMigrationVerifyEmailFinal,
		domain.TemplateMigrationUpgradeNudgeFinal,
		domain.TemplateFeatureAnnouncement,
	}

	for _, name := range requiredTemplates {
		tmpl, ok := ts.Get(name)
		if !ok {
			t.Errorf("required template %q not found", name)
			continue
		}
		if tmpl.Subject == "" {
			t.Errorf("template %q has empty subject", name)
		}
		if tmpl.Body == "" {
			t.Errorf("template %q has empty body", name)
		}
	}
}

func TestTemplateService_RenderWelcome(t *testing.T) {
	ts := newTestTemplateService(t)

	data := map[string]interface{}{
		"Name":    "Alice",
		"AppName": "WASBOT",
	}

	subject, body, htmlBody, err := ts.Render(domain.TemplateWelcome, data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	if !strings.Contains(subject, "WASBOT") {
		t.Errorf("expected subject to contain 'WASBOT', got %q", subject)
	}
	if !strings.Contains(body, "Alice") {
		t.Errorf("expected body to contain 'Alice'")
	}
	if htmlBody == "" {
		t.Error("expected non-empty HTML body")
	}
}

func TestTemplateService_EmptyDirReturnsError(t *testing.T) {
	dir := t.TempDir()
	_, err := NewTemplateService(dir, zerolog.Nop())
	if err == nil {
		t.Error("expected error when loading from empty directory")
	}
}
