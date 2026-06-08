// Package api 暴露 gateway 的所有 HTTP/SSE 入口。
// Router 只组装路由，不持有业务依赖；通过 Deps 注入。
package api

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/marsagent/gateway/internal/grpcc"
	"github.com/marsagent/gateway/internal/stream"
	"github.com/marsagent/gateway/internal/store"
)

// Deps 聚合 handler 间共享的依赖。
type Deps struct {
	Producer   stream.TaskProducer
	Subscriber stream.ProgressSubscriber // Task 4 后半再用到
	GRPC       *grpcc.WikiClient
	DB         *sql.DB
	CourseStore *store.CourseStore
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
	if d.DB != nil {
		api.GET("/wiki/tree", wikiTreeHandler(d.DB))
		api.GET("/wiki/doc/:slug", wikiDocHandler(d.DB))
	}
	if d.GRPC != nil {
		api.POST("/wiki/search", wikiSearchHandler(d.GRPC))
	}
	if d.CourseStore != nil && d.Producer != nil {
		api.POST("/courses", createCourseHandler(d.CourseStore, d.Producer))
		api.GET("/courses/:id", getCourseHandler(d.CourseStore))
	}
	return r
}