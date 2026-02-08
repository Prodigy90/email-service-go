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
func NewTemplateService(templateDir string, logger zerolog.Logger) (*TemplateService, error) {
	ts := &TemplateService{
		templates: make(map[string]*domain.Template),
		logger:    logger.With().Str("component", "template").Logger(),
	}

	// Load built-in templates
	ts.loadBuiltinTemplates()

	// Load custom templates from directory if it exists
	if templateDir != "" {
		if err := ts.loadFromDirectory(templateDir); err != nil {
			ts.logger.Warn().Err(err).Str("dir", templateDir).Msg("Failed to load custom templates")
		}
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

	// Use default branding if not provided
	if branding == nil {
		branding = domain.DefaultBranding()
	}

	// Create a copy of data with branding added for template rendering
	// This allows templates to use {{.Branding.X}} syntax
	templateData := make(map[string]interface{})
	for k, v := range data {
		templateData[k] = v
	}
	templateData["Branding"] = ts.prepareBrandingData(branding)

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
		"PrimaryColor":    primaryColor,
		"SecondaryColor":  secondaryColor,
		"AccentColor":     accentColor,
		"DangerColor":     dangerColor,
		"CompanyName":     branding.CompanyName,
		"LogoURL":         branding.LogoURL,
		"LogoContent":     logoContent,
		"DashboardURL":    branding.DashboardURL,
		"SupportEmail":    branding.SupportEmail,
		"WebsiteURL":      branding.WebsiteURL,
		"SocialTwitter":   branding.SocialTwitter,
		"SocialInstagram": branding.SocialInstagram,
		"Year":            time.Now().UTC().Year(),
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
func (ts *TemplateService) renderString(tmplStr string, data map[string]interface{}) (string, error) {
	tmpl, err := template.New("").Parse(tmplStr)
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

// loadFromDirectory loads templates from a YAML file in the directory.
func (ts *TemplateService) loadFromDirectory(dir string) error {
	yamlPath := filepath.Join(dir, "templates.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return err
	}

	var templates map[string]*domain.Template
	if err := yaml.Unmarshal(data, &templates); err != nil {
		return err
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()
	for name, tmpl := range templates {
		tmpl.Name = name
		ts.templates[name] = tmpl
		ts.logger.Debug().Str("template", name).Msg("Loaded custom template")
	}

	return nil
}

// loadBuiltinTemplates loads the default templates.
func (ts *TemplateService) loadBuiltinTemplates() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Affiliate System templates
	ts.templates[domain.TemplatePayoutApproved] = &domain.Template{
		Name:        domain.TemplatePayoutApproved,
		Subject:     "Payout Request Approved - {{.Amount}}",
		Body:        "Hi {{.Name}},\n\nYour payout request of {{.Amount}} has been approved and is being processed.\n\nExpected arrival: 1-3 business days.\n\nThank you for being part of our affiliate program!",
		HTMLBody:    ts.wrapHTML("Payout Approved", "<p>Hi {{.Name}},</p><p>Your payout request of <strong>{{.Amount}}</strong> has been approved and is being processed.</p><p>Expected arrival: 1-3 business days.</p><p>Thank you for being part of our affiliate program!</p>"),
		Description: "Sent when an affiliate payout is approved",
	}

	ts.templates[domain.TemplatePayoutRejected] = &domain.Template{
		Name:        domain.TemplatePayoutRejected,
		Subject:     "Payout Request Update",
		Body:        "Hi {{.Name}},\n\nYour payout request of {{.Amount}} was not approved.\n\nReason: {{.Reason}}\n\nPlease contact support if you have questions.",
		HTMLBody:    ts.wrapHTML("Payout Update", "<p>Hi {{.Name}},</p><p>Your payout request of <strong>{{.Amount}}</strong> was not approved.</p><p><strong>Reason:</strong> {{.Reason}}</p><p>Please contact support if you have questions.</p>"),
		Description: "Sent when an affiliate payout is rejected",
	}

	ts.templates[domain.TemplatePayoutProcessed] = &domain.Template{
		Name:        domain.TemplatePayoutProcessed,
		Subject:     "Payout Completed - {{.Amount}}",
		Body:        "Hi {{.Name}},\n\nGreat news! Your payout of {{.Amount}} has been successfully transferred to your account.\n\nTransaction Reference: {{.Reference}}\n\nThank you!",
		HTMLBody:    ts.wrapHTML("Payout Completed", "<p>Hi {{.Name}},</p><p>Great news! Your payout of <strong>{{.Amount}}</strong> has been successfully transferred to your account.</p><p><strong>Transaction Reference:</strong> {{.Reference}}</p><p>Thank you!</p>"),
		Description: "Sent when a payout transfer completes",
	}

	ts.templates[domain.TemplateCommissionEarned] = &domain.Template{
		Name:        domain.TemplateCommissionEarned,
		Subject:     "You earned a commission! - {{.Amount}}",
		Body:        "Hi {{.Name}},\n\nYou just earned {{.Amount}} from {{.ProductName}}{{if .PlanName}} ({{.PlanName}}){{end}}!\n\nThis commission will be available for withdrawal after the holding period.\n\nKeep up the great work!",
		HTMLBody:    ts.wrapHTML("Commission Earned", "<p>Hi {{.Name}},</p><p>You just earned <strong>{{.Amount}}</strong> from {{.ProductName}}{{if .PlanName}} ({{.PlanName}}){{end}}!</p><p>This commission will be available for withdrawal after the holding period.</p><p>Keep up the great work!</p>"),
		Description: "Sent when an affiliate earns a commission",
	}

	ts.templates[domain.TemplateCommissionRefunded] = &domain.Template{
		Name:        domain.TemplateCommissionRefunded,
		Subject:     "Commission Reversed - {{.ProductName}}",
		Body:        "Hi {{.Name}},\n\nA commission of {{.Amount}} for {{.ProductName}} has been reversed.\n\nReason: {{.Reason}}\n\nThis adjustment has been applied to your affiliate account balance.\n\nIf you have any questions about this reversal, please contact our support team.",
		HTMLBody:    ts.wrapHTML("Commission Reversed", ts.commissionRefundedContent()),
		Description: "Sent when an affiliate commission is reversed due to a refund",
	}

	ts.templates[domain.TemplateAccessRevoked] = &domain.Template{
		Name:        domain.TemplateAccessRevoked,
		Subject:     "Access Revoked - Your Subscription Has Been Cancelled",
		Body:        "Hi {{.CustomerName}},\n\nYour refund of {{.Amount}} for {{.ProductName}} has been processed.\n\nAs a result, your access to {{.ProductName}} has been revoked.\n\nTransaction ID: {{.TransactionID}}\n\nIf you have any questions or believe this was done in error, please contact our support team.",
		HTMLBody:    ts.wrapHTML("Access Revoked", ts.accessRevokedContent()),
		Description: "Sent when a customer's access is revoked due to a refund",
	}

	// Payment/Webhook templates - RECURRING SUBSCRIPTIONS
	ts.templates[domain.TemplatePaymentSuccess] = &domain.Template{
		Name:        domain.TemplatePaymentSuccess,
		Subject:     "Payment Confirmed - {{.ProductName}}",
		Body:        "Hi {{.CustomerName}},\n\nThank you for your payment of {{.Amount}}.\n\nProduct: {{.ProductName}}\nTransaction ID: {{.TransactionID}}\n\nYour subscription is now active and will automatically renew on {{.NextBillingDate}}.\n\nYou can manage your subscription anytime from your dashboard.",
		HTMLBody:    ts.wrapHTML("Payment Confirmed", ts.paymentSuccessContent()),
		Description: "Sent after successful recurring payment",
	}

	ts.templates[domain.TemplatePaymentFailed] = &domain.Template{
		Name:        domain.TemplatePaymentFailed,
		Subject:     "Payment Failed - Action Required",
		Body:        "Hi {{.CustomerName}},\n\nWe were unable to process your payment of {{.Amount}} for {{.ProductName}}.\n\nPlease update your payment method to continue your subscription.\n\nIf you need help, contact our support team.",
		HTMLBody:    ts.wrapHTML("Payment Failed", "<p>Hi {{.CustomerName}},</p><p>We were unable to process your payment of <strong>{{.Amount}}</strong> for {{.ProductName}}.</p><p>Please update your payment method to continue your subscription.</p><p>If you need help, contact our support team.</p>"),
		Description: "Sent when payment fails",
	}

	ts.templates[domain.TemplateSubscriptionRenewed] = &domain.Template{
		Name:        domain.TemplateSubscriptionRenewed,
		Subject:     "Subscription Renewed - {{.PlanName}}",
		Body:        "Hi {{.CustomerName}},\n\nYour {{.PlanName}} subscription has been automatically renewed.\n\nAmount: {{.Amount}}\nNext billing date: {{.NextBillingDate}}\n\nYour subscription will continue to renew automatically. You can manage or cancel anytime from your dashboard.\n\nThank you for your continued support!",
		HTMLBody:    ts.wrapHTML("Subscription Renewed", ts.subscriptionRenewedContent()),
		Description: "Sent when recurring subscription auto-renews",
	}

	ts.templates[domain.TemplateSubscriptionExpiring] = &domain.Template{
		Name:        domain.TemplateSubscriptionExpiring,
		Subject:     "Your subscription expires in {{.Days}} days",
		Body:        "Hi {{.CustomerName}},\n\nYour {{.PlanName}} subscription will expire in {{.Days}} days.\n\nRenew now to avoid service interruption.\n\nThank you!",
		HTMLBody:    ts.wrapHTML("Subscription Expiring", "<p>Hi {{.CustomerName}},</p><p>Your <strong>{{.PlanName}}</strong> subscription will expire in <strong>{{.Days}} days</strong>.</p><p>Renew now to avoid service interruption.</p><p>Thank you!</p>"),
		Description: "Sent before subscription expires",
	}

	ts.templates[domain.TemplateSubscriptionCancelled] = &domain.Template{
		Name:        domain.TemplateSubscriptionCancelled,
		Subject:     "Subscription Cancelled",
		Body:        "Hi {{.CustomerName}},\n\nYour {{.PlanName}} subscription has been cancelled.\n\nYou will continue to have access until {{.ExpiryDate}}. Your subscription will NOT automatically renew.\n\nWe're sorry to see you go. If you change your mind, you can resubscribe anytime.",
		HTMLBody:    ts.wrapHTML("Subscription Cancelled", ts.subscriptionCancelledContent()),
		Description: "Sent when subscription is cancelled",
	}

	// ONE-TIME PAYMENT TEMPLATES (non-recurring)
	ts.templates[domain.TemplatePaymentSuccessOnetime] = &domain.Template{
		Name:        domain.TemplatePaymentSuccessOnetime,
		Subject:     "Payment Confirmed - {{.ProductName}} ({{.Duration}})",
		Body:        "Hi {{.CustomerName}},\n\nThank you for your payment of {{.Amount}}.\n\nProduct: {{.ProductName}}\nDuration: {{.Duration}}\nAccess until: {{.ExpiryDate}}\nTransaction ID: {{.TransactionID}}\n\nIMPORTANT: This is a one-time purchase. Your subscription will NOT automatically renew. You'll need to manually renew before {{.ExpiryDate}} to continue access.\n\nWe'll send you a reminder before your access expires.",
		HTMLBody:    ts.wrapHTML("Payment Confirmed", ts.paymentSuccessOnetimeContent()),
		Description: "Sent after successful one-time payment",
	}

	ts.templates[domain.TemplateSubscriptionActivated] = &domain.Template{
		Name:        domain.TemplateSubscriptionActivated,
		Subject:     "Welcome to {{.PlanName}} - Subscription Activated!",
		Body:        "Hi {{.CustomerName}},\n\nWelcome! Your {{.PlanName}} subscription is now active.\n\nAmount: {{.Amount}}/{{.Interval}}\nNext billing date: {{.NextBillingDate}}\n\nYour subscription will automatically renew. You can manage or cancel anytime from your dashboard.\n\nThank you for choosing {{.Branding.CompanyName}}!",
		HTMLBody:    ts.wrapHTML("Subscription Activated", ts.subscriptionActivatedContent()),
		Description: "Sent when new recurring subscription is activated",
	}

	ts.templates[domain.TemplateSubscriptionActivatedOnetime] = &domain.Template{
		Name:        domain.TemplateSubscriptionActivatedOnetime,
		Subject:     "Welcome to {{.PlanName}} - Access Activated!",
		Body:        "Hi {{.CustomerName}},\n\nWelcome! Your {{.PlanName}} access is now active.\n\nDuration: {{.Duration}}\nAccess until: {{.ExpiryDate}}\n\nIMPORTANT: This is a one-time purchase. Your access will NOT automatically renew. Please renew manually before {{.ExpiryDate}} to avoid service interruption.\n\nWe'll send you reminders before your access expires.\n\nThank you for choosing {{.Branding.CompanyName}}!",
		HTMLBody:    ts.wrapHTML("Access Activated", ts.subscriptionActivatedOnetimeContent()),
		Description: "Sent when new one-time subscription is activated",
	}

	// WASBOT/General templates
	ts.templates[domain.TemplateWelcome] = &domain.Template{
		Name:        domain.TemplateWelcome,
		Subject:     "Welcome to {{.AppName}} - Your WhatsApp Automation Journey Starts Now!",
		Body:        "Hi {{.Name}},\n\nWelcome to {{.AppName}}! We're thrilled to have you on board.\n\nYou now have access to:\n- Broadcast status updates to up to 1,000 contacts\n- Auto-save new contacts to Google Contacts\n- Send messages to multiple groups + tag members\n- Post up to 5 status updates per day\n\nHere's how to get started:\n1. Connect your WhatsApp by scanning the QR code\n2. Link your Google account for contact sync\n3. Create your first status broadcast\n\nYour 7-day trial starts now. Make the most of it!\n\nBest,\nThe {{.AppName}} Team",
		HTMLBody:    ts.wrapHTML("Welcome!", ts.welcomeContent()),
		Description: "Welcome email for new users",
	}

	ts.templates[domain.TemplateTrialExpiring] = &domain.Template{
		Name:        domain.TemplateTrialExpiring,
		Subject:     "Your {{.AppName}} trial expires in {{.Days}} days",
		Body:        "Hi {{.Name}},\n\nYour free trial of {{.AppName}} expires in {{.Days}} days.\n\nDon't lose access to:\n- Status broadcasting to your contacts\n- Auto-save contacts to Google Contacts\n- Group messaging and tagging\n\nUpgrade now to keep your WhatsApp automation running:\n{{.UpgradeURL}}\n\nQuestions? Reply to this email.",
		HTMLBody:    ts.wrapHTML("Trial Expiring Soon", ts.trialExpiringContent()),
		Description: "Sent before trial expires",
	}

	// Trial sequence templates
	ts.templates[domain.TemplateTrialDay3] = &domain.Template{
		Name:        domain.TemplateTrialDay3,
		Subject:     "How's your {{.AppName}} trial going?",
		Body:        "Hi {{.Name}},\n\nYou've been using {{.AppName}} for 3 days now! How's it going?\n\nHere are some features you might not have tried yet:\n- Broadcast status updates to all your contacts at once\n- Auto-save new WhatsApp contacts to Google Contacts\n- Send messages to multiple groups and tag members\n\nNeed help getting started? Just reply to this email.\n\nBest,\nThe {{.AppName}} Team",
		HTMLBody:    ts.wrapHTML("How's Your Trial Going?", ts.trialDay3Content()),
		Description: "Day 3 engagement check",
	}

	ts.templates[domain.TemplateTrialDay5] = &domain.Template{
		Name:        domain.TemplateTrialDay5,
		Subject:     "Unlock the full power of {{.AppName}}",
		Body:        "Hi {{.Name}},\n\nYou're halfway through your trial! Here's what paid users unlock:\n\nBasic Plan ($5.50/mo):\n- 15 status updates per day\n- 5,000 status contacts\n- 50 group messages/day\n- 25 tag messages/day\n\nPremium Plan ($14/mo):\n- 50 status updates per day\n- 30,000 status contacts\n- Bulk import contacts\n- Delete old status updates\n\nUpgrade now:\n{{.UpgradeURL}}\n\nBest,\nThe {{.AppName}} Team",
		HTMLBody:    ts.wrapHTML("Unlock Full Power", ts.trialDay5Content()),
		Description: "Day 5 feature highlight",
	}

	ts.templates[domain.TemplateTrialDay6] = &domain.Template{
		Name:        domain.TemplateTrialDay6,
		Subject:     "Your {{.AppName}} trial ends tomorrow!",
		Body:        "Hi {{.Name}},\n\nHeads up - your {{.AppName}} trial ends tomorrow!\n\nAfter your trial:\n- Status broadcasting will stop working\n- Contact auto-save to Google Contacts will be disabled\n- Group messaging and tagging will stop\n\nDon't lose your WhatsApp automation! Upgrade now:\n{{.UpgradeURL}}\n\nBest,\nThe {{.AppName}} Team",
		HTMLBody:    ts.wrapHTML("Trial Ends Tomorrow!", ts.trialDay6Content()),
		Description: "Day 6 urgent reminder",
	}

	ts.templates[domain.TemplateTrialDay10] = &domain.Template{
		Name:        domain.TemplateTrialDay10,
		Subject:     "We miss you! Here's 30% off {{.AppName}}",
		Body:        "Hi {{.Name}},\n\nYour {{.AppName}} trial ended a few days ago, and we noticed you haven't upgraded yet.\n\nWe'd love to have you back! Use code COMEBACK30 for 30% off your first month.\n\nWith {{.AppName}}, you can:\n- Broadcast status to thousands of contacts instantly\n- Auto-save contacts to Google Contacts\n- Send messages to multiple groups and tag members\n\nUpgrade now:\n{{.UpgradeURL}}\n\nThis offer expires in 48 hours.\n\nBest,\nThe {{.AppName}} Team",
		HTMLBody:    ts.wrapHTML("We Miss You!", ts.trialDay10Content()),
		Description: "Day 10 win-back after expiry",
	}

	ts.templates[domain.TemplateAccountUpgraded] = &domain.Template{
		Name:        domain.TemplateAccountUpgraded,
		Subject:     "Welcome to {{.PlanName}} - Your upgrade is complete!",
		Body:        "Hi {{.Name}},\n\nCongratulations! You've been upgraded to {{.PlanName}}.\n\nYou now have access to:\n{{.Features}}\n\nStart using your new features now:\n{{.DashboardURL}}\n\nThank you for your support!",
		HTMLBody:    ts.wrapHTML("Upgrade Complete!", ts.accountUpgradedContent()),
		Description: "Sent when user upgrades their plan",
	}

	// Refund templates
	ts.templates[domain.TemplateRefundPending] = &domain.Template{
		Name:        domain.TemplateRefundPending,
		Subject:     "Refund Request Received - {{.Amount}}",
		Body:        "Hi {{.CustomerName}},\n\nWe have received your refund request for {{.Amount}}.\n\nTransaction ID: {{.TransactionID}}\n\nYour refund is being processed and should be completed within 5-10 business days.\n\nWe'll send you another email once the refund has been processed.\n\nIf you have any questions, please don't hesitate to contact our support team.",
		HTMLBody:    ts.wrapHTML("Refund Request Received", ts.refundPendingContent()),
		Description: "Sent when a refund request is initiated",
	}

	ts.templates[domain.TemplateRefundProcessed] = &domain.Template{
		Name:        domain.TemplateRefundProcessed,
		Subject:     "Refund Processed - {{.Amount}}",
		Body:        "Hi {{.CustomerName}},\n\nGreat news! Your refund of {{.Amount}} has been successfully processed.\n\nTransaction ID: {{.TransactionID}}\n\nThe funds should appear in your account within 5-10 business days, depending on your bank.\n\nThank you for your patience.",
		HTMLBody:    ts.wrapHTML("Refund Processed", ts.refundProcessedContent()),
		Description: "Sent when a refund has been processed successfully",
	}

	ts.templates[domain.TemplateRefundFailed] = &domain.Template{
		Name:        domain.TemplateRefundFailed,
		Subject:     "Refund Update - Action Required",
		Body:        "Hi {{.CustomerName}},\n\nWe were unable to process your refund of {{.Amount}}.\n\nTransaction ID: {{.TransactionID}}\nReason: {{.Reason}}\n\nPlease contact our support team to resolve this issue and complete your refund.\n\nWe apologize for any inconvenience.",
		HTMLBody:    ts.wrapHTML("Refund Issue", ts.refundFailedContent()),
		Description: "Sent when a refund could not be processed",
	}

	// Subscription reminder templates
	ts.templates[domain.TemplateSubscriptionReminder3d] = &domain.Template{
		Name:        domain.TemplateSubscriptionReminder3d,
		Subject:     "Your {{.PlanName}} subscription renews in 3 days",
		Body:        "Hi {{.CustomerName}},\n\nThis is a friendly reminder that your {{.PlanName}} subscription will automatically renew in 3 days.\n\nRenewal Date: {{.RenewalDate}}\nAmount: {{.Amount}}\n\nIf you'd like to make any changes to your subscription, you can manage it here:\n{{.ProfileURL}}\n\nThank you for being a valued customer!",
		HTMLBody:    ts.wrapHTML("Subscription Renewal Reminder", ts.subscriptionReminder3dContent()),
		Description: "Sent 3 days before subscription renewal",
	}

	ts.templates[domain.TemplateSubscriptionReminder1d] = &domain.Template{
		Name:        domain.TemplateSubscriptionReminder1d,
		Subject:     "Reminder: {{.PlanName}} renews tomorrow",
		Body:        "Hi {{.CustomerName}},\n\nThis is a final reminder that your {{.PlanName}} subscription will renew tomorrow.\n\nAmount: {{.Amount}}\n\nIf you need to update your payment method or make changes, please do so before the renewal:\n{{.ProfileURL}}\n\nThank you for your continued support!",
		HTMLBody:    ts.wrapHTML("Subscription Renews Tomorrow", ts.subscriptionReminder1dContent()),
		Description: "Sent 1 day before subscription renewal",
	}

	// Subscription expiration templates (for one-off payments)
	ts.templates[domain.TemplateSubscriptionExpiring3d] = &domain.Template{
		Name:        domain.TemplateSubscriptionExpiring3d,
		Subject:     "Your {{.PlanName}} access expires in 3 days",
		Body:        "Hi {{.CustomerName}},\n\nThis is a friendly reminder that your {{.PlanName}} access will expire in 3 days.\n\nExpiry Date: {{.ExpiryDate}}\n\nTo continue enjoying uninterrupted access, renew your subscription before it expires:\n{{.ProfileURL}}\n\nThank you for being a valued customer!",
		HTMLBody:    ts.wrapHTML("Subscription Expiring Soon", ts.subscriptionExpiring3dContent()),
		Description: "Sent 3 days before one-off subscription expires",
	}

	ts.templates[domain.TemplateSubscriptionExpiring1d] = &domain.Template{
		Name:        domain.TemplateSubscriptionExpiring1d,
		Subject:     "⚠️ Final Reminder: {{.PlanName}} expires tomorrow",
		Body:        "Hi {{.CustomerName}},\n\nThis is a final reminder that your {{.PlanName}} access will expire tomorrow.\n\nExpiry Date: {{.ExpiryDate}}\n\nDon't lose access! Renew now to continue using all features:\n{{.ProfileURL}}\n\nIf you have any questions, our support team is here to help.",
		HTMLBody:    ts.wrapHTML("Subscription Expires Tomorrow", ts.subscriptionExpiring1dContent()),
		Description: "Sent 1 day before one-off subscription expires",
	}

	// Authentication templates (Migration 022)
	ts.templates[domain.TemplateEmailVerification] = &domain.Template{
		Name:        domain.TemplateEmailVerification,
		Subject:     "Verify your {{.AppName}} account",
		Body:        "Hi {{.Name}},\n\nWelcome to {{.AppName}}! Please verify your email address by clicking the link below:\n\n{{.VerificationURL}}\n\nThis link will expire in 24 hours.\n\nIf you didn't create an account, you can safely ignore this email.\n\nBest,\nThe {{.AppName}} Team",
		HTMLBody:    ts.wrapHTML("Verify Your Email", ts.emailVerificationContent()),
		Description: "Sent when a user signs up to verify their email address",
	}

	ts.templates[domain.TemplatePasswordReset] = &domain.Template{
		Name:        domain.TemplatePasswordReset,
		Subject:     "Reset your {{.AppName}} password",
		Body:        "Hi {{.Name}},\n\nWe received a request to reset your password. Click the link below to set a new password:\n\n{{.ResetURL}}\n\nThis link will expire in 1 hour.\n\nIf you didn't request a password reset, you can safely ignore this email. Your password will remain unchanged.\n\nBest,\nThe {{.AppName}} Team",
		HTMLBody:    ts.wrapHTML("Reset Your Password", ts.passwordResetContent()),
		Description: "Sent when a user requests a password reset",
	}

	// Affiliate link update template
	ts.templates[domain.TemplateAffiliateLinkUpdated] = &domain.Template{
		Name:        domain.TemplateAffiliateLinkUpdated,
		Subject:     "Your affiliate link for {{.ProductName}} has been updated",
		Body:        "Hi {{.AffiliateName}},\n\nYour affiliate referral link for {{.ProductName}} has been updated.\n\nReason: {{.ChangeReason}}\n\nYour new affiliate link:\n{{.NewLink}}\n\nPlease update any promotional materials, websites, or social media posts that contain your old link to use your new link.\n\nYou can view all your affiliate links and manage your account from your dashboard:\n{{.DashboardURL}}\n\nIf you have any questions, please don't hesitate to contact our support team.\n\nBest,\nThe {{.Branding.CompanyName}} Team",
		HTMLBody:    ts.wrapHTML("Affiliate Link Updated", ts.affiliateLinkUpdatedContent()),
		Description: "Sent when an affiliate's referral link changes due to product URL updates",
	}

	// Migration campaign templates
	ts.templates[domain.TemplateMigrationAnnouncement] = &domain.Template{
		Name:    domain.TemplateMigrationAnnouncement,
		Subject: "WasBot has evolved - Your new platform is ready",
		Body:    "Hi {{.Name}},\n\nWe've rebuilt WasBot from the ground up, and your new platform is ready at wasbot.app.\n\nWhat's new:\n- Lightning-fast dashboard with real-time WhatsApp session monitoring\n- Auto-reply, scheduled messages, and bulk messaging\n- Team collaboration with role-based access\n- Reliable session management that stays connected\n\nYour legacy account will remain active, but all new features are exclusively on the new platform.\n\nGet started: {{.DashboardURL}}\n\nIf you have any questions, just reply to this email.\n\nBest,\nThe WasBot Team",
		HTMLBody:    ts.wrapHTML("WasBot Has Evolved", ts.migrationAnnouncementContent()),
		Description: "Migration announcement email for legacy users",
	}

	ts.templates[domain.TemplateMigrationFollowUp] = &domain.Template{
		Name:    domain.TemplateMigrationFollowUp,
		Subject: "Don't miss out - Claim your WasBot account",
		Body:    "Hi {{.Name}},\n\nWe noticed you haven't set up your new WasBot account yet.\n\nThe new platform at wasbot.app has everything you loved about WasBot, plus:\n- A completely redesigned dashboard\n- Faster, more reliable WhatsApp connections\n- New automation features\n\nIt only takes 2 minutes to get started.\n\nClaim your account: {{.DashboardURL}}\n\nIf you have any questions, just reply to this email.\n\nBest,\nThe WasBot Team",
		HTMLBody:    ts.wrapHTML("Don't Miss Out", ts.migrationFollowUpContent()),
		Description: "Follow-up email for legacy users who haven't migrated",
	}
}

// wrapHTML wraps content in a professional HTML email template using default branding.
// This is kept for backward compatibility - new code should use wrapHTMLWithBranding.
func (ts *TemplateService) wrapHTML(title, content string) string {
	return ts.wrapHTMLWithBranding(title, content)
}

// wrapHTMLWithBranding wraps content in a professional HTML email template with branding placeholders.
// The branding placeholders will be replaced at render time by applyBranding.
func (ts *TemplateService) wrapHTMLWithBranding(title, content string) string {
	return strings.TrimSpace(fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="X-UA-Compatible" content="IE=edge">
  <title>%s</title>
  <!--[if mso]>
  <noscript>
    <xml>
      <o:OfficeDocumentSettings>
        <o:PixelsPerInch>96</o:PixelsPerInch>
      </o:OfficeDocumentSettings>
    </xml>
  </noscript>
  <![endif]-->
  <style>
    /* Reset */
    body, table, td, p, a, li { -webkit-text-size-adjust: 100%%; -ms-text-size-adjust: 100%%; }
    table, td { mso-table-lspace: 0pt; mso-table-rspace: 0pt; }
    img { -ms-interpolation-mode: bicubic; border: 0; height: auto; line-height: 100%%; outline: none; text-decoration: none; }
    body { margin: 0 !important; padding: 0 !important; width: 100%% !important; }

    /* Client-specific */
    #outlook a { padding: 0; }
    .ReadMsgBody { width: 100%%; }
    .ExternalClass { width: 100%%; }
    .ExternalClass, .ExternalClass p, .ExternalClass span, .ExternalClass font, .ExternalClass td, .ExternalClass div { line-height: 100%%; }

    /* Base styles */
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
      background-color: #f3f4f6;
      margin: 0;
      padding: 0;
    }

    /* Links */
    a { color: {{.Branding.PrimaryColor}}; text-decoration: none; }
    a:hover { text-decoration: underline; }

    /* Button styles */
    .btn {
      display: inline-block;
      padding: 14px 28px;
      background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%%, {{.Branding.SecondaryColor}} 100%%);
      color: #ffffff !important;
      text-decoration: none;
      border-radius: 8px;
      font-weight: 600;
      font-size: 15px;
      box-shadow: 0 4px 14px 0 rgba(16, 185, 129, 0.39);
      transition: all 0.2s ease;
    }
    .btn:hover {
      background: linear-gradient(135deg, {{.Branding.SecondaryColor}} 0%%, {{.Branding.AccentColor}} 100%%);
      box-shadow: 0 6px 20px 0 rgba(16, 185, 129, 0.5);
    }
    .btn-danger {
      background: linear-gradient(135deg, {{.Branding.DangerColor}} 0%%, #dc2626 100%%);
      box-shadow: 0 4px 14px 0 rgba(239, 68, 68, 0.39);
    }

    /* Responsive */
    @media only screen and (max-width: 620px) {
      .container { width: 100%% !important; padding: 0 !important; }
      .content { padding: 24px 20px !important; }
      .header { padding: 24px 20px !important; }
      .footer-content { padding: 24px 20px !important; }
    }
  </style>
</head>
<body style="background-color: #f3f4f6; margin: 0; padding: 0;">
  <!-- Preview text - uses PreviewText if set, otherwise falls back to title -->
  <div style="display: none; max-height: 0; overflow: hidden;">
    {{if .PreviewText}}{{.PreviewText}}{{else}}%s{{end}}
    &nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;
  </div>

  <!-- Email wrapper -->
  <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%" style="background-color: #f3f4f6;">
    <tr>
      <td style="padding: 40px 20px;">
        <!-- Main container -->
        <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="600" align="center" class="container" style="max-width: 600px; margin: 0 auto;">

          <!-- Header with logo -->
          <tr>
            <td class="header" style="background: linear-gradient(135deg, #111827 0%%, #1f2937 100%%); padding: 32px 40px; border-radius: 16px 16px 0 0;">
              <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%">
                <tr>
                  <td>
                    <!-- Logo/Brand -->
                    <table role="presentation" cellspacing="0" cellpadding="0" border="0">
                      <tr>
                        <td style="vertical-align: middle; padding-right: 14px;">
                          <!-- Logo placeholder - uses image if LogoURL provided, otherwise first letter of company name -->
                          <div style="width: 44px; height: 44px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%%, {{.Branding.SecondaryColor}} 100%%); border-radius: 12px; text-align: center; line-height: 44px;">
                            {{.Branding.LogoContent}}
                          </div>
                        </td>
                        <td style="vertical-align: middle;">
                          <span style="color: #ffffff; font-size: 24px; font-weight: 700; letter-spacing: 1px;">{{.Branding.CompanyName}}</span>
                        </td>
                      </tr>
                    </table>
                  </td>
                </tr>
              </table>
            </td>
          </tr>

          <!-- Main content -->
          <tr>
            <td class="content" style="background-color: #ffffff; padding: 40px; border-left: 1px solid #e5e7eb; border-right: 1px solid #e5e7eb;">
              <!-- Title -->
              <h1 style="margin: 0 0 24px 0; font-size: 26px; font-weight: 700; color: #111827; line-height: 1.3;">%s</h1>

              <!-- Content -->
              <div style="font-size: 16px; line-height: 1.7; color: #4b5563;">
                %s
              </div>
            </td>
          </tr>

          <!-- Footer -->
          <tr>
            <td style="background-color: #f9fafb; padding: 0; border-radius: 0 0 16px 16px; border: 1px solid #e5e7eb; border-top: none;">
              <!-- Divider -->
              <div style="height: 1px; background: linear-gradient(to right, transparent, #e5e7eb, transparent); margin: 0 40px;"></div>

              <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%">
                <tr>
                  <td class="footer-content" style="padding: 32px 40px; text-align: center;">
                    <!-- Help text -->
                    <p style="margin: 0 0 20px 0; font-size: 14px; color: #6b7280; line-height: 1.6;">
                      Need help? Simply reply to this email and we'll get back to you.
                    </p>

                    <!-- Social links -->
                    <table role="presentation" cellspacing="0" cellpadding="0" border="0" align="center" style="margin-bottom: 20px;">
                      <tr>
                        <td style="padding: 0 8px;">
                          <a href="{{.Branding.SocialTwitter}}" style="display: inline-block;">
                            <img src="https://www.wasbot.app/icons/x.png" alt="X" width="24" height="24" style="opacity: 0.6;">
                          </a>
                        </td>
                        <td style="padding: 0 8px;">
                          <a href="{{.Branding.SocialInstagram}}" style="display: inline-block;">
                            <img src="https://www.wasbot.app/icons/instagram.png" alt="Instagram" width="24" height="24" style="opacity: 0.6;">
                          </a>
                        </td>
                      </tr>
                    </table>

                    <!-- Company info -->
                    <p style="margin: 0; font-size: 13px; color: #9ca3af; line-height: 1.5;">
                      {{.Branding.CompanyName}} Technologies<br>
                      Lagos, Nigeria
                    </p>

                    <!-- Legal -->
                    <p style="margin: 16px 0 0 0; font-size: 12px; color: #9ca3af;">
                      &copy; {{.Branding.Year}} {{.Branding.CompanyName}}. All rights reserved.
                    </p>
                  </td>
                </tr>
              </table>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>
`, title, title, title, content))
}

// paymentSuccessContent returns rich HTML content for payment success emails.
func (ts *TemplateService) paymentSuccessContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.CustomerName}},</p>

<p style="margin: 0 0 8px 0;">Thank you for subscribing to <strong>{{.ProductName}}</strong>! 🎉</p>

<p style="margin: 0 0 24px 0;">Your subscription is now <span style="color: {{.Branding.SecondaryColor}}; font-weight: 600;">active</span> and you have full access to all features.</p>

<!-- Subscription info box -->
{{if .IsRecurring}}
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #eff6ff; border-left: 4px solid #3b82f6; border-radius: 0 8px 8px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <p style="margin: 0; font-size: 14px; color: #1e40af;">
        <strong>📅 Auto-renewal:</strong> Your subscription will automatically renew on <strong>{{.NextBillingDate}}</strong>.<br>
        <span style="color: #6b7280;">We'll send you a reminder before your next payment.</span>
      </p>
    </td>
  </tr>
</table>
{{else}}
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef3c7; border-left: 4px solid #f59e0b; border-radius: 0 8px 8px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <p style="margin: 0; font-size: 14px; color: #92400e;">
        <strong>📅 Access expires:</strong> Your subscription will end on <strong>{{.ExpiryDate}}</strong>.<br>
        <span style="color: #6b7280;">We'll remind you before it expires so you don't lose access.</span>
      </p>
    </td>
  </tr>
</table>
{{end}}

<!-- Payment details card -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #f0fdf4; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding-bottom: 16px; border-bottom: 1px solid #bbf7d0;">
            <span style="font-size: 14px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Amount Paid</span><br>
            <span style="font-size: 32px; font-weight: 700; color: {{.Branding.SecondaryColor}};">{{.Amount}}</span>
          </td>
        </tr>
        <tr>
          <td style="padding-top: 16px;">
            <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
              <tr>
                <td width="50%" style="vertical-align: top;">
                  <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Plan</span><br>
                  <span style="font-size: 15px; font-weight: 600; color: #111827;">{{.ProductName}}</span>
                </td>
                <td width="50%" style="vertical-align: top;">
                  <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Transaction ID</span><br>
                  <span style="font-size: 13px; font-family: monospace; color: #111827;">{{.TransactionID}}</span>
                </td>
              </tr>
            </table>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<p style="margin: 0 0 24px 0; color: #6b7280;">You're all set! Start using {{.Branding.CompanyName}} to automate your WhatsApp business today.</p>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.Branding.DashboardURL}}" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        Go to Dashboard →
      </a>
    </td>
  </tr>
</table>
`
}

// subscriptionReminder3dContent returns content for 3-day renewal reminder.
func (ts *TemplateService) subscriptionReminder3dContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.CustomerName}},</p>

<p style="margin: 0 0 24px 0;">This is a friendly reminder that your <strong>{{.PlanName}}</strong> subscription will automatically renew in <strong>3 days</strong>.</p>

<!-- Renewal info card -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #eff6ff; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td width="50%" style="vertical-align: top; padding-right: 12px;">
            <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Renewal Date</span><br>
            <span style="font-size: 18px; font-weight: 600; color: #1e40af;">{{.RenewalDate}}</span>
          </td>
          <td width="50%" style="vertical-align: top; padding-left: 12px;">
            <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Amount</span><br>
            <span style="font-size: 18px; font-weight: 600; color: #1e40af;">{{.Amount}}</span>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<p style="margin: 0 0 24px 0; color: #6b7280;">No action is needed if you wish to continue. Your subscription will renew automatically using your saved payment method.</p>

<p style="margin: 0 0 24px 0; color: #6b7280;">Need to make changes? You can update your payment method or cancel anytime from your account settings.</p>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);">
      <a href="{{.ProfileURL}}" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        Manage Subscription
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #9ca3af;">Thank you for being a valued {{.Branding.CompanyName}} customer!</p>
`
}

// subscriptionReminder1dContent returns content for 1-day renewal reminder.
func (ts *TemplateService) subscriptionReminder1dContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.CustomerName}},</p>

<p style="margin: 0 0 24px 0;">⏰ <strong>Final reminder:</strong> Your <strong>{{.PlanName}}</strong> subscription renews <strong>tomorrow</strong>.</p>

<!-- Renewal info card -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef3c7; border-left: 4px solid #f59e0b; border-radius: 0 12px 12px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 20px 24px;">
      <p style="margin: 0; font-size: 16px; color: #92400e;">
        <strong>{{.Amount}}</strong> will be charged to your payment method tomorrow.
      </p>
    </td>
  </tr>
</table>

<p style="margin: 0 0 24px 0; color: #6b7280;">If you need to update your payment method or make any changes, please do so before the renewal.</p>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, #f59e0b 0%, #d97706 100%);">
      <a href="{{.ProfileURL}}" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        Review Subscription
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #6b7280;">Thank you for your continued support!</p>
<p style="margin: 8px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.Branding.CompanyName}} Team</p>
`
}

