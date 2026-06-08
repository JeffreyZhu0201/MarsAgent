package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/marsagent/gateway/internal/stream"
)

// echoRequest 是 POST /api/echo 的请求体。
type echoRequest struct {
	Msg string `json:"msg" binding:"required"`
}

// newEchoHandler 接收 deps 闭包，便于在 router 处注入 producer。
func newEchoHandler(p stream.TaskProducer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req echoRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		args, _ := json.Marshal(req)
		taskID := uuid.NewString()
		if err := p.Enqueue(c.Request.Context(), stream.TaskEnvelope{
			TaskID: taskID, Kind: "echo", Args: args,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"task_id": taskID})
	}
}