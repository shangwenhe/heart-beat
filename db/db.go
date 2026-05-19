package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Heartbeat struct {
	ID        int64     `json:"id"`
	SourceIP  string    `json:"source_ip"`
	ReceivedAt time.Time `json:"received_at"`
}

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("set wal mode: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &DB{db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS heartbeats (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			source_ip   TEXT    NOT NULL,
			received_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_heartbeats_received_at ON heartbeats(received_at);
	`)
	return err
}

func (db *DB) InsertHeartbeat(sourceIP string) error {
	_, err := db.Exec(
		"INSERT INTO heartbeats (source_ip, received_at) VALUES (?, ?)",
		sourceIP,
		time.Now().Unix(),
	)
	return err
}

func (db *DB) LatestHeartbeat() (*Heartbeat, error) {
	row := db.QueryRow(
		"SELECT id, source_ip, received_at FROM heartbeats ORDER BY received_at DESC LIMIT 1",
	)
	var h Heartbeat
	var ts int64
	if err := row.Scan(&h.ID, &h.SourceIP, &ts); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	h.ReceivedAt = time.Unix(ts, 0)
	return &h, nil
}

func (db *DB) ListHeartbeats(limit, offset int) ([]Heartbeat, error) {
	rows, err := db.Query(
		"SELECT id, source_ip, received_at FROM heartbeats ORDER BY received_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Heartbeat
	for rows.Next() {
		var h Heartbeat
		var ts int64
		if err := rows.Scan(&h.ID, &h.SourceIP, &ts); err != nil {
			return nil, err
		}
		h.ReceivedAt = time.Unix(ts, 0)
		list = append(list, h)
	}
	return list, rows.Err()
}

// CleanOldRecords removes records older than the given duration.
func (db *DB) CleanOldRecords(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan).Unix()
	res, err := db.Exec("DELETE FROM heartbeats WHERE received_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
