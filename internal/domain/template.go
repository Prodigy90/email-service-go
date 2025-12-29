package domain

// Template represents an email template.
type Template struct {
	Name        string `json:"name" yaml:"name"`
	Subject     string `json:"subject" yaml:"subject"`
	Body        string `json:"body" yaml:"body"`
	HTMLBody    string `json:"html_body,omitempty" yaml:"html_body"`
	Description string `json:"description,omitempty" yaml:"description"`
}

// TemplateListResponse represents the list of available templates.
type TemplateListResponse struct {
	Templates []TemplateInfo `json:"templates"`
}

// TemplateInfo provides basic info about a template.
type TemplateInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Variables   []string `json:"variables,omitempty"`
}

// Pre-defined template names for consistency across services.
const (
	// Affiliate System templates
	TemplatePayoutApproved  = "payout_approved"
	TemplatePayoutRejected  = "payout_rejected"
	TemplatePayoutProcessed = "payout_processed"
	TemplateCommissionEarned = "commission_earned"

	// Webhook Router / Payment templates
	TemplatePaymentSuccess      = "payment_success"
	TemplatePaymentFailed       = "payment_failed"
	TemplateSubscriptionRenewed = "subscription_renewed"
	TemplateSubscriptionExpiring = "subscription_expiring"
	TemplateSubscriptionCancelled = "subscription_cancelled"

	// WasBot templates
	TemplateWelcome          = "welcome"
	TemplateTrialExpiring    = "trial_expiring"
	TemplateAccountUpgraded  = "account_upgraded"

	// Refund templates
	TemplateRefundPending   = "refund_pending"
	TemplateRefundProcessed = "refund_processed"
	TemplateRefundFailed    = "refund_failed"

	// Subscription reminder templates
	TemplateSubscriptionReminder3d = "subscription_reminder_3d"
	TemplateSubscriptionReminder1d = "subscription_reminder_1d"
)
