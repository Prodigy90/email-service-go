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

	// Render subject
	subject, err = ts.renderString(tmpl.Subject, data)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to render subject: %w", err)
	}

	// Render body
	body, err = ts.renderString(tmpl.Body, data)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to render body: %w", err)
	}

	// Render HTML body if present
	if tmpl.HTMLBody != "" {
		// First render the template with data
		var renderedHTML string
		renderedHTML, err = ts.renderString(tmpl.HTMLBody, data)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to render html body: %w", err)
		}
		// Then apply branding replacements
		htmlBody = ts.applyBranding(renderedHTML, branding)
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

// applyBranding replaces branding placeholders in the rendered HTML.
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
		Subject:     "Payout Request Approved - {{.Currency}}{{.Amount}}",
		Body:        "Hi {{.Name}},\n\nYour payout request of {{.Currency}}{{.Amount}} has been approved and is being processed.\n\nExpected arrival: 1-3 business days.\n\nThank you for being part of our affiliate program!",
		HTMLBody:    ts.wrapHTML("Payout Approved", "<p>Hi {{.Name}},</p><p>Your payout request of <strong>{{.Currency}}{{.Amount}}</strong> has been approved and is being processed.</p><p>Expected arrival: 1-3 business days.</p><p>Thank you for being part of our affiliate program!</p>"),
		Description: "Sent when an affiliate payout is approved",
	}

	ts.templates[domain.TemplatePayoutRejected] = &domain.Template{
		Name:        domain.TemplatePayoutRejected,
		Subject:     "Payout Request Update",
		Body:        "Hi {{.Name}},\n\nYour payout request of {{.Currency}}{{.Amount}} was not approved.\n\nReason: {{.Reason}}\n\nPlease contact support if you have questions.",
		HTMLBody:    ts.wrapHTML("Payout Update", "<p>Hi {{.Name}},</p><p>Your payout request of <strong>{{.Currency}}{{.Amount}}</strong> was not approved.</p><p><strong>Reason:</strong> {{.Reason}}</p><p>Please contact support if you have questions.</p>"),
		Description: "Sent when an affiliate payout is rejected",
	}

	ts.templates[domain.TemplatePayoutProcessed] = &domain.Template{
		Name:        domain.TemplatePayoutProcessed,
		Subject:     "Payout Completed - {{.Currency}}{{.Amount}}",
		Body:        "Hi {{.Name}},\n\nGreat news! Your payout of {{.Currency}}{{.Amount}} has been successfully transferred to your account.\n\nTransaction Reference: {{.Reference}}\n\nThank you!",
		HTMLBody:    ts.wrapHTML("Payout Completed", "<p>Hi {{.Name}},</p><p>Great news! Your payout of <strong>{{.Currency}}{{.Amount}}</strong> has been successfully transferred to your account.</p><p><strong>Transaction Reference:</strong> {{.Reference}}</p><p>Thank you!</p>"),
		Description: "Sent when a payout transfer completes",
	}

	ts.templates[domain.TemplateCommissionEarned] = &domain.Template{
		Name:        domain.TemplateCommissionEarned,
		Subject:     "You earned a commission! - {{.Currency}}{{.Amount}}",
		Body:        "Hi {{.Name}},\n\nYou just earned {{.Currency}}{{.Amount}} from {{.ProductName}}!\n\nThis commission will be available for withdrawal after the holding period.\n\nKeep up the great work!",
		HTMLBody:    ts.wrapHTML("Commission Earned", "<p>Hi {{.Name}},</p><p>You just earned <strong>{{.Currency}}{{.Amount}}</strong> from {{.ProductName}}!</p><p>This commission will be available for withdrawal after the holding period.</p><p>Keep up the great work!</p>"),
		Description: "Sent when an affiliate earns a commission",
	}

	ts.templates[domain.TemplateCommissionRefunded] = &domain.Template{
		Name:        domain.TemplateCommissionRefunded,
		Subject:     "Commission Reversed - {{.ProductName}}",
		Body:        "Hi {{.CustomerName}},\n\nA commission of {{.Currency}}{{.Amount}} for {{.ProductName}} has been reversed.\n\nReason: {{.Reason}}\n\nThis adjustment has been applied to your affiliate account balance.\n\nIf you have any questions about this reversal, please contact our support team.",
		HTMLBody:    ts.wrapHTML("Commission Reversed", ts.commissionRefundedContent()),
		Description: "Sent when an affiliate commission is reversed due to a refund",
	}

	ts.templates[domain.TemplateAccessRevoked] = &domain.Template{
		Name:        domain.TemplateAccessRevoked,
		Subject:     "Access Revoked - Your Subscription Has Been Cancelled",
		Body:        "Hi {{.CustomerName}},\n\nYour refund of {{.Currency}}{{.Amount}} for {{.ProductName}} has been processed.\n\nAs a result, your access to {{.ProductName}} has been revoked.\n\nTransaction ID: {{.TransactionID}}\n\nIf you have any questions or believe this was done in error, please contact our support team.",
		HTMLBody:    ts.wrapHTML("Access Revoked", ts.accessRevokedContent()),
		Description: "Sent when a customer's access is revoked due to a refund",
	}

	// Payment/Webhook templates
	ts.templates[domain.TemplatePaymentSuccess] = &domain.Template{
		Name:        domain.TemplatePaymentSuccess,
		Subject:     "Payment Confirmed - {{.ProductName}}",
		Body:        "Hi {{.CustomerName}},\n\nThank you for your payment of {{.Currency}}{{.Amount}}.\n\nProduct: {{.ProductName}}\nTransaction ID: {{.TransactionID}}\n\nYour subscription is now active.",
		HTMLBody:    ts.wrapHTML("Payment Confirmed", ts.paymentSuccessContent()),
		Description: "Sent after successful payment",
	}

	ts.templates[domain.TemplatePaymentFailed] = &domain.Template{
		Name:        domain.TemplatePaymentFailed,
		Subject:     "Payment Failed - Action Required",
		Body:        "Hi {{.CustomerName}},\n\nWe were unable to process your payment of {{.Currency}}{{.Amount}} for {{.ProductName}}.\n\nPlease update your payment method to continue your subscription.\n\nIf you need help, contact our support team.",
		HTMLBody:    ts.wrapHTML("Payment Failed", "<p>Hi {{.CustomerName}},</p><p>We were unable to process your payment of <strong>{{.Currency}}{{.Amount}}</strong> for {{.ProductName}}.</p><p>Please update your payment method to continue your subscription.</p><p>If you need help, contact our support team.</p>"),
		Description: "Sent when payment fails",
	}

	ts.templates[domain.TemplateSubscriptionRenewed] = &domain.Template{
		Name:        domain.TemplateSubscriptionRenewed,
		Subject:     "Subscription Renewed - {{.PlanName}}",
		Body:        "Hi {{.CustomerName}},\n\nYour {{.PlanName}} subscription has been successfully renewed.\n\nAmount: {{.Currency}}{{.Amount}}\nNext billing date: {{.NextBillingDate}}\n\nThank you for your continued support!",
		HTMLBody:    ts.wrapHTML("Subscription Renewed", "<p>Hi {{.CustomerName}},</p><p>Your <strong>{{.PlanName}}</strong> subscription has been successfully renewed.</p><p><strong>Amount:</strong> {{.Currency}}{{.Amount}}<br><strong>Next billing date:</strong> {{.NextBillingDate}}</p><p>Thank you for your continued support!</p>"),
		Description: "Sent when subscription renews",
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
		Body:        "Hi {{.CustomerName}},\n\nYour {{.PlanName}} subscription has been cancelled.\n\nYou will continue to have access until {{.ExpiryDate}}.\n\nWe're sorry to see you go. If you change your mind, you can resubscribe anytime.",
		HTMLBody:    ts.wrapHTML("Subscription Cancelled", "<p>Hi {{.CustomerName}},</p><p>Your <strong>{{.PlanName}}</strong> subscription has been cancelled.</p><p>You will continue to have access until <strong>{{.ExpiryDate}}</strong>.</p><p>We're sorry to see you go. If you change your mind, you can resubscribe anytime.</p>"),
		Description: "Sent when subscription is cancelled",
	}

	// WasBot/General templates
	ts.templates[domain.TemplateWelcome] = &domain.Template{
		Name:        domain.TemplateWelcome,
		Subject:     "Welcome to {{.AppName}}!",
		Body:        "Hi {{.Name}},\n\nWelcome to {{.AppName}}! We're excited to have you on board.\n\nGet started by exploring our features and let us know if you need any help.\n\nBest,\nThe {{.AppName}} Team",
		HTMLBody:    ts.wrapHTML("Welcome!", "<p>Hi {{.Name}},</p><p>Welcome to <strong>{{.AppName}}</strong>! We're excited to have you on board.</p><p>Get started by exploring our features and let us know if you need any help.</p><p>Best,<br>The {{.AppName}} Team</p>"),
		Description: "Welcome email for new users",
	}

	ts.templates[domain.TemplateTrialExpiring] = &domain.Template{
		Name:        domain.TemplateTrialExpiring,
		Subject:     "Your trial expires in {{.Days}} days",
		Body:        "Hi {{.Name}},\n\nYour free trial of {{.AppName}} expires in {{.Days}} days.\n\nUpgrade now to keep access to all features:\n{{.UpgradeURL}}\n\nQuestions? Reply to this email.",
		HTMLBody:    ts.wrapHTML("Trial Expiring", "<p>Hi {{.Name}},</p><p>Your free trial of <strong>{{.AppName}}</strong> expires in <strong>{{.Days}} days</strong>.</p><p><a href=\"{{.UpgradeURL}}\" style=\"background:#10b981;color:white;padding:12px 24px;text-decoration:none;border-radius:6px;display:inline-block;\">Upgrade Now</a></p><p>Questions? Reply to this email.</p>"),
		Description: "Sent before trial expires",
	}

	// Trial sequence templates
	ts.templates[domain.TemplateTrialDay3] = &domain.Template{
		Name:        domain.TemplateTrialDay3,
		Subject:     "How's your {{.AppName}} trial going?",
		Body:        "Hi {{.Name}},\n\nYou've been using {{.AppName}} for 3 days now! How's it going?\n\nHere are some features you might not have tried yet:\n- Schedule messages to send later\n- Set up auto-replies\n- Create broadcast lists\n\nNeed help? Just reply to this email.\n\nBest,\nThe {{.AppName}} Team",
		HTMLBody:    ts.wrapHTML("How's Your Trial Going?", ts.trialDay3Content()),
		Description: "Day 3 engagement check",
	}

	ts.templates[domain.TemplateTrialDay5] = &domain.Template{
		Name:        domain.TemplateTrialDay5,
		Subject:     "Unlock the full power of {{.AppName}}",
		Body:        "Hi {{.Name}},\n\nYou're halfway through your trial! Here's what premium users love most:\n\n- Unlimited message scheduling\n- Priority support\n- Advanced automation\n- Multi-device support\n\nUpgrade now and get 20% off your first month:\n{{.UpgradeURL}}\n\nBest,\nThe {{.AppName}} Team",
		HTMLBody:    ts.wrapHTML("Unlock Full Power", ts.trialDay5Content()),
		Description: "Day 5 feature highlight",
	}

	ts.templates[domain.TemplateTrialDay6] = &domain.Template{
		Name:        domain.TemplateTrialDay6,
		Subject:     "Your {{.AppName}} trial ends tomorrow!",
		Body:        "Hi {{.Name}},\n\nHeads up - your {{.AppName}} trial ends tomorrow!\n\nAfter your trial:\n- Your scheduled messages will stop\n- Auto-replies will be disabled\n- You'll lose access to your dashboard\n\nDon't lose your progress! Upgrade now:\n{{.UpgradeURL}}\n\nBest,\nThe {{.AppName}} Team",
		HTMLBody:    ts.wrapHTML("Trial Ends Tomorrow!", ts.trialDay6Content()),
		Description: "Day 6 urgent reminder",
	}

	ts.templates[domain.TemplateTrialDay10] = &domain.Template{
		Name:        domain.TemplateTrialDay10,
		Subject:     "We miss you! Here's 30% off {{.AppName}}",
		Body:        "Hi {{.Name}},\n\nYour {{.AppName}} trial ended a few days ago, and we noticed you haven't upgraded yet.\n\nWe'd love to have you back! Use code COMEBACK30 for 30% off your first month.\n\nUpgrade now:\n{{.UpgradeURL}}\n\nThis offer expires in 48 hours.\n\nBest,\nThe {{.AppName}} Team",
		HTMLBody:    ts.wrapHTML("We Miss You!", ts.trialDay10Content()),
		Description: "Day 10 win-back after expiry",
	}

	ts.templates[domain.TemplateAccountUpgraded] = &domain.Template{
		Name:        domain.TemplateAccountUpgraded,
		Subject:     "Welcome to {{.PlanName}}!",
		Body:        "Hi {{.Name}},\n\nCongratulations! You've been upgraded to {{.PlanName}}.\n\nYou now have access to:\n{{.Features}}\n\nThank you for your support!",
		HTMLBody:    ts.wrapHTML("Account Upgraded", "<p>Hi {{.Name}},</p><p>Congratulations! You've been upgraded to <strong>{{.PlanName}}</strong>.</p><p>You now have access to:</p><ul>{{.FeaturesHTML}}</ul><p>Thank you for your support!</p>"),
		Description: "Sent when user upgrades their plan",
	}

	// Refund templates
	ts.templates[domain.TemplateRefundPending] = &domain.Template{
		Name:        domain.TemplateRefundPending,
		Subject:     "Refund Request Received - {{.Currency}}{{.Amount}}",
		Body:        "Hi {{.CustomerName}},\n\nWe have received your refund request for {{.Currency}}{{.Amount}}.\n\nTransaction ID: {{.TransactionID}}\n\nYour refund is being processed and should be completed within 5-10 business days.\n\nWe'll send you another email once the refund has been processed.\n\nIf you have any questions, please don't hesitate to contact our support team.",
		HTMLBody:    ts.wrapHTML("Refund Request Received", ts.refundPendingContent()),
		Description: "Sent when a refund request is initiated",
	}

	ts.templates[domain.TemplateRefundProcessed] = &domain.Template{
		Name:        domain.TemplateRefundProcessed,
		Subject:     "✅ Refund Processed - {{.Currency}}{{.Amount}}",
		Body:        "Hi {{.CustomerName}},\n\nGreat news! Your refund of {{.Currency}}{{.Amount}} has been successfully processed.\n\nTransaction ID: {{.TransactionID}}\n\nThe funds should appear in your account within 5-10 business days, depending on your bank.\n\nThank you for your patience.",
		HTMLBody:    ts.wrapHTML("Refund Processed", ts.refundProcessedContent()),
		Description: "Sent when a refund has been processed successfully",
	}

	ts.templates[domain.TemplateRefundFailed] = &domain.Template{
		Name:        domain.TemplateRefundFailed,
		Subject:     "⚠️ Refund Update - Action Required",
		Body:        "Hi {{.CustomerName}},\n\nWe were unable to process your refund of {{.Currency}}{{.Amount}}.\n\nTransaction ID: {{.TransactionID}}\nReason: {{.Reason}}\n\nPlease contact our support team to resolve this issue and complete your refund.\n\nWe apologize for any inconvenience.",
		HTMLBody:    ts.wrapHTML("Refund Issue", ts.refundFailedContent()),
		Description: "Sent when a refund could not be processed",
	}

	// Subscription reminder templates
	ts.templates[domain.TemplateSubscriptionReminder3d] = &domain.Template{
		Name:        domain.TemplateSubscriptionReminder3d,
		Subject:     "Your {{.PlanName}} subscription renews in 3 days",
		Body:        "Hi {{.CustomerName}},\n\nThis is a friendly reminder that your {{.PlanName}} subscription will automatically renew in 3 days.\n\nRenewal Date: {{.RenewalDate}}\nAmount: {{.Currency}}{{.Amount}}\n\nIf you'd like to make any changes to your subscription, you can manage it here:\n{{.ProfileURL}}\n\nThank you for being a valued WASBOT customer!",
		HTMLBody:    ts.wrapHTML("Subscription Renewal Reminder", ts.subscriptionReminder3dContent()),
		Description: "Sent 3 days before subscription renewal",
	}

	ts.templates[domain.TemplateSubscriptionReminder1d] = &domain.Template{
		Name:        domain.TemplateSubscriptionReminder1d,
		Subject:     "⏰ Reminder: {{.PlanName}} renews tomorrow",
		Body:        "Hi {{.CustomerName}},\n\nThis is a final reminder that your {{.PlanName}} subscription will renew tomorrow.\n\nAmount: {{.Currency}}{{.Amount}}\n\nIf you need to update your payment method or make changes, please do so before the renewal:\n{{.ProfileURL}}\n\nThank you for your continued support!\n\nThe WASBOT Team",
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
		Body:        "Hi {{.CustomerName}},\n\nThis is a final reminder that your {{.PlanName}} access will expire tomorrow.\n\nExpiry Date: {{.ExpiryDate}}\n\nDon't lose access! Renew now to continue using all features:\n{{.ProfileURL}}\n\nIf you have any questions, our support team is here to help.\n\nThe WASBOT Team",
		HTMLBody:    ts.wrapHTML("Subscription Expires Tomorrow", ts.subscriptionExpiring1dContent()),
		Description: "Sent 1 day before one-off subscription expires",
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
  <!-- Preview text -->
  <div style="display: none; max-height: 0; overflow: hidden;">
    %s
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
                            <img src="https://cdn-icons-png.flaticon.com/32/733/733579.png" alt="Twitter" width="24" height="24" style="opacity: 0.6;">
                          </a>
                        </td>
                        <td style="padding: 0 8px;">
                          <a href="{{.Branding.SocialInstagram}}" style="display: inline-block;">
                            <img src="https://cdn-icons-png.flaticon.com/32/2111/2111463.png" alt="Instagram" width="24" height="24" style="opacity: 0.6;">
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
                      &copy; 2025 {{.Branding.CompanyName}}. All rights reserved.
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
            <span style="font-size: 32px; font-weight: 700; color: {{.Branding.SecondaryColor}};">{{.Currency}}{{.Amount}}</span>
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
            <span style="font-size: 18px; font-weight: 600; color: #1e40af;">{{.Currency}}{{.Amount}}</span>
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
        <strong>{{.Currency}}{{.Amount}}</strong> will be charged to your payment method tomorrow.
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
            <span style="font-size: 28px; font-weight: 700; color: #1e40af;">{{.Currency}}{{.Amount}}</span>
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
            <span style="font-size: 28px; font-weight: 700; color: {{.Branding.SecondaryColor}};">{{.Currency}}{{.Amount}}</span>
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
            <span style="font-size: 28px; font-weight: 700; color: #dc2626;">{{.Currency}}{{.Amount}}</span>
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
<p style="margin: 0 0 20px 0;">Hi {{.CustomerName}},</p>

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
            <span style="font-size: 28px; font-weight: 700; color: #92400e;">{{.Currency}}{{.Amount}}</span>
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
            <span style="font-size: 28px; font-weight: 700; color: #dc2626;">{{.Currency}}{{.Amount}}</span>
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
            <span style="color: {{.Branding.PrimaryColor}}; font-size: 18px;">📅</span>
            <span style="margin-left: 12px; color: #374151;"><strong>Schedule Messages</strong> - Send messages at the perfect time</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: {{.Branding.PrimaryColor}}; font-size: 18px;">🤖</span>
            <span style="margin-left: 12px; color: #374151;"><strong>Auto-Replies</strong> - Never miss a customer message</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: {{.Branding.PrimaryColor}}; font-size: 18px;">📢</span>
            <span style="margin-left: 12px; color: #374151;"><strong>Broadcast Lists</strong> - Message multiple contacts at once</span>
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

<p style="margin: 0 0 24px 0;">You're <strong>halfway through your trial!</strong> Here's what our premium users love most:</p>

<!-- Premium features -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #eff6ff; border-radius: 12px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 24px;">
      <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%">
        <tr>
          <td style="padding: 12px 0; border-bottom: 1px solid #dbeafe;">
            <span style="color: #1d4ed8; font-size: 20px;">✨</span>
            <span style="margin-left: 12px; font-weight: 600; color: #1e40af;">Unlimited Message Scheduling</span>
            <p style="margin: 4px 0 0 32px; font-size: 14px; color: #6b7280;">Schedule as many messages as you need</p>
          </td>
        </tr>
        <tr>
          <td style="padding: 12px 0; border-bottom: 1px solid #dbeafe;">
            <span style="color: #1d4ed8; font-size: 20px;">🎯</span>
            <span style="margin-left: 12px; font-weight: 600; color: #1e40af;">Priority Support</span>
            <p style="margin: 4px 0 0 32px; font-size: 14px; color: #6b7280;">Get help when you need it most</p>
          </td>
        </tr>
        <tr>
          <td style="padding: 12px 0; border-bottom: 1px solid #dbeafe;">
            <span style="color: #1d4ed8; font-size: 20px;">⚡</span>
            <span style="margin-left: 12px; font-weight: 600; color: #1e40af;">Advanced Automation</span>
            <p style="margin: 4px 0 0 32px; font-size: 14px; color: #6b7280;">Build complex workflows effortlessly</p>
          </td>
        </tr>
        <tr>
          <td style="padding: 12px 0;">
            <span style="color: #1d4ed8; font-size: 20px;">📱</span>
            <span style="margin-left: 12px; font-weight: 600; color: #1e40af;">Multi-Device Support</span>
            <p style="margin: 4px 0 0 32px; font-size: 14px; color: #6b7280;">Manage multiple WhatsApp numbers</p>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- Discount offer -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%" style="background-color: #fef3c7; border-left: 4px solid #f59e0b; border-radius: 0 12px 12px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 20px 24px;">
      <p style="margin: 0; font-size: 16px; color: #92400e;">
        <strong>🎁 Special Offer:</strong> Upgrade now and get <strong>20% off</strong> your first month!
      </p>
    </td>
  </tr>
</table>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.PrimaryColor}} 0%, {{.Branding.SecondaryColor}} 100%);">
      <a href="{{.UpgradeURL}}" target="_blank" style="display: inline-block; padding: 16px 40px; font-size: 16px; font-weight: 700; color: #ffffff; text-decoration: none;">
        Upgrade Now — 20% Off
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
            <span style="margin-left: 12px; color: #7f1d1d;">Scheduled messages will stop sending</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: #dc2626;">✗</span>
            <span style="margin-left: 12px; color: #7f1d1d;">Auto-replies will be disabled</span>
          </td>
        </tr>
        <tr>
          <td style="padding: 8px 0;">
            <span style="color: #dc2626;">✗</span>
            <span style="margin-left: 12px; color: #7f1d1d;">Dashboard access will be revoked</span>
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
        <strong>Good news:</strong> Upgrade now and keep all your settings, scheduled messages, and configurations intact!
      </p>
    </td>
  </tr>
</table>

<!-- CTA Button -->
<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin: 0 auto;">
  <tr>
    <td style="border-radius: 8px; background: linear-gradient(135deg, {{.Branding.DangerColor}} 0%, #dc2626 100%);">
      <a href="{{.UpgradeURL}}" target="_blank" style="display: inline-block; padding: 16px 40px; font-size: 16px; font-weight: 700; color: #ffffff; text-decoration: none;">
        Upgrade Now — Keep My Progress
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

<p style="margin: 0 0 24px 0; color: #6b7280;">We know life gets busy. But if WhatsApp automation is something you need, there's no better time to start than now.</p>

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
