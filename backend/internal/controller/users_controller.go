package controller

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/errs"
	"go-notes/internal/models"
	"go-notes/internal/respond"
	"go-notes/internal/service"
	"go-notes/internal/utils/httpx"
)

type UsersController struct {
	service *service.UserService
}

func NewUsersController(service *service.UserService) *UsersController {
	return &UsersController{service: service}
}

func (c *UsersController) List(ctx *gin.Context) {
	users, err := c.service.List(ctx.Request.Context())
	if err != nil {
		respond.Err(ctx, err, errs.CodeUserListFailed, "failed to load users")
		return
	}
	respond.OK(ctx, gin.H{"users": models.NewUserResponses(users)})
}

func (c *UsersController) UpdateRole(ctx *gin.Context) {
	userID, err := httpx.ParseUintParam(ctx.Param("id"))
	if err != nil {
		respond.Err(ctx, errs.New(errs.CodeUserInvalidID, "invalid user id"), errs.CodeUserInvalidID, "invalid user id")
		return
	}

	var req models.UpdateUserRoleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respond.Err(ctx, errs.New(errs.CodeUserInvalidPayload, "invalid payload"), errs.CodeUserInvalidPayload, "invalid payload")
		return
	}

	user, err := c.service.UpdateRole(ctx.Request.Context(), userID, req.Role)
	if err != nil {
		respond.Err(ctx, err, errs.CodeUserUpdateFailed, "failed to update role")
		return
	}
	respond.OK(ctx, models.NewUserResponse(user))
}

func (c *UsersController) Create(ctx *gin.Context) {
	var req models.CreateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respond.Err(ctx, errs.New(errs.CodeUserInvalidPayload, "invalid payload"), errs.CodeUserInvalidPayload, "invalid payload")
		return
	}

	user, err := c.service.AddAllowlistUser(ctx.Request.Context(), req.Login)
	if err != nil {
		respond.Err(ctx, err, errs.CodeUserCreateFailed, "failed to add user")
		return
	}
	respond.OK(ctx, models.NewUserResponse(user))
}

func (c *UsersController) Delete(ctx *gin.Context) {
	userID, err := httpx.ParseUintParam(ctx.Param("id"))
	if err != nil {
		respond.Err(ctx, errs.New(errs.CodeUserInvalidID, "invalid user id"), errs.CodeUserInvalidID, "invalid user id")
		return
	}

	if err := c.service.RemoveAllowlistUser(ctx.Request.Context(), userID); err != nil {
		respond.Err(ctx, err, errs.CodeUserDeleteFailed, "failed to remove user")
		return
	}
	respond.NoContent(ctx)
}
