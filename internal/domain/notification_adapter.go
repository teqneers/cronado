package domain

import (
	"fmt"
	"strings"

	cronadoCtx "github.com/teqneers/cronado/internal/context"

	"github.com/teqneers/cronado/internal/notification"
)

// NotificationService defines the interface for sending notifications
type NotificationService interface {
	SendNotification(title, message string) error
	SendJobFailure(container *Container, jobName string, err error, output string) error
}

// DefaultNotificationService uses the existing notification package
type DefaultNotificationService struct{}

// NewDefaultNotificationService creates a new DefaultNotificationService
func NewDefaultNotificationService() *DefaultNotificationService {
	return &DefaultNotificationService{}
}

// SendNotification sends a general notification
func (s *DefaultNotificationService) SendNotification(title, message string) error {
	notification.Notify(title, message)
	return nil
}

// Limits for the command output included in failure notifications.
const (
	notifyOutputLines = 20
	notifyOutputBytes = 1500
)

// SendJobFailure sends a notification about a job failure, including the tail
// of the command output unless notify.include_output is disabled.
func (s *DefaultNotificationService) SendJobFailure(container *Container, jobName string, err error, output string) error {
	title, message := failureMessage(container, jobName, err, output)
	notification.Notify(title, message)
	return nil
}

// failureMessage builds the notification title and body for a failed job.
func failureMessage(container *Container, jobName string, err error, output string) (title, message string) {
	// The title doubles as the throttle key, so it must identify the container
	// too: otherwise a failing job in one container would silence alerts for a
	// same-named job in another.
	containerName := container.Name
	if containerName == "" {
		containerName = container.ShortID()
	}
	title = fmt.Sprintf("Cron Job Failed: %s on %s", jobName, containerName)
	message = fmt.Sprintf("Job '%s' failed in container '%s': %v",
		jobName, container.DisplayName(), err)
	if includeOutput() {
		if tail := outputTail(output, notifyOutputLines, notifyOutputBytes); tail != "" {
			message += "\n\nOutput (last lines):\n" + tail
		}
	}
	return title, message
}

// includeOutput reports whether failure notifications should contain output.
func includeOutput() bool {
	if cronadoCtx.AppCtx == nil || cronadoCtx.AppCtx.Config == nil {
		return false
	}
	return cronadoCtx.AppCtx.Config.Notify.IncludeOutput
}

// outputTail returns at most maxLines lines and maxBytes bytes from the end of
// output, without trailing whitespace.
func outputTail(output string, maxLines, maxBytes int) string {
	output = strings.TrimRight(output, " \t\r\n")
	if output == "" {
		return ""
	}
	lines := strings.Split(output, "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	tail := strings.Join(lines, "\n")
	if len(tail) > maxBytes {
		tail = strings.ToValidUTF8(tail[len(tail)-maxBytes:], "")
	}
	return tail
}
