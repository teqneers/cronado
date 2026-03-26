package notification

import (
	"log/slog"
	"sync"
	"github.com/teqneers/cronado/internal/context"
	"time"
)

const (
	SUBJECT_DAEMON_WATCHER = "Docker Daemon Failed"
)

type LastSend struct {
	list map[string]int64
	mu   sync.Mutex
}

func (l *LastSend) Get(subject string) (int64, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	last, ok := l.list[subject]
	return last, ok
}

func (l *LastSend) Set(subject string, ts int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list[subject] = ts
}

// LastSend keeps track of the last sent notification timestamps to throttle duplicate notifications.
var lastSent = LastSend{list: make(map[string]int64)}

// Notify sends a notification with the given subject and body via all enabled channels.
// It uses the IntervalSeconds from notification config to throttle duplicate notifications.
func Notify(subject, body string) {
	cfg := context.AppCtx.Config.Notify

	interval := cfg.IntervalSeconds
	ts := timeNowUnix()

	if last, ok := lastSent.Get(subject); ok {
		if ts-last < int64(interval) {
			nextSendDate := time.Unix(last+int64(interval), 0)
			slog.Debug("notification: skipping duplicate notification", "subject", subject, "interval", interval, "next", nextSendDate)
			return
		}
	}
	// update timestamp
	lastSent.Set(subject, ts)

	if cfg.Email.Enabled {
		if err := SendEmail(subject, body); err != nil {
			slog.Error("notification: email send failed", "error", err)
		}
	}

	if cfg.Ntfy.Enabled {
		if err := SendNtfy(subject, body); err != nil {
			slog.Error("notification: ntfy send failed", "error", err)
		}
	}
}

// timeNowUnix returns current time as Unix seconds. Abstracted for testing.
var timeNowUnix = func() int64 {
	return time.Now().Unix()
}
