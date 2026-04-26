package notification

import (
	"fmt"

	mail "github.com/wneessen/go-mail"

	appctx "github.com/teqneers/cronado/internal/context"
)

// SendEmail sends an email with the given subject and body if email notifications are enabled.
// Uses go-mail for proper MIME encoding, header injection prevention, and TLS support.
func SendEmail(subject, body string) error {
	cfg := appctx.AppCtx.Config.Notify.Email

	if !cfg.Enabled {
		return nil
	}

	msg := mail.NewMsg()
	if err := msg.From(cfg.From); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}
	if err := msg.To(cfg.To...); err != nil {
		return fmt.Errorf("invalid to address: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, body)

	// Determine TLS policy
	tlsPolicy := mail.TLSOpportunistic
	if cfg.RequireTLS {
		tlsPolicy = mail.TLSMandatory
	}

	// Build client options
	opts := []mail.Option{
		mail.WithPort(cfg.SMTPPort),
		mail.WithTLSPolicy(tlsPolicy),
	}

	// Add authentication if credentials are provided
	if cfg.Username != "" && cfg.Password != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(cfg.Username),
			mail.WithPassword(cfg.Password),
		)
	}

	client, err := mail.NewClient(cfg.SMTPHost, opts...)
	if err != nil {
		return fmt.Errorf("email client creation failed: %w", err)
	}

	if err := client.DialAndSend(msg); err != nil {
		return fmt.Errorf("email send failed: %w", err)
	}

	return nil
}
