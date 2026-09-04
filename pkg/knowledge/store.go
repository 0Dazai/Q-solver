package knowledge

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("资料库路径为空")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func DefaultDatabasePath(appName string) (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, appName, "knowledge", "knowledge.db"), nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY,
			path TEXT NOT NULL UNIQUE,
			content_hash TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
			path TEXT NOT NULL,
			title_path TEXT NOT NULL,
			content TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			ordinal INTEGER NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS chunk_fts USING fts5(
			chunk_id UNINDEXED,
			title_path,
			content,
			tokenize='trigram'
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("初始化本地资料索引失败: %w", err)
		}
	}
	return nil
}

func (s *Store) IndexMarkdown(ctx context.Context, path, content string) error {
	path = filepath.Clean(path)
	chunks := ChunkMarkdown(path, content)
	documentID := shortHash(path)
	contentHash := shortHash(content)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM chunk_fts WHERE chunk_id IN (SELECT id FROM chunks WHERE document_id = ?)`, documentID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM chunks WHERE document_id = ?`, documentID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO documents(id,path,content_hash,updated_at)
		VALUES(?,?,?,CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET path=excluded.path,content_hash=excluded.content_hash,updated_at=CURRENT_TIMESTAMP`,
		documentID, path, contentHash); err != nil {
		return err
	}
	for _, chunk := range chunks {
		if _, err := tx.ExecContext(ctx, `INSERT INTO chunks(id,document_id,path,title_path,content,content_hash,ordinal,updated_at)
			VALUES(?,?,?,?,?,?,?,CURRENT_TIMESTAMP)`,
			chunk.ID, chunk.DocumentID, chunk.Path, chunk.TitlePath, chunk.Content, chunk.ContentHash, chunk.Ordinal); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO chunk_fts(chunk_id,title_path,content) VALUES(?,?,?)`,
			chunk.ID, chunk.TitlePath, chunk.Content); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) DeleteDocument(ctx context.Context, path string) error {
	documentID := shortHash(filepath.Clean(path))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM chunk_fts WHERE chunk_id IN (SELECT id FROM chunks WHERE document_id = ?)`, documentID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, documentID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListDocuments(ctx context.Context) ([]Document, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,path,content_hash,updated_at FROM documents ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	documents := make([]Document, 0)
	for rows.Next() {
		var document Document
		var updatedAt string
		if err := rows.Scan(&document.ID, &document.Path, &document.ContentHash, &updatedAt); err != nil {
			return nil, err
		}
		document.UpdatedAt = parseSQLiteTime(updatedAt)
		documents = append(documents, document)
	}
	return documents, rows.Err()
}

func (s *Store) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []SearchResult{}, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 6
	}
	rows, err := s.db.QueryContext(ctx, `SELECT c.id,c.document_id,c.path,c.title_path,c.content,c.content_hash,c.ordinal,c.updated_at,
		-bm25(chunk_fts, 0.0, 3.0, 1.0) AS score
		FROM chunk_fts JOIN chunks c ON c.id=chunk_fts.chunk_id
		WHERE chunk_fts MATCH ?
		ORDER BY score DESC LIMIT ?`, quoteFTSQuery(query), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]SearchResult, 0)
	for rows.Next() {
		var result SearchResult
		var updatedAt string
		if err := rows.Scan(&result.ID, &result.DocumentID, &result.Path, &result.TitlePath, &result.Content,
			&result.ContentHash, &result.Ordinal, &updatedAt, &result.Score); err != nil {
			return nil, err
		}
		result.UpdatedAt = parseSQLiteTime(updatedAt)
		result.Source = "local_fts"
		results = append(results, result)
	}
	return results, rows.Err()
}

// PinnedMarkdownContext returns bounded Markdown content for interview-session
// preloading. PDF text remains available through question-time retrieval.
func (s *Store) PinnedMarkdownContext(ctx context.Context, maxRunes int) ([]string, error) {
	if maxRunes <= 0 {
		maxRunes = 6000
	}
	rows, err := s.db.QueryContext(ctx, `SELECT path,title_path,content FROM chunks
		WHERE lower(path) LIKE '%.md' OR lower(path) LIKE '%.markdown'
		ORDER BY path,ordinal`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	documents := make([]string, 0)
	currentPath := ""
	var current strings.Builder
	used := 0
	flush := func() {
		if current.Len() > 0 {
			documents = append(documents, current.String())
			current.Reset()
		}
	}
	for rows.Next() {
		var path, title, content string
		if err := rows.Scan(&path, &title, &content); err != nil {
			return nil, err
		}
		if currentPath != path {
			flush()
			currentPath = path
			current.WriteString("# ")
			current.WriteString(filepath.Base(path))
			current.WriteString("\n")
		}
		chunk := strings.TrimSpace(title + "\n" + content)
		runes := []rune(chunk)
		remaining := maxRunes - used
		if remaining <= 0 {
			break
		}
		if len(runes) > remaining {
			runes = runes[:remaining]
		}
		current.WriteString(string(runes))
		current.WriteString("\n")
		used += len(runes)
	}
	flush()
	return documents, rows.Err()
}

func parseSQLiteTime(value string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, time.RFC3339Nano} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func quoteFTSQuery(query string) string {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return `""`
	}
	terms := make([]string, 0, len(query))
	seen := make(map[string]bool)
	for _, field := range fields {
		runes := []rune(strings.TrimSpace(field))
		if len(runes) == 0 {
			continue
		}
		if len(runes) < 3 {
			term := strings.ReplaceAll(string(runes), `"`, `""`)
			quoted := `"` + term + `"`
			if !seen[quoted] {
				seen[quoted] = true
				terms = append(terms, quoted)
			}
			continue
		}
		for i := 0; i+3 <= len(runes); i++ {
			term := strings.ReplaceAll(string(runes[i:i+3]), `"`, `""`)
			quoted := `"` + term + `"`
			if seen[quoted] {
				continue
			}
			seen[quoted] = true
			terms = append(terms, quoted)
		}
	}
	if len(terms) == 0 {
		return `""`
	}
	return strings.Join(terms, " OR ")
}
