package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

type ServerConfig struct {
	Host string
	Port string
}

type LogConfig struct {
	Level  string
	Format string
}

type DaemonWatcher struct {
	Enabled bool
	Timeout int
}

// EmailConfig holds settings for sending notification emails
type EmailConfig struct {
	Enabled  bool     // enable or disable email notifications
	SMTPHost string   // SMTP server host
	SMTPPort int      // SMTP server port
	Username string   // SMTP auth username
	Password string   // SMTP auth password
	From     string   // sender email address
	To       []string // recipient email addresses
}

// NtfyConfig holds settings for ntfy.sh notifications
type NtfyConfig struct {
	Enabled bool   // enable or disable ntfy notifications
	Server  string // ntfy server URL (e.g., https://ntfy.sh)
	Topic   string // ntfy topic
	Token   string // authentication token for ntfy (if required)
}

// NotifyConfig holds settings for notifications
type Notify struct {
	IntervalSeconds int // minimum interval between failure notifications for the same job
	Email           EmailConfig
	Ntfy            NtfyConfig
}

type Metrics struct {
	Enabled  bool   // enable or disable Prometheus metrics
	Endpoint string // metrics endpoint URL
}

// Config holds application settings
type Config struct {
	AppName         string
	CronLabelPrefix string
	DaemonWatcher   DaemonWatcher
	ServerConfig    ServerConfig
	Log             LogConfig
	Notify          Notify
	Metrics         Metrics
}

// LoadConfig reads configurations using Viper
func LoadConfig() *Config {
	// set server defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.host", "127.0.0.1")

	// set default values for the application
	viper.SetDefault("app_name", "cronado")
	viper.SetDefault("cron_label_prefix", "cronado")

	// set default values for the daemon watcher
	viper.SetDefault("daemon_watcher.enabled", true)
	viper.SetDefault("daemon_watcher.timeout_seconds", 5)

	// set default values for logging
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "text")

	// notification defaults
	viper.SetDefault("notify.interval_seconds", 3600)

	// email notification defaults
	viper.SetDefault("notify.email.enabled", false)
	viper.SetDefault("notify.email.smtp_host", "")
	viper.SetDefault("notify.email.smtp_port", 587)
	viper.SetDefault("notify.email.username", "")
	viper.SetDefault("notify.email.password", "")
	viper.SetDefault("notify.email.from", "")
	viper.SetDefault("notify.email.to", []string{})

	// ntfy notification defaults
	viper.SetDefault("notify.ntfy.enabled", false)
	viper.SetDefault("notify.ntfy.server", "https://ntfy.sh")
	viper.SetDefault("notify.ntfy.topic", "")
	viper.SetDefault("notify.ntfy.token", "")

	// metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.endpoint", "/metrics")

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("CRONADO")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Fatalf("Failed to read config: %v", err)
		} else {
			log.Println("No config file found, using defaults and env vars")
		}
	} else {
		log.Printf("Using config file: %s\n", viper.ConfigFileUsed())
	}

	return &Config{
		AppName:         viper.GetString("app_name"),
		CronLabelPrefix: viper.GetString("cron_label_prefix"),
		DaemonWatcher: DaemonWatcher{
			Enabled: viper.GetBool("daemon_watcher.enabled"),
			Timeout: viper.GetInt("daemon_watcher.timeout_seconds"),
		},
		ServerConfig: ServerConfig{
			Host: viper.GetString("server.host"),
			Port: viper.GetString("server.port"),
		},
		Log: LogConfig{
			Level:  viper.GetString("log.level"),
			Format: viper.GetString("log.format"),
		},
		Notify: Notify{
			IntervalSeconds: viper.GetInt("notify.interval_seconds"),
			// Email notifications configuration
			Email: EmailConfig{
				Enabled:  viper.GetBool("notify.email.enabled"),
				SMTPHost: viper.GetString("notify.email.smtp_host"),
				SMTPPort: viper.GetInt("notify.email.smtp_port"),
				Username: viper.GetString("notify.email.username"),
				Password: viper.GetString("notify.email.password"),
				From:     viper.GetString("notify.email.from"),
				To:       viper.GetStringSlice("notify.email.to"),
			},
			// Ntfy notifications configuration
			Ntfy: NtfyConfig{
				Enabled: viper.GetBool("notify.ntfy.enabled"),
				Server:  viper.GetString("notify.ntfy.server"),
				Topic:   viper.GetString("notify.ntfy.topic"),
				Token:   viper.GetString("notify.ntfy.token"),
			},
		},
		// Metrics configuration
		Metrics: Metrics{
			Enabled:  viper.GetBool("metrics.enabled"),
			Endpoint: viper.GetString("metrics.endpoint"),
		},
	}
}
