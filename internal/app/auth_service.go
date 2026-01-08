package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"blogcms/internal/domain"
)

type AuthService struct {
	users      domain.UserRepository
	sessionKey []byte
}

func NewAuthService(users domain.UserRepository, sessionKey string) *AuthService {
	return &AuthService{
		users:      users,
		sessionKey: []byte(sessionKey),
	}
}

func (s *AuthService) Authenticate(ctx context.Context, username, password string) (domain.User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return domain.User{}, domain.ErrUnauthorized
	}

	u, err := s.users.ByUsername(ctx, username)
	if err != nil {
		return domain.User{}, domain.ErrUnauthorized
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return domain.User{}, domain.ErrUnauthorized
	}
	return u, nil
}

// SignSessionToken creates an HMAC-signed token for a session id.
func (s *AuthService) SignSessionToken(sessionID string) string {
	mac := hmac.New(sha256.New, s.sessionKey)
	mac.Write([]byte(sessionID))
	sig := hex.EncodeToString(mac.Sum(nil))
	return sessionID + "." + sig
}

func (s *AuthService) VerifySessionToken(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", domain.ErrUnauthorized
	}
	sessionID, sig := parts[0], parts[1]

	mac := hmac.New(sha256.New, s.sessionKey)
	mac.Write([]byte(sessionID))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return "", domain.ErrUnauthorized
	}
	return sessionID, nil
}

// CSRFToken returns a stable per-session CSRF token derived from the session id and the server session key.
// This avoids token drift and works reliably across all request types (including multipart uploads).
func (s *AuthService) CSRFToken(sessionID string) string {
	mac := hmac.New(sha256.New, s.sessionKey)
	mac.Write([]byte("csrf:"))
	mac.Write([]byte(sessionID))
	return hex.EncodeToString(mac.Sum(nil))
}

func HashPassword(password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", fmt.Errorf("%w: password is required", domain.ErrInvalidArgument)
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt: %w", err)
	}
	return string(b), nil
}
