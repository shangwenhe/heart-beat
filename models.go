package main

import "time"

type Child struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

type Subject struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Teacher   string    `json:"teacher"`
	Color     string    `json:"color,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

type ScheduleItem struct {
	ID        int64     `json:"id"`
	DayOfWeek int       `json:"dayOfWeek"` // 1=周一, 2=周二, ..., 5=周五
	Period    int       `json:"period"`    // 第几节课 1-8
	SubjectID int64     `json:"subjectId"`
	ChildID   int64     `json:"childId"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	// 关联数据
	Subject *Subject `json:"subject,omitempty"`
	Child   *Child   `json:"child,omitempty"`
}

// 日程/活动
type Activity struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`      // 日程标题
	Content   string    `json:"content"`     // 日程内容
	StartTime time.Time `json:"startTime"`   // 开始时间
	EndTime   time.Time `json:"endTime"`     // 结束时间
	ChildID   int64     `json:"childId"`     // 归属孩子
	CreatedAt time.Time `json:"createdAt,omitempty"`
	Child     *Child    `json:"child,omitempty"`
}

type SwapRequest struct {
	Item1ID int64 `json:"item1Id"`
	Item2ID int64 `json:"item2Id"`
}

type ScheduleGrid struct {
	Children []Child                   `json:"children"`
	Subjects []Subject                 `json:"subjects"`
	Items    []ScheduleItem            `json:"items"`
	Periods  []Period                  `json:"periods"`
}

type Period struct {
	Index  int    `json:"index"`
	Start  string `json:"start"`
	End    string `json:"end"`
	Name   string `json:"name"`
}

var DefaultPeriods = []Period{
	{Index: 1, Start: "08:00", End: "08:40", Name: "第一节"},
	{Index: 2, Start: "08:50", End: "09:30", Name: "第二节"},
	{Index: 3, Start: "09:40", End: "10:20", Name: "第三节"},
	{Index: 4, Start: "10:30", End: "11:10", Name: "第四节"},
	{Index: 5, Start: "11:20", End: "12:00", Name: "第五节"},
	{Index: 6, Start: "14:00", End: "14:40", Name: "第六节"},
	{Index: 7, Start: "14:50", End: "15:30", Name: "第七节"},
	{Index: 8, Start: "15:40", End: "16:20", Name: "第八节"},
}
