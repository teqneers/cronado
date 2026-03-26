package notification

import (
	"testing"
	"time"

	"github.com/teqneers/cronado/internal/config"
	"github.com/teqneers/cronado/internal/context"
)

func setupTestContext(emailEnabled, ntfyEnabled bool, intervalSeconds int) {
	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				IntervalSeconds: intervalSeconds,
				Email: config.EmailConfig{
					Enabled: emailEnabled,
				},
				Ntfy: config.NtfyConfig{
					Enabled: ntfyEnabled,
				},
			},
		},
	}
}

func TestLastSend_GetSet(t *testing.T) {
	ls := LastSend{list: make(map[string]int64)}

	// Initially not found
	_, ok := ls.Get("test")
	if ok {
		t.Error("expected not found for new key")
	}

	// Set and get
	ls.Set("test", 12345)
	val, ok := ls.Get("test")
	if !ok {
		t.Error("expected found after set")
	}
	if val != 12345 {
		t.Errorf("expected 12345, got %d", val)
	}

	// Overwrite
	ls.Set("test", 99999)
	val, ok = ls.Get("test")
	if !ok {
		t.Error("expected found after overwrite")
	}
	if val != 99999 {
		t.Errorf("expected 99999, got %d", val)
	}
}

func TestLastSend_ConcurrentAccess(t *testing.T) {
	ls := LastSend{list: make(map[string]int64)}

	done := make(chan bool, 20)
	for i := 0; i < 10; i++ {
		go func(i int) {
			ls.Set("key", int64(i))
			done <- true
		}(i)
		go func() {
			ls.Get("key")
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestNotify_Throttling(t *testing.T) {
	// Reset lastSent for test isolation
	lastSent = LastSend{list: make(map[string]int64)}

	// Set up context with both notifications disabled (so we only test throttling)
	setupTestContext(false, false, 60)

	// Mock time to control throttling
	currentTime := int64(1000)
	originalTimeNow := timeNowUnix
	timeNowUnix = func() int64 { return currentTime }
	defer func() { timeNowUnix = originalTimeNow }()

	// First call should go through
	Notify("test subject", "test body")

	// Verify timestamp was recorded
	ts, ok := lastSent.Get("test subject")
	if !ok {
		t.Error("expected timestamp to be recorded")
	}
	if ts != 1000 {
		t.Errorf("expected timestamp 1000, got %d", ts)
	}

	// Second call within interval should be throttled
	currentTime = 1030 // 30 seconds later, within 60s interval
	Notify("test subject", "test body")

	// Timestamp should not have changed (throttled)
	ts, _ = lastSent.Get("test subject")
	if ts != 1000 {
		t.Errorf("expected timestamp to remain 1000 (throttled), got %d", ts)
	}

	// Third call after interval should go through
	currentTime = 1061 // 61 seconds after first call
	Notify("test subject", "test body")

	ts, _ = lastSent.Get("test subject")
	if ts != 1061 {
		t.Errorf("expected timestamp 1061 (new notification), got %d", ts)
	}
}

func TestNotify_DifferentSubjectsNotThrottled(t *testing.T) {
	lastSent = LastSend{list: make(map[string]int64)}
	setupTestContext(false, false, 60)

	currentTime := int64(1000)
	originalTimeNow := timeNowUnix
	timeNowUnix = func() int64 { return currentTime }
	defer func() { timeNowUnix = originalTimeNow }()

	Notify("subject A", "body A")
	Notify("subject B", "body B")

	tsA, okA := lastSent.Get("subject A")
	tsB, okB := lastSent.Get("subject B")

	if !okA || !okB {
		t.Error("expected both timestamps to be recorded")
	}
	if tsA != 1000 || tsB != 1000 {
		t.Errorf("expected both timestamps to be 1000, got A=%d B=%d", tsA, tsB)
	}
}

func TestTimeNowUnix(t *testing.T) {
	now := timeNowUnix()
	actual := time.Now().Unix()

	// Allow 1 second tolerance
	if now < actual-1 || now > actual+1 {
		t.Errorf("timeNowUnix() = %d, want approximately %d", now, actual)
	}
}
