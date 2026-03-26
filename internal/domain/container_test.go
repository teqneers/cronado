package domain

import (
	"fmt"
	"reflect"
	"testing"
)

func TestContainer_ShortID(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		expected    string
	}{
		{
			name:        "full length ID",
			containerID: "abcdef123456789012345678901234567890",
			expected:    "abcdef123456",
		},
		{
			name:        "exactly 12 characters",
			containerID: "abcdef123456",
			expected:    "abcdef123456",
		},
		{
			name:        "short ID (less than 12)",
			containerID: "abc123",
			expected:    "abc123",
		},
		{
			name:        "empty ID",
			containerID: "",
			expected:    "",
		},
		{
			name:        "single character",
			containerID: "a",
			expected:    "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := &Container{ID: tt.containerID}
			result := container.ShortID()
			if result != tt.expected {
				t.Errorf("ShortID() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestContainer_DisplayName(t *testing.T) {
	tests := []struct {
		name         string
		container    Container
		expectedName string
	}{
		{
			name: "container with name",
			container: Container{
				ID:   "abcdef123456789",
				Name: "my-container",
			},
			expectedName: "my-container (abcdef123456)",
		},
		{
			name: "container without name",
			container: Container{
				ID:   "abcdef123456789",
				Name: "",
			},
			expectedName: "abcdef123456",
		},
		{
			name: "container with short ID and name",
			container: Container{
				ID:   "abc123",
				Name: "short-container",
			},
			expectedName: "short-container (abc123)",
		},
		{
			name: "container with empty ID and name",
			container: Container{
				ID:   "",
				Name: "",
			},
			expectedName: "",
		},
		{
			name: "container with special characters in name",
			container: Container{
				ID:   "abcdef123456789",
				Name: "/my-container_with-special.chars",
			},
			expectedName: "/my-container_with-special.chars (abcdef123456)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.container.DisplayName()
			if result != tt.expectedName {
				t.Errorf("DisplayName() = %q, want %q", result, tt.expectedName)
			}
		})
	}
}

func TestContainer_IsRunning(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{
			name:     "running status",
			status:   "running",
			expected: true,
		},
		{
			name:     "stopped status",
			status:   "exited",
			expected: false,
		},
		{
			name:     "paused status",
			status:   "paused",
			expected: false,
		},
		{
			name:     "created status",
			status:   "created",
			expected: false,
		},
		{
			name:     "empty status",
			status:   "",
			expected: false,
		},
		{
			name:     "case sensitivity",
			status:   "RUNNING",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := &Container{Status: tt.status}
			result := container.IsRunning()
			if result != tt.expected {
				t.Errorf("IsRunning() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestContainer_GetLabel(t *testing.T) {
	labels := map[string]string{
		"app":         "my-app",
		"version":     "1.0.0",
		"environment": "production",
		"empty-value": "",
	}

	container := &Container{
		ID:     "test-container",
		Labels: labels,
	}

	tests := []struct {
		name          string
		labelKey      string
		expectedValue string
		expectedFound bool
	}{
		{
			name:          "existing label",
			labelKey:      "app",
			expectedValue: "my-app",
			expectedFound: true,
		},
		{
			name:          "non-existent label",
			labelKey:      "non-existent",
			expectedValue: "",
			expectedFound: false,
		},
		{
			name:          "empty value label",
			labelKey:      "empty-value",
			expectedValue: "",
			expectedFound: true,
		},
		{
			name:          "empty key",
			labelKey:      "",
			expectedValue: "",
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found := container.GetLabel(tt.labelKey)
			if value != tt.expectedValue {
				t.Errorf("GetLabel(%q) value = %q, want %q", tt.labelKey, value, tt.expectedValue)
			}
			if found != tt.expectedFound {
				t.Errorf("GetLabel(%q) found = %v, want %v", tt.labelKey, found, tt.expectedFound)
			}
		})
	}

	// Test with nil labels
	t.Run("nil labels", func(t *testing.T) {
		container := &Container{Labels: nil}
		value, found := container.GetLabel("any-key")
		if value != "" {
			t.Errorf("GetLabel() with nil labels, value = %q, want empty string", value)
		}
		if found {
			t.Error("GetLabel() with nil labels, found = true, want false")
		}
	})
}

func TestContainer_HasLabel(t *testing.T) {
	labels := map[string]string{
		"app":         "my-app",
		"empty-value": "",
	}

	container := &Container{
		ID:     "test-container",
		Labels: labels,
	}

	tests := []struct {
		name     string
		labelKey string
		expected bool
	}{
		{
			name:     "existing label",
			labelKey: "app",
			expected: true,
		},
		{
			name:     "non-existent label",
			labelKey: "non-existent",
			expected: false,
		},
		{
			name:     "empty value label exists",
			labelKey: "empty-value",
			expected: true,
		},
		{
			name:     "empty key",
			labelKey: "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := container.HasLabel(tt.labelKey)
			if result != tt.expected {
				t.Errorf("HasLabel(%q) = %v, want %v", tt.labelKey, result, tt.expected)
			}
		})
	}

	// Test with nil labels
	t.Run("nil labels", func(t *testing.T) {
		container := &Container{Labels: nil}
		result := container.HasLabel("any-key")
		if result {
			t.Error("HasLabel() with nil labels = true, want false")
		}
	})
}

func TestContainer_GetLabelsWithPrefix(t *testing.T) {
	labels := map[string]string{
		"cronado.job1.enabled":  "true",
		"cronado.job1.schedule": "@every 5m",
		"cronado.job2.enabled":  "false",
		"cronado.job2.command":  "echo hello",
		"other.label":           "value",
		"cronado":               "base-value",
		"cronado.":              "empty-suffix",
		"app":                   "my-app",
	}

	container := &Container{
		ID:     "test-container",
		Labels: labels,
	}

	tests := []struct {
		name     string
		prefix   string
		expected map[string]string
	}{
		{
			name:   "cronado prefix",
			prefix: "cronado",
			expected: map[string]string{
				"job1.enabled":  "true",
				"job1.schedule": "@every 5m",
				"job2.enabled":  "false",
				"job2.command":  "echo hello",
				"":              "empty-suffix", // cronado. becomes empty key
			},
		},
		{
			name:   "specific job prefix",
			prefix: "cronado.job1",
			expected: map[string]string{
				"enabled":  "true",
				"schedule": "@every 5m",
			},
		},
		{
			name:     "non-existent prefix",
			prefix:   "nonexistent",
			expected: map[string]string{},
		},
		{
			name:     "empty prefix",
			prefix:   "",
			expected: labels, // All labels should be returned with original keys
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := container.GetLabelsWithPrefix(tt.prefix)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("GetLabelsWithPrefix(%q) = %v, want %v", tt.prefix, result, tt.expected)
			}
		})
	}

	// Test with nil labels
	t.Run("nil labels", func(t *testing.T) {
		container := &Container{Labels: nil}
		result := container.GetLabelsWithPrefix("any-prefix")
		expected := map[string]string{}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("GetLabelsWithPrefix() with nil labels = %v, want %v", result, expected)
		}
	})
}

