package domain

import (
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// cronParser accepts standard 5-field expressions, an optional leading seconds
// field (6 fields), descriptors like @daily/@every and the CRON_TZ= prefix.
// The builder and the scheduler must use the same parser so that every
// schedule that passes validation can also be registered.
var cronParser = cron.NewParser(
	cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// intervalSamples is the number of consecutive activations inspected when
// computing the smallest gap of a schedule.
const intervalSamples = 1000

// shortestInterval returns the smallest gap between consecutive activations of
// schedule within the next intervalSamples runs starting at from. It returns 0
// if the schedule never fires.
func shortestInterval(schedule cron.Schedule, from time.Time) time.Duration {
	prev := schedule.Next(from)
	if prev.IsZero() {
		return 0
	}
	var shortest time.Duration
	for i := 0; i < intervalSamples; i++ {
		next := schedule.Next(prev)
		if next.IsZero() {
			break
		}
		if gap := next.Sub(prev); shortest == 0 || gap < shortest {
			shortest = gap
		}
		prev = next
	}
	return shortest
}

// hasTimezonePrefix reports whether schedule already sets its own time zone.
func hasTimezonePrefix(schedule string) bool {
	return strings.HasPrefix(schedule, "CRON_TZ=") || strings.HasPrefix(schedule, "TZ=")
}
