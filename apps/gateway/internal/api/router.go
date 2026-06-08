// Package api 暴露 gateway 的所有 HTTP/SSE 入口。
// Router 只组装路由，不持有业务依赖；通过 Deps 注入。
package api

import "github.com/gin-gonic/gin"

// Deps 聚合 handler 间共享的依赖（M1 仅占位，后续补 stream/grpc client）。
type Deps struct{}

// NewRouter 返回一个完全配置好的 gin.Engine。
// 调用方负责 Run() 或挂到 http.Server。
func NewRouter(d Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/healthz", healthHandler)
	return r
}