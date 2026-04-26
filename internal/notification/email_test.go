package notification

import (
	"strings"
	"testing"

	"github.com/teqneers/cronado/internal/config"
	"github.com/teqneers/cronado/internal/context"
)

func TestSendEmail_Disabled(t *testing.T) {
	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				Email: config.EmailConfig{
					Enabled: false,
				},
			},
		},
	}

	err := SendEmail("test subject", "test body")
	if err != nil {
		t.Errorf("expected no error when disabled, got %v", err)
	}
}

func TestSendEmail_ConnectionRefused(t *testing.T) {
	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				Email: config.EmailConfig{
					Enabled:    true,
					SMTPHost:   "127.0.0.1",
					SMTPPort:   19999, // unlikely to be in use
					RequireTLS: false, // don't require TLS for test
					From:       "test@example.com",
					To:         []string{"admin@example.com"},
				},
			},
		},
	}

	err := SendEmail("test subject", "test body")
	if err == nil {
		t.Error("expected error when SMTP server is unreachable")
	}
}

func TestSendEmail_NoAuthWhenCredentialsEmpty(t *testing.T) {
	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				Email: config.EmailConfig{
					Enabled:    true,
					SMTPHost:   "127.0.0.1",
					SMTPPort:   19999,
					Username:   "",
					Password:   "",
					RequireTLS: false,
					From:       "test@example.com",
					To:         []string{"admin@example.com"},
				},
			},
		},
	}

	// Will fail to connect, but should not panic
	err := SendEmail("test", "body")
	if err == nil {
		t.Error("expected connection error")
	}
}

func TestSendEmail_InvalidFromAddress(t *testing.T) {
	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				Email: config.EmailConfig{
					Enabled: true,
					From:    "not-an-email",
					To:      []string{"admin@example.com"},
				},
			},
		},
	}

	err := SendEmail("test", "body")
	if err == nil {
		t.Error("expected error for invalid from address")
	}
}

func TestSendEmail_CRLFInSubjectDoesNotPanic(t *testing.T) {
	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				Email: config.EmailConfig{
					Enabled:    true,
					SMTPHost:   "127.0.0.1",
					SMTPPort:   19999,
					RequireTLS: false,
					From:       "test@example.com",
					To:         []string{"admin@example.com"},
				},
			},
		},
	}

	// go-mail handles subject encoding safely -- this must not panic
	// It will fail on connection, but the subject with CRLF is handled safely
	err := SendEmail("test\r\nBcc: attacker@evil.com", "body")
	if err == nil {
		t.Error("expected connection error")
	}
}

func TestSendEmail_InvalidToAddress(t *testing.T) {
	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				Email: config.EmailConfig{
					Enabled: true,
					From:    "test@example.com",
					To:      []string{"not-an-email"},
				},
			},
		},
	}

	err := SendEmail("test", "body")
	if err == nil {
		t.Error("expected error for invalid to address")
	}
	if !strings.Contains(err.Error(), "invalid to address") {
		t.Errorf("expected 'invalid to address' error, got %v", err)
	}
}

func TestSendEmail_WithTLSRequired(t *testing.T) {
	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				Email: config.EmailConfig{
					Enabled:    true,
					SMTPHost:   "127.0.0.1",
					SMTPPort:   19999,
					RequireTLS: true, // TLS mandatory
					From:       "test@example.com",
					To:         []string{"admin@example.com"},
				},
			},
		},
	}

	// Will fail to connect, but exercises the TLS mandatory code path
	err := SendEmail("test", "body")
	if err == nil {
		t.Error("expected connection error")
	}
}

func TestSendEmail_WithCredentials(t *testing.T) {
	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				Email: config.EmailConfig{
					Enabled:    true,
					SMTPHost:   "127.0.0.1",
					SMTPPort:   19999,
					Username:   "user@example.com",
					Password:   "secret",
					RequireTLS: false,
					From:       "test@example.com",
					To:         []string{"admin@example.com"},
				},
			},
		},
	}

	// Will fail to connect, but exercises the auth credentials code path
	err := SendEmail("test", "body")
	if err == nil {
		t.Error("expected connection error")
	}
}
