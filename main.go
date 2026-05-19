package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"heart-beat/alert"
	"heart-beat/db"
	"heart-beat/handler"
)

//go:embed static
var staticFiles embed.FS

func main() {
	listen := envOrDefault("LISTEN", ":51502")
	dbPath := envOrDefault("DB_PATH", "heartbeat.db")
	alertProvider := envOrDefault("ALERT_PROVIDER", "")    // "pushplus" or "serverchan"
	alertWebhook := envOrDefault("ALERT_WEBHOOK", "")       // token or sendkey

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	h := handler.New(database)

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("POST /api/heartbeat", h.PostHeartbeat)
	mux.HandleFunc("GET /api/status", h.GetStatus)
	mux.HandleFunc("GET /api/heartbeats", h.GetHeartbeats)
	mux.HandleFunc("GET /api/timeline", h.GetTimeline)

	// Static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	// Alert notifier
	if alertProvider != "" && alertWebhook != "" {
		n := alert.NewNotifier(database, alertProvider, alertWebhook)
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				n.Check()
			}
		}()
		log.Printf("alert enabled: provider=%s", alertProvider)
	} else {
		log.Println("alert disabled (set ALERT_PROVIDER and ALERT_WEBHOOK to enable)")
	}

	// Periodic cleanup: remove records older than 30 days
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			affected, err := database.CleanOldRecords(30 * 24 * time.Hour)
			if err != nil {
				log.Printf("cleanup error: %v", err)
			} else if affected > 0 {
				log.Printf("cleaned up %d old records", affected)
			}
		}
	}()

	fmt.Printf("heart-beat server listening on %s\n", listen)

	// Graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	server := &http.Server{Addr: listen, Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-quit
	fmt.Println("\nshutting down...")
	server.Close() // stop accepting new connections
	database.Close() // checkpoint WAL and close
	fmt.Println("stopped")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
