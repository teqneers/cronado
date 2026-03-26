package domain

import (
	"context"
	"fmt"
	"strings"
)

// Container represents a Docker container with all its metadata
type Container struct {
	ID     string
	Name   string
	Labels map[string]string
	Image  string
	Status string
}

// NewContainer creates a Container instance by fetching data from Docker
func NewContainer(ctx context.Context, containerID string) (*Container, error) {
	client := GetDockerClient()
	if client == nil {
		return nil, fmt.Errorf("docker client not initialized")
	}

	inspect, err := client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Remove leading "/" from container name
	name := strings.TrimPrefix(inspect.Name, "/")

	return &Container{
		ID:     inspect.ID,
		Name:   name,
		Labels: inspect.Config.Labels,
		Image:  inspect.Config.Image,
		Status: inspect.State.Status,
	}, nil
}

// GetOrCreateContainer fetches full container info from Docker API with error handling
// This is the centralized way to get container information
func GetOrCreateContainer(ctx context.Context, containerID string) (*Container, error) {
	client := GetDockerClient()
	if client == nil {
		return nil, fmt.Errorf("docker client not initialized")
	}

	inspect, err := client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err // Let caller handle specific error types
	}

	// Remove leading "/" from container name
	name := strings.TrimPrefix(inspect.Name, "/")

	return &Container{
		ID:     inspect.ID,
		Name:   name,
		Labels: inspect.Config.Labels,
		Image:  inspect.Config.Image,
		Status: inspect.State.Status,
	}, nil
}

// ShortID returns the first 12 characters of the container ID
// Safe to call even with short IDs
func (c *Container) ShortID() string {
	if len(c.ID) >= 12 {
		return c.ID[:12]
	}
	return c.ID
}

// DisplayName returns a human-friendly identifier for the container
// Format: "name (short_id)" or just "short_id" if name is empty
func (c *Container) DisplayName() string {
	shortID := c.ShortID()
	if c.Name != "" {
		return fmt.Sprintf("%s (%s)", c.Name, shortID)
	}
	return shortID
}

// IsRunning checks if the container is in running state
func (c *Container) IsRunning() bool {
	return c.Status == "running"
}

// GetLabel safely retrieves a label value
func (c *Container) GetLabel(key string) (string, bool) {
	if c.Labels == nil {
		return "", false
	}
	val, ok := c.Labels[key]
	return val, ok
}

// HasLabel checks if a label exists
func (c *Container) HasLabel(key string) bool {
	_, ok := c.GetLabel(key)
	return ok
}

// GetLabelWithPrefix returns all labels that start with the given prefix
// Returns a map with the prefix removed from keys
func (c *Container) GetLabelsWithPrefix(prefix string) map[string]string {
	result := make(map[string]string)
	for key, value := range c.Labels {
		if trimmedKey, found := strings.CutPrefix(key, prefix); found {
			trimmedKey = strings.TrimPrefix(trimmedKey, ".")
			result[trimmedKey] = value
		}
	}
	return result
}
