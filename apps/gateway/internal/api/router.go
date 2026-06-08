// Package api 暴露 gateway 的所有 HTTP/SSE 入口。
// Router 只组装路由，不持有业务依赖；通过 Deps 注入。
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/marsagent/gateway/internal/grpcc"
	"github.com/marsagent/gateway/internal/stream"
)

// Deps 聚合 handler 间共享的依赖。
type Deps struct {
	Producer   stream.TaskProducer
	Subscriber stream.ProgressSubscriber // Task 4 后半再用到
	GRPC       *grpcc.WikiClient
}

func NewRouter(d Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/healthz", healthHandler)

	api := r.Group("/api")
	if d.Producer != nil {
		api.POST("/echo", newEchoHandler(d.Producer))
	}
	if d.Subscriber != nil {
		api.GET("/stream/:task_id", newSSEHandler(d.Subscriber))
	}
	if d.GRPC != nil {
		api.GET("/grpc-ping", newGRPCPingHandler(d.GRPC))
	}
	return r
}