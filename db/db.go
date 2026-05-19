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
	// Auto-checkpoint every 1000 frames to keep WAL file small
	if _, err := db.Exec("PRAGMA wal_autocheckpoint=1000"); err != nil {
		return nil, fmt.Errorf("set wal autocheckpoint: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &DB{db}, nil
}

// Close truncates WAL and shuts down cleanly.
func (db *DB) Close() error {
	db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return db.DB.Close()
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

// TimelineSlot represents a 5-minute aggregated slot.
type TimelineSlot struct {
	Hour   int    `json:"hour"`    // 0-23
	Minute int    `json:"minute"`  // 0,5,10,...,55
	Count  int    `json:"count"`   // heartbeat count in this slot
	MaxGap int    `json:"max_gap"` // max seconds without heartbeat in this slot
	Status string `json:"status"` // "online", "warning", "offline", "future"
}

const slotMinutes = 5
const slotsPerHour = 60 / slotMinutes // 12
const totalSlots = 24 * slotsPerHour   // 288

// Timeline returns 288 slots (5-min each) for a given date (format: "2006-01-02").
// Arranged as 24 columns (hours) × 12 rows (5-min intervals within each hour).
//
// Status logic:
//   - green/online:  has heartbeat, max_gap <= 90s
//   - yellow/warning: has heartbeat, max_gap > 90s (short outage within slot)
//   - red/offline:   no heartbeat at all (at least 5-min outage)
func (db *DB) Timeline(date string) ([]TimelineSlot, error) {
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %w", err)
	}
	loc := time.Local
	startOfDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	endOfDay := startOfDay.Add(24 * time.Hour)

	rows, err := db.Query(
		"SELECT received_at FROM heartbeats WHERE received_at >= ? AND received_at < ? ORDER BY received_at",
		startOfDay.Unix(), endOfDay.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Collect timestamps per slot [hour][row]
	slotTimes := make([][]time.Time, totalSlots)
	for rows.Next() {
		var ts int64
		if err := rows.Scan(&ts); err != nil {
			return nil, err
		}
		tt := time.Unix(ts, 0).In(loc)
		hour := tt.Hour()
		row := tt.Minute() / slotMinutes
		idx := hour*slotsPerHour + row
		if idx >= 0 && idx < totalSlots {
			slotTimes[idx] = append(slotTimes[idx], tt)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	now := time.Now()
	slots := make([]TimelineSlot, totalSlots)
	for i := 0; i < totalSlots; i++ {
		hour := i / slotsPerHour
		minute := (i % slotsPerHour) * slotMinutes
		slotStart := startOfDay.Add(time.Duration(i*slotMinutes) * time.Minute)

		slots[i] = TimelineSlot{
			Hour:   hour,
			Minute: minute,
			Count:  len(slotTimes[i]),
		}

		// Future slot
		if slotStart.After(now) {
			slots[i].Status = "future"
			continue
		}

		// No heartbeat → offline (red)
		if len(slotTimes[i]) == 0 {
			slots[i].MaxGap = slotMinutes * 60
			slots[i].Status = "offline"
			continue
		}

		// Compute max gap within this slot
		maxGap := 0.0
		if g := slotTimes[i][0].Sub(slotStart).Seconds(); g > maxGap {
			maxGap = g
		}
		for j := 1; j < len(slotTimes[i]); j++ {
			if g := slotTimes[i][j].Sub(slotTimes[i][j-1]).Seconds(); g > maxGap {
				maxGap = g
			}
		}
		slotEnd := slotStart.Add(time.Duration(slotMinutes) * time.Minute)
		boundary := now
		if slotEnd.Before(now) {
			boundary = slotEnd
		}
		if g := boundary.Sub(slotTimes[i][len(slotTimes[i])-1]).Seconds(); g > maxGap {
			maxGap = g
		}

		slots[i].MaxGap = int(maxGap)
		switch {
		case maxGap > 90:
			slots[i].Status = "warning"
		default:
			slots[i].Status = "online"
		}
	}

	return slots, nil
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
