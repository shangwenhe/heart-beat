package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"heart-beat/db"
)

type Handler struct {
	db *db.DB
}

func New(d *db.DB) *Handler {
	return &Handler{db: d}
}

func (h *Handler) PostHeartbeat(w http.ResponseWriter, r *http.Request) {
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}

	if err := h.db.InsertHeartbeat(ip); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type StatusResponse struct {
	Online        bool       `json:"online"`
	LastHeartbeat *db.Heartbeat `json:"last_heartbeat"`
	CheckedAt     time.Time  `json:"checked_at"`
}

func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	last, err := h.db.LatestHeartbeat()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	online := false
	if last != nil {
		online = time.Since(last.ReceivedAt) < 2*time.Minute
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StatusResponse{
		Online:        online,
		LastHeartbeat: last,
		CheckedAt:     time.Now(),
	})
}

func (h *Handler) GetHeartbeats(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	list, err := h.db.ListHeartbeats(limit, offset)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"list":  list,
		"count": len(list),
	})
}
