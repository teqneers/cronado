package cmd

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/teqneers/cronado/internal/api"
	"github.com/teqneers/cronado/internal/context"
	"github.com/teqneers/cronado/internal/domain"
	"github.com/teqneers/cronado/internal/metrics"

	"github.com/spf13/cobra"
)

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Run the cron job scheduler",
	Long:  "Runs the cron job scheduler that executes scheduled tasks.",
	Run: func(cmd *cobra.Command, args []string) {
		config := context.AppCtx.Config

		domain.SanityChecks()

		// Initialize Prometheus metrics
		metrics.Init()

		go startServer()

		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

		if config.Notify.Email.Enabled {
			slog.Info("Email notifications enabled")
		}

		if config.Notify.Ntfy.Enabled {
			slog.Info("Ntfy notifications enabled")
		}

		domain.SetupDockerClient()

		if config.DaemonWatcher.Enabled {
			slog.Info("Docker Daemon watcher enabled")
			go domain.MonitorDockerAPI(time.Duration(config.DaemonWatcher.Timeout) * time.Second)
		}

		// Initialize CronJobManager in the context
		cronJobManager := domain.NewCronJobManager(domain.CronJobManagerOptions{})
		cronJobManager.Start()
		context.AppCtx.CronJobManager = cronJobManager

		domain.InitializeAlreadyRunningContainers()
		domain.StartEventListener(stop)

		<-stop
	},
}


func startServer() {
	cfg := context.AppCtx.Config.ServerConfig

	router := api.SetupRouter()

	// Start the server
	address := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	log.Printf("Listening on http://%s\n", address)
	if err := http.ListenAndServe(address, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// Register the config command
func init() {
	rootCmd.AddCommand(cronCmd)
}
