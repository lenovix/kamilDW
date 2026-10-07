package store

import (
	"database/sql"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type TaskRecord struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	Filename     string `json:"filename"`
	FilePath     string `json:"filePath"`
	TotalBytes   int64  `json:"totalBytes"`
	Downloaded   int64  `json:"downloaded"`
	Status       string `json:"status"`
	Connections  int    `json:"connections"`
	Category     string `json:"category"`
	ETag         string `json:"etag"`
	AcceptRanges bool   `json:"acceptRanges"`
	Error        string `json:"error"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type ChunkRecord struct {
	ID          int64  `json:"id"`
	TaskID      string `json:"taskId"`
	Idx         int    `json:"idx"`
	StartOffset int64  `json:"startOffset"`
	EndOffset   int64  `json:"endOffset"`
	Downloaded  int64  `json:"downloaded"`
	Status      string `json:"status"`
}

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(Schema); err != nil {
		db.Close()
		return nil, err
	}
	return &DB{db}, nil
}

func Categorize(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".iso", ".bz2", ".xz":
		return "Compressed"
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt":
		return "Documents"
	case ".mp3", ".flac", ".wav", ".aac", ".ogg", ".m4a":
		return "Music"
	case ".mp4", ".mkv", ".webm", ".avi", ".mov", ".ts", ".m4v":
		return "Videos"
	case ".exe", ".msi", ".dmg", ".pkg", ".deb", ".rpm", ".apk":
		return "Programs"
	default:
		return "Other"
	}
}

func (db *DB) UpsertTask(t TaskRecord) error {
	q := `INSERT INTO tasks (id, url, filename, filepath, total_bytes, downloaded, status, connections, category, etag, accept_ranges, error)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		downloaded=excluded.downloaded,
		status=excluded.status,
		error=excluded.error,
		updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')`
	ar := 0
	if t.AcceptRanges {
		ar = 1
	}
	_, err := db.Exec(q, t.ID, t.URL, t.Filename, t.FilePath, t.TotalBytes, t.Downloaded, t.Status, t.Connections, t.Category, t.ETag, ar, t.Error)
	return err
}

func (db *DB) GetTask(id string) (*TaskRecord, error) {
	row := db.QueryRow(`SELECT id, url, filename, filepath, total_bytes, downloaded, status, connections, category, COALESCE(etag,''), accept_ranges, COALESCE(error,''), created_at, updated_at FROM tasks WHERE id = ?`, id)
	var t TaskRecord
	var ar int
	err := row.Scan(&t.ID, &t.URL, &t.Filename, &t.FilePath, &t.TotalBytes, &t.Downloaded, &t.Status, &t.Connections, &t.Category, &t.ETag, &ar, &t.Error, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	t.AcceptRanges = ar == 1
	return &t, nil
}

func (db *DB) DeleteTask(id string) error {
	_, err := db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

func (db *DB) ListTasks(category string) ([]TaskRecord, error) {
	query := `SELECT id, url, filename, filepath, total_bytes, downloaded, status, connections, category, COALESCE(etag,''), accept_ranges, COALESCE(error,''), created_at, updated_at FROM tasks`
	var args []any
	if category != "" && category != "All" {
		query += ` WHERE category = ?`
		args = append(args, category)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []TaskRecord
	for rows.Next() {
		var t TaskRecord
		var ar int
		if err := rows.Scan(&t.ID, &t.URL, &t.Filename, &t.FilePath, &t.TotalBytes, &t.Downloaded, &t.Status, &t.Connections, &t.Category, &t.ETag, &ar, &t.Error, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.AcceptRanges = ar == 1
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (db *DB) SaveChunks(taskID string, chunks []ChunkRecord) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO chunks (task_id, idx, start_offset, end_offset, downloaded, status)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id, idx) DO UPDATE SET downloaded=excluded.downloaded, status=excluded.status`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range chunks {
		if _, err := stmt.Exec(taskID, c.Idx, c.StartOffset, c.EndOffset, c.Downloaded, c.Status); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) GetChunks(taskID string) ([]ChunkRecord, error) {
	rows, err := db.Query(`SELECT id, task_id, idx, start_offset, end_offset, downloaded, status FROM chunks WHERE task_id = ? ORDER BY idx ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []ChunkRecord
	for rows.Next() {
		var c ChunkRecord
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Idx, &c.StartOffset, &c.EndOffset, &c.Downloaded, &c.Status); err != nil {
			return nil, err
		}
		res = append(res, c)
	}
	return res, nil
}
