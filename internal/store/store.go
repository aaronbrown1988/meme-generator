// Package store is the SQLite-backed persistence adapter for the meme
// domain. The store knows about rows; meme.Generation is the only type
// crossing the seam.
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"

	"meme-generator/internal/meme"
)

type Store struct {
	db *sql.DB
}

func Open(dataSourceName string) (*Store, error) {
	db, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS generations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			prompt TEXT NOT NULL,
			image_path TEXT NOT NULL,
			top_text TEXT DEFAULT '',
			bottom_text TEXT DEFAULT '',
			status TEXT NOT NULL,
			error_message TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
		`INSERT OR IGNORE INTO settings (key, value)
		 VALUES ('system_prompt', 'You are a creative meme generator. Generate images based on the following description:');`,
		// Best-effort migrations for older databases. Errors ignored;
		// columns may already exist.
		`ALTER TABLE generations ADD COLUMN top_text TEXT DEFAULT '';`,
		`ALTER TABLE generations ADD COLUMN bottom_text TEXT DEFAULT '';`,
	}
	for _, q := range queries {
		s.db.Exec(q)
	}
	return nil
}

func (s *Store) InsertGeneration(prompt, imagePath string, status meme.Status, errorMessage string) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO generations (prompt, image_path, top_text, bottom_text, status, error_message)
		 VALUES (?, ?, '', '', ?, ?)`,
		prompt, imagePath, string(status), errorMessage,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateGenerationStatus(id int64, status meme.Status, imagePath, errorMessage string) error {
	_, err := s.db.Exec(
		`UPDATE generations SET status = ?, image_path = ?, error_message = ? WHERE id = ?`,
		string(status), imagePath, errorMessage, id,
	)
	return err
}

func (s *Store) UpdateGenerationCaption(id int64, c meme.Caption) error {
	_, err := s.db.Exec(
		`UPDATE generations SET top_text = ?, bottom_text = ? WHERE id = ?`,
		c.Top, c.Bottom, id,
	)
	return err
}

// MarkProcessingFailed flips every row currently in the processing
// state to failed with the given message. Used at startup to reconcile
// generations stranded by a crash or restart.
func (s *Store) MarkProcessingFailed(message string) (int64, error) {
	res, err := s.db.Exec(
		`UPDATE generations SET status = ?, error_message = ? WHERE status = ?`,
		string(meme.StatusFailed), message, string(meme.StatusProcessing),
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) GetGeneration(id int64) (meme.Generation, error) {
	var (
		gen      meme.Generation
		status   string
		errMsg   sql.NullString
		topText  string
		botText  string
	)
	err := s.db.QueryRow(
		`SELECT id, prompt, image_path, top_text, bottom_text, status, error_message, created_at
		 FROM generations WHERE id = ?`, id,
	).Scan(&gen.ID, &gen.Prompt, &gen.ImagePath, &topText, &botText, &status, &errMsg, &gen.CreatedAt)
	if err != nil {
		return meme.Generation{}, err
	}
	gen.Caption = meme.Caption{Top: topText, Bottom: botText}
	gen.Status = meme.Status(status)
	if errMsg.Valid {
		gen.ErrorMessage = errMsg.String
	}
	return gen, nil
}

func (s *Store) ListGenerations(limit int) ([]meme.Generation, error) {
	rows, err := s.db.Query(
		`SELECT id, prompt, image_path, top_text, bottom_text, status, error_message, created_at
		 FROM generations ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []meme.Generation
	for rows.Next() {
		var (
			gen     meme.Generation
			status  string
			errMsg  sql.NullString
			topText string
			botText string
		)
		if err := rows.Scan(&gen.ID, &gen.Prompt, &gen.ImagePath, &topText, &botText, &status, &errMsg, &gen.CreatedAt); err != nil {
			return nil, err
		}
		gen.Caption = meme.Caption{Top: topText, Bottom: botText}
		gen.Status = meme.Status(status)
		if errMsg.Valid {
			gen.ErrorMessage = errMsg.String
		}
		out = append(out, gen)
	}
	return out, rows.Err()
}

func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`, key, value)
	return err
}
