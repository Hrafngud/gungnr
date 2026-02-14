package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetSSEHeaders(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("X-Accel-Buffering", "no")
}

func SSEFlusher(ctx *gin.Context) (http.Flusher, bool) {
	flusher, ok := ctx.Writer.(http.Flusher)
	return flusher, ok
}

func SendSSEEvent(ctx *gin.Context, flusher http.Flusher, event string, payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return
	}
	fmt.Fprintf(ctx.Writer, "event: %s\n", event)
	fmt.Fprintf(ctx.Writer, "data: %s\n\n", encoded)
	flusher.Flush()
}
