package service

import (
	"bytes"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/prodigy90/email-service-go/internal/domain"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// TemplateService manages email templates.
type TemplateService struct {
	templates map[string]*domain.Template
	mu        sync.RWMutex
	logger    zerolog.Logger
}

// NewTemplateService creates a new template service.
// Templates are loaded from YAML files in the templateDir directory.
func NewTemplateService(templateDir string, logger zerolog.Logger) (*TemplateService, error) {
	ts := &TemplateService{
		templates: make(map[string]*domain.Template),
		logger:    logger.With().Str("component", "template").Logger(),
	}

	// Load templates from YAML files in directory
	if templateDir != "" {
		if err := ts.loadFromDirectory(templateDir); err != nil {
			return nil, fmt.Errorf("failed to load templates from %s: %w", templateDir, err)
		}
	}

	if len(ts.templates) == 0 {
		ts.logger.Warn().Msg("No templates loaded - check TEMPLATE_DIR configuration")
	} else {
		ts.logger.Info().Int("count", len(ts.templates)).Msg("Templates loaded")
	}

	return ts, nil
}

// Render renders a template with the given data using default branding.
func (ts *TemplateService) Render(templateName string, data map[string]interface{}) (subject, body, htmlBody string, err error) {
	return ts.RenderWithBranding(templateName, data, nil)
}

// RenderWithBranding renders a template with the given data and branding config.
func (ts *TemplateService) RenderWithBranding(templateName string, data map[string]interface{}, branding *domain.BrandingConfig) (subject, body, htmlBody string, err error) {
	ts.mu.RLock()
	tmpl, ok := ts.templates[templateName]
	ts.mu.RUnlock()

	if !ok {
		return "", "", "", fmt.Errorf("template not found: %s", templateName)
	}

	// Use default branding if not provided, or merge with defaults for partial configs
	if branding == nil {
		branding = domain.DefaultBranding()
	} else {
		copied := *branding
		copied.MergeWithDefaults()
		branding = &copied
	}

	// Create a copy of data with branding added for template rendering
	// This allows templates to use {{.Branding.X}} syntax
	templateData := make(map[string]interface{})
	for k, v := range data {
		templateData[k] = v
	}
	templateData["Branding"] = ts.prepareBrandingData(branding)

	// Inject PreviewText from template if not already provided by caller
	if _, ok := templateData["PreviewText"]; !ok && tmpl.PreviewText != "" {
		templateData["PreviewText"] = tmpl.PreviewText
	}

	// Render subject
	subject, err = ts.renderString(tmpl.Subject, templateData)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to render subject: %w", err)
	}

	// Render body
	body, err = ts.renderString(tmpl.Body, templateData)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to render body: %w", err)
	}

	// Render HTML body if present
	if tmpl.HTMLBody != "" {
		// Render the template with data (including branding)
		var renderedHTML string
		renderedHTML, err = ts.renderString(tmpl.HTMLBody, templateData)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to render html body: %w", err)
		}
		htmlBody = renderedHTML
	}

	return subject, body, htmlBody, nil
}

