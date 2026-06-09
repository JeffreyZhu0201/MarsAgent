package tests

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/marsagent/gateway/internal/store"
	"github.com/stretchr/testify/require"
)

func TestDraftLifecycleStore(t *testing.T) {
	db, err := sql.Open("postgres", "postgres://mars:mars_dev_pw@localhost:5432/marsagent?sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	defer db.Close()

	s := store.NewWikiStore(db)
	ctx := context.Background()

	draft, err := s.CreateDraft(ctx, store.DraftInput{
		Title:     "Draft Title",
		ContentMD: "# Draft\n\nBody",
		URL:       "https://example.com/draft-lifecycle/" + uuid.NewString(),
		Source:    "test",
		Category:  "general",
	})
	require.NoError(t, err)
	require.Equal(t, "draft", draft.Status)

	require.NoError(t, s.UpdateDraft(ctx, draft.ID, store.DraftInput{
		Title:     "Updated Draft",
		ContentMD: "# Updated",
		URL:       draft.URL,
		Source:    "test",
		Category:  "web",
	}))

	got, err := s.GetDraft(ctx, draft.ID)
	require.NoError(t, err)
	require.Equal(t, "Updated Draft", got.Title)
	require.Equal(t, "web", got.Category)

	list, err := s.ListDrafts(ctx, "draft", 20)
	require.NoError(t, err)
	require.NotEmpty(t, list)

	require.NoError(t, s.MarkDraftRejected(ctx, draft.ID))
	got, err = s.GetDraft(ctx, draft.ID)
	require.NoError(t, err)
	require.Equal(t, "rejected", got.Status)
}
