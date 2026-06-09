package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/marsagent/gateway/internal/grpcc"
	"github.com/marsagent/gateway/internal/store"
	"github.com/marsagent/gateway/internal/stream"
)

// POST /api/wiki/collect — 触发信息收集 Agent。
func wikiCollectHandler(prod stream.TaskProducer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Topic        string   `json:"topic" binding:"required"`
			Sources      []string `json:"sources"`
			MaxPerSource int      `json:"max_per_source"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.MaxPerSource == 0 {
			req.MaxPerSource = 5
		}
		args, _ := json.Marshal(map[string]any{
			"topic":          req.Topic,
			"sources":        req.Sources,
			"max_per_source": req.MaxPerSource,
		})
		taskID := uuid.NewString()
		if err := prod.Enqueue(c.Request.Context(), stream.TaskEnvelope{
			TaskID: taskID,
			Kind:   "wiki.collect",
			Args:   args,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"task_id": taskID})
	}
}

// GET /api/wiki/tree
func wikiTreeHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.QueryContext(c.Request.Context(),
			`SELECT slug, title, category, source, updated_at FROM wiki_docs ORDER BY category, updated_at DESC`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		type Doc struct {
			Slug     string `json:"slug"`
			Title    string `json:"title"`
			Category string `json:"category"`
			Source   string `json:"source"`
			Updated  string `json:"updated_at"`
		}
		docs := make([]Doc, 0)
		for rows.Next() {
			var d Doc
			if err := rows.Scan(&d.Slug, &d.Title, &d.Category, &d.Source, &d.Updated); err == nil {
				docs = append(docs, d)
			}
		}
		c.JSON(http.StatusOK, gin.H{"docs": docs})
	}
}

// GET /api/wiki/doc/:slug
func wikiDocHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		var title, url, source, storagePath string
		err := db.QueryRowContext(c.Request.Context(),
			`SELECT title, url, source, storage_path FROM wiki_docs WHERE slug=$1`, slug,
		).Scan(&title, &url, &source, &storagePath)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// MinIO content loading comes in M3; return metadata for now
		c.JSON(http.StatusOK, gin.H{
			"slug": slug, "title": title, "url": url,
			"source": source, "storage_path": storagePath, "content": "",
		})
	}
}

// POST /api/wiki/search
func wikiSearchHandler(wc *grpcc.WikiClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Q string `json:"q" binding:"required"`
			K int    `json:"k"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.K == 0 {
			req.K = 10
		}
		resp, err := wc.HybridSearch(c.Request.Context(), req.Q, req.K, nil)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "search failed"})
			return
		}
		var hits any = resp.Hits
		if resp.Hits == nil {
			hits = []any{}
		}
		c.JSON(http.StatusOK, gin.H{"hits": hits})
	}
}

// GET /api/wiki/drafts
func listWikiDraftsHandler(ws *store.WikiStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, err := parseOptionalInt(c.Query("limit"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		drafts, err := ws.ListDrafts(c.Request.Context(), c.DefaultQuery("status", "draft"), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"drafts": drafts})
	}
}

// POST /api/wiki/drafts
func createWikiDraftHandler(ws *store.WikiStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			TaskID       string  `json:"task_id"`
			Title        string  `json:"title" binding:"required"`
			ContentMD    string  `json:"content_md"`
			URL          string  `json:"url"`
			Source       string  `json:"source"`
			Category     string  `json:"category"`
			Summary      string  `json:"summary"`
			QualityScore float64 `json:"quality_score"`
			Language     string  `json:"language"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		draft, err := ws.CreateDraft(c.Request.Context(), store.DraftInput{
			TaskID:       req.TaskID,
			Title:        req.Title,
			ContentMD:    req.ContentMD,
			URL:          req.URL,
			Source:       req.Source,
			Category:     req.Category,
			Summary:      req.Summary,
			QualityScore: req.QualityScore,
			Language:     req.Language,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, draft)
	}
}

// GET /api/wiki/drafts/:id
func getWikiDraftHandler(ws *store.WikiStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		draft, err := ws.GetDraft(c.Request.Context(), c.Param("id"))
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, draft)
	}
}

// PUT /api/wiki/drafts/:id
func updateWikiDraftHandler(ws *store.WikiStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Title     string `json:"title" binding:"required"`
			ContentMD string `json:"content_md"`
			Category  string `json:"category"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		id := c.Param("id")
		if err := ws.UpdateDraft(c.Request.Context(), id, store.DraftInput{
			Title:     req.Title,
			ContentMD: req.ContentMD,
			Category:  req.Category,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		draft, err := ws.GetDraft(c.Request.Context(), id)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, draft)
	}
}

// DELETE /api/wiki/drafts/:id
func deleteWikiDraftHandler(ws *store.WikiStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := ws.DeleteDraft(c.Request.Context(), c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// POST /api/wiki/drafts/:id/reject
func rejectWikiDraftHandler(ws *store.WikiStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := ws.MarkDraftRejected(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		draft, err := ws.GetDraft(c.Request.Context(), id)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, draft)
	}
}

// POST /api/wiki/drafts/:id/approve
func approveWikiDraftHandler(ws *store.WikiStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug, err := ws.ApproveDraft(c.Request.Context(), c.Param("id"))
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"slug": slug})
	}
}

func parseOptionalInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}
