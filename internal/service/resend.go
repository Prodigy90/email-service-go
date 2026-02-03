package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/prodigy90/email-service-go/internal/config"
	"github.com/prodigy90/email-service-go/internal/domain"
	"github.com/rs/zerolog"
)

const resendAPIURL = "https://api.resend.com/emails"

// ResendClient handles sending emails via Resend API.
type ResendClient struct {
	config     config.ResendConfig
	httpClient *http.Client
	logger     zerolog.Logger
}

// NewResendClient creates a new Resend client.
func NewResendClient(cfg config.ResendConfig, logger zerolog.Logger) *ResendClient {
	return &ResendClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger.With().Str("component", "resend").Logger(),
	}
}

// resendRequest is the request body for Resend API.
type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text,omitempty"`
	HTML    string   `json:"html,omitempty"`
}

// resendResponse is the response from Resend API.
type resendResponse struct {
	ID string `json:"id,omitempty"`
}

// resendError is the error response from Resend API.
type resendError struct {
	StatusCode int    `json:"statusCode,omitempty"`
	Name       string `json:"name,omitempty"`
	Message    string `json:"message,omitempty"`
}

// Send sends an email via Resend API.
func (c *ResendClient) Send(email *domain.Email) error {
	from := email.From
	if from == "" {
		if c.config.FromAddress == "" {
			return fmt.Errorf("no from address configured: set FROM_EMAIL")
		}
		if c.config.FromName != "" {
			from = fmt.Sprintf("%s <%s>", c.config.FromName, c.config.FromAddress)
		} else {
			from = c.config.FromAddress
		}
	}

	reqBody := resendRequest{
		From:    from,
		To:      []string{email.To},
		Subject: email.Subject,
		Text:    email.Body,
		HTML:    email.HTMLBody,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", resendAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error().Err(err).
			Str("to", email.To).
			Str("subject", email.Subject).
			Msg("Failed to send email via Resend")
		return fmt.Errorf("resend request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp resendError
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
			c.logger.Error().
				Int("status_code", resp.StatusCode).
				Str("error", errResp.Message).
				Str("to", email.To).
				Msg("Resend API error")
			return fmt.Errorf("resend API error (%d): %s", resp.StatusCode, errResp.Message)
		}
		return fmt.Errorf("resend API error (%d): %s", resp.StatusCode, string(body))
	}

	var result resendResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	c.logger.Info().
		Str("to", email.To).
		Str("subject", email.Subject).
		Str("email_id", email.ID.String()).
		Str("resend_id", result.ID).
		Msg("Email sent successfully via Resend")

	return nil
}
