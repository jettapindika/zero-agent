package storage

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

func (db *DB) GetOrCreateProject(ctx context.Context, path, name string) (*Project, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT id, path, name, created_at, updated_at FROM projects WHERE path = ?`, path)
	var p Project
	err := row.Scan(&p.ID, &p.Path, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if err == nil {
		return &p, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	now := time.Now().UnixMilli()
	p = Project{
		ID:        uuid.New().String(),
		Path:      path,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err = db.conn.ExecContext(ctx, `INSERT INTO projects (id, path, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.Path, p.Name, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (db *DB) GetProjectByPath(ctx context.Context, path string) (*Project, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT id, path, name, created_at, updated_at FROM projects WHERE path = ?`, path)
	var p Project
	err := row.Scan(&p.ID, &p.Path, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}
