// Package oj 实现在线判题（OJ）引擎：题目管理、代码提交、沙箱判题。
package oj

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

// Problem 对应 problems 表的一行。
type Problem struct {
	ID            string   `json:"id"`
	UserID        *string  `json:"user_id,omitempty"`
	Title         string   `json:"title"`
	DescriptionMD string   `json:"description_md"`
	Tags          []string `json:"tags"`
	Difficulty    string   `json:"difficulty"`
	TimeLimitMs   int      `json:"time_limit_ms"`
	MemoryLimitMb int      `json:"memory_limit_mb"`
	Visible       bool     `json:"visible"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

// TestCase 是题目的单个测试点。
type TestCase struct {
	ID             string `json:"id"`
	ProblemID      string `json:"problem_id"`
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	IsSample       bool   `json:"is_sample"`
	IsHidden       bool   `json:"is_hidden"`
	Score          int    `json:"score"`
	Ordering       int    `json:"ordering"`
}

// Submission 是一次代码提交。
type Submission struct {
	ID         string  `json:"id"`
	UserID     *string `json:"user_id,omitempty"`
	ProblemID  string  `json:"problem_id"`
	Code       string  `json:"code"`
	Lang       string  `json:"lang"`
	Status     string  `json:"status"`
	Score      int     `json:"score"`
	DurationMs int     `json:"duration_ms"`
	MemoryKb   int     `json:"memory_kb"`
	ErrorMsg   string  `json:"error_msg,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

// SubmissionResult 是单个测试点的运行结果。
type SubmissionResult struct {
	ID           string `json:"id"`
	SubmissionID string `json:"submission_id"`
	TestCaseID   string `json:"test_case_id"`
	Status       string `json:"status"`
	ActualOutput string `json:"actual_output,omitempty"`
	DurationMs   int    `json:"duration_ms"`
	MemoryKb     int    `json:"memory_kb"`
	Score       int    `json:"score"`
}

// OJStore 提供题目、测试点、提交及结果的 CRUD。
type OJStore struct {
	db *sql.DB
}

// NewOJStore 创建 OJ 数据访问层。
func NewOJStore(db *sql.DB) *OJStore { return &OJStore{db: db} }

// tagsFromScan 将 PostgreSQL text[] 扫描结果转为 []string。
func tagsFromScan(arr pq.StringArray) []string {
	if arr == nil {
		return []string{}
	}
	return []string(arr)
}

// --- 题目 CRUD ---

// ListProblems 分页返回可见题目，可按难度或标签过滤。
func (s *OJStore) ListProblems(ctx context.Context, limit, offset int, difficulty, tag string) ([]Problem, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	baseQuery := `FROM problems WHERE visible = true`
	var args []any
	argIdx := 1
	if difficulty != "" {
		baseQuery += fmt.Sprintf(" AND difficulty = $%d", argIdx)
		args = append(args, difficulty)
		argIdx++
	}
	if tag != "" {
		baseQuery += fmt.Sprintf(" AND $%d = ANY(tags)", argIdx)
		args = append(args, tag)
		argIdx++
	}

	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) "+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, coalesce(user_id::text,''), title, description_md, tags, difficulty,
		       time_limit_ms, memory_limit_mb, visible, created_at::text, updated_at::text
		`+baseQuery+fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []Problem
	for rows.Next() {
		var p Problem
		var tags pq.StringArray
		if err := rows.Scan(&p.ID, &p.UserID, &p.Title, &p.DescriptionMD, &tags,
			&p.Difficulty, &p.TimeLimitMs, &p.MemoryLimitMb, &p.Visible,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		p.Tags = tagsFromScan(tags)
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// GetProblem 按 ID 读取单个题目。
func (s *OJStore) GetProblem(ctx context.Context, id string) (*Problem, error) {
	var p Problem
	var tags pq.StringArray
	var userID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, coalesce(user_id::text,''), title, description_md, tags, difficulty,
		       time_limit_ms, memory_limit_mb, visible, created_at::text, updated_at::text
		FROM problems WHERE id = $1`, id).Scan(
		&p.ID, &userID, &p.Title, &p.DescriptionMD, &tags, &p.Difficulty,
		&p.TimeLimitMs, &p.MemoryLimitMb, &p.Visible, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		p.UserID = &userID.String
	}
	p.Tags = tagsFromScan(tags)
	return &p, nil
}