func TestNewContainerFromLabels(t *testing.T) {
	labels := map[string]string{
		"app":     "my-app",
		"version": "1.0.0",
	}

	container := &Container{ID: "test-id", Labels: labels}

	if container.ID != "test-id" {
		t.Errorf("ID = %q, want %q", container.ID, "test-id")
	}
	if !reflect.DeepEqual(container.Labels, labels) {
		t.Errorf("Labels = %v, want %v", container.Labels, labels)
	}
	if container.Name != "" {
		t.Errorf("Name = %q, want empty string", container.Name)
	}
	if container.Image != "" {
		t.Errorf("Image = %q, want empty string", container.Image)
	}
	if container.Status != "" {
		t.Errorf("Status = %q, want empty string", container.Status)
	}
}

func TestContainer_FieldsAndStructure(t *testing.T) {
	labels := map[string]string{
		"app": "test-app",
	}

	container := Container{
		ID:     "abc123456789",
		Name:   "test-container",
		Labels: labels,
		Image:  "nginx:latest",
		Status: "running",
	}

	// Test all fields are accessible
	if container.ID != "abc123456789" {
		t.Errorf("ID = %q, want %q", container.ID, "abc123456789")
	}
	if container.Name != "test-container" {
		t.Errorf("Name = %q, want %q", container.Name, "test-container")
	}
	if !reflect.DeepEqual(container.Labels, labels) {
		t.Errorf("Labels = %v, want %v", container.Labels, labels)
	}
	if container.Image != "nginx:latest" {
		t.Errorf("Image = %q, want %q", container.Image, "nginx:latest")
	}
	if container.Status != "running" {
		t.Errorf("Status = %q, want %q", container.Status, "running")
	}
}

