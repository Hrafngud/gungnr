package httpx

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func ParseBoolQuery(ctx *gin.Context, key string, fallback bool) bool {
	raw := strings.TrimSpace(ctx.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func ParseIntQuery(ctx *gin.Context, key string, fallback int) int {
	raw := strings.TrimSpace(ctx.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func ParsePositiveIntQuery(ctx *gin.Context, key string, fallback int) int {
	value := strings.TrimSpace(ctx.Query(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func ParseOffset(raw string) int {
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func ClampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