// CreateProblem 插入新题目。
func (s *OJStore) CreateProblem(ctx context.Context, p *Problem) error {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	var uid any
	if p.UserID != nil && *p.UserID != "" {
		uid = *p.UserID
	}
	return s.db.QueryRowContext(ctx, `
		INSERT INTO problems (title, description_md, tags, difficulty, time_limit_ms, memory_limit_mb, user_id)
		VALUES ($1, $2, $3, $4, $5, $6, nullif($7::text,'')::uuid)
		RETURNING id, created_at::text, updated_at::text`,
		p.Title, p.DescriptionMD, pq.Array(tags), p.Difficulty, p.TimeLimitMs, p.MemoryLimitMb, uid,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

// UpdateProblem 更新已有题目。
func (s *OJStore) UpdateProblem(ctx context.Context, id string, p *Problem) error {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE problems SET title=$2, description_md=$3, tags=$4, difficulty=$5,
			time_limit_ms=$6, memory_limit_mb=$7, updated_at=now()
		WHERE id=$1`,
		id, p.Title, p.DescriptionMD, pq.Array(tags), p.Difficulty, p.TimeLimitMs, p.MemoryLimitMb)
	return err
}

// --- 测试点 CRUD ---

// ListTestCases 返回题目的测试点；includeHidden=false 时仅返回样例点。
func (s *OJStore) ListTestCases(ctx context.Context, problemID string, includeHidden bool) ([]TestCase, error) {
	query := `SELECT id, problem_id, input, expected_output, is_sample, is_hidden, score, ordering
		FROM test_cases WHERE problem_id = $1`
	if !includeHidden {
		query += ` AND is_hidden = false`
	}
	query += ` ORDER BY ordering`
	rows, err := s.db.QueryContext(ctx, query, problemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TestCase
	for rows.Next() {
		var tc TestCase
		if err := rows.Scan(&tc.ID, &tc.ProblemID, &tc.Input, &tc.ExpectedOutput,
			&tc.IsSample, &tc.IsHidden, &tc.Score, &tc.Ordering); err != nil {
			return nil, err
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}

// AddTestCase 为题目插入测试点。
func (s *OJStore) AddTestCase(ctx context.Context, tc *TestCase) error {
	return s.db.QueryRowContext(ctx, `
		INSERT INTO test_cases (problem_id, input, expected_output, is_sample, is_hidden, score, ordering)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		tc.ProblemID, tc.Input, tc.ExpectedOutput, tc.IsSample, tc.IsHidden, tc.Score, tc.Ordering,
	).Scan(&tc.ID)
}

// --- 提交 CRUD ---

// CreateSubmission 插入新提交并返回 ID。
func (s *OJStore) CreateSubmission(ctx context.Context, sub *Submission) error {
	var uid any
	if sub.UserID != nil && *sub.UserID != "" {
		uid = *sub.UserID
	}
	return s.db.QueryRowContext(ctx, `
		INSERT INTO submissions (problem_id, code, lang, user_id)
		VALUES ($1, $2, $3, nullif($4::text,'')::uuid)
		RETURNING id, status, created_at::text`,
		sub.ProblemID, sub.Code, sub.Lang, uid,
	).Scan(&sub.ID, &sub.Status, &sub.CreatedAt)
}

// GetSubmission 按 ID 读取提交。
func (s *OJStore) GetSubmission(ctx context.Context, id string) (*Submission, error) {
	var sub Submission
	var userID, errMsg sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, coalesce(user_id::text,''), problem_id, code, lang, status, score,
		       duration_ms, memory_kb, coalesce(error_msg,''), created_at::text
		FROM submissions WHERE id = $1`, id).Scan(
		&sub.ID, &userID, &sub.ProblemID, &sub.Code, &sub.Lang, &sub.Status,
		&sub.Score, &sub.DurationMs, &sub.MemoryKb, &errMsg, &sub.CreatedAt)
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		sub.UserID = &userID.String
	}
	if errMsg.Valid {
		sub.ErrorMsg = errMsg.String
	}
	return &sub, nil
}

// ListSubmissions 分页返回提交，可按题目或用户过滤。
func (s *OJStore) ListSubmissions(ctx context.Context, problemID, userID string, limit, offset int) ([]Submission, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	baseQuery := `FROM submissions WHERE ($1 = '' OR problem_id = $1::uuid) AND ($2 = '' OR user_id = $2::uuid)`
	args := []any{problemID, userID}

	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) "+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, coalesce(user_id::text,''), problem_id, lang, status, score,
		       duration_ms, memory_kb, coalesce(error_msg,''), created_at::text
		`+baseQuery+` ORDER BY created_at DESC LIMIT $3 OFFSET $4`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []Submission
	for rows.Next() {
		var sub Submission
		var uid, errMsg sql.NullString
		if err := rows.Scan(&sub.ID, &uid, &sub.ProblemID, &sub.Lang, &sub.Status,
			&sub.Score, &sub.DurationMs, &sub.MemoryKb, &errMsg, &sub.CreatedAt); err != nil {
			return nil, 0, err
		}
		if uid.Valid {
			sub.UserID = &uid.String
		}
		if errMsg.Valid {
			sub.ErrorMsg = errMsg.String
		}
		out = append(out, sub)
	}
	return out, total, rows.Err()
}

// UpdateSubmission 更新提交的判题结果。
func (s *OJStore) UpdateSubmission(ctx context.Context, id, status string, score, durationMs, memoryKb int, errMsg string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE submissions SET status=$2, score=$3, duration_ms=$4, memory_kb=$5, error_msg=$6
		WHERE id=$1`,
		id, status, score, durationMs, memoryKb, errMsg)
	return err
}

// SaveSubmissionResult 保存单个测试点的运行结果。
func (s *OJStore) SaveSubmissionResult(ctx context.Context, r *SubmissionResult) error {
	return s.db.QueryRowContext(ctx, `
		INSERT INTO submission_results (submission_id, test_case_id, status, actual_output, duration_ms, memory_kb, score)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		r.SubmissionID, r.TestCaseID, r.Status, r.ActualOutput, r.DurationMs, r.MemoryKb, r.Score,
	).Scan(&r.ID)
}

// GetSubmissionResults 返回一次提交的全部测试点结果。
func (s *OJStore) GetSubmissionResults(ctx context.Context, submissionID string) ([]SubmissionResult, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, submission_id, test_case_id, status, coalesce(actual_output,''), duration_ms, memory_kb, score
		FROM submission_results WHERE submission_id = $1
		ORDER BY (SELECT ordering FROM test_cases WHERE id = test_case_id)`,
		submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SubmissionResult
	for rows.Next() {
		var r SubmissionResult
		if err := rows.Scan(&r.ID, &r.SubmissionID, &r.TestCaseID, &r.Status,
			&r.ActualOutput, &r.DurationMs, &r.MemoryKb, &r.Score); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- 输出比对 ---

// NormalizeOutput 去除尾部空白并统一换行符。
func NormalizeOutput(s string) string {
	s = strings.TrimRight(s, " \t\r\n")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return s
}
