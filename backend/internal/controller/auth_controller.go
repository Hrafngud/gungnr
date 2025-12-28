package controller

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-notes/internal/auth"
	"go-notes/internal/service"
)

const oauthStateCookie = "warp_oauth_state"

type AuthController struct {
	service      *service.AuthService
	sessions     *auth.Manager
	secureCookie bool
	cookieDomain string
}

type authUserResponse struct {
	ID        uint      `json:"id"`
	Login     string    `json:"login"`
	AvatarURL string    `json:"avatarUrl"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func NewAuthController(service *service.AuthService, sessions *auth.Manager, secureCookie bool, cookieDomain string) *AuthController {
	return &AuthController{
		service:      service,
		sessions:     sessions,
		secureCookie: secureCookie,
		cookieDomain: cookieDomain,
	}
}

func (c *AuthController) Register(r *gin.Engine) {
	r.GET("/auth/login", c.Login)
	r.GET("/auth/callback", c.Callback)
	r.GET("/auth/me", c.Me)
	r.POST("/auth/logout", c.Logout)
}

func (c *AuthController) Login(ctx *gin.Context) {
	state, err := generateState()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate state"})
		return
	}

	c.setCookie(ctx, oauthStateCookie, state, 300)
	callbackURL := c.resolveCallbackURL(ctx)
	ctx.Redirect(http.StatusFound, c.service.AuthURL(state, callbackURL))
}

func (c *AuthController) Callback(ctx *gin.Context) {
	state := ctx.Query("state")
	code := ctx.Query("code")
	if state == "" || code == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "missing code or state"})
		return
	}

	cookieState, err := ctx.Cookie(oauthStateCookie)
	if err != nil || cookieState != state {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid oauth state"})
		return
	}

	c.clearCookie(ctx, oauthStateCookie)

	callbackURL := c.resolveCallbackURL(ctx)
	user, err := c.service.Exchange(ctx, code, callbackURL)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUnauthorized):
			ctx.JSON(http.StatusForbidden, gin.H{"error": "user not allowed"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		}
		return
	}

	session := c.sessions.NewSession(user.ID, user.Login, user.AvatarURL)
	value, err := c.sessions.Encode(session)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	maxAge := int(time.Until(session.ExpiresAt).Seconds())
	c.setCookie(ctx, auth.SessionCookieName, value, maxAge)

	ctx.Redirect(http.StatusFound, "/")
}

func (c *AuthController) Me(ctx *gin.Context) {
	session, err := c.readSession(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}

	ctx.JSON(http.StatusOK, authUserResponse{
		ID:        session.UserID,
		Login:     session.Login,
		AvatarURL: session.AvatarURL,
		ExpiresAt: session.ExpiresAt,
	})
}

func (c *AuthController) Logout(ctx *gin.Context) {
	c.clearCookie(ctx, auth.SessionCookieName)
	ctx.Status(http.StatusNoContent)
}

func (c *AuthController) readSession(ctx *gin.Context) (auth.Session, error) {
	value, err := ctx.Cookie(auth.SessionCookieName)
	if err != nil {
		return auth.Session{}, err
	}
	return c.sessions.Decode(value)
}

func (c *AuthController) setCookie(ctx *gin.Context, name, value string, maxAge int) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(name, value, maxAge, "/", c.cookieDomain, c.secureCookie, true)
}

func (c *AuthController) clearCookie(ctx *gin.Context, name string) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(name, "", -1, "/", c.cookieDomain, c.secureCookie, true)
}

func (c *AuthController) resolveCallbackURL(ctx *gin.Context) string {
	configured := c.service.CallbackURL()
	requestHost := requestHost(ctx)

	if configured == "" {
		return callbackURLFromRequest(ctx, requestHost)
	}

	parsed, err := url.Parse(configured)
	if err == nil && isLocalHost(parsed.Hostname()) && !isLocalHost(requestHost) {
		return callbackURLFromRequest(ctx, requestHost)
	}

	return configured
}

func callbackURLFromRequest(ctx *gin.Context, host string) string {
	return fmt.Sprintf("%s://%s/auth/callback", requestScheme(ctx), host)
}

func requestScheme(ctx *gin.Context) string {
	if proto := ctx.GetHeader("X-Forwarded-Proto"); proto != "" {
		return strings.TrimSpace(strings.Split(proto, ",")[0])
	}
	if ctx.Request.TLS != nil {
		return "https"
	}
	return "http"
}

func requestHost(ctx *gin.Context) string {
	if forwarded := ctx.GetHeader("X-Forwarded-Host"); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	return ctx.Request.Host
}

func isLocalHost(host string) bool {
	normalized := strings.ToLower(strings.TrimSpace(hostnameOnly(host)))
	return normalized == "localhost" || normalized == "127.0.0.1" || normalized == "::1"
}

func hostnameOnly(host string) string {
	if host == "" {
		return ""
	}
	if strings.Contains(host, ":") {
		if parsed, _, err := net.SplitHostPort(host); err == nil {
			return parsed
		}
	}
	return host
}

func generateState() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
