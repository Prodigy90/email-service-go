package service

import (
	"bytes"
	"fmt"
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

// Render renders a template with the given data.
func (ts *TemplateService) Render(templateName string, data map[string]interface{}) (subject, body, htmlBody string, err error) {
	ts.mu.RLock()
	tmpl, ok := ts.templates[templateName]
	ts.mu.RUnlock()

	if !ok {
		return "", "", "", fmt.Errorf("template not found: %s", templateName)
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
		htmlBody, err = ts.renderString(tmpl.HTMLBody, data)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to render html body: %w", err)
		}
	}

	return subject, body, htmlBody, nil
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

	// Payment/Webhook templates
	ts.templates[domain.TemplatePaymentSuccess] = &domain.Template{
		Name:        domain.TemplatePaymentSuccess,
		Subject:     "Payment Confirmed - {{.ProductName}}",
		Body:        "Hi {{.CustomerName}},\n\nThank you for your payment of {{.Currency}}{{.Amount}}.\n\nProduct: {{.ProductName}}\nTransaction ID: {{.TransactionID}}\n\nYour subscription is now active.",
		HTMLBody:    ts.wrapHTML("Payment Confirmed", "<p>Hi {{.CustomerName}},</p><p>Thank you for your payment of <strong>{{.Currency}}{{.Amount}}</strong>.</p><p><strong>Product:</strong> {{.ProductName}}<br><strong>Transaction ID:</strong> {{.TransactionID}}</p><p>Your subscription is now active.</p>"),
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
		Subject:     "Refund Request Received",
		Body:        "Hi {{.CustomerName}},\n\nWe have received your refund request for {{.Currency}}{{.Amount}}.\n\nTransaction ID: {{.TransactionID}}\n\nYour refund is being processed and should be completed within 5-10 business days.\n\nWe'll send you another email once the refund has been processed.\n\nIf you have any questions, please don't hesitate to contact our support team.",
		HTMLBody:    ts.wrapHTML("Refund Request Received", "<p>Hi {{.CustomerName}},</p><p>We have received your refund request for <strong>{{.Currency}}{{.Amount}}</strong>.</p><p><strong>Transaction ID:</strong> {{.TransactionID}}</p><p>Your refund is being processed and should be completed within 5-10 business days.</p><p>We'll send you another email once the refund has been processed.</p><p>If you have any questions, please don't hesitate to contact our support team.</p>"),
		Description: "Sent when a refund request is initiated",
	}

	ts.templates[domain.TemplateRefundProcessed] = &domain.Template{
		Name:        domain.TemplateRefundProcessed,
		Subject:     "Refund Processed - {{.Currency}}{{.Amount}}",
		Body:        "Hi {{.CustomerName}},\n\nGreat news! Your refund of {{.Currency}}{{.Amount}} has been successfully processed.\n\nTransaction ID: {{.TransactionID}}\n\nThe funds should appear in your account within 5-10 business days, depending on your bank.\n\nThank you for your patience.",
		HTMLBody:    ts.wrapHTML("Refund Processed", "<p>Hi {{.CustomerName}},</p><p>Great news! Your refund of <strong>{{.Currency}}{{.Amount}}</strong> has been successfully processed.</p><p><strong>Transaction ID:</strong> {{.TransactionID}}</p><p>The funds should appear in your account within 5-10 business days, depending on your bank.</p><p>Thank you for your patience.</p>"),
		Description: "Sent when a refund has been processed successfully",
	}

	ts.templates[domain.TemplateRefundFailed] = &domain.Template{
		Name:        domain.TemplateRefundFailed,
		Subject:     "Refund Update - Action Required",
		Body:        "Hi {{.CustomerName}},\n\nWe were unable to process your refund of {{.Currency}}{{.Amount}}.\n\nTransaction ID: {{.TransactionID}}\nReason: {{.Reason}}\n\nPlease contact our support team to resolve this issue and complete your refund.\n\nWe apologize for any inconvenience.",
		HTMLBody:    ts.wrapHTML("Refund Update", "<p>Hi {{.CustomerName}},</p><p>We were unable to process your refund of <strong>{{.Currency}}{{.Amount}}</strong>.</p><p><strong>Transaction ID:</strong> {{.TransactionID}}<br><strong>Reason:</strong> {{.Reason}}</p><p>Please contact our support team to resolve this issue and complete your refund.</p><p>We apologize for any inconvenience.</p>"),
		Description: "Sent when a refund could not be processed",
	}

	// Subscription reminder templates
	ts.templates[domain.TemplateSubscriptionReminder3d] = &domain.Template{
		Name:        domain.TemplateSubscriptionReminder3d,
		Subject:     "Your {{.PlanName}} subscription renews in 3 days",
		Body:        "Hi {{.CustomerName}},\n\nThis is a friendly reminder that your {{.PlanName}} subscription will automatically renew in 3 days.\n\nRenewal Date: {{.RenewalDate}}\nAmount: {{.Currency}}{{.Amount}}\n\nIf you'd like to make any changes to your subscription, you can manage it here:\n{{.ProfileURL}}\n\nThank you for being a valued {{.AppName}} customer!",
		HTMLBody:    ts.wrapHTML("Subscription Renewal Reminder", "<p>Hi {{.CustomerName}},</p><p>This is a friendly reminder that your <strong>{{.PlanName}}</strong> subscription will automatically renew in 3 days.</p><p><strong>Renewal Date:</strong> {{.RenewalDate}}<br><strong>Amount:</strong> {{.Currency}}{{.Amount}}</p><p>If you'd like to make any changes to your subscription, you can manage it here:</p><p><a href=\"{{.ProfileURL}}\" style=\"background:#10b981;color:white;padding:12px 24px;text-decoration:none;border-radius:6px;display:inline-block;\">Manage Subscription</a></p><p>Thank you for being a valued {{.AppName}} customer!</p>"),
		Description: "Sent 3 days before subscription renewal",
	}

	ts.templates[domain.TemplateSubscriptionReminder1d] = &domain.Template{
		Name:        domain.TemplateSubscriptionReminder1d,
		Subject:     "Reminder: {{.PlanName}} renews tomorrow",
		Body:        "Hi {{.CustomerName}},\n\nThis is a final reminder that your {{.PlanName}} subscription will renew tomorrow.\n\nAmount: {{.Currency}}{{.Amount}}\n\nIf you need to update your payment method or make changes, please do so before the renewal:\n{{.ProfileURL}}\n\nThank you for your continued support!\n\nThe {{.AppName}} Team",
		HTMLBody:    ts.wrapHTML("Subscription Renews Tomorrow", "<p>Hi {{.CustomerName}},</p><p>This is a final reminder that your <strong>{{.PlanName}}</strong> subscription will renew tomorrow.</p><p><strong>Amount:</strong> {{.Currency}}{{.Amount}}</p><p>If you need to update your payment method or make changes, please do so before the renewal:</p><p><a href=\"{{.ProfileURL}}\" style=\"background:#10b981;color:white;padding:12px 24px;text-decoration:none;border-radius:6px;display:inline-block;\">Manage Subscription</a></p><p>Thank you for your continued support!</p><p>The {{.AppName}} Team</p>"),
		Description: "Sent 1 day before subscription renewal",
	}
}

// wrapHTML wraps content in a basic HTML email template.
func (ts *TemplateService) wrapHTML(title, content string) string {
	return strings.TrimSpace(fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #374151; margin: 0; padding: 0; }
    .container { max-width: 600px; margin: 0 auto; padding: 40px 20px; }
    h1 { color: #111827; font-size: 24px; margin-bottom: 24px; }
    p { margin: 16px 0; }
    a { color: #10b981; }
    .footer { margin-top: 40px; padding-top: 20px; border-top: 1px solid #e5e7eb; font-size: 14px; color: #6b7280; }
  </style>
</head>
<body>
  <div class="container">
    <h1>%s</h1>
    %s
    <div class="footer">
      <p>If you have questions, reply to this email or contact our support team.</p>
    </div>
  </div>
</body>
</html>
`, title, title, content))
}
