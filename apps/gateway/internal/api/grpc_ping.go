package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/marsagent/gateway/internal/grpcc"
)

func newGRPCPingHandler(wc *grpcc.WikiClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		msg := c.DefaultQuery("msg", "hello")
		echo, ver, err := wc.Ping(c.Request.Context(), msg)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"echo": echo, "server_version": ver})
	}
}