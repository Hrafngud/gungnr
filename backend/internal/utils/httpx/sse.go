package httpx

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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

// Keep the HTTP writer in one goroutine; collection can block without preventing
// heartbeats or disconnect handling. Closing the pipe releases the reader.
func StreamLogLines(ctx *gin.Context, flusher http.Flusher, reader io.ReadCloser, wait func() error, code string) {
	type event struct {
		line string
		err  error
		done bool
	}
	events := make(chan event)
	done := make(chan struct{})
	defer close(done)
	defer reader.Close()
	go func() {
		emit := func(e event) bool {
			select {
			case events <- e:
				return true
			case <-done:
				return false
			}
		}
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			if line := strings.TrimRight(scanner.Text(), "\r"); line != "" {
				if !emit(event{line: line}) {
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			emit(event{err: err})
			return
		}
		emit(event{err: wait(), done: true})
	}()
	SendSSEEvent(ctx, flusher, "meta", gin.H{"status": "connecting"})
	heartbeat := time.NewTicker(10 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			return
		case <-heartbeat.C:
			SendSSEEvent(ctx, flusher, "heartbeat", gin.H{"time": time.Now().UTC()})
		case e := <-events:
			if e.err != nil {
				SendSSEEvent(ctx, flusher, "error", gin.H{"code": code, "message": e.err.Error()})
				return
			}
			if e.done {
				SendSSEEvent(ctx, flusher, "done", gin.H{"status": "closed"})
				return
			}
			SendSSEEvent(ctx, flusher, "log", gin.H{"line": e.line})
		}
	}
}
