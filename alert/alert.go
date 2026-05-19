package alert

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"heart-beat/db"
)

type Notifier struct {
	db       *db.DB
	webhook  string // PushPlus webhook URL or Server酱 sendkey
	provider string // "pushplus" or "serverchan"
	lastAlert time.Time
	cooldown  time.Duration // alert cooldown, avoid spam
}

func NewNotifier(d *db.DB, provider, webhook string) *Notifier {
	return &Notifier{
		db:       d,
		webhook:  webhook,
		provider: provider,
		cooldown: 10 * time.Minute, // same alert at most once per 10min
	}
}

// Check runs a single check: if heartbeat is missing for >2min, send alert.
func (n *Notifier) Check() {
	last, err := n.db.LatestHeartbeat()
	if err != nil {
		log.Printf("[alert] query heartbeat error: %v", err)
		return
	}

	if last == nil {
		// No heartbeat ever received, nothing to alert about yet
		return
	}

	offline := time.Since(last.ReceivedAt) > 2*time.Minute

	if !offline {
		// Back online — if we were previously alerting, send recovery notice
		if !n.lastAlert.IsZero() && time.Since(n.lastAlert) < 30*time.Minute {
			n.send("房屋电力恢复通知", fmt.Sprintf("心跳已恢复，最近心跳时间: %s", last.ReceivedAt.Format("2006-01-02 15:04:05")))
			n.lastAlert = time.Time{} // reset
		}
		return
	}

	// Offline and cooldown passed
	if time.Since(n.lastAlert) < n.cooldown {
		return
	}

	msg := fmt.Sprintf("房屋可能停电！最近心跳时间: %s，已超时 %d 秒",
		last.ReceivedAt.Format("2006-01-02 15:04:05"),
		int(time.Since(last.ReceivedAt).Seconds()),
	)
	n.send("房屋停电告警", msg)
	n.lastAlert = time.Now()
}

func (n *Notifier) send(title, content string) {
	switch n.provider {
	case "pushplus":
		n.sendPushPlus(title, content)
	case "serverchan":
		n.sendServerChan(title, content)
	default:
		log.Printf("[alert] unknown provider %q, skipping", n.provider)
	}
}

// PushPlus: POST https://www.pushplus.plus/send
func (n *Notifier) sendPushPlus(title, content string) {
	resp, err := http.PostForm("https://www.pushplus.plus/send", url.Values{
		"token":    {n.webhook},
		"title":    {title},
		"content":  {content},
		"template": {"html"},
	})
	if err != nil {
		log.Printf("[alert] pushplus send error: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[alert] pushplus sent: %s - %s (HTTP %d)", title, content, resp.StatusCode)
}

// Server酱: POST https://sctapi.ftqq.com/{sendkey}.send
func (n *Notifier) sendServerChan(title, content string) {
	u := fmt.Sprintf("https://sctapi.ftqq.com/%s.send", n.webhook)
	resp, err := http.PostForm(u, url.Values{
		"title": {title},
		"desp":  {content},
	})
	if err != nil {
		log.Printf("[alert] serverchan send error: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[alert] serverchan sent: %s - %s (HTTP %d)", title, content, resp.StatusCode)
}
