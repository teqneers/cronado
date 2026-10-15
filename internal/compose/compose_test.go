package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_MapAndListLabels(t *testing.T) {
	path := writeFile(t, "compose.yml", `
services:
  db:
    image: postgres:16
    container_name: my-db
    labels:
      cronado.backup.enabled: true
      cronado.backup.schedule: "0 2 * * *"
      cronado.backup.cmd: echo $$HOME
      cronado.backup.user:
  app:
    labels:
      - cronado.report.enabled=true
      - cronado.report.cmd=echo a=b
      - cronado.report.user
`)
	services, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(services) != 2 || services[0].Name != "app" || services[1].Name != "db" {
		t.Fatalf("services = %+v, want app and db sorted", services)
	}

	app, db := services[0], services[1]
	want := map[string]string{
		"cronado.report.enabled": "true",
		"cronado.report.cmd":     "echo a=b",
		"cronado.report.user":    "",
	}
	for k, v := range want {
		if app.Labels[k] != v {
			t.Errorf("app label %s = %q, want %q", k, app.Labels[k], v)
		}
	}

	if db.ContainerName != "my-db" {
		t.Errorf("ContainerName = %q, want my-db", db.ContainerName)
	}
	if got := db.Labels["cronado.backup.enabled"]; got != "true" {
		t.Errorf("bool label = %q, want true", got)
	}
	if got := db.Labels["cronado.backup.cmd"]; got != "echo $HOME" {
		t.Errorf("$$ not unescaped: %q", got)
	}
	if got, ok := db.Labels["cronado.backup.user"]; !ok || got != "" {
		t.Errorf("empty label = %q (present %v), want empty", got, ok)
	}
}

func TestLoad_LaterFilesOverride(t *testing.T) {
	base := writeFile(t, "base.yml", `
services:
  app:
    labels:
      cronado.job.schedule: "@every 1h"
      cronado.job.cmd: echo base
`)
	override := writeFile(t, "override.yml", `
services:
  app:
    labels:
      cronado.job.cmd: echo override
`)
	services, err := Load(base, override)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	labels := services[0].Labels
	if labels["cronado.job.cmd"] != "echo override" || labels["cronado.job.schedule"] != "@every 1h" {
		t.Errorf("labels = %v, want override cmd and base schedule", labels)
	}
}

func TestLoad_Errors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yml")); err == nil {
		t.Error("expected error for missing file")
	}
	bad := writeFile(t, "bad.yml", "services:\n  app:\n    labels: just-a-string\n")
	if _, err := Load(bad); err == nil {
		t.Error("expected error for scalar labels")
	}
}
