package service

import (
	"strings"
	"testing"

	"github.com/prodigy90/email-service-go/internal/domain"
	"github.com/rs/zerolog"
)

func newTestTemplateService(t *testing.T) *TemplateService {
	t.Helper()
	ts, err := NewTemplateService("", zerolog.Nop())
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
	}

	subject, body, htmlBody, err := ts.Render(domain.TemplateMigrationAnnouncement, data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	if !strings.Contains(subject, "upgrade") {
		t.Errorf("expected subject to contain 'upgrade', got %q", subject)
	}
	if !strings.Contains(body, "John") {
		t.Errorf("expected body to contain 'John', got body without it")
	}
	if !strings.Contains(body, "Paystack subscription plan is being discontinued") {
		t.Errorf("expected body to mention Paystack deprecation")
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

	if !strings.Contains(subject, "WhatsApp automation") {
		t.Errorf("expected follow-up subject to mention WhatsApp automation, got %q", subject)
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

func TestTemplateService_AllBuiltinTemplatesLoad(t *testing.T) {
	ts := newTestTemplateService(t)
	templates := ts.List()

	if len(templates) < 25 {
		t.Errorf("expected at least 25 builtin templates, got %d", len(templates))
	}
}
