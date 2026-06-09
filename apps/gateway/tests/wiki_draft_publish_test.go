package tests

import (
	"context"
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

func TestApproveDraft(t *testing.T) {
	db, err := sql.Open("postgres", "postgres://mars:mars_dev_pw@localhost:5432/marsagent?sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	defer db.Close()

	ws := store.NewWikiStore(db)
	ctx := context.Background()

	// Create a draft
	uniqueURL := "https://example.com/approve-draft/" + uuid.NewString()
	draft, err := ws.CreateDraft(ctx, store.DraftInput{
		Title:     "Approve Me",
		ContentMD: "# Approve\n\nBody",
		URL:       uniqueURL,
		Source:    "test",
		Category:  "general",
	})
	require.NoError(t, err)
	require.Equal(t, "draft", draft.Status)

	// Approve via API
	r := api.NewRouter(api.Deps{DB: db, WikiStore: ws})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/wiki/drafts/"+draft.ID+"/approve", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp struct {
		Slug string `json:"slug"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Slug)

	// Verify the slug is in the wiki tree
	treeW := httptest.NewRecorder()
	treeReq := httptest.NewRequest(http.MethodGet, "/api/wiki/tree", nil)
	r.ServeHTTP(treeW, treeReq)
	require.Equal(t, http.StatusOK, treeW.Code)
	require.Contains(t, treeW.Body.String(), resp.Slug)

	// Verify the draft is now published
	published, err := ws.GetDraft(ctx, draft.ID)
	require.NoError(t, err)
	require.Equal(t, "published", published.Status)
	require.NotEmpty(t, published.PublishedAt)
	require.NotEmpty(t, published.WikiDocID)
}
