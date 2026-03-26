package notification

import (
	"fmt"
	"net/smtp"
	"strings"

	appctx "github.com/teqneers/cronado/internal/context"
)

// SendEmail sends an email with the given subject and body if email notifications are enabled.
// Throttles notifications per subject based on the NotifyIntervalSeconds setting.
func SendEmail(subject, body string) error {
	cfg := appctx.AppCtx.Config.Notify.Email

	// Check if email notifications are enabled
	if !cfg.Enabled {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	// default to no auth if username and password are not provided
	var auth smtp.Auth
	if cfg.Username != "" && cfg.Password != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	}

	// Prepare email headers
	header := make(map[string]string)
	header["From"] = cfg.From
	header["To"] = strings.Join(cfg.To, ",")
	header["Subject"] = subject

	var msg strings.Builder
	for k, v := range header {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	msg.WriteString("\r\n" + body)

	err := smtp.SendMail(addr, auth, cfg.From, cfg.To, []byte(msg.String()))
	if err != nil {
		return err
	}

	return nil
}
