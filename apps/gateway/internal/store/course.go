package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Course 代表一门课程。
type Course struct {
	ID            string `json:"id"`
	Topic         string `json:"topic"`
	Audience      string `json:"audience"`
	Depth         string `json:"depth"`
	Status        string `json:"status"`
	OutlineJSON   string `json:"outline_json"`
	StoragePrefix string `json:"storage_prefix"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// CourseStore 对 postgres courses 表提供 CRUD。
type CourseStore struct {
	db *sql.DB
}

// NewCourseStore creates a CourseStore.
func NewCourseStore(db *sql.DB) *CourseStore {
	return &CourseStore{db: db}
}

// ListCourses returns recent courses for the single-tenant MVP.
func (s *CourseStore) ListCourses(ctx context.Context, limit int) ([]Course, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,topic,coalesce(audience,''),coalesce(depth,''),status,
		        coalesce(outline_json,'null'),coalesce(storage_prefix,''),
		        created_at::text,updated_at::text
		 FROM courses ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	courses := make([]Course, 0)
	for rows.Next() {
		var c Course
		var outlineJSON sql.NullString
		if err := rows.Scan(&c.ID, &c.Topic, &c.Audience, &c.Depth, &c.Status,
			&outlineJSON, &c.StoragePrefix, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if outlineJSON.Valid {
			c.OutlineJSON = outlineJSON.String
		}
		courses = append(courses, c)
	}
	return courses, rows.Err()
}

// GetCourse reads a course by id.
func (s *CourseStore) GetCourse(ctx context.Context, id string) (*Course, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id,topic,coalesce(audience,''),coalesce(depth,''),status,
		        coalesce(outline_json,'null'),coalesce(storage_prefix,''),
		        created_at::text,updated_at::text
		 FROM courses WHERE id=$1`, id)
	var c Course
	var outlineJSON sql.NullString
	err := row.Scan(&c.ID, &c.Topic, &c.Audience, &c.Depth, &c.Status,
		&outlineJSON, &c.StoragePrefix, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if outlineJSON.Valid {
		c.OutlineJSON = outlineJSON.String
	}
	return &c, nil
}

// GetChapterMarkdown reads courses/{id}/{chID}.md from MinIO.
func (s *CourseStore) GetChapterMarkdown(ctx context.Context, courseID, chID string) (string, error) {
	course, err := s.GetCourse(ctx, courseID)
	if err != nil {
		return "", err
	}
	prefix := course.StoragePrefix
	if prefix == "" {
		prefix = fmt.Sprintf("courses/%s/", courseID)
	}
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9000"
	}
	accessKey := os.Getenv("MINIO_ROOT_USER")
	if accessKey == "" {
		accessKey = "minio"
	}
	secretKey := os.Getenv("MINIO_ROOT_PASSWORD")
	if secretKey == "" {
		secretKey = "minio_dev_pw"
	}
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return "", err
	}
	obj, err := mc.GetObject(ctx, "marsagent", prefix+chID+".md", minio.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	defer obj.Close()
	b, err := io.ReadAll(obj)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CreateCourse inserts a pending course and returns its id.
func (s *CourseStore) CreateCourse(ctx context.Context, topic string) (id string, err error) {
	row := s.db.QueryRowContext(ctx,
		`INSERT INTO courses (topic, status, storage_prefix) VALUES ($1, 'pending', '')
		 RETURNING id`, topic)
	err = row.Scan(&id)
	return
}

// UpdateCourseStatus updates status and outline_json.
func (s *CourseStore) UpdateCourseStatus(ctx context.Context, id, status string, outline any) error {
	var outlineJSON []byte
	if outline != nil {
		outlineJSON, _ = json.Marshal(outline)
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE courses SET status=$2, outline_json=$3, updated_at=now() WHERE id=$1`,
		id, status, outlineJSON)
	return err
}

// SetStoragePrefix sets the MinIO storage prefix for a course.
func (s *CourseStore) SetStoragePrefix(ctx context.Context, id, prefix string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE courses SET storage_prefix=$2, updated_at=now() WHERE id=$1`,
		id, prefix)
	return err
}
