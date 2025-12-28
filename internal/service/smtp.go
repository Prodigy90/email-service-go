package service

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/prodigy90/email-service-go/internal/config"
	"github.com/prodigy90/email-service-go/internal/domain"
	"github.com/rs/zerolog"
)

// SMTPClient handles sending emails via SMTP.
type SMTPClient struct {
	config config.SMTPConfig
	logger zerolog.Logger
}

// NewSMTPClient creates a new SMTP client.
func NewSMTPClient(cfg config.SMTPConfig, logger zerolog.Logger) *SMTPClient {
	return &SMTPClient{
		config: cfg,
		logger: logger.With().Str("component", "smtp").Logger(),
	}
}

// Send sends an email via SMTP.
func (c *SMTPClient) Send(email *domain.Email) error {
	from := email.From
	if from == "" {
		from = c.config.FromAddress
	}

	// Build the email message
	msg := c.buildMessage(from, email)

	// Connect and send
	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	var auth smtp.Auth
	if c.config.Username != "" && c.config.Password != "" {
		// Use PLAIN auth for AWS SES and most providers
		auth = smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.Host)
	}

	// For port 465 (SSL), we need to use TLS from the start
	if c.config.Port == 465 {
		return c.sendWithTLS(addr, auth, from, email.To, msg)
	}

	// For port 587 (STARTTLS) or 25, use standard SendMail
	err := smtp.SendMail(addr, auth, from, []string{email.To}, msg)
	if err != nil {
		c.logger.Error().Err(err).
			Str("to", email.To).
			Str("subject", email.Subject).
			Msg("Failed to send email")
		return fmt.Errorf("smtp send failed: %w", err)
	}

	c.logger.Info().
		Str("to", email.To).
		Str("subject", email.Subject).
		Str("email_id", email.ID.String()).
		Msg("Email sent successfully")

	return nil
}

// sendWithTLS sends email using implicit TLS (port 465).
func (c *SMTPClient) sendWithTLS(addr string, auth smtp.Auth, from, to string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: c.config.Host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("tls dial failed: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, c.config.Host)
	if err != nil {
		return fmt.Errorf("smtp client creation failed: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth failed: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail command failed: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt command failed: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data command failed: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write failed: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close failed: %w", err)
	}

	return client.Quit()
}

// buildMessage constructs the email message with headers.
func (c *SMTPClient) buildMessage(from string, email *domain.Email) []byte {
	var sb strings.Builder

	// Headers
	fromName := c.config.FromName
	if fromName != "" {
		sb.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, from))
	} else {
		sb.WriteString(fmt.Sprintf("From: %s\r\n", from))
	}

	sb.WriteString(fmt.Sprintf("To: %s\r\n", email.To))
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", email.Subject))

	// MIME headers for HTML support
	if email.HTMLBody != "" {
		sb.WriteString("MIME-Version: 1.0\r\n")
		sb.WriteString("Content-Type: multipart/alternative; boundary=\"boundary42\"\r\n")
		sb.WriteString("\r\n")

		// Plain text part
		sb.WriteString("--boundary42\r\n")
		sb.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
		sb.WriteString("\r\n")
		sb.WriteString(email.Body)
		sb.WriteString("\r\n")

		// HTML part
		sb.WriteString("--boundary42\r\n")
		sb.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
		sb.WriteString("\r\n")
		sb.WriteString(email.HTMLBody)
		sb.WriteString("\r\n")
		sb.WriteString("--boundary42--\r\n")
	} else {
		sb.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
		sb.WriteString("\r\n")
		sb.WriteString(email.Body)
	}

	return []byte(sb.String())
}
