package controller

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"go-notes/internal/auth"
	"go-notes/internal/service"
)

const (
	sessionCookieName = "warp_session"
	oauthStateCookie  = "warp_oauth_state"
)

type AuthController struct {
	service      *service.AuthService
	sessions     *auth.Manager
	secureCookie bool
}

type authUserResponse struct {
	ID        uint      `json:"id"`
	Login     string    `json:"login"`
	AvatarURL string    `json:"avatarUrl"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func NewAuthController(service *service.AuthService, sessions *auth.Manager, secureCookie bool) *AuthController {
	return &AuthController{
		service:      service,
		sessions:     sessions,
		secureCookie: secureCookie,
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
	ctx.Redirect(http.StatusFound, c.service.AuthURL(state))
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

	user, err := c.service.Exchange(ctx, code)
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
	c.setCookie(ctx, sessionCookieName, value, maxAge)

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
	c.clearCookie(ctx, sessionCookieName)
	ctx.Status(http.StatusNoContent)
}

func (c *AuthController) readSession(ctx *gin.Context) (auth.Session, error) {
	value, err := ctx.Cookie(sessionCookieName)
	if err != nil {
		return auth.Session{}, err
	}
	return c.sessions.Decode(value)
}

func (c *AuthController) setCookie(ctx *gin.Context, name, value string, maxAge int) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(name, value, maxAge, "/", "", c.secureCookie, true)
}

func (c *AuthController) clearCookie(ctx *gin.Context, name string) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(name, "", -1, "/", "", c.secureCookie, true)
}

func generateState() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
