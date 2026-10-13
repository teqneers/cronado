package domain

import (
	"fmt"

	"github.com/teqneers/cronado/internal/notification"
)

// NotificationService defines the interface for sending notifications
type NotificationService interface {
	SendNotification(title, message string) error
	SendJobFailure(container *Container, jobName string, err error) error
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

// SendJobFailure sends a notification about a job failure
func (s *DefaultNotificationService) SendJobFailure(container *Container, jobName string, err error) error {
	// The title doubles as the throttle key, so it must identify the container
	// too: otherwise a failing job in one container would silence alerts for a
	// same-named job in another.
	containerName := container.Name
	if containerName == "" {
		containerName = container.ShortID()
	}
	title := fmt.Sprintf("Cron Job Failed: %s on %s", jobName, containerName)
	message := fmt.Sprintf("Job '%s' failed in container '%s': %v",
		jobName, container.DisplayName(), err)

	notification.Notify(title, message)
	return nil
}
