package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func newTestWebhookService(secret string) *WebhookService {
	return &WebhookService{
		signingSecret: secret,
		logger:        zerolog.Nop(),
	}
}

// generateSvixSignature produces a valid Svix v1 signature for testing.
func generateSvixSignature(secret, msgID, timestamp string, body []byte) string {
	// Strip whsec_ prefix and decode
	if len(secret) > 6 && secret[:6] == "whsec_" {
		secret = secret[6:]
	}
	secretBytes, _ := base64.StdEncoding.DecodeString(secret)

	toSign := fmt.Sprintf("%s.%s.%s", msgID, timestamp, string(body))
	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(toSign))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return "v1," + sig
}

func TestVerifySvixSignature_Valid(t *testing.T) {
	// Generate a test secret (base64 encoded random bytes)
	rawSecret := []byte("test-webhook-secret-key-1234")
	b64Secret := base64.StdEncoding.EncodeToString(rawSecret)
	secret := "whsec_" + b64Secret

	svc := newTestWebhookService(secret)

	body := []byte(`{"type":"email.delivered","data":{"email_id":"abc123"}}`)
	msgID := "msg_test123"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := generateSvixSignature(secret, msgID, timestamp, body)

	err := svc.verifySvixSignature(body, msgID, timestamp, signature)
	if err != nil {
		t.Fatalf("expected valid signature, got error: %v", err)
	}
}

func TestVerifySvixSignature_InvalidSignature(t *testing.T) {
	rawSecret := []byte("test-webhook-secret-key-1234")
	b64Secret := base64.StdEncoding.EncodeToString(rawSecret)
	secret := "whsec_" + b64Secret

	svc := newTestWebhookService(secret)

	body := []byte(`{"type":"email.delivered","data":{"email_id":"abc123"}}`)
	msgID := "msg_test123"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	err := svc.verifySvixSignature(body, msgID, timestamp, "v1,invalidsignature")
	if err == nil {
		t.Fatal("expected error for invalid signature, got nil")
	}
}

func TestVerifySvixSignature_MissingHeaders(t *testing.T) {
	svc := newTestWebhookService("whsec_" + base64.StdEncoding.EncodeToString([]byte("key")))

	tests := []struct {
		name      string
		msgID     string
		timestamp string
		signature string
	}{
		{"missing msgID", "", "123", "v1,sig"},
		{"missing timestamp", "msg_1", "", "v1,sig"},
		{"missing signature", "msg_1", "123", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.verifySvixSignature([]byte("body"), tt.msgID, tt.timestamp, tt.signature)
			if err == nil {
				t.Fatal("expected error for missing headers")
			}
		})
	}
}

func TestVerifySvixSignature_ExpiredTimestamp(t *testing.T) {
	rawSecret := []byte("test-webhook-secret-key-1234")
	b64Secret := base64.StdEncoding.EncodeToString(rawSecret)
	secret := "whsec_" + b64Secret

	svc := newTestWebhookService(secret)

	body := []byte(`{"type":"email.delivered"}`)
	msgID := "msg_test123"
	// 10 minutes ago - should be rejected (>5 min tolerance)
	timestamp := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	signature := generateSvixSignature(secret, msgID, timestamp, body)

	err := svc.verifySvixSignature(body, msgID, timestamp, signature)
	if err == nil {
		t.Fatal("expected error for expired timestamp")
	}
}

func TestVerifySvixSignature_MultipleSignatures(t *testing.T) {
	rawSecret := []byte("test-webhook-secret-key-1234")
	b64Secret := base64.StdEncoding.EncodeToString(rawSecret)
	secret := "whsec_" + b64Secret

	svc := newTestWebhookService(secret)

	body := []byte(`{"type":"email.opened"}`)
	msgID := "msg_multi"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	validSig := generateSvixSignature(secret, msgID, timestamp, body)

	// Simulate header with multiple signatures (old + current)
	multiSig := "v1,oldinvalidsig " + validSig
	err := svc.verifySvixSignature(body, msgID, timestamp, multiSig)
	if err != nil {
		t.Fatalf("expected valid with multiple signatures, got error: %v", err)
	}
}

func TestVerifySvixSignature_SecretWithoutPrefix(t *testing.T) {
	rawSecret := []byte("test-no-prefix-secret")
	b64Secret := base64.StdEncoding.EncodeToString(rawSecret)
	// No whsec_ prefix - should still work
	secret := b64Secret

	svc := newTestWebhookService(secret)

	body := []byte(`{"type":"email.clicked"}`)
	msgID := "msg_noprefix"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := generateSvixSignature(secret, msgID, timestamp, body)

	err := svc.verifySvixSignature(body, msgID, timestamp, signature)
	if err != nil {
		t.Fatalf("expected valid without prefix, got error: %v", err)
	}
}

func TestVerifySvixSignature_TamperedBody(t *testing.T) {
	rawSecret := []byte("test-tamper-secret")
	b64Secret := base64.StdEncoding.EncodeToString(rawSecret)
	secret := "whsec_" + b64Secret

	svc := newTestWebhookService(secret)

	originalBody := []byte(`{"type":"email.delivered","data":{"email_id":"abc"}}`)
	msgID := "msg_tamper"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := generateSvixSignature(secret, msgID, timestamp, originalBody)

	// Tamper with body
	tamperedBody := []byte(`{"type":"email.delivered","data":{"email_id":"xyz"}}`)
	err := svc.verifySvixSignature(tamperedBody, msgID, timestamp, signature)
	if err == nil {
		t.Fatal("expected error for tampered body")
	}
}
