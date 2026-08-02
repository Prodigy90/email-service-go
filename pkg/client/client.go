// Package client provides a Go client for the email service.
// Other services can import this package to send emails.
//
// Example usage:
//
//	client := client.New("http://email-service:8083", "your-api-key")
//	resp, err := client.Send(ctx, &client.SendRequest{
//	    To:       "user@example.com",
//	    Template: "payout_approved",
//	    TemplateData: map[string]interface{}{
//	        "Name":     "John",
//	        "Amount":   "5000",
//	        "Currency": "₦",
//	    },
//	})
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Client is the email service client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates a new email service client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// WithHTTPClient sets a custom HTTP client.
func (c *Client) WithHTTPClient(httpClient *http.Client) *Client {
	c.httpClient = httpClient
	return c
}

// BrandingConfig holds product-specific branding for email templates.
// This allows different products to customize email appearance.
type BrandingConfig struct {
	PrimaryColor    string `json:"primary_color,omitempty"`    // e.g., "#10b981" (green for wasbot)
	SecondaryColor  string `json:"secondary_color,omitempty"`  // e.g., "#059669"
	AccentColor     string `json:"accent_color,omitempty"`     // e.g., "#047857"
	DangerColor     string `json:"danger_color,omitempty"`     // e.g., "#ef4444"
	CompanyName     string `json:"company_name,omitempty"`     // e.g., "WASBOT"
	LogoURL         string `json:"logo_url,omitempty"`         // URL to company logo
	DashboardURL    string `json:"dashboard_url,omitempty"`    // e.g., "https://www.wasbot.app/dashboard"
	SupportEmail    string `json:"support_email,omitempty"`    // e.g., "support@wasbot.app"
	WebsiteURL      string `json:"website_url,omitempty"`      // e.g., "https://www.wasbot.app"
	SocialTwitter    string `json:"social_twitter,omitempty"`    // X (Twitter) URL
	SocialInstagram  string `json:"social_instagram,omitempty"`  // Instagram URL
	IconTwitterURL   string `json:"icon_twitter_url,omitempty"`  // URL to X/Twitter icon image
	IconInstagramURL string `json:"icon_instagram_url,omitempty"` // URL to Instagram icon image
	SocialYouTube    string `json:"social_youtube,omitempty"`    // YouTube URL
	IconYouTubeURL   string `json:"icon_youtube_url,omitempty"`  // URL to YouTube icon image
}

// SendRequest represents an email send request.
type SendRequest struct {
	To            string                 `json:"to"`
	From          string                 `json:"from,omitempty"` // optional sender override ("Name <addr@domain>"); falls back to configured default
	Subject       string                 `json:"subject,omitempty"`
	Body          string                 `json:"body,omitempty"`
	HTMLBody      string                 `json:"html_body,omitempty"`
	Template      string                 `json:"template,omitempty"`
	TemplateData  map[string]interface{} `json:"template_data,omitempty"`
	IdempotencyID string                 `json:"idempotency_id,omitempty"`
	SourceService string                 `json:"source_service,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	ProductID     string                 `json:"product_id,omitempty"`
	Headers       map[string]string      `json:"headers,omitempty"`
	Branding      *BrandingConfig        `json:"branding,omitempty"`
}

// SendBulkRequest represents a bulk email send request.
type SendBulkRequest struct {
	Recipients    []string               `json:"recipients"`
	Subject       string                 `json:"subject,omitempty"`
	Body          string                 `json:"body,omitempty"`
	HTMLBody      string                 `json:"html_body,omitempty"`
	Template      string                 `json:"template,omitempty"`
	TemplateData  map[string]interface{} `json:"template_data,omitempty"`
	SourceService string                 `json:"source_service,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	ProductID     string                 `json:"product_id,omitempty"`
	Branding      *BrandingConfig        `json:"branding,omitempty"`
}

// SendResponse represents the response from sending an email.
type SendResponse struct {
	ID      uuid.UUID `json:"id"`
	Status  string    `json:"status"`
	Message string    `json:"message"`
}

// SendBulkResponse represents the response from sending bulk emails.
type SendBulkResponse struct {
	TotalQueued int         `json:"total_queued"`
	EmailIDs    []uuid.UUID `json:"email_ids"`
	Message     string      `json:"message"`
}