func TestContainer_EdgeCases(t *testing.T) {
	t.Run("unicode characters in name and labels", func(t *testing.T) {
		labels := map[string]string{
			"app.名前": "テスト",
			"描述":     "这是一个测试",
			"emoji":  "🚀",
		}

		container := Container{
			ID:     "test-unicode",
			Name:   "测试-container-🚀",
			Labels: labels,
		}

		// Test DisplayName with unicode
		expected := "测试-container-🚀 (test-unicode)"
		if container.DisplayName() != expected {
			t.Errorf("DisplayName() with unicode = %q, want %q", container.DisplayName(), expected)
		}

		// Test label retrieval with unicode
		if value, found := container.GetLabel("emoji"); !found || value != "🚀" {
			t.Errorf("GetLabel(emoji) = %q, %v, want %q, true", value, found, "🚀")
		}
	})

	t.Run("very long container ID", func(t *testing.T) {
		longID := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789"
		container := Container{ID: longID}

		shortID := container.ShortID()
		if shortID != "abcdefghijkl" {
			t.Errorf("ShortID() with very long ID = %q, want %q", shortID, "abcdefghijkl")
		}
	})

	t.Run("labels with dots and special characters", func(t *testing.T) {
		labels := map[string]string{
			"com.docker.compose.service": "web",
			"io.kubernetes.pod.name":     "my-pod",
			"cronado.job-1.enabled":      "true",
			"cronado.job_2.schedule":     "@daily",
			"special.chars!@#$%^&*()":    "value",
		}

		container := Container{Labels: labels}

		// Test getting labels with complex prefix
		result := container.GetLabelsWithPrefix("cronado.job-1")
		expected := map[string]string{"enabled": "true"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("GetLabelsWithPrefix with complex prefix = %v, want %v", result, expected)
		}
	})
}

// Benchmark tests for performance-critical methods
func BenchmarkContainer_ShortID(b *testing.B) {
	container := Container{ID: "abcdefghijklmnopqrstuvwxyz0123456789"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		container.ShortID()
	}
}

func BenchmarkContainer_DisplayName(b *testing.B) {
	container := Container{
		ID:   "abcdefghijklmnopqrstuvwxyz0123456789",
		Name: "test-container-with-long-name",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		container.DisplayName()
	}
}

func BenchmarkContainer_GetLabel(b *testing.B) {
	labels := make(map[string]string)
	for i := 0; i < 100; i++ {
		labels[fmt.Sprintf("label-%d", i)] = fmt.Sprintf("value-%d", i)
	}
	container := Container{Labels: labels}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		container.GetLabel("label-50")
	}
}

func BenchmarkContainer_GetLabelsWithPrefix(b *testing.B) {
	labels := make(map[string]string)
	for i := 0; i < 100; i++ {
		labels[fmt.Sprintf("cronado.job-%d.enabled", i)] = "true"
		labels[fmt.Sprintf("cronado.job-%d.schedule", i)] = "@daily"
		labels[fmt.Sprintf("other.label-%d", i)] = "value"
	}
	container := Container{Labels: labels}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		container.GetLabelsWithPrefix("cronado")
	}
}
