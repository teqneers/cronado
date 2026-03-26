package notification

import (
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
					Enabled:  true,
					SMTPHost: "127.0.0.1",
					SMTPPort: 19999, // unlikely to be in use
					From:     "test@example.com",
					To:       []string{"admin@example.com"},
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
	// This test verifies the code path where username/password are empty
	// and no auth is used. We can't fully test without an SMTP server,
	// but we can verify it doesn't panic.
	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				Email: config.EmailConfig{
					Enabled:  true,
					SMTPHost: "127.0.0.1",
					SMTPPort: 19999,
					Username: "",
					Password: "",
					From:     "test@example.com",
					To:       []string{"admin@example.com"},
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
