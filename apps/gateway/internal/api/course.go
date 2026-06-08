package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/marsagent/gateway/internal/stream"
	"github.com/marsagent/gateway/internal/store"
)

// POST /api/courses — 创建课程并触发建课任务。
func createCourseHandler(cs *store.CourseStore, prod stream.TaskProducer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Topic    string `json:"topic" binding:"required"`
			Audience string `json:"audience"`
			Depth    string `json:"depth"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		id, err := cs.CreateCourse(c.Request.Context(), req.Topic)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		taskID := uuid.NewString()
		args, _ := json.Marshal(map[string]any{
			"course_id": id,
			"topic":     req.Topic,
			"audience":  req.Audience,
			"depth":     req.Depth,
		})
		if err := prod.Enqueue(c.Request.Context(), stream.TaskEnvelope{
			TaskID: taskID,
			Kind:   "course.build",
			Args:   args,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"id": id, "task_id": taskID})
	}
}

// GET /api/courses/:id — 获取课程信息。
func getCourseHandler(cs *store.CourseStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		course, err := cs.GetCourse(c.Request.Context(), id)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, course)
	}
}
