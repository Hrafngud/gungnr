package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidSession = errors.New("invalid session")
	ErrExpiredSession = errors.New("session expired")
)

type Session struct {
	UserID    uint      `json:"userId"`
	Login     string    `json:"login"`
	AvatarURL string    `json:"avatarUrl"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (m *Manager) Encode(session Session) (string, error) {
	payload, err := json.Marshal(session)
	if err != nil {
		return "", err
	}

	encoded := base64.RawURLEncoding.EncodeToString(payload)
	signature := m.sign(encoded)

	return encoded + "." + signature, nil
}

func (m *Manager) Decode(value string) (Session, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return Session{}, ErrInvalidSession
	}

	payload := parts[0]
	if !m.verify(payload, parts[1]) {
		return Session{}, ErrInvalidSession
	}

	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return Session{}, ErrInvalidSession
	}

	var session Session
	if err := json.Unmarshal(raw, &session); err != nil {
		return Session{}, ErrInvalidSession
	}

	if time.Now().After(session.ExpiresAt) {
		return Session{}, ErrExpiredSession
	}

	return session, nil
}

func (m *Manager) NewSession(userID uint, login, avatarURL string) Session {
	return Session{
		UserID:    userID,
		Login:     login,
		AvatarURL: avatarURL,
		ExpiresAt: time.Now().Add(m.ttl),
	}
}

func (m *Manager) sign(value string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m *Manager) verify(value, signature string) bool {
	expected := m.sign(value)
	return hmac.Equal([]byte(expected), []byte(signature))
}
