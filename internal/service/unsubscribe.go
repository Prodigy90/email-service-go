package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// UnsubscribeService handles unsubscribe token generation and verification.
type UnsubscribeService struct {
	secret  string
	baseURL string
}

// NewUnsubscribeService creates a new unsubscribe service.
func NewUnsubscribeService(secret, baseURL string) *UnsubscribeService {
	return &UnsubscribeService{
		secret:  secret,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

// GenerateToken creates an HMAC-SHA256 token for the given email.
func (s *UnsubscribeService) GenerateToken(email string) string {
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write([]byte(strings.ToLower(email)))
	return hex.EncodeToString(mac.Sum(nil))
}

// GenerateURL creates a full unsubscribe URL for the given email.
func (s *UnsubscribeService) GenerateURL(email string) string {
	token := s.GenerateToken(email)
	return fmt.Sprintf("%s/unsubscribe?email=%s&token=%s",
		s.baseURL, url.QueryEscape(strings.ToLower(email)), token)
}

// Verify checks that the token is valid for the given email.
func (s *UnsubscribeService) Verify(email, token string) bool {
	expected := s.GenerateToken(email)
	return hmac.Equal([]byte(expected), []byte(token))
}