// subscriptionExpiring3dContent returns content for 3-day expiry reminder.
func (ts *TemplateService) subscriptionExpiring3dContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.CustomerName}},</p>

<p style="margin: 0 0 24px 0;">This is a friendly reminder that your <strong>{{.PlanName}}</strong> access will expire in <strong>3 days</strong>.</p>

<!-- Expiry info card -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef3c7; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px; text-align: center;">
      <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Access Expires On</span><br>
      <span style="font-size: 24px; font-weight: 700; color: #92400e;">{{.ExpiryDate}}</span>
    </td>
  </tr>
</table>

<p style="margin: 0 0 8px 0; color: #4b5563;"><strong>What happens after expiry?</strong></p>
<ul style="margin: 0 0 24px 0; padding-left: 20px; color: #6b7280;">
  <li style="margin-bottom: 8px;">You'll lose access to all {{.Branding.CompanyName}} features</li>
  <li style="margin-bottom: 8px;">Your automations will stop running</li>
  <li>Your data will be preserved for 30 days</li>
</ul>

<p style="margin: 0 0 24px 0; color: #4b5563;">Renew now to continue enjoying uninterrupted service!</p>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.ProfileURL}}" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        Renew Now →
      </a>
    </td>
  </tr>
</table>
`
}

// subscriptionExpiring1dContent returns content for 1-day expiry reminder (urgent).
func (ts *TemplateService) subscriptionExpiring1dContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.CustomerName}},</p>

<p style="margin: 0 0 24px 0;">⚠️ <strong>Your {{.PlanName}} access expires tomorrow!</strong></p>

<!-- Urgent expiry card -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef2f2; border-left: 4px solid {{.Branding.DangerColor}}; border-radius: 0 12px 12px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 20px 24px;">
      <p style="margin: 0 0 8px 0; font-size: 18px; font-weight: 600; color: #991b1b;">
        Expiry Date: {{.ExpiryDate}}
      </p>
      <p style="margin: 0; font-size: 14px; color: #b91c1c;">
        After this date, your {{.Branding.CompanyName}} automations will stop working.
      </p>
    </td>
  </tr>
</table>

<p style="margin: 0 0 24px 0; color: #4b5563;">Don't lose access to your WhatsApp automations! Renew now to keep everything running smoothly.</p>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.DangerColor}} 0%, #dc2626 100%);">
      <a href="{{.ProfileURL}}" target="_blank" style="display: inline-block; padding: 16px 40px; font-size: 16px; font-weight: 700; color: #ffffff; text-decoration: none;">
        Renew Now — Keep My Access
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #6b7280;">Questions? Reply to this email and we'll help you out.</p>
<p style="margin: 8px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.Branding.CompanyName}} Team</p>
`
}

