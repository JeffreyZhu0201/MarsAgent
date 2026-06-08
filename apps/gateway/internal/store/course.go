package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

// Course 代表一门课程。
type Course struct {
	ID            string
	Topic         string
	Audience      string
	Depth         string
	Status        string
	OutlineJSON   string
	StoragePrefix string
	CreatedAt     string
	UpdatedAt     string
}

// CourseStore 对 postgres courses 表提供 CRUD。
type CourseStore struct {
	db *sql.DB
}

// NewCourseStore creates a CourseStore.
func NewCourseStore(db *sql.DB) *CourseStore {
	return &CourseStore{db: db}
}

// GetCourse reads a course by id.
func (s *CourseStore) GetCourse(ctx context.Context, id string) (*Course, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id,topic,coalesce(audience,''),coalesce(depth,''),status,
		        coalesce(outline_json,'null'),coalesce(storage_prefix,''),
		        created_at,updated_at
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
