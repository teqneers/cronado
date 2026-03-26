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
	title := fmt.Sprintf("Cron Job Failed: %s", jobName)
	message := fmt.Sprintf("Job '%s' failed in container '%s': %v",
		jobName, container.DisplayName(), err)

	notification.Notify(title, message)
	return nil
}