// refundPendingContent returns content for refund pending emails.
func (ts *TemplateService) refundPendingContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.CustomerName}},</p>

<p style="margin: 0 0 24px 0;">We've received your refund request and it's being processed.</p>

<!-- Refund status card -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #eff6ff; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <!-- Status badge -->
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin-bottom: 16px;">
        <tr>
          <td style="background-color: #dbeafe; border-radius: 20px; padding: 6px 14px;">
            <span style="font-size: 13px; font-weight: 600; color: #1d4ed8;">⏳ Processing</span>
          </td>
        </tr>
      </table>

      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding-bottom: 12px;">
            <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Refund Amount</span><br>
            <span style="font-size: 28px; font-weight: 700; color: #1e40af;">{{.Amount}}</span>
          </td>
        </tr>
        <tr>
          <td>
            <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Transaction ID</span><br>
            <span style="font-size: 14px; font-family: monospace; color: #374151;">{{.TransactionID}}</span>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<p style="margin: 0 0 8px 0; color: #4b5563;"><strong>What happens next?</strong></p>
<ul style="margin: 0 0 24px 0; padding-left: 20px; color: #6b7280;">
  <li style="margin-bottom: 8px;">Your refund is being reviewed by our team</li>
  <li style="margin-bottom: 8px;">Processing typically takes 5-10 business days</li>
  <li>We'll email you once the refund is complete</li>
