package service

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/prodigy90/email-service-go/internal/domain"
)

// mockSender is a test double for EmailSender.
type mockSender struct {
	providerID string
	sendErr    error
	sent       []*domain.Email
}

func (m *mockSender) Send(email *domain.Email) (*SendResult, error) {
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	m.sent = append(m.sent, email)
	return &SendResult{ProviderID: m.providerID}, nil
}

func TestMockSenderImplementsInterface(t *testing.T) {
	// Compile-time check that mockSender satisfies EmailSender
	var _ EmailSender = (*mockSender)(nil)
}

func TestSendResult_ProviderID(t *testing.T) {
	sender := &mockSender{providerID: "resend_abc123"}
	email := &domain.Email{
		ID:      uuid.New(),
		To:      "test@example.com",
		Subject: "Test",
		Body:    "Hello",
	}

	result, err := sender.Send(email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ProviderID != "resend_abc123" {
		t.Errorf("expected provider ID 'resend_abc123', got %q", result.ProviderID)
	}
	if len(sender.sent) != 1 {
		t.Errorf("expected 1 sent email, got %d", len(sender.sent))
	}
}

func TestSendResult_EmptyProviderID(t *testing.T) {
	// SMTP sender returns empty provider ID
	sender := &mockSender{providerID: ""}
	email := &domain.Email{
		ID:      uuid.New(),
		To:      "test@example.com",
		Subject: "Test",
		Body:    "Hello",
	}

	result, err := sender.Send(email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ProviderID != "" {
		t.Errorf("expected empty provider ID, got %q", result.ProviderID)
	}
}

func TestSendResult_Error(t *testing.T) {
	sender := &mockSender{sendErr: fmt.Errorf("send failed")}
	email := &domain.Email{
		ID:      uuid.New(),
		To:      "test@example.com",
		Subject: "Test",
		Body:    "Hello",
	}

	result, err := sender.Send(email)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
}
