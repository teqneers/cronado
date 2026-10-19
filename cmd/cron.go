package cmd

import (
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/teqneers/cronado/internal/api"
	"github.com/teqneers/cronado/internal/context"
	"github.com/teqneers/cronado/internal/domain"
	"github.com/teqneers/cronado/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
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
		if config.Notify.Webhook.Enabled {
			slog.Info("Webhook notifications enabled", "format", config.Notify.Webhook.Format)
		}

		domain.SetupDockerClient()

		if config.DaemonWatcher.Enabled {
			slog.Info("Docker Daemon watcher enabled")
			go domain.MonitorDockerAPI(time.Duration(config.DaemonWatcher.Timeout) * time.Second)
		}

		// Initialize CronJobManager in the context
		cronJobManager := domain.NewCronJobManager(domain.CronJobManagerOptions{
			ShutdownTimeout: config.ShutdownTimeout,
		})
		cronJobManager.Start()
		context.AppCtx.CronJobManager = cronJobManager
		prometheus.MustRegister(domain.NewJobStateCollector(cronJobManager))

		domain.LogConfigJobs(config.Jobs)
		domain.RemoveLeftoverRunContainers(domain.GetDockerClient())
		domain.RegisterConfigRunJobs()
		domain.StartEventListener(stop)

		<-stop
	},
}

func startServer() {
	cfg := context.AppCtx.Config.ServerConfig
	router := api.SetupRouter()

	if cfg.APIToken == "" && !isLoopbackHost(cfg.Host) {
		slog.Warn("API is reachable from the network without authentication; job commands are redacted. Set server.api_token (CRONADO_SERVER_API_TOKEN) to protect it",
			"host", cfg.Host)
	}

	// Start the server
	address := net.JoinHostPort(cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:              address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	slog.Info("Listening", "address", "http://"+address)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}

// isLoopbackHost reports whether host only accepts local connections.
func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Register the config command
func init() {
	rootCmd.AddCommand(cronCmd)
}