</ul>

<p style="margin: 0; font-size: 14px; color: #9ca3af;">Thank you for your patience. If you have any questions, just reply to this email.</p>
`
}

// refundProcessedContent returns content for successful refund emails.
func (ts *TemplateService) refundProcessedContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.CustomerName}},</p>

<p style="margin: 0 0 24px 0;">Great news! Your refund has been successfully processed. 🎉</p>

<!-- Refund success card -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #f0fdf4; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <!-- Status badge -->
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin-bottom: 16px;">
        <tr>
          <td style="background-color: #bbf7d0; border-radius: 20px; padding: 6px 14px;">
            <span style="font-size: 13px; font-weight: 600; color: #166534;">✓ Completed</span>
          </td>
        </tr>
      </table>

      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding-bottom: 12px;">
            <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Amount Refunded</span><br>
            <span style="font-size: 28px; font-weight: 700; color: {{.Branding.SecondaryColor}};">{{.Amount}}</span>
          </td>
        </tr>
        <tr>
          <td>
            <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Transaction ID</span><br>
            <span style="font-size: 14px; font-family: monospace; color: #374151;">{{.TransactionID}}</span>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- Timeline -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fefce8; border-left: 4px solid #eab308; border-radius: 0 8px 8px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <p style="margin: 0; font-size: 14px; color: #854d0e;">
        <strong>💳 When will I receive my money?</strong><br>
        <span style="color: #a16207;">The funds should appear in your account within 5-10 business days, depending on your bank.</span>
      </p>
    </td>
  </tr>
</table>

<p style="margin: 0; font-size: 14px; color: #6b7280;">Thank you for your patience throughout this process. We hope to serve you again in the future!</p>
<p style="margin: 16px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.Branding.CompanyName}} Team</p>
`
}

