package tests

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/marsagent/gateway/internal/api"
	"github.com/marsagent/gateway/internal/store"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", "postgres://mars:mars_dev_pw@localhost:5432/marsagent?sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	t.Cleanup(func() { db.Close() })
	return db
}

func TestDraftAPI(t *testing.T) {
	db := openTestDB(t)
	ws := store.NewWikiStore(db)
	r := api.NewRouter(api.Deps{WikiStore: ws})

	// Create a draft via POST /api/wiki/drafts
	createBody, _ := json.Marshal(map[string]any{
		"title":      "API Test Draft",
		"content_md": "# Hello\n\nWorld",
		"url":        "https://example.com/draft-api/" + uuid.NewString(),
		"source":     "test",
		"category":   "general",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/wiki/drafts", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var created store.Draft
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Equal(t, "draft", created.Status)
	require.Equal(t, "API Test Draft", created.Title)

	// List drafts via GET /api/wiki/drafts
	listReq := httptest.NewRequest(http.MethodGet, "/api/wiki/drafts", nil)
	listW := httptest.NewRecorder()
	r.ServeHTTP(listW, listReq)

	require.Equal(t, http.StatusOK, listW.Code)
	var listResp struct {
		Drafts []store.Draft `json:"drafts"`
	}
	require.NoError(t, json.Unmarshal(listW.Body.Bytes(), &listResp))
	require.NotEmpty(t, listResp.Drafts)
}
