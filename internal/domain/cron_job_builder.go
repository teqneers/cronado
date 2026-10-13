package domain

import (
	"fmt"
	"strings"
	"time"
)

// trimOuterQuotes removes a matching pair of outer quotes (" or ') from s.
// Unlike strings.Trim, it only removes quotes when both the first and last
// character are the same quote type, preserving quotes inside the value.
func trimOuterQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// CronJobBuilder helps build a cron job from labels with validation
type CronJobBuilder struct {
	name                string
	container           *Container
	enabled             *bool
	schedule            string
	command             string
	user                string
	timeout             time.Duration
	pingURL             string
	minScheduleInterval time.Duration
	maxTimeout          time.Duration
	hasErrors           bool
	errors              []string
}

// NewCronJobBuilder creates a new builder for the given job name
func NewCronJobBuilder(name string, container *Container, minScheduleInterval, maxTimeout time.Duration) *CronJobBuilder {
	return &CronJobBuilder{
		name:                name,
		container:           container,
		timeout:             DefaultTimeout,
		minScheduleInterval: minScheduleInterval,
		maxTimeout:          maxTimeout,
	}
}

// SetEnabled sets the enabled state with flexible boolean parsing
func (b *CronJobBuilder) SetEnabled(value string) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		enabled := true
		b.enabled = &enabled
	case "false", "0", "no", "off", "":
		enabled := false
		b.enabled = &enabled
	default:
		b.addError(fmt.Sprintf("invalid enabled value: %s (expected true/false)", value))
	}
}

// SetSchedule sets the cron schedule with validation
func (b *CronJobBuilder) SetSchedule(schedule string) {
	schedule = strings.TrimSpace(schedule)
	schedule = trimOuterQuotes(schedule)
	if schedule == "" {
		b.addError("schedule cannot be empty")
		return
	}

	if strings.HasPrefix(schedule, "@every ") {
		// Validate interval duration and enforce minimum
		durationStr := strings.TrimPrefix(schedule, "@every ")
		d, err := time.ParseDuration(durationStr)
		if err != nil {
			b.addError(fmt.Sprintf("invalid @every duration: %s", durationStr))
			return
		}
		if b.minScheduleInterval > 0 && d < b.minScheduleInterval {
			b.addError(fmt.Sprintf("schedule interval %s is below minimum %s", d, b.minScheduleInterval))
			return
		}
	}

	// Parse with the scheduler's own parser so that every accepted schedule
	// can actually be registered.
	parsed, err := cronParser.Parse(schedule)
	if err != nil {
		b.addError(fmt.Sprintf("invalid cron schedule %q: %v", schedule, err))
		return
	}

	// A seconds field allows sub-minute schedules, so check the real gap
	// between runs against the configured minimum.
	if b.minScheduleInterval > 0 {
		if gap := shortestInterval(parsed, time.Now()); gap > 0 && gap < b.minScheduleInterval {
			b.addError(fmt.Sprintf("schedule %q runs every %s, below minimum %s", schedule, gap, b.minScheduleInterval))
			return
		}
	}

	b.schedule = schedule
}

// SetCommand sets the command with validation
func (b *CronJobBuilder) SetCommand(command string) {
	command = strings.TrimSpace(command)
	command = trimOuterQuotes(command)
	if command == "" {
		b.addError("command cannot be empty")
		return
	}
	b.command = command
}

// SetUser sets the user the command runs as. An empty value keeps the
// default, which is the container's own configured user.
func (b *CronJobBuilder) SetUser(user string) {
	b.user = strings.TrimSpace(user)
}

// SetTimeout sets the command execution timeout (e.g. "30s", "5m", "1h")
func (b *CronJobBuilder) SetTimeout(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return // keep default
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		b.addError(fmt.Sprintf("invalid timeout value: %s (expected duration like 30s, 5m, 1h)", value))
		return
	}
	if d <= 0 {
		b.addError("timeout must be positive")
		return
	}
	if b.maxTimeout > 0 && d > b.maxTimeout {
		b.addError(fmt.Sprintf("timeout %s exceeds maximum %s", d, b.maxTimeout))
		return
	}
	b.timeout = d
}

// SetPingURL sets a Healthchecks.io-compatible URL that is pinged when the job
// starts, succeeds or fails.
func (b *CronJobBuilder) SetPingURL(value string) {
	value = trimOuterQuotes(strings.TrimSpace(value))
	if value == "" {
		return
	}
	if err := validatePingURL(value); err != nil {
		b.addError(err.Error())
		return
	}
	b.pingURL = value
}

// addError adds an error to the builder's error collection
func (b *CronJobBuilder) addError(err string) {
	b.hasErrors = true
	b.errors = append(b.errors, err)
}

// IsValid checks if the builder has all required fields and no errors
func (b *CronJobBuilder) IsValid() bool {
	if b.hasErrors {
		return false
	}
	if b.enabled == nil {
		b.addError("enabled state not set")
	}
	if b.schedule == "" {
		b.addError("schedule not set")
	}
	if b.command == "" {
		b.addError("command not set")
	}
	return !b.hasErrors
}

// GetErrors returns all validation errors
func (b *CronJobBuilder) GetErrors() []string {
	return append([]string{}, b.errors...) // Return a copy
}

// Build creates the CronJob if valid, returns error with details if invalid
func (b *CronJobBuilder) Build() (*CronJob, error) {
	if !b.IsValid() {
		return nil, fmt.Errorf("invalid cron job %s: %s", b.name, strings.Join(b.errors, "; "))
	}

	return &CronJob{
		ID:          fmt.Sprintf("%s-%s", b.container.ShortID(), b.name),
		Name:        b.name,
		Container:   b.container,
		Enabled:     *b.enabled,
		Schedule:    b.schedule,
		Command:     b.command,
		User:        b.user,
		Timeout:     b.timeout,
		PingURL:     b.pingURL,
		Status:      StatusIdle,
		SchedulerId: -1,
	}, nil
}

// GetName returns the job name
func (b *CronJobBuilder) GetName() string {
	return b.name
}

// GetContainer returns the associated container
func (b *CronJobBuilder) GetContainer() *Container {
	return b.container
}

// HasErrors returns true if the builder has accumulated any errors
func (b *CronJobBuilder) HasErrors() bool {
	return b.hasErrors
}

// Reset clears all errors and resets the builder to initial state
func (b *CronJobBuilder) Reset() {
	b.enabled = nil
	b.schedule = ""
	b.command = ""
	b.user = ""
	b.timeout = DefaultTimeout
	b.pingURL = ""
	b.hasErrors = false
	b.errors = []string{}
}