// refundFailedContent returns content for failed refund emails.
func (ts *TemplateService) refundFailedContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.CustomerName}},</p>

<p style="margin: 0 0 24px 0;">Unfortunately, we encountered an issue processing your refund.</p>

<!-- Refund failed card -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef2f2; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <!-- Status badge -->
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin-bottom: 16px;">
        <tr>
          <td style="background-color: #fecaca; border-radius: 20px; padding: 6px 14px;">
            <span style="font-size: 13px; font-weight: 600; color: #991b1b;">✗ Action Required</span>
          </td>
        </tr>
      </table>

      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding-bottom: 12px;">
            <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Refund Amount</span><br>
            <span style="font-size: 28px; font-weight: 700; color: #dc2626;">{{.Amount}}</span>
          </td>
        </tr>
        <tr>
          <td style="padding-bottom: 12px;">
            <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Transaction ID</span><br>
            <span style="font-size: 14px; font-family: monospace; color: #374151;">{{.TransactionID}}</span>
          </td>
        </tr>
        <tr>
          <td>
            <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Reason</span><br>
            <span style="font-size: 14px; color: #991b1b;">{{.Reason}}</span>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<p style="margin: 0 0 24px 0; color: #4b5563;">Please reply to this email with your preferred resolution, and our team will assist you promptly.</p>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.DangerColor}} 0%, #dc2626 100%);">
      <a href="mailto:{{.Branding.SupportEmail}}?subject=Refund Issue - {{.TransactionID}}" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        Contact Support
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #6b7280;">We sincerely apologize for the inconvenience and will work to resolve this as quickly as possible.</p>
<p style="margin: 8px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.Branding.CompanyName}} Team</p>
`
}

// commissionRefundedContent returns content for commission reversal emails.
func (ts *TemplateService) commissionRefundedContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.Name}},</p>

<p style="margin: 0 0 24px 0;">A commission you previously earned has been reversed due to a refund.</p>

<!-- Commission reversal card -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef3c7; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <!-- Status badge -->
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin-bottom: 16px;">
        <tr>
          <td style="background-color: #fde68a; border-radius: 20px; padding: 6px 14px;">
            <span style="font-size: 13px; font-weight: 600; color: #92400e;">↩ Reversed</span>
          </td>
        </tr>
      </table>

      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding-bottom: 12px;">
            <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Commission Amount</span><br>
            <span style="font-size: 28px; font-weight: 700; color: #92400e;">{{.Amount}}</span>
          </td>
        </tr>
        <tr>
          <td style="padding-bottom: 12px;">
            <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Product</span><br>
            <span style="font-size: 15px; font-weight: 600; color: #374151;">{{.ProductName}}</span>
          </td>
        </tr>
        <tr>
          <td>
            <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Reason</span><br>
            <span style="font-size: 14px; color: #92400e;">{{.Reason}}</span>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<p style="margin: 0 0 8px 0; color: #4b5563;"><strong>What does this mean?</strong></p>
<ul style="margin: 0 0 24px 0; padding-left: 20px; color: #6b7280;">
  <li style="margin-bottom: 8px;">The original customer requested a refund for their purchase</li>
  <li style="margin-bottom: 8px;">This commission has been deducted from your available balance</li>
  <li>Your pending payouts may be adjusted accordingly</li>
</ul>

<p style="margin: 0; font-size: 14px; color: #6b7280;">This is a standard part of the affiliate program. If you have any questions, please don't hesitate to reach out.</p>
<p style="margin: 16px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.Branding.CompanyName}} Team</p>
`
}

// accessRevokedContent returns content for access revocation emails.
func (ts *TemplateService) accessRevokedContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.CustomerName}},</p>

<p style="margin: 0 0 24px 0;">Your refund has been processed and your access has been revoked.</p>

<!-- Access revoked card -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef2f2; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <!-- Status badge -->
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin-bottom: 16px;">
        <tr>
          <td style="background-color: #fecaca; border-radius: 20px; padding: 6px 14px;">
            <span style="font-size: 13px; font-weight: 600; color: #991b1b;">Access Revoked</span>
          </td>
        </tr>
      </table>

      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding-bottom: 12px;">
            <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Refund Amount</span><br>
            <span style="font-size: 28px; font-weight: 700; color: #dc2626;">{{.Amount}}</span>
          </td>
        </tr>
        <tr>
          <td style="padding-bottom: 12px;">
            <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Product</span><br>
            <span style="font-size: 15px; font-weight: 600; color: #374151;">{{.ProductName}}</span>
          </td>
        </tr>
        <tr>
          <td>
            <span style="font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Transaction ID</span><br>
            <span style="font-size: 14px; font-family: monospace; color: #374151;">{{.TransactionID}}</span>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<p style="margin: 0 0 8px 0; color: #4b5563;"><strong>What happens now?</strong></p>
