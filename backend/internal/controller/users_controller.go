package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-notes/internal/apierror"
	"go-notes/internal/errs"
	"go-notes/internal/models"
	"go-notes/internal/repository"
	"go-notes/internal/service"
)

type UsersController struct {
	service *service.UserService
}

type userResponse struct {
	ID          uint      `json:"id"`
	Login       string    `json:"login"`
	Role        string    `json:"role"`
	LastLoginAt time.Time `json:"lastLoginAt"`
}

type updateUserRoleRequest struct {
	Role string `json:"role"`
}

type createUserRequest struct {
	Login string `json:"login"`
}

func NewUsersController(service *service.UserService) *UsersController {
	return &UsersController{service: service}
}

func (c *UsersController) Register(r gin.IRoutes) {
	r.GET("/users", c.List)
}

func (c *UsersController) RegisterAdmin(r gin.IRoutes) {
	r.POST("/users", c.Create)
	r.PATCH("/users/:id/role", c.UpdateRole)
	r.DELETE("/users/:id", c.Delete)
}

func (c *UsersController) List(ctx *gin.Context) {
	users, err := c.service.List(ctx.Request.Context())
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeUserListFailed, "failed to load users")
		return
	}

	response := make([]userResponse, 0, len(users))
	for _, user := range users {
		response = append(response, userResponse{
			ID:          user.ID,
			Login:       user.Login,
			Role:        user.Role,
			LastLoginAt: user.LastLoginAt,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{"users": response})
}

func (c *UsersController) UpdateRole(ctx *gin.Context) {
	userID, err := parseUserID(ctx.Param("id"))
	if err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeUserInvalidID, "invalid user id", nil)
		return
	}

	var req updateUserRoleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeUserInvalidPayload, "invalid payload", nil)
		return
	}

	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role != models.RoleAdmin && role != models.RoleUser {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeUserInvalidRole, "role must be admin or user", nil)
		return
	}

	user, err := c.service.UpdateRole(ctx.Request.Context(), userID, role)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			apierror.RespondWithError(ctx, http.StatusNotFound, err, errs.CodeUserNotFound, "user not found")
		case errors.Is(err, service.ErrLastSuperUser):
			apierror.RespondWithError(ctx, http.StatusBadRequest, err, errs.CodeUserLastSuperUser, "cannot demote last superuser")
		default:
			apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeUserUpdateFailed, "failed to update role")
		}
		return
	}

	ctx.JSON(http.StatusOK, userResponse{
		ID:          user.ID,
		Login:       user.Login,
		Role:        user.Role,
		LastLoginAt: user.LastLoginAt,
	})
}

func (c *UsersController) Create(ctx *gin.Context) {
	var req createUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeUserInvalidPayload, "invalid payload", nil)
		return
	}

	login := strings.TrimSpace(req.Login)
	if login == "" {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeUserLoginRequired, "login is required", nil)
		return
	}

	user, err := c.service.AddAllowlistUser(ctx.Request.Context(), login)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAllowlistLoginRequired):
			apierror.RespondWithError(ctx, http.StatusBadRequest, err, errs.CodeUserLoginRequired, "login is required")
		case errors.Is(err, service.ErrAllowlistUserNotFound):
			apierror.RespondWithError(ctx, http.StatusNotFound, err, errs.CodeUserGitHubNotFound, "github user not found")
		default:
			apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeUserCreateFailed, "failed to add user")
		}
		return
	}

	ctx.JSON(http.StatusOK, userResponse{
		ID:          user.ID,
		Login:       user.Login,
		Role:        user.Role,
		LastLoginAt: user.LastLoginAt,
	})
}

func (c *UsersController) Delete(ctx *gin.Context) {
	userID, err := parseUserID(ctx.Param("id"))
	if err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeUserInvalidID, "invalid user id", nil)
		return
	}

	if err := c.service.RemoveAllowlistUser(ctx.Request.Context(), userID); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			apierror.RespondWithError(ctx, http.StatusNotFound, err, errs.CodeUserNotFound, "user not found")
		case errors.Is(err, service.ErrCannotRemoveSuperUser):
			apierror.RespondWithError(ctx, http.StatusBadRequest, err, errs.CodeUserRemoveSuperUser, "cannot remove superuser")
		default:
			apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeUserDeleteFailed, "failed to remove user")
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

func parseUserID(raw string) (uint, error) {
	value, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || value == 0 {
		if err == nil {
			return 0, errors.New("invalid id")
		}
		return 0, err
	}
	return uint(value), nil
}
