package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
)

// WikiStore provides CRUD for knowledge review drafts.
type WikiStore struct {
	db *sql.DB
}

// NewWikiStore creates a WikiStore backed by the given database.
func NewWikiStore(db *sql.DB) *WikiStore { return &WikiStore{db: db} }

// Draft represents a row in the drafts table.
type Draft struct {
	ID           string  `json:"id"`
	TaskID       string  `json:"task_id"`
	Status       string  `json:"status"`
	Title        string  `json:"title"`
	ContentMD    string  `json:"content_md"`
	URL          string  `json:"url"`
	Source       string  `json:"source"`
	Category     string  `json:"category"`
	Revision     int     `json:"revision"`
	Summary      string  `json:"summary"`
	QualityScore float64 `json:"quality_score"`
	Language     string  `json:"language"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	PublishedAt  string  `json:"published_at"`
	WikiDocID    string  `json:"wiki_doc_id"`
}

// DraftInput holds the caller-provided fields when creating or updating a draft.
type DraftInput struct {
	TaskID       string
	Title        string
	ContentMD    string
	URL          string
	Source       string
	Category     string
	Summary      string
	QualityScore float64
	Language     string
}

// CreateDraft inserts a new draft. If a draft with the same url_hash already
// exists it upserts (updates title/content and bumps revision).
func (s *WikiStore) CreateDraft(ctx context.Context, in DraftInput) (*Draft, error) {
	urlHash := sha256.Sum256([]byte(in.URL))
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO drafts (task_id, title, content_md, url, url_hash, source, category, summary, quality_score, language)
		VALUES (nullif($1,'')::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (url_hash) DO UPDATE SET
			title      = EXCLUDED.title,
			content_md = EXCLUDED.content_md,
			updated_at = now(),
			revision   = drafts.revision + 1
		RETURNING id, status, title, content_md, url, source, category, revision,
		          coalesce(summary,''), coalesce(quality_score,0), coalesce(language,''),
		          created_at::text, updated_at::text`,
		in.TaskID, in.Title, in.ContentMD, in.URL, urlHash[:],
		in.Source, fallbackString(in.Category, "general"),
		in.Summary, in.QualityScore, fallbackString(in.Language, "en"))
	return scanDraft(row)
}

// GetDraft reads a single draft by id.
func (s *WikiStore) GetDraft(ctx context.Context, id string) (*Draft, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, status, title, content_md, url, source, category, revision,
		       coalesce(summary,''), coalesce(quality_score,0), coalesce(language,''),
		       created_at::text, updated_at::text
		FROM drafts WHERE id = $1`, id)
	return scanDraft(row)
}

// ListDrafts returns drafts matching the given status. An empty status matches all.
func (s *WikiStore) ListDrafts(ctx context.Context, status string, limit int) ([]Draft, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, status, title, content_md, url, source, category, revision,
		       coalesce(summary,''), coalesce(quality_score,0), coalesce(language,''),
		       created_at::text, updated_at::text
		FROM drafts WHERE ($1 = '' OR status = $1)
		ORDER BY updated_at DESC LIMIT $2`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Draft
	for rows.Next() {
		d, err := scanDraft(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

// UpdateDraft modifies title, content, and category, bumping revision.
func (s *WikiStore) UpdateDraft(ctx context.Context, id string, in DraftInput) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE drafts SET title = $2, content_md = $3, category = $4,
		                  revision = revision + 1, updated_at = now()
		WHERE id = $1`,
		id, in.Title, in.ContentMD, fallbackString(in.Category, "general"))
	return err
}

// MarkDraftRejected sets the draft status to "rejected".
func (s *WikiStore) MarkDraftRejected(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE drafts SET status = 'rejected', updated_at = now() WHERE id = $1`, id)
	return err
}

// DeleteDraft removes a draft by id.
func (s *WikiStore) DeleteDraft(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM drafts WHERE id = $1`, id)
	return err
}

// scanDraft scans a single draft row from any sql.Scanner (Row or Rows).
func scanDraft(sc interface{ Scan(...any) error }) (*Draft, error) {
	var d Draft
	err := sc.Scan(&d.ID, &d.Status, &d.Title, &d.ContentMD, &d.URL, &d.Source,
		&d.Category, &d.Revision, &d.Summary, &d.QualityScore, &d.Language,
		&d.CreatedAt, &d.UpdatedAt)
	return &d, err
}

// fallbackString returns fallback when v is empty.
func fallbackString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
