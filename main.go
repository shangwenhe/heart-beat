package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	if err := initDB("./schedule.db"); err != nil {
		log.Fatal("数据库初始化失败:", err)
	}
	defer db.Close()

	if err := InitDefaultData(); err != nil {
		log.Println("初始化默认数据失败:", err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
	}))

	r.Static("/static", "./static")
	r.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	// Children API
	r.GET("/api/children", func(c *gin.Context) {
		children, err := GetChildren()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, children)
	})

	r.POST("/api/children", func(c *gin.Context) {
		var req struct {
			Name  string `json:"name" binding:"required"`
			Color string `json:"color" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		child, err := CreateChild(req.Name, req.Color)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, child)
	})

	r.PUT("/api/children/:id", func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Name  string `json:"name" binding:"required"`
			Color string `json:"color" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := UpdateChild(int64(mustInt(id)), req.Name, req.Color); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	r.DELETE("/api/children/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := DeleteChild(int64(mustInt(id))); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// Subjects API
	r.GET("/api/subjects", func(c *gin.Context) {
		subjects, err := GetSubjects()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, subjects)
	})

	r.POST("/api/subjects", func(c *gin.Context) {
		var req struct {
			Name    string `json:"name" binding:"required"`
			Teacher string `json:"teacher"`
			Color   string `json:"color"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		subject, err := CreateSubject(req.Name, req.Teacher, req.Color)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, subject)
	})

	r.PUT("/api/subjects/:id", func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Name    string `json:"name" binding:"required"`
			Teacher string `json:"teacher"`
			Color   string `json:"color"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := UpdateSubject(int64(mustInt(id)), req.Name, req.Teacher, req.Color); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	r.DELETE("/api/subjects/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := DeleteSubject(int64(mustInt(id))); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// Schedule API
	r.GET("/api/schedule", func(c *gin.Context) {
		log.Printf("[GET /api/schedule] 请求课程表数据")
		items, err := GetSchedule()
		if err != nil {
			log.Printf("[GET /api/schedule] ERROR: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		children, _ := GetChildren()
		subjects, _ := GetSubjects()
		activities, _ := GetActivities()
		log.Printf("[GET /api/schedule] 成功: children=%d, subjects=%d, items=%d, activities=%d", len(children), len(subjects), len(items), len(activities))
		c.JSON(http.StatusOK, gin.H{
			"children":   children,
			"subjects":   subjects,
			"items":      items,
			"periods":    DefaultPeriods,
			"activities": activities,
		})
	})

	r.POST("/api/schedule", func(c *gin.Context) {
		var item ScheduleItem
		if err := c.ShouldBindJSON(&item); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		created, err := CreateScheduleItem(item.DayOfWeek, item.Period, item.SubjectID, item.ChildID, item.Notes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, created)
	})

	r.PUT("/api/schedule/:id", func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			SubjectID int64  `json:"subjectId"`
			ChildID   int64  `json:"childId"`
			Notes     string `json:"notes"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := UpdateScheduleItem(int64(mustInt(id)), req.SubjectID, req.ChildID, req.Notes); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	r.DELETE("/api/schedule/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := DeleteScheduleItem(int64(mustInt(id))); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	r.POST("/api/schedule/swap", func(c *gin.Context) {
		var req SwapRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := SwapScheduleItems(req.Item1ID, req.Item2ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// Activities API
	r.GET("/api/activities", func(c *gin.Context) {
		activities, err := GetActivities()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, activities)
	})

	r.POST("/api/activities", func(c *gin.Context) {
		var req struct {
			Title     string `json:"title" binding:"required"`
			Content   string `json:"content"`
			StartTime string `json:"startTime" binding:"required"`
			EndTime   string `json:"endTime" binding:"required"`
			ChildID   int64  `json:"childId" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		startTime, _ := time.Parse("2006-01-02T15:04", req.StartTime)
		endTime, _ := time.Parse("2006-01-02T15:04", req.EndTime)
		activity, err := CreateActivity(req.Title, req.Content, startTime, endTime, req.ChildID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, activity)
	})

	r.PUT("/api/activities/:id", func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Title     string `json:"title" binding:"required"`
			Content   string `json:"content"`
			StartTime string `json:"startTime" binding:"required"`
			EndTime   string `json:"endTime" binding:"required"`
			ChildID   int64  `json:"childId" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		startTime, _ := time.Parse("2006-01-02T15:04", req.StartTime)
		endTime, _ := time.Parse("2006-01-02T15:04", req.EndTime)
		if err := UpdateActivity(int64(mustInt(id)), req.Title, req.Content, startTime, endTime, req.ChildID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	r.DELETE("/api/activities/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := DeleteActivity(int64(mustInt(id))); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	log.Println("课程表服务启动: http://localhost:8080")
	r.Run(":8080")
}

func mustInt(s string) int {
	var n int
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
