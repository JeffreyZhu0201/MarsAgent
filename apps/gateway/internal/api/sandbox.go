package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/marsagent/gateway/internal/sandbox"
)

// POST /api/sandbox/run — 执行一次性代码容器
func sandboxRunHandler(sch *sandbox.Scheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req sandbox.RunRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
			return
		}
		result, err := sch.Run(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}