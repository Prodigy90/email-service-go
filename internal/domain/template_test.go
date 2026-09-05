package domain

import (
	"testing"
)

func TestTemplateConstants(t *testing.T) {
	// Verify all template constants have the expected string values
	// that match the template names defined in the YAML files.
	tests := []struct {
		constant string
		expected string
	}{
		// Escalating dunning templates
		{TemplatePaymentFailed, "payment_failed"},
		{TemplatePaymentFailedAttempt2, "payment_failed_attempt2"},
		{TemplatePaymentFailedFinal, "payment_failed_final"},

		// Subscription reminder templates (recurring, with addon support)
		{TemplateSubscriptionReminder3d, "subscription_reminder_3d"},
		{TemplateSubscriptionReminder1d, "subscription_reminder_1d"},

		// Subscription expiring templates (one-off, with consequences)
		{TemplateSubscriptionExpiring3d, "subscription_expiring_3d"},
		{TemplateSubscriptionExpiring1d, "subscription_expiring_1d"},

		// Other payment templates for completeness
		{TemplatePaymentSuccess, "payment_success"},
		{TemplateSubscriptionRenewed, "subscription_renewed"},
		{TemplateSubscriptionCancelled, "subscription_cancelled"},
		{TemplatePaymentSuccessOnetime, "payment_success_onetime"},
		{TemplateSubscriptionActivated, "subscription_activated"},
		{TemplateSubscriptionActivatedOnetime, "subscription_activated_onetime"},

		// Affiliate templates
		{TemplatePayoutApproved, "payout_approved"},
		{TemplatePayoutRejected, "payout_rejected"},
		{TemplatePayoutProcessed, "payout_processed"},
		{TemplateCommissionEarned, "commission_earned"},
		{TemplateCommissionRefunded, "commission_refunded"},
		{TemplateAccessRevoked, "access_revoked"},
		{TemplateAffiliateLinkUpdated, "affiliate_link_updated"},

		// Auth templates
		{TemplateEmailVerification, "email_verification"},
		{TemplatePasswordReset, "password_reset"},

		// WASBOT templates
		{TemplateWelcome, "welcome"},
		{TemplateAccountUpgraded, "account_upgraded"},

		// Refund templates
		{TemplateRefundPending, "refund_pending"},
		{TemplateRefundProcessed, "refund_processed"},
		{TemplateRefundFailed, "refund_failed"},

		// Migration templates
		{TemplateMigrationAnnouncement, "migration_announcement"},
		{TemplateMigrationFollowUp, "migration_follow_up"},
		{TemplateMigrationVerifyEmail, "migration_verify_email"},
		{TemplateMigrationUpgradeNudge, "migration_upgrade_nudge"},
		{TemplateMigrationFinalNotice, "migration_final_notice"},
		{TemplateMigrationVerifyEmailFinal, "migration_verify_email_final"},
		{TemplateMigrationUpgradeNudgeFinal, "migration_upgrade_nudge_final"},

		// Feature announcement
		{TemplateFeatureAnnouncement, "feature_announcement"},

		// Generic expiring (deprecated)
		{TemplateSubscriptionExpiring, "subscription_expiring"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("constant value = %q, expected %q", tt.constant, tt.expected)
			}
		})
	}
}

func TestTemplateConstantsNotEmpty(t *testing.T) {
	constants := []string{
		TemplatePaymentFailed,
		TemplatePaymentFailedAttempt2,
		TemplatePaymentFailedFinal,
		TemplateSubscriptionReminder3d,
		TemplateSubscriptionReminder1d,
		TemplateSubscriptionExpiring3d,
		TemplateSubscriptionExpiring1d,
		TemplatePaymentSuccess,
		TemplateSubscriptionRenewed,
		TemplateSubscriptionCancelled,
		TemplatePaymentSuccessOnetime,
		TemplateSubscriptionActivated,
		TemplateSubscriptionActivatedOnetime,
		TemplatePayoutApproved,
		TemplatePayoutRejected,
		TemplatePayoutProcessed,
		TemplateCommissionEarned,
		TemplateCommissionRefunded,
		TemplateAccessRevoked,
		TemplateAffiliateLinkUpdated,
		TemplateEmailVerification,
		TemplatePasswordReset,
		TemplateWelcome,
		TemplateAccountUpgraded,
		TemplateRefundPending,
		TemplateRefundProcessed,
		TemplateRefundFailed,
		TemplateMigrationAnnouncement,
		TemplateMigrationFollowUp,
		TemplateMigrationVerifyEmail,
		TemplateMigrationUpgradeNudge,
		TemplateMigrationFinalNotice,
		TemplateMigrationVerifyEmailFinal,
		TemplateMigrationUpgradeNudgeFinal,
		TemplateFeatureAnnouncement,
		TemplateSubscriptionExpiring,
	}

	for _, c := range constants {
		if c == "" {
			t.Errorf("found empty template constant")
		}
	}
}
