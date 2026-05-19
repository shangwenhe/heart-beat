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

// TimelineSlot represents a 30-minute aggregated slot.
type TimelineSlot struct {
	SlotIndex int    `json:"slot"`  // 0-47 (0=00:00, 1=00:30, ..., 47=23:30)
	Hour      int    `json:"hour"`
	Minute    int    `json:"minute"`
	Count     int    `json:"count"` // heartbeat count in this slot
	MaxGap    int    `json:"max_gap"` // max seconds without heartbeat in this slot
	Status    string `json:"status"` // "online", "warning", "offline", "future"
}

// Timeline returns 48 slots (30-min each) for a given date (format: "2006-01-02").
// Status logic:
//   - green/online:  no gap > 90s (all heartbeats present)
//   - yellow/warning: gap > 90s but < 15min (short outage)
//   - red/offline:   gap >= 15min (long outage) or no heartbeat at all
func (db *DB) Timeline(date string) ([]TimelineSlot, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %w", err)
	}
	loc := t.Location()
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

	// Collect timestamps per slot
	slotTimes := make([][]time.Time, 48)
	for rows.Next() {
		var ts int64
		if err := rows.Scan(&ts); err != nil {
			return nil, err
		}
		tt := time.Unix(ts, 0).In(loc)
		slot := (tt.Hour()*60 + tt.Minute()) / 30
		if slot >= 0 && slot < 48 {
			slotTimes[slot] = append(slotTimes[slot], tt)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	now := time.Now()
	slots := make([]TimelineSlot, 48)
	for i := 0; i < 48; i++ {
		hour := (i * 30) / 60
		minute := (i * 30) % 60
		slotStart := startOfDay.Add(time.Duration(i*30) * time.Minute)

		slots[i] = TimelineSlot{
			SlotIndex: i,
			Hour:      hour,
			Minute:    minute,
			Count:     len(slotTimes[i]),
		}

		// Future slot
		if slotStart.After(now) {
			slots[i].Status = "future"
			continue
		}

		// No heartbeat at all → offline (red)
		if len(slotTimes[i]) == 0 {
			slots[i].MaxGap = 30 * 60 // entire 30-min slot is a gap
			slots[i].Status = "offline"
			continue
		}

		// Compute max gap within this slot
		maxGap := 0.0

		// Gap from slot start to first heartbeat
		if g := slotTimes[i][0].Sub(slotStart).Seconds(); g > maxGap {
			maxGap = g
		}

		// Gap between consecutive heartbeats
		for j := 1; j < len(slotTimes[i]); j++ {
			if g := slotTimes[i][j].Sub(slotTimes[i][j-1]).Seconds(); g > maxGap {
				maxGap = g
			}
		}

		// Gap from last heartbeat to slot end (or now, whichever is earlier)
		slotEnd := slotStart.Add(30 * time.Minute)
		boundary := now
		if slotEnd.Before(now) {
			boundary = slotEnd
		}
		if g := boundary.Sub(slotTimes[i][len(slotTimes[i])-1]).Seconds(); g > maxGap {
			maxGap = g
		}

		slots[i].MaxGap = int(maxGap)

		// Determine status by max gap
		switch {
		case maxGap >= 15*60:
			slots[i].Status = "offline" // red: outage >= 15min
		case maxGap > 90:
			slots[i].Status = "warning" // yellow: short outage
		default:
			slots[i].Status = "online" // green: all good
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
