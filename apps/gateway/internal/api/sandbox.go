package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const agentsSandboxURL = "http://127.0.0.1:8001/sandbox/run"

// POST /api/sandbox/run — 代理到 agents Python service（warm container pool）
func sandboxRunHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		proxyReq, err := http.NewRequest(http.MethodPost, agentsSandboxURL, bytes.NewReader(body))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		proxyReq.Header.Set("Content-Type", "application/json")

		// Use a fresh 60s timeout context — the incoming request's context may be very short
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		client := &http.Client{Timeout: 65 * time.Second}
		resp, err := client.Do(proxyReq.WithContext(ctx))
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "sandbox service unavailable: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		body2, _ := io.ReadAll(resp.Body)
		c.Header("Content-Type", resp.Header.Get("Content-Type"))
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body2)
	}
}