// isValidHexColor validates that a string is a valid hex color code.
func isValidHexColor(color string) bool {
	if len(color) != 7 || color[0] != '#' {
		return false
	}
	for _, c := range color[1:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// prepareBrandingData converts BrandingConfig to a map with validated/computed values for templates.
func (ts *TemplateService) prepareBrandingData(branding *domain.BrandingConfig) map[string]interface{} {
	// Validate and sanitize color values - use safe defaults for invalid colors
	primaryColor := branding.PrimaryColor
	if !isValidHexColor(primaryColor) {
		primaryColor = "#10b981"
	}
	secondaryColor := branding.SecondaryColor
	if !isValidHexColor(secondaryColor) {
		secondaryColor = "#059669"
	}
	accentColor := branding.AccentColor
	if !isValidHexColor(accentColor) {
		accentColor = "#047857"
	}
	dangerColor := branding.DangerColor
	if !isValidHexColor(dangerColor) {
		dangerColor = "#ef4444"
	}

	// Generate logo content - use image if LogoURL provided, otherwise first letter
	var logoContent string
	if branding.LogoURL != "" {
		logoContent = fmt.Sprintf(`<img src="%s" alt="%s" style="max-width: 44px; max-height: 44px;">`,
			html.EscapeString(branding.LogoURL), html.EscapeString(branding.CompanyName))
	} else {
		firstChar := "W"
		if len(branding.CompanyName) > 0 {
			firstChar = string([]rune(branding.CompanyName)[0])
		}
		logoContent = fmt.Sprintf(`<span style="color: white; font-size: 22px; font-weight: bold;">%s</span>`,
			html.EscapeString(firstChar))
	}

	return map[string]interface{}{
		"PrimaryColor":     primaryColor,
		"SecondaryColor":   secondaryColor,
		"AccentColor":      accentColor,
		"DangerColor":      dangerColor,
		"CompanyName":      branding.CompanyName,
		"LogoURL":          branding.LogoURL,
		"LogoContent":      logoContent,
		"DashboardURL":     branding.DashboardURL,
		"SupportEmail":     branding.SupportEmail,
		"WebsiteURL":       branding.WebsiteURL,
		"SocialTwitter":    branding.SocialTwitter,
		"SocialInstagram":  branding.SocialInstagram,
		"IconTwitterURL":   branding.IconTwitterURL,
		"IconInstagramURL": branding.IconInstagramURL,
		"SocialYouTube":    branding.SocialYouTube,
		"IconYouTubeURL":   branding.IconYouTubeURL,
		"Year":             time.Now().UTC().Year(),
	}
}

// applyBranding replaces branding placeholders in the rendered HTML.
// NOTE: This is kept for backward compatibility but is no longer used by RenderWithBranding.
func (ts *TemplateService) applyBranding(htmlContent string, branding *domain.BrandingConfig) string {
	// Validate and sanitize color values - use safe defaults for invalid colors
	primaryColor := branding.PrimaryColor
	if !isValidHexColor(primaryColor) {
		primaryColor = "#10b981" // safe default
	}
	secondaryColor := branding.SecondaryColor
	if !isValidHexColor(secondaryColor) {
		secondaryColor = "#059669" // safe default
	}
	accentColor := branding.AccentColor
	if !isValidHexColor(accentColor) {
		accentColor = "#047857" // safe default
	}
	dangerColor := branding.DangerColor
	if !isValidHexColor(dangerColor) {
		dangerColor = "#ef4444" // safe default
	}

	// Replace color placeholders with validated values
	htmlContent = strings.ReplaceAll(htmlContent, "{{.Branding.PrimaryColor}}", primaryColor)
	htmlContent = strings.ReplaceAll(htmlContent, "{{.Branding.SecondaryColor}}", secondaryColor)
	htmlContent = strings.ReplaceAll(htmlContent, "{{.Branding.AccentColor}}", accentColor)
	htmlContent = strings.ReplaceAll(htmlContent, "{{.Branding.DangerColor}}", dangerColor)

	// Generate logo content - use image if LogoURL provided, otherwise first letter
	var logoContent string
	if branding.LogoURL != "" {
		logoContent = fmt.Sprintf(`<img src="%s" alt="%s" style="max-width: 44px; max-height: 44px;">`,
			html.EscapeString(branding.LogoURL), html.EscapeString(branding.CompanyName))
	} else {
		firstChar := "W"
		if len(branding.CompanyName) > 0 {
			firstChar = string([]rune(branding.CompanyName)[0])
		}
		logoContent = fmt.Sprintf(`<span style="color: white; font-size: 22px; font-weight: bold;">%s</span>`,
			html.EscapeString(firstChar))
	}
	htmlContent = strings.ReplaceAll(htmlContent, "{{.Branding.LogoContent}}", logoContent)

	// Escape text values to prevent XSS
	htmlContent = strings.ReplaceAll(htmlContent, "{{.Branding.CompanyName}}", html.EscapeString(branding.CompanyName))
	htmlContent = strings.ReplaceAll(htmlContent, "{{.Branding.LogoURL}}", html.EscapeString(branding.LogoURL))
	htmlContent = strings.ReplaceAll(htmlContent, "{{.Branding.DashboardURL}}", html.EscapeString(branding.DashboardURL))
	htmlContent = strings.ReplaceAll(htmlContent, "{{.Branding.SupportEmail}}", html.EscapeString(branding.SupportEmail))
	htmlContent = strings.ReplaceAll(htmlContent, "{{.Branding.WebsiteURL}}", html.EscapeString(branding.WebsiteURL))
	htmlContent = strings.ReplaceAll(htmlContent, "{{.Branding.SocialTwitter}}", html.EscapeString(branding.SocialTwitter))
	htmlContent = strings.ReplaceAll(htmlContent, "{{.Branding.SocialInstagram}}", html.EscapeString(branding.SocialInstagram))
	return htmlContent
}

// List returns all available templates.
func (ts *TemplateService) List() []domain.TemplateInfo {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	result := make([]domain.TemplateInfo, 0, len(ts.templates))
	for name, tmpl := range ts.templates {
		result = append(result, domain.TemplateInfo{
			Name:        name,
			Description: tmpl.Description,
			Variables:   ts.extractVariables(tmpl),
		})
	}
	return result
}

// Get returns a specific template.
func (ts *TemplateService) Get(name string) (*domain.Template, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	tmpl, ok := ts.templates[name]
	return tmpl, ok
}

// renderString renders a template string with data.
// Uses missingkey=zero so undefined variables render as empty strings instead of "<no value>".
func (ts *TemplateService) renderString(tmplStr string, data map[string]interface{}) (string, error) {
	tmpl, err := template.New("").Option("missingkey=zero").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// extractVariables extracts template variable names.
func (ts *TemplateService) extractVariables(tmpl *domain.Template) []string {
	re := regexp.MustCompile(`\{\{\.(\w+)\}\}`)
	vars := make(map[string]bool)

	for _, match := range re.FindAllStringSubmatch(tmpl.Subject, -1) {
		vars[match[1]] = true
	}
	for _, match := range re.FindAllStringSubmatch(tmpl.Body, -1) {
		vars[match[1]] = true
	}
	for _, match := range re.FindAllStringSubmatch(tmpl.HTMLBody, -1) {
		vars[match[1]] = true
	}

	result := make([]string, 0, len(vars))
	for v := range vars {
		result = append(result, v)
	}
	return result
}

// loadFromDirectory loads templates from all YAML files in the directory.
func (ts *TemplateService) loadFromDirectory(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return fmt.Errorf("failed to glob yaml files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no yaml files found in %s", dir)
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", f, err)
		}

		var templates map[string]*domain.Template
		if err := yaml.Unmarshal(data, &templates); err != nil {
			return fmt.Errorf("failed to parse %s: %w", f, err)
		}

		for name, tmpl := range templates {
			tmpl.Name = name
			ts.templates[name] = tmpl
		}

		ts.logger.Debug().
			Str("file", filepath.Base(f)).
			Int("count", len(templates)).
			Msg("Loaded templates from file")
	}

	return nil
}
