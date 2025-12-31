package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/auth"
)

const sessionContextKey = "session"

func AuthRequired(sessions *auth.Manager) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		session, err := ReadSession(ctx, sessions)
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			ctx.Abort()
			return
		}

		ctx.Set(sessionContextKey, session)
		ctx.Next()
	}
}

func ReadSession(ctx *gin.Context, sessions *auth.Manager) (auth.Session, error) {
	session, err := readSessionFromCookie(ctx, sessions)
	if err == nil {
		return session, nil
	}

	token := bearerToken(ctx.GetHeader("Authorization"))
	if token == "" {
		return auth.Session{}, err
	}

	return sessions.Decode(token)
}

func readSessionFromCookie(ctx *gin.Context, sessions *auth.Manager) (auth.Session, error) {
	value, err := ctx.Cookie(auth.SessionCookieName)
	if err != nil {
		return auth.Session{}, err
	}
	return sessions.Decode(value)
}

func bearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func SessionFromContext(ctx *gin.Context) (auth.Session, bool) {
	value, ok := ctx.Get(sessionContextKey)
	if !ok {
		return auth.Session{}, false
	}

	session, ok := value.(auth.Session)
	return session, ok
}
