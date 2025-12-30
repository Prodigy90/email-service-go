// Package client provides a Go client for the email service.
// Other services can import this package to send emails.
//
// Example usage:
//
//	client := client.New("http://email-service:8082", "your-api-key")
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
		baseURL: baseURL,
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
	DashboardURL    string `json:"dashboard_url,omitempty"`    // e.g., "https://wasbot.ng/dashboard"
	SupportEmail    string `json:"support_email,omitempty"`    // e.g., "support@wasbot.ng"
	WebsiteURL      string `json:"website_url,omitempty"`      // e.g., "https://wasbot.ng"
	SocialTwitter   string `json:"social_twitter,omitempty"`   // Twitter URL
	SocialInstagram string `json:"social_instagram,omitempty"` // Instagram URL
}

// SendRequest represents an email send request.
type SendRequest struct {
	To            string                 `json:"to"`
	Subject       string                 `json:"subject,omitempty"`
	Body          string                 `json:"body,omitempty"`
	HTMLBody      string                 `json:"html_body,omitempty"`
	Template      string                 `json:"template,omitempty"`
	TemplateData  map[string]interface{} `json:"template_data,omitempty"`
	IdempotencyID string                 `json:"idempotency_id,omitempty"`
	SourceService string                 `json:"source_service,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	ProductID     string                 `json:"product_id,omitempty"`
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
	TemplatePayoutApproved   = "payout_approved"
	TemplatePayoutRejected   = "payout_rejected"
	TemplatePayoutProcessed  = "payout_processed"
	TemplateCommissionEarned = "commission_earned"

	// Payments (recurring subscriptions)
	TemplatePaymentSuccess        = "payment_success"          // Recurring payment success
	TemplatePaymentFailed         = "payment_failed"           // Payment failure
	TemplateSubscriptionRenewed   = "subscription_renewed"     // Auto-renewal success
	TemplateSubscriptionExpiring  = "subscription_expiring"    // Generic expiring
	TemplateSubscriptionCancelled = "subscription_cancelled"   // Subscription cancelled

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
)
