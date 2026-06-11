// Package api 暴露 gateway 的所有 HTTP/SSE 入口。
// Router 只组装路由，不持有业务依赖；通过 Deps 注入。
package api

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/marsagent/gateway/internal/grpcc"
	"github.com/marsagent/gateway/internal/oj"
	"github.com/marsagent/gateway/internal/sandbox"
	"github.com/marsagent/gateway/internal/store"
	"github.com/marsagent/gateway/internal/stream"
)

// Deps 聚合 handler 间共享的依赖。
type Deps struct {
	Producer    stream.TaskProducer
	Subscriber  stream.ProgressSubscriber // Task 4 后半再用到
	GRPC        *grpcc.WikiClient
	DB          *sql.DB
	CourseStore *store.CourseStore
	WikiStore   *store.WikiStore
	Sandbox     *sandbox.Scheduler
	OJStore     *oj.OJStore
	OJEngine    *oj.JudgeEngine
}

func NewRouter(d Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/healthz", healthHandler)

	api := r.Group("/api")
	if d.Producer != nil {
		api.POST("/echo", newEchoHandler(d.Producer))
		api.POST("/wiki/collect", wikiCollectHandler(d.Producer))
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
	if d.WikiStore != nil {
		api.GET("/wiki/drafts", listWikiDraftsHandler(d.WikiStore))
		api.POST("/wiki/drafts", createWikiDraftHandler(d.WikiStore))
		api.GET("/wiki/drafts/:id", getWikiDraftHandler(d.WikiStore))
		api.PUT("/wiki/drafts/:id", updateWikiDraftHandler(d.WikiStore))
		api.DELETE("/wiki/drafts/:id", deleteWikiDraftHandler(d.WikiStore))
		api.POST("/wiki/drafts/:id/reject", rejectWikiDraftHandler(d.WikiStore))
		api.POST("/wiki/drafts/:id/approve", approveWikiDraftHandler(d.WikiStore))
	}
	if d.CourseStore != nil {
		api.GET("/courses", listCoursesHandler(d.CourseStore))
		api.GET("/courses/:id", getCourseHandler(d.CourseStore))
		api.GET("/courses/:id/chapter/:ch_id", getCourseChapterHandler(d.CourseStore))
	}
	if d.CourseStore != nil && d.Producer != nil {
		api.POST("/courses", createCourseHandler(d.CourseStore, d.Producer))
	}
	api.POST("/sandbox/run", sandboxRunHandler())
	if d.OJStore != nil {
		od := &oj.Deps{Store: d.OJStore, Sandbox: d.Sandbox, Engine: d.OJEngine}
		api.GET("/oj/problems", oj.ListProblemsHandler(od))
		api.POST("/oj/problems", oj.CreateProblemHandler(od))
		api.GET("/oj/problems/:id", oj.GetProblemHandler(od))
		api.POST("/oj/problems/:id/test-cases", oj.AddTestCaseHandler(od))
		api.GET("/oj/submissions", oj.ListSubmissionsHandler(od))
		api.POST("/oj/submissions", oj.CreateSubmissionHandler(od))
		api.GET("/oj/submissions/:id", oj.GetSubmissionHandler(od))
	}
	return r
}
