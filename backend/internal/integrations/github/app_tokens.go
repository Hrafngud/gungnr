package github

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	gogithub "github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
)

type AppInstallationCredentials struct {
	AppID          int64
	InstallationID int64
	PrivateKey     string
}

func ParseAppInstallationCredentials(appIDRaw, installationIDRaw, privateKey string) (AppInstallationCredentials, error) {
	appIDRaw = strings.TrimSpace(appIDRaw)
	installationIDRaw = strings.TrimSpace(installationIDRaw)
	privateKey = normalizePrivateKey(privateKey)

	if appIDRaw == "" || installationIDRaw == "" || privateKey == "" {
		return AppInstallationCredentials{}, errors.New("github app credentials are incomplete")
	}

	appID, err := strconv.ParseInt(appIDRaw, 10, 64)
	if err != nil || appID <= 0 {
		return AppInstallationCredentials{}, fmt.Errorf("invalid github app id: %s", appIDRaw)
	}

	installationID, err := strconv.ParseInt(installationIDRaw, 10, 64)
	if err != nil || installationID <= 0 {
		return AppInstallationCredentials{}, fmt.Errorf("invalid github app installation id: %s", installationIDRaw)
	}

	return AppInstallationCredentials{
		AppID:          appID,
		InstallationID: installationID,
		PrivateKey:     privateKey,
	}, nil
}

func MintInstallationToken(ctx context.Context, creds AppInstallationCredentials) (string, error) {
	jwtToken, err := createAppJWT(creds.AppID, creds.PrivateKey)
	if err != nil {
		return "", err
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: jwtToken})
	httpClient := WrapHTTPClient(oauth2.NewClient(ctx, ts))
	api := gogithub.NewClient(httpClient)

	token, _, err := api.Apps.CreateInstallationToken(ctx, creds.InstallationID, &gogithub.InstallationTokenOptions{})
	if err != nil {
		detail := FormatError(err)
		if detail == "" {
			return "", fmt.Errorf("mint installation token: %w", err)
		}
		return "", fmt.Errorf("mint installation token: %w; %s", err, detail)
	}
	if token == nil || strings.TrimSpace(token.GetToken()) == "" {
		return "", errors.New("installation token missing from response")
	}
	return strings.TrimSpace(token.GetToken()), nil
}

func createAppJWT(appID int64, privateKey string) (string, error) {
	key, err := parsePrivateKey(privateKey)
	if err != nil {
		return "", err
	}

	now := time.Now()
	header, err := json.Marshal(map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", fmt.Errorf("encode github app jwt header: %w", err)
	}
	claims := appJWTClaims{
		Issuer:    fmt.Sprintf("%d", appID),
		IssuedAt:  now.Add(-1 * time.Minute).Unix(),
		ExpiresAt: now.Add(9 * time.Minute).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encode github app jwt claims: %w", err)
	}

	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign github app jwt: %w", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func parsePrivateKey(privateKey string) (*rsa.PrivateKey, error) {
	normalized := normalizePrivateKey(privateKey)
	block, _ := pem.Decode([]byte(normalized))
	if block == nil {
		return nil, errors.New("github app private key is not valid PEM")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse github app private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("github app private key must be RSA")
	}
	return key, nil
}

func normalizePrivateKey(input string) string {
	trimmed := strings.TrimSpace(input)
	if strings.Contains(trimmed, "\\n") {
		trimmed = strings.ReplaceAll(trimmed, "\\n", "\n")
	}
	return trimmed
}

type appJWTClaims struct {
	Issuer    string `json:"iss"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}
