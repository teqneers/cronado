package domain

import (
	"context"
	"fmt"
	"math"
	"strconv"
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

// Labels Docker Compose sets on the containers it creates
const (
	composeProjectLabel = "com.docker.compose.project"
	composeServiceLabel = "com.docker.compose.service"
	composeNumberLabel  = "com.docker.compose.container-number"
	composeOneoffLabel  = "com.docker.compose.oneoff"
)

// ComposeService returns the Compose project and service the container
// belongs to, or ok=false if it was not created by Docker Compose.
func (c *Container) ComposeService() (project, service string, ok bool) {
	project, hasProject := c.GetLabel(composeProjectLabel)
	service, hasService := c.GetLabel(composeServiceLabel)
	if !hasProject || !hasService || service == "" {
		return "", "", false
	}
	return project, service, true
}

// replicaRank orders the replicas of a Compose service: regular replicas by
// container number before one-off containers ("docker compose run"), with
// the container ID as a tie-breaker. The lowest rank runs the shared jobs.
func (c *Container) replicaRank() (oneoff bool, number int, id string) {
	value, _ := c.GetLabel(composeOneoffLabel)
	oneoff = strings.EqualFold(value, "true")
	number = math.MaxInt
	if value, ok := c.GetLabel(composeNumberLabel); ok {
		if n, err := strconv.Atoi(value); err == nil {
			number = n
		}
	}
	return oneoff, number, c.ID
}

// ranksBefore reports whether c comes before other in replicaRank order.
func (c *Container) ranksBefore(other *Container) bool {
	oneoff, number, id := c.replicaRank()
	otherOneoff, otherNumber, otherID := other.replicaRank()
	if oneoff != otherOneoff {
		return !oneoff
	}
	if number != otherNumber {
		return number < otherNumber
	}
	return id < otherID
}
