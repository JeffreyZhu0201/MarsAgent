package store

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

var slugNonAlnum = regexp.MustCompile(`[^a-zA-Z0-9]`)

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
		          created_at::text, updated_at::text,
		          coalesce(published_at::text,''), coalesce(wiki_doc_id::text,'')`,
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
		       created_at::text, updated_at::text,
		       coalesce(published_at::text,''), coalesce(wiki_doc_id::text,'')
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
		       created_at::text, updated_at::text,
		       coalesce(published_at::text,''), coalesce(wiki_doc_id::text,'')
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
		&d.CreatedAt, &d.UpdatedAt, &d.PublishedAt, &d.WikiDocID)
	return &d, err
}

// fallbackString returns fallback when v is empty.
func fallbackString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// slugify replicates the Python _slugify logic:
// lowercase, replace non-alnum with '-', trim '-', truncate 80, md5 hash suffix 4 hex.
func slugify(title string) string {
	slug := strings.ToLower(title)
	slug = slugNonAlnum.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 80 {
		slug = slug[:80]
	}
	if slug != "" {
		h := md5.Sum([]byte(title))
		suffix := fmt.Sprintf("%x", h[:2]) // 4 hex chars from first 2 bytes
		return slug + "-" + suffix
	}
	h := sha256.Sum256([]byte(title))
	return fmt.Sprintf("%x", h[:6]) // 12 hex chars from first 6 bytes
}

// ApproveDraft reads a draft, inserts a wiki_doc row, updates the draft
// to published status, and returns the slug.
func (s *WikiStore) ApproveDraft(ctx context.Context, draftID string) (slug string, err error) {
	draft, err := s.GetDraft(ctx, draftID)
	if err != nil {
		return "", fmt.Errorf("get draft: %w", err)
	}
	if draft.Status == "published" {
		return "", fmt.Errorf("draft already published")
	}

	slug = slugify(draft.Title)
	urlHash := sha256.Sum256([]byte(draft.URL))
	contentHash := sha256.Sum256([]byte(draft.ContentMD))
	storagePath := fmt.Sprintf("wiki/%s/%s.md", draft.Category, slug)

	var docID string
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO wiki_docs
			(slug, category, title, url, url_hash, content_hash,
			 source, quality_score, language, storage_path, fetched_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6,
			 $7, $8, $9, $10, now(), now())
		ON CONFLICT (slug) DO UPDATE SET
			updated_at = EXCLUDED.updated_at,
			quality_score = EXCLUDED.quality_score
		RETURNING id`,
		slug, draft.Category, draft.Title, draft.URL,
		urlHash[:], contentHash[:8],
		draft.Source, draft.QualityScore, draft.Language, storagePath,
	).Scan(&docID)
	if err != nil {
		return "", fmt.Errorf("insert wiki_doc: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE drafts
		SET status = 'published', published_at = now(), wiki_doc_id = $2, updated_at = now()
		WHERE id = $1`,
		draftID, docID)
	if err != nil {
		return "", fmt.Errorf("update draft: %w", err)
	}

	return slug, nil
}
