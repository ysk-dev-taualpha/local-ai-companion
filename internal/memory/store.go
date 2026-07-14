package memory

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Message struct {
	Role    string
	Content string
}

type ConversationTurn struct {
	UserText      string
	AssistantText string
	CreatedAt     time.Time
}

type Store struct {
	db        *sql.DB
	maxTurns  int
}

func NewStore(dbPath string, maxTurns int) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("memory store open: %w", err)
	}
	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("memory store init: %w", err)
	}
	if maxTurns <= 0 {
		maxTurns = 10
	}
	return &Store{db: db, maxTurns: maxTurns}, nil
}

func NewInMemoryStore(maxTurns int) (*Store, error) {
	return NewStore(":memory:", maxTurns)
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS session_history (
			session_id TEXT NOT NULL,
			turn       INTEGER NOT NULL,
			role       TEXT NOT NULL,
			content    TEXT NOT NULL,
			created_at TEXT NOT NULL,
			PRIMARY KEY (session_id, turn)
		);
		CREATE INDEX IF NOT EXISTS idx_session_history_created
			ON session_history(created_at);
	`)
	return err
}

func (s *Store) SaveMessage(sessionID, role, content string) error {
	var maxTurn int
	err := s.db.QueryRow(
		"SELECT COALESCE(MAX(turn), -1) FROM session_history WHERE session_id=?",
		sessionID,
	).Scan(&maxTurn)
	if err != nil {
		return err
	}

	nextTurn := maxTurn + 1
	_, err = s.db.Exec(
		"INSERT OR REPLACE INTO session_history VALUES(?,?,?,?,?)",
		sessionID, nextTurn, role, content, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return err
	}

	// Trim old turns beyond maxTurns
	cutoff := nextTurn - s.maxTurns
	if cutoff >= 0 {
		_, _ = s.db.Exec(
			"DELETE FROM session_history WHERE session_id=? AND turn <= ?",
			sessionID, cutoff,
		)
	}
	return nil
}

func (s *Store) LoadHistory(sessionID string) ([]Message, error) {
	rows, err := s.db.Query(
		"SELECT role, content FROM session_history WHERE session_id=? ORDER BY turn ASC",
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.Role, &m.Content); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (s *Store) SaveTurn(sessionID, userText, assistantText string) error {
	var maxTurn int
	err := s.db.QueryRow(
		"SELECT COALESCE(MAX(turn), -1) FROM session_history WHERE session_id=?",
		sessionID,
	).Scan(&maxTurn)
	if err != nil {
		return err
	}

	nextTurn := maxTurn/2 + 1
	now := time.Now().UTC().Format(time.RFC3339)

	// Store user at even turn numbers, assistant at odd turn numbers
	// to avoid primary key collision on (session_id, turn).
	userTurn := nextTurn * 2
	assistantTurn := nextTurn*2 + 1

	_, err = s.db.Exec(
		"INSERT OR REPLACE INTO session_history VALUES(?,?,?,?,?)",
		sessionID, userTurn, "user", userText, now,
	)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(
		"INSERT OR REPLACE INTO session_history VALUES(?,?,?,?,?)",
		sessionID, assistantTurn, "assistant", assistantText, now,
	)
	if err != nil {
		return err
	}

	// Trim: keep last maxTurns turns (each turn = 2 messages)
	cutoff := nextTurn - s.maxTurns
	if cutoff >= 0 {
		_, _ = s.db.Exec(
			"DELETE FROM session_history WHERE session_id=? AND turn <= ?",
			sessionID, cutoff*2+1,
		)
	}
	return nil
}

func (s *Store) LoadTurns(sessionID string) ([]ConversationTurn, error) {
	rows, err := s.db.Query(
		"SELECT role, content, created_at FROM session_history WHERE session_id=? ORDER BY turn ASC, role ASC",
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var turns []ConversationTurn
	var current *ConversationTurn
	for rows.Next() {
		var role, content, createdAt string
		if err := rows.Scan(&role, &content, &createdAt); err != nil {
			return nil, err
		}
		switch role {
		case "user":
			if current != nil {
				turns = append(turns, *current)
			}
			t, _ := time.Parse(time.RFC3339, createdAt)
			current = &ConversationTurn{UserText: content, CreatedAt: t}
		case "assistant":
			if current != nil {
				current.AssistantText = content
				turns = append(turns, *current)
				current = nil
			}
		}
	}
	if current != nil {
		turns = append(turns, *current)
	}
	return turns, rows.Err()
}

func (s *Store) CleanOldSessions(maxAge time.Duration) error {
	if maxAge <= 0 {
		_, err := s.db.Exec("DELETE FROM session_history")
		return err
	}
	cutoff := time.Now().UTC().Add(-maxAge).Format(time.RFC3339)
	_, err := s.db.Exec("DELETE FROM session_history WHERE created_at < ?", cutoff)
	return err
}

func (s *Store) Close() error {
	return s.db.Close()
}