<ul style="margin: 0 0 24px 0; padding-left: 20px; color: #6b7280;">
  <li style="margin-bottom: 8px;">Your access to {{.ProductName}} has been immediately revoked</li>
  <li style="margin-bottom: 8px;">The refund amount will appear in your account within 5-10 business days</li>
  <li>Any associated data or settings have been preserved for 30 days</li>
</ul>

<p style="margin: 0 0 24px 0; color: #4b5563;">Changed your mind? You can resubscribe anytime to regain access.</p>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.Branding.WebsiteURL}}/pricing" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        View Plans
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #6b7280;">If you have any questions or believe this was done in error, please contact our support team.</p>
<p style="margin: 8px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.Branding.CompanyName}} Team</p>
`
}

// buttonHTML returns an email-compatible button.
func (ts *TemplateService) buttonHTML(text, url, color string) string {
	if color == "" {
		color = "#10b981"
	}
	return fmt.Sprintf(`
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 24px 0;">
  <tr>
    <td style="border-radius: 8px; background: %s;">
      <a href="%s" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        %s
      </a>
    </td>
  </tr>
</table>
`, color, url, text)
}

// trialDay3Content returns content for day 3 engagement email.
func (ts *TemplateService) trialDay3Content() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.Name}},</p>

<p style="margin: 0 0 24px 0;">You've been using <strong>{{.AppName}}</strong> for 3 days now! How's it going? We'd love to hear your thoughts.</p>

<!-- Feature suggestions -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #f0fdf4; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <p style="margin: 0 0 16px 0; font-weight: 600; color: #166534;">Have you tried these features yet?</p>
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: {{.Branding.PrimaryColor}}; font-size: 18px;">📢</span>
            <span style="margin-left: 12px; color: #374151;"><strong>Status Broadcasting</strong> - Post status updates to all your contacts at once</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: {{.Branding.PrimaryColor}}; font-size: 18px;">📇</span>
            <span style="margin-left: 12px; color: #374151;"><strong>Google Contacts Sync</strong> - Auto-save new WhatsApp contacts</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: {{.Branding.PrimaryColor}}; font-size: 18px;">👥</span>
            <span style="margin-left: 12px; color: #374151;"><strong>Group Messaging</strong> - Send to multiple groups and tag members</span>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<p style="margin: 0 0 24px 0; color: #6b7280;">Need help getting started? Just reply to this email and we'll walk you through it!</p>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.DashboardURL}}" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        Continue Exploring →
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.AppName}} Team</p>
`
}

// trialDay5Content returns content for day 5 feature highlight email.
func (ts *TemplateService) trialDay5Content() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.Name}},</p>

<p style="margin: 0 0 24px 0;">You're <strong>halfway through your trial!</strong> Here's what you unlock when you upgrade:</p>

<!-- Pricing comparison -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="margin-bottom: 24px;">
  <tr>
    <!-- Basic Plan -->
    <td width="48%" style="vertical-align: top; padding-right: 2%;">
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #f0fdf4; border-radius: 12px; border: 2px solid #bbf7d0;">
        <tr>
          <td style="padding: 20px;">
            <p style="margin: 0 0 4px 0; font-size: 14px; color: #166534; font-weight: 600;">BASIC</p>
            <p style="margin: 0 0 16px 0; font-size: 28px; font-weight: 700; color: #15803d;">$5.50<span style="font-size: 14px; font-weight: 400; color: #6b7280;">/mo</span></p>
            <ul style="margin: 0; padding-left: 18px; font-size: 13px; color: #374151; line-height: 1.8;">
              <li>15 status updates/day</li>
              <li>5,000 status contacts</li>
              <li>50 group messages/day</li>
              <li>25 tag messages/day</li>
            </ul>
          </td>
        </tr>
      </table>
    </td>
    <!-- Premium Plan -->
    <td width="48%" style="vertical-align: top; padding-left: 2%;">
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #eff6ff; border-radius: 12px; border: 2px solid #3b82f6;">
        <tr>
          <td style="padding: 20px;">
            <p style="margin: 0 0 4px 0; font-size: 14px; color: #1d4ed8; font-weight: 600;">PREMIUM ⭐</p>
            <p style="margin: 0 0 16px 0; font-size: 28px; font-weight: 700; color: #1e40af;">$14<span style="font-size: 14px; font-weight: 400; color: #6b7280;">/mo</span></p>
            <ul style="margin: 0; padding-left: 18px; font-size: 13px; color: #374151; line-height: 1.8;">
              <li>50 status updates/day</li>
              <li>30,000 status contacts</li>
              <li><strong>Bulk import contacts</strong></li>
              <li><strong>Delete old status</strong></li>
            </ul>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- Enterprise mention -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #faf5ff; border-left: 4px solid #a855f7; border-radius: 0 12px 12px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <p style="margin: 0; font-size: 14px; color: #7e22ce;">
        <strong>Need more?</strong> Enterprise ($35/mo) gives you unlimited status, 3 WhatsApp sessions, and cross-posting across sessions.
      </p>
    </td>
  </tr>
</table>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.UpgradeURL}}" target="_blank" style="display: inline-block; padding: 16px 40px; font-size: 16px; font-weight: 700; color: #ffffff; text-decoration: none;">
        View Plans & Upgrade
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.AppName}} Team</p>
`
}

// trialDay6Content returns content for day 6 urgent reminder email.
func (ts *TemplateService) trialDay6Content() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.Name}},</p>

<p style="margin: 0 0 24px 0;">⚠️ <strong>Your {{.AppName}} trial ends tomorrow!</strong></p>

<!-- What happens next -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef2f2; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <p style="margin: 0 0 16px 0; font-weight: 600; color: #991b1b;">After your trial ends:</p>
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: #dc2626;">✗</span>
            <span style="margin-left: 12px; color: #7f1d1d;">Status broadcasting will stop working</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: #dc2626;">✗</span>
            <span style="margin-left: 12px; color: #7f1d1d;">Contact auto-save to Google Contacts will be disabled</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: #dc2626;">✗</span>
            <span style="margin-left: 12px; color: #7f1d1d;">Group messaging and tagging will stop</span>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- Keep your progress -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #f0fdf4; border-left: 4px solid {{.Branding.PrimaryColor}}; border-radius: 0 12px 12px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 20px 24px;">
      <p style="margin: 0; font-size: 16px; color: #166534;">
        <strong>Good news:</strong> Upgrade now and keep your WhatsApp connection, contacts, and all your settings intact!
      </p>
    </td>
  </tr>
</table>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.DangerColor}} 0%, #dc2626 100%);">
      <a href="{{.UpgradeURL}}" target="_blank" style="display: inline-block; padding: 16px 40px; font-size: 16px; font-weight: 700; color: #ffffff; text-decoration: none;">
        Upgrade Now — Keep My Access
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #6b7280;">Questions? Reply to this email and we'll help you out.</p>
<p style="margin: 8px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.AppName}} Team</p>
`
}

// trialDay10Content returns content for day 10 win-back email.
func (ts *TemplateService) trialDay10Content() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.Name}},</p>

<p style="margin: 0 0 24px 0;">Your {{.AppName}} trial ended a few days ago, and we noticed you haven't upgraded yet. <strong>We'd love to have you back!</strong></p>

<!-- What you're missing -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #f0fdf4; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 20px;">
      <p style="margin: 0 0 12px 0; font-weight: 600; color: #166534;">What you're missing out on:</p>
      <ul style="margin: 0; padding-left: 20px; font-size: 14px; color: #374151; line-height: 1.8;">
        <li>Broadcast status to thousands of WhatsApp contacts instantly</li>
        <li>Auto-save every new contact to Google Contacts</li>
        <li>Send messages to multiple groups at once</li>
        <li>Tag and mention members in group messages</li>
      </ul>
    </td>
  </tr>
</table>

<!-- Special offer -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background: linear-gradient(135deg, #fef3c7 0%, #fde68a 100%); border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 32px; text-align: center;">
      <p style="margin: 0 0 8px 0; font-size: 14px; color: #92400e; text-transform: uppercase; letter-spacing: 1px;">Exclusive Comeback Offer</p>
      <p style="margin: 0 0 16px 0; font-size: 48px; font-weight: 800; color: #78350f;">30% OFF</p>
      <p style="margin: 0; font-size: 16px; color: #92400e;">Use code: <strong style="background: #78350f; color: white; padding: 4px 12px; border-radius: 4px; font-family: monospace;">COMEBACK30</strong></p>
    </td>
  </tr>
</table>

<!-- Urgency -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef2f2; border-left: 4px solid {{.Branding.DangerColor}}; border-radius: 0 12px 12px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <p style="margin: 0; font-size: 14px; color: #991b1b;">
        <strong>⏰ This offer expires in 48 hours!</strong>
      </p>
    </td>
  </tr>
</table>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.UpgradeURL}}" target="_blank" style="display: inline-block; padding: 16px 40px; font-size: 16px; font-weight: 700; color: #ffffff; text-decoration: none;">
        Claim 30% Off Now →
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #6b7280;">If you have any questions or feedback about your trial experience, we'd love to hear from you.</p>
<p style="margin: 8px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.AppName}} Team</p>
`
}

// welcomeContent returns content for welcome email.
func (ts *TemplateService) welcomeContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.Name}},</p>

<p style="margin: 0 0 24px 0;">Welcome to <strong>{{.AppName}}</strong>! We're thrilled to have you on board. Your 7-day free trial starts now.</p>

<!-- What you get -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #f0fdf4; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <p style="margin: 0 0 16px 0; font-weight: 600; color: #166534;">Your trial includes:</p>
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding: 10px 0;">
            <span style="color: {{.Branding.PrimaryColor}}; font-size: 18px;">📢</span>
            <span style="margin-left: 12px; color: #374151;"><strong>Status Broadcasting</strong> - Post to up to 1,000 contacts at once</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 10px 0;">
            <span style="color: {{.Branding.PrimaryColor}}; font-size: 18px;">📇</span>
            <span style="margin-left: 12px; color: #374151;"><strong>Google Contacts Sync</strong> - Auto-save new contacts automatically</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 10px 0;">
            <span style="color: {{.Branding.PrimaryColor}}; font-size: 18px;">👥</span>
            <span style="margin-left: 12px; color: #374151;"><strong>Group Messaging</strong> - Send to multiple groups + tag members</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 10px 0;">
            <span style="color: {{.Branding.PrimaryColor}}; font-size: 18px;">📱</span>
            <span style="margin-left: 12px; color: #374151;"><strong>5 Status/Day</strong> - Post up to 5 status updates daily</span>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- Getting started steps -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #eff6ff; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <p style="margin: 0 0 16px 0; font-weight: 600; color: #1e40af;">Get started in 3 steps:</p>
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding: 8px 0;">
            <span style="background: #3b82f6; color: white; border-radius: 50%; width: 24px; height: 24px; display: inline-block; text-align: center; line-height: 24px; font-size: 13px; font-weight: 600;">1</span>
            <span style="margin-left: 12px; color: #374151;">Connect your WhatsApp by scanning the QR code</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="background: #3b82f6; color: white; border-radius: 50%; width: 24px; height: 24px; display: inline-block; text-align: center; line-height: 24px; font-size: 13px; font-weight: 600;">2</span>
            <span style="margin-left: 12px; color: #374151;">Link your Google account for contact sync</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="background: #3b82f6; color: white; border-radius: 50%; width: 24px; height: 24px; display: inline-block; text-align: center; line-height: 24px; font-size: 13px; font-weight: 600;">3</span>
            <span style="margin-left: 12px; color: #374151;">Create your first status broadcast!</span>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.Branding.DashboardURL}}" target="_blank" style="display: inline-block; padding: 16px 40px; font-size: 16px; font-weight: 700; color: #ffffff; text-decoration: none;">
        Go to Dashboard →
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #6b7280;">Need help getting started? Just reply to this email!</p>
<p style="margin: 8px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.AppName}} Team</p>
`
}

