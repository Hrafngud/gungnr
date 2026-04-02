package models

import "time"

// TestTokenRequest is the request body for generating a test token.
type TestTokenRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// AuthUserResponse is the API response shape for an authenticated user.
type AuthUserResponse struct {
	ID        uint      `json:"id"`
	Login     string    `json:"login"`
	AvatarURL string    `json:"avatarUrl"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// TestTokenResponse is the API response for a generated test token.
type TestTokenResponse struct {
	Token     string           `json:"token"`
	TokenType string           `json:"tokenType"`
	ExpiresAt time.Time        `json:"expiresAt"`
	User      AuthUserResponse `json:"user"`
}
