package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-notes/internal/auth"
)

const sessionContextKey = "session"

func AuthRequired(sessions *auth.Manager) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		value, err := ctx.Cookie(auth.SessionCookieName)
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			ctx.Abort()
			return
		}

		session, err := sessions.Decode(value)
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			ctx.Abort()
			return
		}

		ctx.Set(sessionContextKey, session)
		ctx.Next()
	}
}

func SessionFromContext(ctx *gin.Context) (auth.Session, bool) {
	value, ok := ctx.Get(sessionContextKey)
	if !ok {
		return auth.Session{}, false
	}

	session, ok := value.(auth.Session)
	return session, ok
}
