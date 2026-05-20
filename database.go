package main

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func initDB(dbPath string) error {
	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err = createTables(); err != nil {
		return err
	}

	return nil
}

func createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS children (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			color TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS subjects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			teacher TEXT DEFAULT '',
			color TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS schedule (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			day_of_week INTEGER NOT NULL,
			period INTEGER NOT NULL,
			subject_id INTEGER REFERENCES subjects(id),
			child_id INTEGER REFERENCES children(id),
			notes TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(day_of_week, period, child_id)
		)`,
		`CREATE TABLE IF NOT EXISTS activities (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT DEFAULT '',
			start_time DATETIME NOT NULL,
			end_time DATETIME NOT NULL,
			child_id INTEGER REFERENCES children(id),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// Children CRUD
func GetChildren() ([]Child, error) {
	rows, err := db.Query("SELECT id, name, color, created_at FROM children ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var children []Child
	for rows.Next() {
		var c Child
		if err := rows.Scan(&c.ID, &c.Name, &c.Color, &c.CreatedAt); err != nil {
			return nil, err
		}
		children = append(children, c)
	}
	return children, nil
}

func CreateChild(name, color string) (*Child, error) {
	result, err := db.Exec("INSERT INTO children (name, color) VALUES (?, ?)", name, color)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &Child{ID: id, Name: name, Color: color}, nil
}

func UpdateChild(id int64, name, color string) error {
	_, err := db.Exec("UPDATE children SET name=?, color=? WHERE id=?", name, color, id)
	return err
}

func DeleteChild(id int64) error {
	_, err := db.Exec("DELETE FROM children WHERE id=?", id)
	return err
}

// Subjects CRUD
func GetSubjects() ([]Subject, error) {
	rows, err := db.Query("SELECT id, name, teacher, color FROM subjects ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subjects []Subject
	for rows.Next() {
		var s Subject
		if err := rows.Scan(&s.ID, &s.Name, &s.Teacher, &s.Color); err != nil {
			return nil, err
		}
		subjects = append(subjects, s)
	}
	return subjects, nil
}

func CreateSubject(name, teacher, color string) (*Subject, error) {
	result, err := db.Exec("INSERT INTO subjects (name, teacher, color) VALUES (?, ?, ?)", name, teacher, color)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &Subject{ID: id, Name: name, Teacher: teacher, Color: color}, nil
}

func UpdateSubject(id int64, name, teacher, color string) error {
	_, err := db.Exec("UPDATE subjects SET name=?, teacher=?, color=? WHERE id=?", name, teacher, color, id)
	return err
}

func DeleteSubject(id int64) error {
	_, err := db.Exec("DELETE FROM subjects WHERE id=?", id)
	return err
}

// Schedule CRUD
func GetSchedule() ([]ScheduleItem, error) {
	query := `SELECT s.id, s.day_of_week, s.period, s.subject_id, s.child_id, s.notes, s.created_at,
		sub.id, sub.name, sub.teacher, sub.color, c.id, c.name, c.color
		FROM schedule s
		LEFT JOIN subjects sub ON s.subject_id = sub.id
		LEFT JOIN children c ON s.child_id = c.id
		ORDER BY s.day_of_week, s.period`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ScheduleItem
	for rows.Next() {
		var item ScheduleItem
		var subject Subject
		var child Child
		var subjectColor, childColor sql.NullString
		var childCreatedAt sql.NullTime

		if err := rows.Scan(
			&item.ID, &item.DayOfWeek, &item.Period, &item.SubjectID, &item.ChildID, &item.Notes, &item.CreatedAt,
			&subject.ID, &subject.Name, &subject.Teacher, &subjectColor,
			&child.ID, &child.Name, &childColor,
		); err != nil {
			return nil, err
		}

		if subjectColor.Valid {
			subject.Color = subjectColor.String
		}
		if childColor.Valid {
			child.Color = childColor.String
		}
		if childCreatedAt.Valid {
			child.CreatedAt = childCreatedAt.Time
		}

		item.Subject = &subject
		item.Child = &child
		items = append(items, item)
	}
	return items, nil
}

func CreateScheduleItem(dayOfWeek, period int, subjectID, childID int64, notes string) (*ScheduleItem, error) {
	result, err := db.Exec(
		"INSERT INTO schedule (day_of_week, period, subject_id, child_id, notes) VALUES (?, ?, ?, ?, ?)",
		dayOfWeek, period, subjectID, childID, notes,
	)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &ScheduleItem{
		ID: id, DayOfWeek: dayOfWeek, Period: period,
		SubjectID: subjectID, ChildID: childID, Notes: notes,
		CreatedAt: time.Now(),
	}, nil
}

func UpdateScheduleItem(id int64, subjectID, childID int64, notes string) error {
	_, err := db.Exec(
		"UPDATE schedule SET subject_id=?, child_id=?, notes=? WHERE id=?",
		subjectID, childID, notes, id,
	)
	return err
}

func DeleteScheduleItem(id int64) error {
	_, err := db.Exec("DELETE FROM schedule WHERE id=?", id)
	return err
}

func SwapScheduleItems(id1, id2 int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var day1, period1, child1, subject1 int64
	var notes1 string
	var day2, period2, child2, subject2 int64
	var notes2 string

	row1 := tx.QueryRow("SELECT day_of_week, period, child_id, subject_id, notes FROM schedule WHERE id=?", id1)
	if err := row1.Scan(&day1, &period1, &child1, &subject1, &notes1); err != nil {
		return err
	}

	row2 := tx.QueryRow("SELECT day_of_week, period, child_id, subject_id, notes FROM schedule WHERE id=?", id2)
	if err := row2.Scan(&day2, &period2, &child2, &subject2, &notes2); err != nil {
		return err
	}

	if _, err := tx.Exec("UPDATE schedule SET day_of_week=?, period=?, child_id=?, subject_id=?, notes=? WHERE id=?",
		day2, period2, child1, subject2, notes2, id1); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE schedule SET day_of_week=?, period=?, child_id=?, subject_id=?, notes=? WHERE id=?",
		day1, period1, child2, subject1, notes1, id2); err != nil {
		return err
	}

	return tx.Commit()
}

// Activities CRUD
func GetActivities() ([]Activity, error) {
	query := `SELECT a.id, a.title, a.content, a.start_time, a.end_time, a.child_id, a.created_at,
		c.id, c.name, c.color
		FROM activities a
		LEFT JOIN children c ON a.child_id = c.id
		ORDER BY a.start_time`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []Activity
	for rows.Next() {
		var a Activity
		var child Child
		var childColor sql.NullString

		if err := rows.Scan(
			&a.ID, &a.Title, &a.Content, &a.StartTime, &a.EndTime, &a.ChildID, &a.CreatedAt,
			&child.ID, &child.Name, &childColor,
		); err != nil {
			return nil, err
		}

		if childColor.Valid {
			child.Color = childColor.String
		}
		a.Child = &child
		activities = append(activities, a)
	}
	return activities, nil
}

func CreateActivity(title, content string, startTime, endTime time.Time, childID int64) (*Activity, error) {
	result, err := db.Exec(
		"INSERT INTO activities (title, content, start_time, end_time, child_id) VALUES (?, ?, ?, ?, ?)",
		title, content, startTime, endTime, childID,
	)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &Activity{
		ID: id, Title: title, Content: content,
		StartTime: startTime, EndTime: endTime, ChildID: childID,
		CreatedAt: time.Now(),
	}, nil
}

func UpdateActivity(id int64, title, content string, startTime, endTime time.Time, childID int64) error {
	_, err := db.Exec(
		"UPDATE activities SET title=?, content=?, start_time=?, end_time=?, child_id=? WHERE id=?",
		title, content, startTime, endTime, childID, id,
	)
	return err
}

func DeleteActivity(id int64) error {
	_, err := db.Exec("DELETE FROM activities WHERE id=?", id)
	return err
}

// InitDefaultData 初始化默认数据
func InitDefaultData() error {
	children, _ := GetChildren()
	if len(children) > 0 {
		return nil // 已有数据
	}

	// 添加两个孩子
	_, err := db.Exec("INSERT INTO children (name, color) VALUES ('姐姐', '#f472b6')")
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO children (name, color) VALUES ('妹妹', '#38bdf8')")
	if err != nil {
		return err
	}

	// 添加默认课程
	subjects := []struct {
		name    string
		teacher string
	}{
		{"语文", "张老师"},
		{"数学", "李老师"},
		{"英语", "王老师"},
		{"音乐", "刘老师"},
		{"体育", "赵老师"},
		{"美术", "陈老师"},
		{"科学", "周老师"},
		{"道德与法治", "吴老师"},
	}

	for _, s := range subjects {
		_, err := db.Exec("INSERT INTO subjects (name, teacher) VALUES (?, ?)", s.name, s.teacher)
		if err != nil {
			return err
		}
	}

	return nil
}
