// Package oj 的 HTTP 处理器：题目 CRUD、提交代码、查询判题结果。
package oj

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/marsagent/gateway/internal/sandbox"
)

// Deps 聚合 OJ 路由所需的依赖。
type Deps struct {
	Store   *OJStore
	Sandbox *sandbox.Scheduler
	Engine  *JudgeEngine
}

// applyProblemDefaults 填充题目的默认难度与资源限制。
func applyProblemDefaults(difficulty string, timeLimitMs, memoryLimitMb int) (string, int, int) {
	if difficulty == "" {
		difficulty = "medium"
	}
	if timeLimitMs == 0 {
		timeLimitMs = 2000
	}
	if memoryLimitMb == 0 {
		memoryLimitMb = 256
	}
	return difficulty, timeLimitMs, memoryLimitMb
}

// ListProblemsHandler GET /api/oj/problems — 分页列出题目。
func ListProblemsHandler(d *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		problems, total, err := d.Store.ListProblems(c.Request.Context(), limit, offset, c.Query("difficulty"), c.Query("tag"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"problems": problems, "total": total})
	}
}

// GetProblemHandler GET /api/oj/problems/:id — 获取题目详情及样例测试点。
func GetProblemHandler(d *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		problem, err := d.Store.GetProblem(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
			return
		}
		samples, _ := d.Store.ListTestCases(c.Request.Context(), id, false)
		c.JSON(http.StatusOK, gin.H{"problem": problem, "sample_test_cases": samples})
	}
}

// CreateProblemHandler POST /api/oj/problems — 创建新题目。
func CreateProblemHandler(d *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Title         string   `json:"title" binding:"required"`
			DescriptionMD string   `json:"description_md"`
			Tags          []string `json:"tags"`
			Difficulty    string   `json:"difficulty"`
			TimeLimitMs   int      `json:"time_limit_ms"`
			MemoryLimitMb int      `json:"memory_limit_mb"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		req.Difficulty, req.TimeLimitMs, req.MemoryLimitMb = applyProblemDefaults(req.Difficulty, req.TimeLimitMs, req.MemoryLimitMb)
		p := &Problem{
			Title:         req.Title,
			DescriptionMD: req.DescriptionMD,
			Tags:          req.Tags,
			Difficulty:    req.Difficulty,
			TimeLimitMs:   req.TimeLimitMs,
			MemoryLimitMb: req.MemoryLimitMb,
		}
		if err := d.Store.CreateProblem(c.Request.Context(), p); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, p)
	}
}

// AddTestCaseHandler POST /api/oj/problems/:id/test-cases — 为题目添加测试点。
func AddTestCaseHandler(d *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		problemID := c.Param("id")
		var req struct {
			Input          string `json:"input"`
			ExpectedOutput string `json:"expected_output" binding:"required"`
			IsSample       bool   `json:"is_sample"`
			Score          int    `json:"score"`
			Ordering       int    `json:"ordering"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Score == 0 {
			req.Score = 100
		}
		tc := &TestCase{
			ProblemID:      problemID,
			Input:          req.Input,
			ExpectedOutput: req.ExpectedOutput,
			IsSample:       req.IsSample,
			IsHidden:       !req.IsSample,
			Score:          req.Score,
			Ordering:       req.Ordering,
		}
		if err := d.Store.AddTestCase(c.Request.Context(), tc); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, tc)
	}
}

// CreateSubmissionHandler POST /api/oj/submissions — 提交代码并触发异步判题。
func CreateSubmissionHandler(d *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ProblemID string `json:"problem_id" binding:"required"`
			Code      string `json:"code" binding:"required"`
			Lang      string `json:"lang" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(req.Code) > sandbox.MaxCodeBytes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "code exceeds maximum size"})
			return
		}
		if req.Lang != "python" && req.Lang != "node" && req.Lang != "go" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported language"})
			return
		}
		sub := &Submission{ProblemID: req.ProblemID, Code: req.Code, Lang: req.Lang}
		if err := d.Store.CreateSubmission(c.Request.Context(), sub); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		d.Engine.JudgeSubmissionAsync(c.Request.Context(), sub.ID)
		c.JSON(http.StatusAccepted, gin.H{"submission_id": sub.ID})
	}
}

// GetSubmissionHandler GET /api/oj/submissions/:id — 获取提交详情与各测试点结果。
func GetSubmissionHandler(d *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		sub, err := d.Store.GetSubmission(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "submission not found"})
			return
		}
		results, _ := d.Store.GetSubmissionResults(c.Request.Context(), id)
		c.JSON(http.StatusOK, gin.H{"submission": sub, "results": results})
	}
}

// ListSubmissionsHandler GET /api/oj/submissions — 分页列出提交记录。
func ListSubmissionsHandler(d *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		subs, total, err := d.Store.ListSubmissions(c.Request.Context(), c.Query("problem_id"), c.Query("user_id"), limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"submissions": subs, "total": total})
	}
}
