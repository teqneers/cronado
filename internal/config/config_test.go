package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadConfig_Jobs(t *testing.T) {
	disabled := false
	viper.Reset()
	t.Cleanup(viper.Reset)

	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
jobs:
  - name: backup
    container: db
    schedule: "0 2 * * *"
    cmd: pg_dump mydb
    user: postgres
    timeout: 10m
    enabled: false
    ping_url: https://hc-ping.com/abc
  - name: cleanup
    container: app-web-1
    schedule: "@every 1h"
    cmd: php bin/console app:cleanup
    replicas: all
  - name: offsite
    image: restic/restic:0.17.3
    schedule: "0 3 * * *"
    cmd: backup /data
    network: backend
    volumes:
      - /srv/data:/data:ro
      - restic-cache:/cache
    environment:
      - RESTIC_REPOSITORY=s3:example.com/bucket
      - RESTIC_PASSWORD=secret
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	viper.Set("config", path)

	cfg := LoadConfig()
	want := []JobConfig{
		{Name: "backup", Container: "db", Enabled: &disabled, Schedule: "0 2 * * *", Cmd: "pg_dump mydb", User: "postgres", Timeout: "10m", PingURL: "https://hc-ping.com/abc"},
		{Name: "cleanup", Container: "app-web-1", Schedule: "@every 1h", Cmd: "php bin/console app:cleanup", Replicas: "all"},
		{Name: "offsite", Image: "restic/restic:0.17.3", Schedule: "0 3 * * *", Cmd: "backup /data", Network: "backend",
			Volumes:     []string{"/srv/data:/data:ro", "restic-cache:/cache"},
			Environment: []string{"RESTIC_REPOSITORY=s3:example.com/bucket", "RESTIC_PASSWORD=secret"}},
	}
	if len(cfg.Jobs) != len(want) {
		t.Fatalf("Jobs = %+v", cfg.Jobs)
	}
	for i := range want {
		got := cfg.Jobs[i]
		if (got.Enabled == nil) != (want[i].Enabled == nil) || (got.Enabled != nil && *got.Enabled != *want[i].Enabled) {
			t.Errorf("Jobs[%d].Enabled = %v, want %v", i, got.Enabled, want[i].Enabled)
		}
		got.Enabled, want[i].Enabled = nil, nil
		if !reflect.DeepEqual(got, want[i]) {
			t.Errorf("Jobs[%d] = %+v, want %+v", i, got, want[i])
		}
	}
}