// StatusResponse represents the email status.
type StatusResponse struct {
	ID           uuid.UUID  `json:"id"`
	To           string     `json:"to"`
	Subject      string     `json:"subject"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"error_message,omitempty"`
	RetryCount   int        `json:"retry_count"`
	SentAt       *time.Time `json:"sent_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// TemplateInfo represents template information.
type TemplateInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Variables   []string `json:"variables,omitempty"`
}

// TemplatesResponse represents the list of templates.
type TemplatesResponse struct {
	Templates []TemplateInfo `json:"templates"`
}

// Send sends an email.
func (c *Client) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	var resp SendResponse
	if err := c.doRequest(ctx, "POST", "/api/v1/send", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SendBulk sends emails to multiple recipients.
func (c *Client) SendBulk(ctx context.Context, req *SendBulkRequest) (*SendBulkResponse, error) {
	var resp SendBulkResponse
	if err := c.doRequest(ctx, "POST", "/api/v1/send/bulk", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetStatus retrieves the status of an email.
func (c *Client) GetStatus(ctx context.Context, emailID uuid.UUID) (*StatusResponse, error) {
	var resp StatusResponse
	if err := c.doRequest(ctx, "GET", "/api/v1/status/"+emailID.String(), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListTemplates returns available email templates.
func (c *Client) ListTemplates(ctx context.Context) (*TemplatesResponse, error) {
	var resp TemplatesResponse
	if err := c.doRequest(ctx, "GET", "/api/v1/templates", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CampaignStatsResponse represents campaign aggregate stats.
type CampaignStatsResponse struct {
	CampaignTag string `json:"campaign_tag"`
	TotalSent   int    `json:"total_sent"`
	Delivered   int    `json:"delivered"`
	Opened      int    `json:"opened"`
	Clicked     int    `json:"clicked"`
	Bounced     int    `json:"bounced"`
	Complained  int    `json:"complained"`
}

// NonOpenersResponse represents the list of non-opener email addresses.
type NonOpenersResponse struct {
	CampaignTag string   `json:"campaign_tag"`
	Count       int      `json:"count"`
	Addresses   []string `json:"addresses"`
}

// GetCampaignStats retrieves aggregate stats for a campaign.
func (c *Client) GetCampaignStats(ctx context.Context, campaignTag string) (*CampaignStatsResponse, error) {
	var resp CampaignStatsResponse
	if err := c.doRequest(ctx, "GET", "/api/v1/campaigns/"+url.PathEscape(campaignTag)+"/stats", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetCampaignNonOpeners retrieves email addresses that haven't opened emails from a campaign.
func (c *Client) GetCampaignNonOpeners(ctx context.Context, campaignTag string) (*NonOpenersResponse, error) {
	var resp NonOpenersResponse
	if err := c.doRequest(ctx, "GET", "/api/v1/campaigns/"+url.PathEscape(campaignTag)+"/non-openers", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// BouncedResponse represents the list of bounced email addresses for a campaign.
type BouncedResponse struct {
	CampaignTag string   `json:"campaign_tag"`
	Count       int      `json:"count"`
	Addresses   []string `json:"addresses"`
}

// SuppressionsCheckResponse represents the result of checking emails against the suppression list.
type SuppressionsCheckResponse struct {
	Suppressed []string `json:"suppressed"`
	Count      int      `json:"count"`
}

// GetCampaignBounced retrieves bounced/complained email addresses for a campaign.
func (c *Client) GetCampaignBounced(ctx context.Context, campaignTag string) (*BouncedResponse, error) {
	var resp BouncedResponse
	if err := c.doRequest(ctx, "GET", "/api/v1/campaigns/"+url.PathEscape(campaignTag)+"/bounced", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CheckSuppressions checks which emails from a list are in the suppression list.
func (c *Client) CheckSuppressions(ctx context.Context, emails []string) (*SuppressionsCheckResponse, error) {
	var resp SuppressionsCheckResponse
	req := struct {
		Emails []string `json:"emails"`
	}{Emails: emails}
	if err := c.doRequest(ctx, "POST", "/api/v1/suppressions/check", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// doRequest performs an HTTP request.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error != "" {
			return fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// Template name constants for convenience.
const (
	// Affiliate System
	TemplatePayoutApproved       = "payout_approved"
	TemplatePayoutRejected       = "payout_rejected"
	TemplatePayoutProcessed      = "payout_processed"
	TemplateCommissionEarned     = "commission_earned"
	TemplateAffiliateSignup      = "affiliate_signup"
	TemplateAffiliateLinkUpdated = "affiliate_link_updated"

	// Payments (recurring subscriptions)
	TemplatePaymentSuccess        = "payment_success"          // Recurring payment success
	TemplatePaymentFailed         = "payment_failed"           // Payment failure (attempt 1)
	TemplatePaymentFailedAttempt2 = "payment_failed_attempt2"  // Payment failure (attempt 2 — more urgent)
	TemplatePaymentFailedFinal    = "payment_failed_final"     // Payment failure (final — subscription suspended)
	TemplateSubscriptionRenewed   = "subscription_renewed"     // Auto-renewal success
	TemplateSubscriptionExpiring  = "subscription_expiring"    // Generic expiring
	TemplateSubscriptionCancelled = "subscription_cancelled"   // Subscription cancelled
	TemplateCheckoutAbandoned     = "checkout_abandoned"       // Checkout started but never completed

	// One-time payment templates (non-recurring)
	TemplatePaymentSuccessOnetime        = "payment_success_onetime"        // One-time payment confirmation
	TemplateSubscriptionActivated        = "subscription_activated"         // New recurring subscription
	TemplateSubscriptionActivatedOnetime = "subscription_activated_onetime" // One-time subscription

	// Refunds
	TemplateRefundPending      = "refund_pending"
	TemplateRefundProcessed    = "refund_processed"
	TemplateRefundFailed       = "refund_failed"
	TemplateCommissionRefunded = "commission_refunded"
	TemplateAccessRevoked      = "access_revoked"

	// Subscription reminders (for recurring subscriptions - renewal reminders)
	TemplateSubscriptionReminder3d = "subscription_reminder_3d"
	TemplateSubscriptionReminder1d = "subscription_reminder_1d"

	// Subscription expiration (for one-off payments - expiry reminders)
	TemplateSubscriptionExpiring3d = "subscription_expiring_3d"
	TemplateSubscriptionExpiring1d = "subscription_expiring_1d"

	// General
	TemplateWelcome         = "welcome"
	TemplateTrialExpiring   = "trial_expiring"
	TemplateAccountUpgraded = "account_upgraded"

	// Trial sequence
	TemplateTrialDay3  = "trial_day3"  // Day 3: Engagement check
	TemplateTrialDay5  = "trial_day5"  // Day 5: Feature highlight
	TemplateTrialDay6  = "trial_day6"  // Day 6: Urgent reminder
	TemplateTrialDay10 = "trial_day10" // Day 10: Win-back after expiry

	// Authentication
	TemplateEmailVerification = "email_verification"
	TemplatePasswordReset     = "password_reset"

	// Migration campaigns
	TemplateMigrationAnnouncement  = "migration_announcement"
	TemplateMigrationFollowUp      = "migration_follow_up"
	TemplateMigrationVerifyEmail   = "migration_verify_email"
	TemplateMigrationUpgradeNudge  = "migration_upgrade_nudge"
	TemplateMigrationFinalNotice   = "migration_final_notice"

	TemplateMigrationVerifyEmailFinal  = "migration_verify_email_final"
	TemplateMigrationUpgradeNudgeFinal = "migration_upgrade_nudge_final"

	// Campaign templates (flexible, content-driven)
	TemplateCampaignUpdate      = "campaign_update"       // Flexible shell — pass Subject, Intro, HTMLBody, optional VideoURL
	TemplateFeatureAnnouncement = "feature_announcement"  // Pre-built 4-feature announcement

	// Onboarding sequence (Migration 062)
	TemplateOnboardingDay0 = "onboarding_day0" // Signup — link your WhatsApp
	TemplateOnboardingDay1 = "onboarding_day1" // Auto-save contacts
	TemplateOnboardingDay2 = "onboarding_day2" // Cloud status posting
	TemplateOnboardingDay3 = "onboarding_day3" // Autoresponder
	TemplateOnboardingDay4 = "onboarding_day4" // Group blast
	TemplateOnboardingDay5 = "onboarding_day5" // Tier comparison + social proof
	TemplateOnboardingDay6 = "onboarding_day6" // Trial ends tomorrow
	TemplateOnboardingDay7 = "onboarding_day7" // Discount offer

	// Post-trial sequence
	TemplatePostTrialDay8  = "post_trial_day8"  // Your setup is saved
	TemplatePostTrialDay9  = "post_trial_day9"  // Social proof
	TemplatePostTrialDay10 = "post_trial_day10" // Final discount

	// Triggered emails
	TemplateStalledNoSession = "stalled_no_session" // Signed up but no session created
	TemplateStalledSession   = "stalled_session"    // Session created but not connected

	// Migration blast (April 2026 — wasbot.ng → wasbot.app)
	TemplateMigrationBlastApr2026 = "migration_blast_apr2026"

	// Win-back campaign — ad-origin leads who signed up but never paid (Ticket A)
	TemplateWinbackAdLeadsD0 = "winback_ad_leads_d0" // Day 0: reminder + 20% off
	TemplateWinbackAdLeadsD3 = "winback_ad_leads_d3" // Day 3: auto-save + full-reach status pitch
	TemplateWinbackAdLeadsD7 = "winback_ad_leads_d7" // Day 7: last call, offer sunsets
)