// trialExpiringContent returns content for trial expiring email.
func (ts *TemplateService) trialExpiringContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.Name}},</p>

<p style="margin: 0 0 24px 0;">Your free trial of <strong>{{.AppName}}</strong> expires in <strong>{{.Days}} days</strong>.</p>

<!-- What you'll lose -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef2f2; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <p style="margin: 0 0 16px 0; font-weight: 600; color: #991b1b;">Don't lose access to:</p>
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: #dc2626;">✗</span>
            <span style="margin-left: 12px; color: #7f1d1d;">Status broadcasting to your contacts</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: #dc2626;">✗</span>
            <span style="margin-left: 12px; color: #7f1d1d;">Auto-save contacts to Google Contacts</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: #dc2626;">✗</span>
            <span style="margin-left: 12px; color: #7f1d1d;">Group messaging and tagging</span>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.UpgradeURL}}" target="_blank" style="display: inline-block; padding: 16px 40px; font-size: 16px; font-weight: 700; color: #ffffff; text-decoration: none;">
        Upgrade Now
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #6b7280;">Questions? Reply to this email.</p>
<p style="margin: 8px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.AppName}} Team</p>
`
}

// accountUpgradedContent returns content for account upgrade email.
func (ts *TemplateService) accountUpgradedContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.Name}},</p>

<p style="margin: 0 0 24px 0;">Congratulations! 🎉 You've been upgraded to <strong>{{.PlanName}}</strong>.</p>

<!-- Success message -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #f0fdf4; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px; text-align: center;">
      <div style="font-size: 48px; margin-bottom: 16px;">✓</div>
      <p style="margin: 0 0 8px 0; font-size: 20px; font-weight: 700; color: #166534;">Upgrade Complete!</p>
      <p style="margin: 0; font-size: 14px; color: #6b7280;">Your new features are now active</p>
    </td>
  </tr>
</table>

<!-- What's unlocked -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #eff6ff; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <p style="margin: 0 0 16px 0; font-weight: 600; color: #1e40af;">You now have access to:</p>
      <div style="font-size: 14px; color: #374151; line-height: 1.8;">
        {{.FeaturesHTML}}
      </div>
    </td>
  </tr>
</table>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.DashboardURL}}" target="_blank" style="display: inline-block; padding: 16px 40px; font-size: 16px; font-weight: 700; color: #ffffff; text-decoration: none;">
        Start Using New Features →
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #6b7280;">Thank you for your support! If you have any questions, just reply to this email.</p>
<p style="margin: 8px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.Branding.CompanyName}} Team</p>
`
}

// subscriptionRenewedContent returns content for recurring subscription renewal email.
func (ts *TemplateService) subscriptionRenewedContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.CustomerName}},</p>

<p style="margin: 0 0 24px 0;">Your <strong>{{.PlanName}}</strong> subscription has been automatically renewed.</p>

<!-- Payment details -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #f0fdf4; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding: 8px 0; border-bottom: 1px solid #bbf7d0;">
            <span style="color: #6b7280;">Amount charged:</span>
            <span style="float: right; font-weight: 600; color: #166534;">{{.Amount}}</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: #6b7280;">Next billing date:</span>
            <span style="float: right; font-weight: 600; color: #374151;">{{.NextBillingDate}}</span>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- Auto-renewal notice -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #eff6ff; border-left: 4px solid #3b82f6; border-radius: 0 12px 12px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <p style="margin: 0; font-size: 14px; color: #1e40af;">
        <strong>Auto-renewal enabled:</strong> Your subscription will automatically renew. You can manage or cancel anytime from your dashboard.
      </p>
    </td>
  </tr>
</table>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.ProfileURL}}" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        Manage Subscription
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #6b7280;">Thank you for your continued support!</p>
<p style="margin: 8px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.Branding.CompanyName}} Team</p>
`
}

// subscriptionCancelledContent returns content for subscription cancellation email.
func (ts *TemplateService) subscriptionCancelledContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.CustomerName}},</p>

<p style="margin: 0 0 24px 0;">Your <strong>{{.PlanName}}</strong> subscription has been cancelled.</p>

<!-- Access info -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef3c7; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px; text-align: center;">
      <p style="margin: 0 0 8px 0; font-size: 14px; color: #92400e;">You still have access until</p>
      <p style="margin: 0; font-size: 28px; font-weight: 700; color: #78350f;">{{.ExpiryDate}}</p>
    </td>
  </tr>
</table>

<!-- No auto-renewal notice -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #f0fdf4; border-left: 4px solid #22c55e; border-radius: 0 12px 12px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <p style="margin: 0; font-size: 14px; color: #166534;">
        <strong>✓ No more charges:</strong> Your subscription will NOT automatically renew. You will not be charged again unless you resubscribe.
      </p>
    </td>
  </tr>
</table>

<p style="margin: 0 0 24px 0; color: #6b7280;">We're sorry to see you go. If you change your mind, you can resubscribe anytime.</p>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.ProfileURL}}" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        Resubscribe
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.Branding.CompanyName}} Team</p>
`
}

// paymentSuccessOnetimeContent returns content for one-time payment success email.
func (ts *TemplateService) paymentSuccessOnetimeContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.CustomerName}},</p>

<p style="margin: 0 0 24px 0;">Thank you for your purchase! Your payment has been confirmed.</p>

<!-- Payment details -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #f0fdf4; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <div style="text-align: center; margin-bottom: 16px;">
        <span style="font-size: 48px;">✓</span>
      </div>
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding: 8px 0; border-bottom: 1px solid #bbf7d0;">
            <span style="color: #6b7280;">Product:</span>
            <span style="float: right; font-weight: 600; color: #374151;">{{.ProductName}}</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0; border-bottom: 1px solid #bbf7d0;">
            <span style="color: #6b7280;">Duration:</span>
            <span style="float: right; font-weight: 600; color: #374151;">{{.Duration}}</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0; border-bottom: 1px solid #bbf7d0;">
            <span style="color: #6b7280;">Amount paid:</span>
            <span style="float: right; font-weight: 600; color: #166534;">{{.Amount}}</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: #6b7280;">Access until:</span>
            <span style="float: right; font-weight: 600; color: #374151;">{{.ExpiryDate}}</span>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- IMPORTANT: One-time notice -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef3c7; border-left: 4px solid #f59e0b; border-radius: 0 12px 12px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <p style="margin: 0 0 8px 0; font-size: 14px; font-weight: 600; color: #92400e;">⚠️ One-Time Purchase</p>
      <p style="margin: 0; font-size: 14px; color: #92400e;">
        This subscription will <strong>NOT automatically renew</strong>. Your access will end on {{.ExpiryDate}}. We'll send you reminders before it expires so you can renew manually if you'd like to continue.
      </p>
    </td>
  </tr>
</table>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.DashboardURL}}" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        Go to Dashboard →
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #6b7280;">Transaction ID: {{.TransactionID}}</p>
<p style="margin: 8px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.Branding.CompanyName}} Team</p>
`
}

// subscriptionActivatedContent returns content for new recurring subscription activation.
func (ts *TemplateService) subscriptionActivatedContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.CustomerName}},</p>

<p style="margin: 0 0 24px 0;">Welcome! Your <strong>{{.PlanName}}</strong> subscription is now active. 🎉</p>

<!-- Subscription details -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #f0fdf4; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding: 8px 0; border-bottom: 1px solid #bbf7d0;">
            <span style="color: #6b7280;">Plan:</span>
            <span style="float: right; font-weight: 600; color: #374151;">{{.PlanName}}</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0; border-bottom: 1px solid #bbf7d0;">
            <span style="color: #6b7280;">Amount:</span>
            <span style="float: right; font-weight: 600; color: #166534;">{{.Amount}}/{{.Interval}}</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: #6b7280;">Next billing date:</span>
            <span style="float: right; font-weight: 600; color: #374151;">{{.NextBillingDate}}</span>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- Auto-renewal notice -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #eff6ff; border-left: 4px solid #3b82f6; border-radius: 0 12px 12px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <p style="margin: 0; font-size: 14px; color: #1e40af;">
        <strong>Auto-renewal enabled:</strong> Your subscription renews {{.Interval}}. You can manage or cancel anytime from your dashboard.
      </p>
    </td>
  </tr>
</table>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.DashboardURL}}" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        Get Started →
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #6b7280;">Thank you for choosing {{.Branding.CompanyName}}!</p>
<p style="margin: 8px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.Branding.CompanyName}} Team</p>
`
}

// subscriptionActivatedOnetimeContent returns content for new one-time subscription activation.
func (ts *TemplateService) subscriptionActivatedOnetimeContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.CustomerName}},</p>

