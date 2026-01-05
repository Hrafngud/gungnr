package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

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
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load users"})
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req updateUserRoleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role != models.RoleAdmin && role != models.RoleUser {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "role must be admin or user"})
		return
	}

	user, err := c.service.UpdateRole(ctx.Request.Context(), userID, role)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		case errors.Is(err, service.ErrLastSuperUser):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "cannot demote last superuser"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update role"})
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	login := strings.TrimSpace(req.Login)
	if login == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "login is required"})
		return
	}

	user, err := c.service.AddAllowlistUser(ctx.Request.Context(), login)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAllowlistLoginRequired):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "login is required"})
		case errors.Is(err, service.ErrAllowlistUserNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "github user not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add user"})
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	if err := c.service.RemoveAllowlistUser(ctx.Request.Context(), userID); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		case errors.Is(err, service.ErrCannotRemoveSuperUser):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "cannot remove superuser"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove user"})
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
