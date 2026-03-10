package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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
	Tags      []string  `json:"tags"`
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

		CREATE TABLE IF NOT EXISTS tags (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE
		);

		CREATE TABLE IF NOT EXISTS explanation_tags (
			explanation_id TEXT NOT NULL REFERENCES explanations(id),
			tag_id TEXT NOT NULL REFERENCES tags(id),
			PRIMARY KEY (explanation_id, tag_id)
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
		Tags:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (db *DB) ListExplanations(filterTags []string) ([]Explanation, error) {
	baseQ := `
		SELECT e.id, e.title, e.topic, e.file_path, e.created_at, e.updated_at,
		       COALESCE(GROUP_CONCAT(t.name, ','), '') AS tags
		FROM explanations e
		LEFT JOIN explanation_tags et ON et.explanation_id = e.id
		LEFT JOIN tags t ON t.id = et.tag_id`

	var rows *sql.Rows
	var err error

	if len(filterTags) == 0 {
		rows, err = db.Query(baseQ + " GROUP BY e.id ORDER BY e.updated_at DESC")
	} else {
		ph := strings.Repeat("?,", len(filterTags))
		ph = ph[:len(ph)-1]
		args := make([]any, len(filterTags)+1)
		for i, tag := range filterTags {
			args[i] = tag
		}
		args[len(filterTags)] = int64(len(filterTags))
		query := baseQ + `
			WHERE e.id IN (
				SELECT et2.explanation_id
				FROM explanation_tags et2
				JOIN tags t2 ON t2.id = et2.tag_id
				WHERE t2.name IN (` + ph + `)
				GROUP BY et2.explanation_id
				HAVING COUNT(DISTINCT t2.id) = ?
			)
			GROUP BY e.id ORDER BY e.updated_at DESC`
		rows, err = db.Query(query, args...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var explanations []Explanation
	for rows.Next() {
		var e Explanation
		var tagsStr string
		if err := rows.Scan(&e.ID, &e.Title, &e.Topic, &e.FilePath, &e.CreatedAt, &e.UpdatedAt, &tagsStr); err != nil {
			return nil, err
		}
		e.Tags = splitTags(tagsStr)
		explanations = append(explanations, e)
	}
	return explanations, rows.Err()
}

func (db *DB) GetExplanation(id string) (*Explanation, error) {
	var e Explanation
	var tagsStr string
	err := db.QueryRow(`
		SELECT e.id, e.title, e.topic, e.file_path, e.created_at, e.updated_at,
		       COALESCE(GROUP_CONCAT(t.name, ','), '') AS tags
		FROM explanations e
		LEFT JOIN explanation_tags et ON et.explanation_id = e.id
		LEFT JOIN tags t ON t.id = et.tag_id
		WHERE e.id = ?
		GROUP BY e.id`, id,
	).Scan(&e.ID, &e.Title, &e.Topic, &e.FilePath, &e.CreatedAt, &e.UpdatedAt, &tagsStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	e.Tags = splitTags(tagsStr)
	return &e, nil
}

func (db *DB) TouchExplanation(id string) error {
	_, err := db.Exec(`UPDATE explanations SET updated_at = ? WHERE id = ?`, time.Now().UTC(), id)
	return err
}

func (db *DB) DeleteExplanation(id string) error {
	_, err := db.Exec(`DELETE FROM explanation_tags WHERE explanation_id = ?`, id)
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM section_threads WHERE explanation_id = ?`, id)
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM explanations WHERE id = ?`, id)
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

func (db *DB) ListTags() ([]string, error) {
	rows, err := db.Query(`SELECT name FROM tags ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tags = append(tags, name)
	}
	return tags, rows.Err()
}

func (db *DB) GetOrCreateTag(name string) (string, error) {
	var id string
	err := db.QueryRow(`SELECT id FROM tags WHERE name = ?`, name).Scan(&id)
	if err == sql.ErrNoRows {
		id = uuid.New().String()
		_, err = db.Exec(`INSERT INTO tags (id, name) VALUES (?, ?)`, id, name)
		return id, err
	}
	return id, err
}

func (db *DB) AddTagToExplanation(explanationID, tagID string) error {
	_, err := db.Exec(
		`INSERT OR IGNORE INTO explanation_tags (explanation_id, tag_id) VALUES (?, ?)`,
		explanationID, tagID,
	)
	return err
}

func (db *DB) RemoveTagFromExplanation(explanationID, tagName string) error {
	_, err := db.Exec(`
		DELETE FROM explanation_tags
		WHERE explanation_id = ? AND tag_id = (SELECT id FROM tags WHERE name = ?)`,
		explanationID, tagName,
	)
	return err
}

func (db *DB) DeleteTag(name string) error {
	_, err := db.Exec(`
		DELETE FROM explanation_tags
		WHERE tag_id = (SELECT id FROM tags WHERE name = ?)`, name)
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM tags WHERE name = ?`, name)
	return err
}

func splitTags(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}
