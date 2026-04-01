package respond

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/errs"
)

// Response is the JSON envelope for error responses.
type Response struct {
	Code    errs.Code         `json:"code"`
	Message string            `json:"message"`
	Error   string            `json:"error"`
	Fields  map[string]string `json:"fields,omitempty"`
	Details any               `json:"details,omitempty"`
	DocsURL string            `json:"docsUrl,omitempty"`
}

// OK sends a 200 JSON response.
func OK(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusOK, data)
}

// Created sends a 201 JSON response.
func Created(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusCreated, data)
}

// Accepted sends a 202 JSON response.
func Accepted(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusAccepted, data)
}

// NoContent sends a 204 with no body.
func NoContent(ctx *gin.Context) {
	ctx.Status(http.StatusNoContent)
}

// Err inspects the error: if it carries an errs.Error, it extracts
// code, message, fields, details and derives HTTP status from the code.
// Otherwise it uses the supplied fallback code and message.
func Err(ctx *gin.Context, err error, fallbackCode errs.Code, fallbackMessage string) {
	code, message, fields, details := classify(err, fallbackCode, fallbackMessage)
	status := code.HTTPStatus()
	ctx.JSON(status, Response{
		Code:    code,
		Message: message,
		Error:   message,
		Fields:  fields,
		Details: details,
		DocsURL: docsURL(code),
	})
}

// ErrStatus sends an error response with an explicit HTTP status override while
// still extracting typed code, message, fields, and details from the error.
func ErrStatus(ctx *gin.Context, status int, err error, fallbackCode errs.Code, fallbackMessage string) {
	code, message, fields, details := classify(err, fallbackCode, fallbackMessage)
	ctx.JSON(status, Response{
		Code:    code,
		Message: message,
		Error:   message,
		Fields:  fields,
		Details: details,
		DocsURL: docsURL(code),
	})
}

// ErrManual sends an error response with an explicit HTTP status override.
func ErrManual(ctx *gin.Context, status int, code errs.Code, message string) {
	ctx.JSON(status, Response{
		Code:    code,
		Message: message,
		Error:   message,
		DocsURL: docsURL(code),
	})
}

func classify(err error, fallbackCode errs.Code, fallbackMessage string) (errs.Code, string, map[string]string, any) {
	if err == nil {
		return fallbackCode, fallbackMessage, nil, nil
	}
	if typed, ok := errs.From(err); ok {
		msg := strings.TrimSpace(typed.Message)
		if msg == "" {
			msg = fallbackMessage
		}
		return typed.Code, msg, typed.Fields, typed.Details
	}
	return fallbackCode, fallbackMessage, nil, nil
}

func docsURL(code errs.Code) string {
	trimmed := strings.TrimSpace(string(code))
	if trimmed == "" {
		return ""
	}
	return "/errors.html#" + trimmed
}