<p style="margin: 0 0 24px 0;">Welcome! Your <strong>{{.PlanName}}</strong> access is now active. 🎉</p>

<!-- Subscription details -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #f0fdf4; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding: 8px 0; border-bottom: 1px solid #bbf7d0;">
            <span style="color: #6b7280;">Plan:</span>
            <span style="float: right; font-weight: 600; color: #374151;">{{.PlanName}}</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0; border-bottom: 1px solid #bbf7d0;">
            <span style="color: #6b7280;">Duration:</span>
            <span style="float: right; font-weight: 600; color: #374151;">{{.Duration}}</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: #6b7280;">Access until:</span>
            <span style="float: right; font-weight: 600; color: #166534;">{{.ExpiryDate}}</span>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- IMPORTANT: One-time notice -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef3c7; border-left: 4px solid #f59e0b; border-radius: 0 12px 12px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <p style="margin: 0 0 8px 0; font-size: 14px; font-weight: 600; color: #92400e;">⚠️ One-Time Purchase</p>
      <p style="margin: 0; font-size: 14px; color: #92400e;">
        This is a one-time purchase. Your access will <strong>NOT automatically renew</strong>. We'll send you reminders before {{.ExpiryDate}} so you can renew manually if you'd like to continue.
      </p>
    </td>
  </tr>
</table>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.DashboardURL}}" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        Get Started →
      </a>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #6b7280;">Thank you for choosing {{.Branding.CompanyName}}!</p>
<p style="margin: 8px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.Branding.CompanyName}} Team</p>
`
}

// emailVerificationContent returns content for email verification emails.
func (ts *TemplateService) emailVerificationContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.Name}},</p>

<p style="margin: 0 0 24px 0;">Welcome to <strong>{{.AppName}}</strong>! Please verify your email address to complete your registration.</p>

<!-- Verification card -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #f0f9ff; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 32px; text-align: center;">
      <p style="margin: 0 0 16px 0; font-size: 16px; color: #0369a1;">Click the button below to verify your email:</p>

      <!-- CTA Button -->
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
        <tr>
          <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
            <a href="{{.VerificationURL}}" target="_blank" style="display: inline-block; padding: 16px 40px; font-size: 16px; font-weight: 700; color: #ffffff; text-decoration: none;">
              Verify Email Address
            </a>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- Link expiry notice -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef3c7; border-left: 4px solid #f59e0b; border-radius: 0 12px 12px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <p style="margin: 0; font-size: 14px; color: #92400e;">
        <strong>This link expires in 24 hours.</strong> If you didn't create an account with {{.AppName}}, you can safely ignore this email.
      </p>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #6b7280;">If the button doesn't work, copy and paste this link into your browser:</p>
<p style="margin: 8px 0 0 0; font-size: 12px; color: #9ca3af; word-break: break-all;">{{.VerificationURL}}</p>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.AppName}} Team</p>
`
}

// passwordResetContent returns content for password reset emails.
func (ts *TemplateService) passwordResetContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.Name}},</p>

<p style="margin: 0 0 24px 0;">We received a request to reset your password for your <strong>{{.AppName}}</strong> account.</p>

<!-- Reset card -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef2f2; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 32px; text-align: center;">
      <p style="margin: 0 0 16px 0; font-size: 16px; color: #991b1b;">Click the button below to reset your password:</p>

      <!-- CTA Button -->
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
        <tr>
          <td style="border-radius: 8px; background: {{.Branding.DangerColor}};">
            <a href="{{.ResetURL}}" target="_blank" style="display: inline-block; padding: 16px 40px; font-size: 16px; font-weight: 700; color: #ffffff; text-decoration: none;">
              Reset Password
            </a>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- Link expiry notice -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef3c7; border-left: 4px solid #f59e0b; border-radius: 0 12px 12px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <p style="margin: 0; font-size: 14px; color: #92400e;">
        <strong>This link expires in 1 hour.</strong> If you didn't request a password reset, you can safely ignore this email. Your password will remain unchanged.
      </p>
    </td>
  </tr>
</table>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #6b7280;">If the button doesn't work, copy and paste this link into your browser:</p>
<p style="margin: 8px 0 0 0; font-size: 12px; color: #9ca3af; word-break: break-all;">{{.ResetURL}}</p>

<p style="margin: 24px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.AppName}} Team</p>
`
}

// affiliateLinkUpdatedContent returns content for affiliate link update emails.
func (ts *TemplateService) affiliateLinkUpdatedContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.AffiliateName}},</p>

<p style="margin: 0 0 24px 0;">Your affiliate referral link for <strong>{{.ProductName}}</strong> has been updated.</p>

<!-- Change reason card -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef3c7; border-left: 4px solid #f59e0b; border-radius: 0 12px 12px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <p style="margin: 0; font-size: 14px; color: #92400e;">
        <strong>Why did this change?</strong> {{.ChangeReason}}
      </p>
    </td>
  </tr>
</table>

<!-- New link card -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #ecfdf5; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <!-- Status badge -->
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin-bottom: 16px;">
        <tr>
          <td style="background-color: #a7f3d0; border-radius: 20px; padding: 6px 14px;">
            <span style="font-size: 13px; font-weight: 600; color: #065f46;">New Link</span>
          </td>
        </tr>
      </table>

      <p style="margin: 0 0 8px 0; font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Your New Affiliate Link</p>
      <p style="margin: 0; font-size: 14px; font-weight: 600; color: {{.Branding.PrimaryColor}}; word-break: break-all;">
        <a href="{{.NewLink}}" target="_blank" style="color: {{.Branding.PrimaryColor}}; text-decoration: none;">{{.NewLink}}</a>
      </p>
    </td>
  </tr>
</table>

{{if .OldLink}}
<!-- Old link reference -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #f3f4f6; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <p style="margin: 0 0 4px 0; font-size: 12px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px;">Previous Link (No Longer Active)</p>
      <p style="margin: 0; font-size: 13px; color: #9ca3af; word-break: break-all; text-decoration: line-through;">{{.OldLink}}</p>
    </td>
  </tr>
</table>
{{end}}

<!-- Action required -->
<p style="margin: 0 0 8px 0; color: #4b5563;"><strong>Action Required:</strong></p>
<ul style="margin: 0 0 24px 0; padding-left: 20px; color: #6b7280;">
  <li style="margin-bottom: 8px;">Update any promotional materials with your new link</li>
  <li style="margin-bottom: 8px;">Replace links on your website or blog</li>
  <li style="margin-bottom: 8px;">Update social media posts and bios</li>
  <li>Check email signatures and other marketing channels</li>
</ul>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto 24px auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.DashboardURL}}" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        View Dashboard
      </a>
    </td>
  </tr>
</table>

<p style="margin: 0; font-size: 14px; color: #6b7280;">If you have any questions about this change, please don't hesitate to contact our support team.</p>
<p style="margin: 16px 0 0 0; font-size: 14px; color: #9ca3af;">— The {{.Branding.CompanyName}} Team</p>
`
}

func (ts *TemplateService) migrationAnnouncementContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.Name}},</p>

<p style="margin: 0 0 16px 0; font-size: 16px;">We've rebuilt <strong>WasBot</strong> from the ground up, and your new platform is ready.</p>

<!-- Features card -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #ecfdf5; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <p style="margin: 0 0 16px 0; font-weight: 600; color: #065f46;">What's new:</p>
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding: 8px 0; color: #047857; font-size: 14px;">&#10003; Lightning-fast dashboard with real-time session monitoring</td>
        </tr>
        <tr>
          <td style="padding: 8px 0; color: #047857; font-size: 14px;">&#10003; Auto-reply, scheduled messages, and bulk messaging</td>
        </tr>
        <tr>
          <td style="padding: 8px 0; color: #047857; font-size: 14px;">&#10003; Team collaboration with role-based access</td>
        </tr>
        <tr>
          <td style="padding: 8px 0; color: #047857; font-size: 14px;">&#10003; Reliable session management that stays connected</td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<p style="margin: 0 0 24px 0; color: #6b7280;">Your legacy account will remain active, but all new features are exclusively on the new platform.</p>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto 24px auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.DashboardURL}}" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        Get Started on WasBot
      </a>
    </td>
  </tr>
</table>

<p style="margin: 0; font-size: 14px; color: #6b7280;">If you have any questions, just reply to this email.</p>
<p style="margin: 16px 0 0 0; font-size: 14px; color: #9ca3af;">— The WasBot Team</p>
`
}

func (ts *TemplateService) migrationFollowUpContent() string {
	return `
<p style="margin: 0 0 20px 0;">Hi {{.Name}},</p>

<p style="margin: 0 0 16px 0; font-size: 16px;">We noticed you haven't set up your new <strong>WasBot</strong> account yet.</p>

<p style="margin: 0 0 16px 0; color: #4b5563;">The new platform at <a href="{{.DashboardURL}}" style="color: {{.Branding.PrimaryColor}}; font-weight: 600;">wasbot.app</a> has everything you loved about WasBot, plus:</p>

<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="margin-bottom: 24px;">
  <tr>
    <td style="padding: 8px 0; color: #4b5563; font-size: 14px;">&#8226; A completely redesigned dashboard</td>
  </tr>
  <tr>
    <td style="padding: 8px 0; color: #4b5563; font-size: 14px;">&#8226; Faster, more reliable WhatsApp connections</td>
  </tr>
  <tr>
    <td style="padding: 8px 0; color: #4b5563; font-size: 14px;">&#8226; New automation features</td>
  </tr>
</table>

<p style="margin: 0 0 24px 0; color: #6b7280;">It only takes 2 minutes to get started.</p>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto 24px auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.DashboardURL}}" target="_blank" style="display: inline-block; padding: 14px 32px; font-size: 15px; font-weight: 600; color: #ffffff; text-decoration: none;">
        Claim Your Account
      </a>
    </td>
  </tr>
</table>

<p style="margin: 0; font-size: 14px; color: #6b7280;">If you have any questions, just reply to this email.</p>
<p style="margin: 16px 0 0 0; font-size: 14px; color: #9ca3af;">— The WasBot Team</p>
`
}
