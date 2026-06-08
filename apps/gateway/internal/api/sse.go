package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/marsagent/gateway/internal/stream"
)

// newSSEHandler 把 ProgressSubscriber 暴露为 Server-Sent Events。
// 协议：每个事件一行 `data: <json>\n\n`，event 类型暂不细分。
func newSSEHandler(sub stream.ProgressSubscriber) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("task_id")
		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "task_id required"})
			return
		}

		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError,
				gin.H{"error": "streaming unsupported"})
			return
		}

		ctx := c.Request.Context()
		ch, err := sub.Subscribe(ctx, taskID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				data, err := json.Marshal(ev)
				if err != nil {
					continue
				}
				fmt.Fprintf(c.Writer, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}