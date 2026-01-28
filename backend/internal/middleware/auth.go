package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/apierror"
	"go-notes/internal/auth"
	"go-notes/internal/errs"
	"go-notes/internal/models"
)

const sessionContextKey = "session"

var roleRank = map[string]int{
	models.RoleUser:      1,
	models.RoleAdmin:     2,
	models.RoleSuperUser: 3,
}

func AuthRequired(sessions *auth.Manager) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		session, err := ReadSession(ctx, sessions)
		if err != nil {
			apierror.Respond(ctx, http.StatusUnauthorized, errs.CodeAuthUnauthenticated, "unauthenticated", nil)
			ctx.Abort()
			return
		}
		ctx.Set(sessionContextKey, session)
		ctx.Next()
	}
}

func RequireUser(sessions *auth.Manager) gin.HandlerFunc {
	return requireRole(sessions, models.RoleUser)
}

func RequireAdmin(sessions *auth.Manager) gin.HandlerFunc {
	return requireRole(sessions, models.RoleAdmin)
}

func RequireSuperUser(sessions *auth.Manager) gin.HandlerFunc {
	return requireRole(sessions, models.RoleSuperUser)
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

func requireRole(sessions *auth.Manager, requiredRole string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		session, err := ReadSession(ctx, sessions)
		if err != nil {
			apierror.Respond(ctx, http.StatusUnauthorized, errs.CodeAuthUnauthenticated, "unauthenticated", nil)
			ctx.Abort()
			return
		}

		if !roleAllowed(session.Role, requiredRole) {
			apierror.Respond(ctx, http.StatusForbidden, errs.CodeAuthForbidden, "forbidden", nil)
			ctx.Abort()
			return
		}

		ctx.Set(sessionContextKey, session)
		ctx.Next()
	}
}

func roleAllowed(userRole, requiredRole string) bool {
	userRank, ok := roleRank[strings.ToLower(userRole)]
	if !ok {
		return false
	}
	requiredRank, ok := roleRank[strings.ToLower(requiredRole)]
	if !ok {
		return false
	}
	return userRank >= requiredRank
}
