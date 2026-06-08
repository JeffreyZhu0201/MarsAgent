package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// healthHandler 用于 liveness 探针。永远返回 200 + {"status":"ok"}。
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}