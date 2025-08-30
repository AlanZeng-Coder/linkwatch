package storage

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Storage interface {
	CreateTarget(ctx context.Context, url string, idempotencyKey string) (*Target, bool, error)
	ListTargets(ctx context.Context, host string, limit int, pageToken string) ([]*Target, string, error)
	GetCheckResults(ctx context.Context, targetID string, since time.Time, limit int) ([]*CheckResult, error)
	SaveCheckResult(ctx context.Context, targetID string, result *CheckResult) error
	Close() error
	Init(ctx context.Context) error
}

type Target struct {
	ID        string
	URL       string
	CreatedAt time.Time
}

type CheckResult struct {
	CheckedAt  time.Time
	StatusCode int
	LatencyMs  int
	Error      string
}

type SQLiteStorage struct {
	db *sql.DB
}

func NewSQLiteStorage(db *sql.DB) *SQLiteStorage {
	return &SQLiteStorage{db: db}
}

func (s *SQLiteStorage) Init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS targets (
			id TEXT PRIMARY KEY,
			url TEXT UNIQUE NOT NULL,
			created_at DATETIME NOT NULL
		);
		CREATE TABLE IF NOT EXISTS check_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			target_id TEXT NOT NULL,
			checked_at DATETIME NOT NULL,
			status_code INTEGER,
			latency_ms INTEGER,
			error TEXT
		);
		CREATE TABLE IF NOT EXISTS idempotency_keys (
			key TEXT PRIMARY KEY,
			target_id TEXT NOT NULL
		);
	`)
	return err
}

func (s *SQLiteStorage) CreateTarget(ctx context.Context, url string, idempotencyKey string) (*Target, bool, error) {
	var isNew bool
	id := "t_" + uuid.NewString()
	createdAt := time.Now().UTC()

	if idempotencyKey != "" {
		var existingID string
		err := s.db.QueryRowContext(ctx, `SELECT target_id FROM idempotency_keys WHERE key = ?`, idempotencyKey).Scan(&existingID)
		if err == nil {
			target, err := s.getTarget(ctx, existingID)
			return target, false, err
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, false, err
		}
	}

	res, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO targets (id, url, created_at) VALUES (?, ?, ?)`, id, url, createdAt)
	if err != nil {
		return nil, false, err
	}

	rowsAffected, _ := res.RowsAffected()
	isNew = (rowsAffected > 0)

	err = s.db.QueryRowContext(ctx, `SELECT id, created_at FROM targets WHERE url = ?`, url).Scan(&id, &createdAt)
	if err != nil {
		return nil, false, err
	}

	if idempotencyKey != "" {
		_, err = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO idempotency_keys (key, target_id) VALUES (?, ?)`, idempotencyKey, id)
		if err != nil {
			return nil, false, err
		}
	}

	return &Target{ID: id, URL: url, CreatedAt: createdAt}, isNew, nil
}

func (s *SQLiteStorage) getTarget(ctx context.Context, id string) (*Target, error) {
	t := &Target{ID: id}
	err := s.db.QueryRowContext(ctx, `SELECT url, created_at FROM targets WHERE id = ?`, id).Scan(&t.URL, &t.CreatedAt)
	return t, err
}

func (s *SQLiteStorage) ListTargets(ctx context.Context, host string, limit int, pageToken string) ([]*Target, string, error) {
	var whereClauses []string
	var args []interface{}

	if host != "" {
		whereClauses = append(whereClauses, "LOWER(url) LIKE ?")
		args = append(args, "%://"+strings.ToLower(host)+"%")
	}

	var createdAt time.Time
	var cursorID string
	if pageToken != "" {
		decoded, err := base64.StdEncoding.DecodeString(pageToken)
		if err != nil {
			return nil, "", err
		}
		parts := strings.Split(string(decoded), "|")
		if len(parts) != 2 {
			return nil, "", fmt.Errorf("invalid page token")
		}
		createdAt, err = time.Parse(time.RFC3339, parts[0])
		if err != nil {
			return nil, "", err
		}
		cursorID = parts[1]
		whereClauses = append(whereClauses, "(created_at > ? OR (created_at = ? AND id > ?))")
		args = append(args, createdAt, createdAt, cursorID)
	}

	where := ""
	if len(whereClauses) > 0 {
		where = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	query := `SELECT id, url, created_at FROM targets ` + where + ` ORDER BY created_at ASC, id ASC LIMIT ?`
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var items []*Target
	for rows.Next() {
		t := &Target{}
		if err := rows.Scan(&t.ID, &t.URL, &t.CreatedAt); err != nil {
			return nil, "", err
		}
		items = append(items, t)
	}

	var nextToken string
	if len(items) > limit {
		last := items[limit]
		nextToken = base64.StdEncoding.EncodeToString([]byte(last.CreatedAt.Format(time.RFC3339) + "|" + last.ID))
		items = items[:limit]
	}
	return items, nextToken, nil
}

func (s *SQLiteStorage) GetCheckResults(ctx context.Context, targetID string, since time.Time, limit int) ([]*CheckResult, error) {
	query := `SELECT checked_at, status_code, latency_ms, error FROM check_results WHERE target_id = ?`
	args := []interface{}{targetID}
	if !since.IsZero() {
		query += ` AND checked_at >= ?`
		args = append(args, since)
	}
	query += ` ORDER BY checked_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*CheckResult
	for rows.Next() {
		r := &CheckResult{}
		if err := rows.Scan(&r.CheckedAt, &r.StatusCode, &r.LatencyMs, &r.Error); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

func (s *SQLiteStorage) SaveCheckResult(ctx context.Context, targetID string, result *CheckResult) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO check_results (target_id, checked_at, status_code, latency_ms, error) VALUES (?, ?, ?, ?, ?)`,
		targetID, result.CheckedAt, result.StatusCode, result.LatencyMs, result.Error)
	return err
}

func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}
