package util

import (
	"fmt"
	"log/slog"
	"os"
	"github.com/teqneers/cronado/internal/notification"
)

func HandleDockerClientError(err error) {
	slog.Error("Docker client error", "error", err)
	notification.Notify(notification.SUBJECT_DAEMON_WATCHER, fmt.Sprintf("Docker client error: %v", err))
	os.Exit(4)
}

func HandleDockerDaemonPanic(err error) {
	hostname, hostnameError := os.Hostname()
	if hostnameError != nil {
		slog.Error("Failed to get hostname", "error", hostnameError)
		hostname = "unknown-host"
	}
	slog.Error("Docker API unreachable, exiting process", "error", err)
	notification.Notify(notification.SUBJECT_DAEMON_WATCHER, fmt.Sprintf("Docker API unreachable on host %s\nError: %v", hostname, err))
	os.Exit(5)
}
