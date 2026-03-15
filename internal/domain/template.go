package domain

// Template represents an email template.
type Template struct {
	Name        string `json:"name" yaml:"name"`
	Subject     string `json:"subject" yaml:"subject"`
	PreviewText string `json:"preview_text,omitempty" yaml:"preview_text"`
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

	// Webhook Router / Payment templates (recurring subscriptions)
	TemplatePaymentSuccess        = "payment_success"          // Recurring payment success
	TemplatePaymentFailed         = "payment_failed"           // Payment failure
	TemplateSubscriptionRenewed   = "subscription_renewed"     // Auto-renewal success
	TemplateSubscriptionExpiring  = "subscription_expiring"    // Generic expiring (deprecated, use specific ones)
	TemplateSubscriptionCancelled = "subscription_cancelled"   // Subscription cancelled

	// One-time payment templates (non-recurring)
	TemplatePaymentSuccessOnetime     = "payment_success_onetime"     // One-time payment confirmation
	TemplateSubscriptionActivated     = "subscription_activated"      // New recurring subscription activated
	TemplateSubscriptionActivatedOnetime = "subscription_activated_onetime" // One-time subscription activated

	// WasBot templates
	TemplateWelcome          = "welcome"
	TemplateTrialExpiring    = "trial_expiring"
	TemplateAccountUpgraded  = "account_upgraded"

	// Trial sequence templates
	TemplateTrialDay3  = "trial_day3"  // Day 3: Engagement check
	TemplateTrialDay5  = "trial_day5"  // Day 5: Feature highlight
	TemplateTrialDay6  = "trial_day6"  // Day 6: Urgent reminder
	TemplateTrialDay10 = "trial_day10" // Day 10: Win-back after expiry

	// Refund templates
	TemplateRefundPending   = "refund_pending"
	TemplateRefundProcessed = "refund_processed"
	TemplateRefundFailed    = "refund_failed"

	// Subscription reminder templates (for recurring subscriptions)
	TemplateSubscriptionReminder3d = "subscription_reminder_3d"
	TemplateSubscriptionReminder1d = "subscription_reminder_1d"

	// Subscription expiration templates (for one-off payments)
	TemplateSubscriptionExpiring3d = "subscription_expiring_3d"
	TemplateSubscriptionExpiring1d = "subscription_expiring_1d"

	// Refund-related affiliate templates
	TemplateCommissionRefunded = "commission_refunded"
	TemplateAccessRevoked      = "access_revoked"

	// Authentication templates (Migration 022)
	TemplateEmailVerification = "email_verification"
	TemplatePasswordReset     = "password_reset"

	// Affiliate link update template
	TemplateAffiliateLinkUpdated = "affiliate_link_updated"

	// Migration campaign templates
	TemplateMigrationAnnouncement  = "migration_announcement"
	TemplateMigrationFollowUp      = "migration_follow_up"
	TemplateMigrationVerifyEmail   = "migration_verify_email"
	TemplateMigrationUpgradeNudge  = "migration_upgrade_nudge"
	TemplateMigrationFinalNotice   = "migration_final_notice"

	TemplateMigrationVerifyEmailFinal  = "migration_verify_email_final"
	TemplateMigrationUpgradeNudgeFinal = "migration_upgrade_nudge_final"

	// Feature announcement template
	TemplateFeatureAnnouncement = "feature_announcement"
)
