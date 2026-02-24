package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func New(dataSourceName string) (*DB, error) {
	db, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	database := &DB{db}
	if err := database.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return database, nil
}

func (db *DB) createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS generations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			prompt TEXT NOT NULL,
			image_path TEXT NOT NULL,
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
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) InsertGeneration(prompt, imagePath, status, errorMessage string) (int64, error) {
	query := `
	INSERT INTO generations (prompt, image_path, status, error_message)
	VALUES (?, ?, ?, ?)
	`

	result, err := db.Exec(query, prompt, imagePath, status, errorMessage)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func (db *DB) UpdateGenerationStatus(id int64, status, imagePath, errorMessage string) error {
	query := `
	UPDATE generations
	SET status = ?, image_path = ?, error_message = ?
	WHERE id = ?
	`

	_, err := db.Exec(query, status, imagePath, errorMessage, id)
	return err
}

func (db *DB) GetGeneration(id int64) (*Generation, error) {
	query := `
	SELECT id, prompt, image_path, status, error_message, created_at
	FROM generations
	WHERE id = ?
	`

	var gen Generation
	err := db.QueryRow(query, id).Scan(
		&gen.ID,
		&gen.Prompt,
		&gen.ImagePath,
		&gen.Status,
		&gen.ErrorMessage,
		&gen.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &gen, nil
}

func (db *DB) ListGenerations(limit int) ([]Generation, error) {
	query := `
	SELECT id, prompt, image_path, status, error_message, created_at
	FROM generations
	ORDER BY created_at DESC
	LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var generations []Generation
	for rows.Next() {
		var gen Generation
		err := rows.Scan(
			&gen.ID,
			&gen.Prompt,
			&gen.ImagePath,
			&gen.Status,
			&gen.ErrorMessage,
			&gen.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		generations = append(generations, gen)
	}

	return generations, rows.Err()
}

func (db *DB) GetSetting(key string) (string, error) {
	query := `SELECT value FROM settings WHERE key = ?`

	var value string
	err := db.QueryRow(query, key).Scan(&value)
	if err != nil {
		return "", err
	}

	return value, nil
}

func (db *DB) SetSetting(key, value string) error {
	query := `INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`

	_, err := db.Exec(query, key, value)
	return err
}
