package api

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/marsagent/gateway/internal/grpcc"
)

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
		var docs []Doc
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
		c.JSON(http.StatusOK, gin.H{"hits": resp.Hits})
	}
}
