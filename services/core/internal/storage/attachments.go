package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Attachment struct {
	ID          string  `json:"id"`
	SessionID   string  `json:"sessionId"`
	OrigName    string  `json:"origName"`
	MIMEType    string  `json:"mimeType"`
	SizeBytes   int64   `json:"sizeBytes"`
	StoragePath string  `json:"-"`
	Extracted   *string `json:"-"`
	IsChunked   bool    `json:"isChunked"`
	ChunkCount  int     `json:"chunkCount"`
	CreatedAt   int64   `json:"createdAt"`
	DeletedAt   *int64  `json:"deletedAt,omitempty"`
}

type CreateAttachmentInput struct {
	SessionID   string
	OrigName    string
	MIMEType    string
	SizeBytes   int64
	StoragePath string
	Extracted   *string
	IsChunked   bool
	ChunkCount  int
}

func (db *DB) CreateAttachment(ctx context.Context, in CreateAttachmentInput) (*Attachment, error) {
	now := time.Now().UnixMilli()
	a := &Attachment{
		ID:          uuid.New().String(),
		SessionID:   in.SessionID,
		OrigName:    in.OrigName,
		MIMEType:    in.MIMEType,
		SizeBytes:   in.SizeBytes,
		StoragePath: in.StoragePath,
		Extracted:   in.Extracted,
		IsChunked:   in.IsChunked,
		ChunkCount:  in.ChunkCount,
		CreatedAt:   now,
	}
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO attachments
		 (id, session_id, orig_name, mime_type, size_bytes, storage_path, extracted, is_chunked, chunk_count, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.SessionID, a.OrigName, a.MIMEType, a.SizeBytes, a.StoragePath,
		nullableString(a.Extracted), boolToInt(a.IsChunked), a.ChunkCount, a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (db *DB) GetAttachment(ctx context.Context, id string) (*Attachment, error) {
	row := db.conn.QueryRowContext(ctx,
		`SELECT id, session_id, orig_name, mime_type, size_bytes, storage_path,
		        extracted, is_chunked, chunk_count, created_at, deleted_at
		   FROM attachments WHERE id = ?`, id)
	return scanAttachment(row)
}

func (db *DB) ListAttachments(ctx context.Context, sessionID string) ([]Attachment, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, session_id, orig_name, mime_type, size_bytes, storage_path,
		        extracted, is_chunked, chunk_count, created_at, deleted_at
		   FROM attachments
		  WHERE session_id = ? AND deleted_at IS NULL
		  ORDER BY created_at DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Attachment{}
	for rows.Next() {
		a, err := scanAttachmentRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (db *DB) SoftDeleteAttachment(ctx context.Context, id string) error {
	now := time.Now().UnixMilli()
	res, err := db.conn.ExecContext(ctx,
		`UPDATE attachments SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`,
		now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanAttachment(r rowScanner) (*Attachment, error) {
	a, err := scanAttachmentRows(r)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

func scanAttachmentRows(r rowScanner) (*Attachment, error) {
	var (
		a         Attachment
		extracted sql.NullString
		isChunked int
		deleted   sql.NullInt64
	)
	if err := r.Scan(&a.ID, &a.SessionID, &a.OrigName, &a.MIMEType, &a.SizeBytes,
		&a.StoragePath, &extracted, &isChunked, &a.ChunkCount, &a.CreatedAt, &deleted); err != nil {
		return nil, err
	}
	if extracted.Valid {
		s := extracted.String
		a.Extracted = &s
	}
	a.IsChunked = isChunked == 1
	if deleted.Valid {
		v := deleted.Int64
		a.DeletedAt = &v
	}
	return &a, nil
}
