package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

type Explanation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Topic     string    `json:"topic"`
	FilePath  string    `json:"file_path"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type SectionThread struct {
	ID            string
	ExplanationID string
	SectionID     string
	Messages      []Message
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func New(path string) (*DB, error) {
	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}

	db := &DB{sqldb}
	if err := db.migrate(); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("migrating: %w", err)
	}

	return db, nil
}

func (db *DB) migrate() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS explanations (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			topic TEXT NOT NULL,
			file_path TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS section_threads (
			id TEXT PRIMARY KEY,
			explanation_id TEXT NOT NULL REFERENCES explanations(id),
			section_id TEXT NOT NULL,
			messages TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
	`)
	return err
}

func (db *DB) CreateExplanation(title, topic, filePath string) (*Explanation, error) {
	now := time.Now().UTC()
	id := uuid.New().String()

	_, err := db.Exec(
		`INSERT INTO explanations (id, title, topic, file_path, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, title, topic, filePath, now, now,
	)
	if err != nil {
		return nil, err
	}

	return &Explanation{
		ID:        id,
		Title:     title,
		Topic:     topic,
		FilePath:  filePath,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (db *DB) ListExplanations() ([]Explanation, error) {
	rows, err := db.Query(
		`SELECT id, title, topic, file_path, created_at, updated_at FROM explanations ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var explanations []Explanation
	for rows.Next() {
		var e Explanation
		if err := rows.Scan(&e.ID, &e.Title, &e.Topic, &e.FilePath, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		explanations = append(explanations, e)
	}
	return explanations, rows.Err()
}

func (db *DB) GetExplanation(id string) (*Explanation, error) {
	var e Explanation
	err := db.QueryRow(
		`SELECT id, title, topic, file_path, created_at, updated_at FROM explanations WHERE id = ?`, id,
	).Scan(&e.ID, &e.Title, &e.Topic, &e.FilePath, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &e, err
}

func (db *DB) TouchExplanation(id string) error {
	_, err := db.Exec(`UPDATE explanations SET updated_at = ? WHERE id = ?`, time.Now().UTC(), id)
	return err
}

func (db *DB) UpdateExplanationTitle(id, title string) error {
	_, err := db.Exec(`UPDATE explanations SET title = ?, updated_at = ? WHERE id = ?`, title, time.Now().UTC(), id)
	return err
}

func (db *DB) GetOrCreateSectionThread(explanationID, sectionID string) (*SectionThread, error) {
	var thread SectionThread
	var messagesJSON string

	err := db.QueryRow(
		`SELECT id, explanation_id, section_id, messages, created_at, updated_at FROM section_threads WHERE explanation_id = ? AND section_id = ?`,
		explanationID, sectionID,
	).Scan(&thread.ID, &thread.ExplanationID, &thread.SectionID, &messagesJSON, &thread.CreatedAt, &thread.UpdatedAt)

	if err == sql.ErrNoRows {
		now := time.Now().UTC()
		thread.ID = uuid.New().String()
		thread.ExplanationID = explanationID
		thread.SectionID = sectionID
		thread.Messages = []Message{}
		thread.CreatedAt = now
		thread.UpdatedAt = now

		_, err = db.Exec(
			`INSERT INTO section_threads (id, explanation_id, section_id, messages, created_at, updated_at) VALUES (?, ?, ?, '[]', ?, ?)`,
			thread.ID, explanationID, sectionID, now, now,
		)
		return &thread, err
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(messagesJSON), &thread.Messages); err != nil {
		return nil, err
	}
	return &thread, nil
}

func (db *DB) UpdateThreadMessages(threadID string, messages []Message) error {
	data, err := json.Marshal(messages)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`UPDATE section_threads SET messages = ?, updated_at = ? WHERE id = ?`,
		string(data), time.Now().UTC(), threadID,
	)
	return err
}
