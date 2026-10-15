package domain

import "sort"

// ContainerLabelIssue is a problem in the cron job labels of a running
// container, as reported by the API.
type ContainerLabelIssue struct {
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name,omitempty"`
	Job           string `json:"job,omitempty"`
	Label         string `json:"label,omitempty"`
	Message       string `json:"message"`
	// JobRejected is true if the job is not scheduled because of this issue.
	// Otherwise only the label is ignored.
	JobRejected bool `json:"job_rejected"`
}

// SetLabelIssues replaces the label issues recorded for container. An empty
// list clears them, e.g. after the labels were fixed.
func (m *CronJobManager) SetLabelIssues(container *Container, issues []LabelIssue) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(issues) == 0 {
		delete(m.labelIssues, container.ID)
		return
	}
	if m.labelIssues == nil {
		m.labelIssues = make(map[string][]ContainerLabelIssue)
	}
	result := make([]ContainerLabelIssue, 0, len(issues))
	for _, issue := range issues {
		result = append(result, ContainerLabelIssue{
			ContainerID:   container.ID,
			ContainerName: container.Name,
			Job:           issue.Job,
			Label:         issue.Label,
			Message:       issue.Message,
			JobRejected:   issue.JobRejected,
		})
	}
	m.labelIssues[container.ID] = result
}

// GetLabelIssues returns the label issues of all containers, sorted by
// container name.
func (m *CronJobManager) GetLabelIssues() []ContainerLabelIssue {
	m.mu.RLock()
	defer m.mu.RUnlock()

	containerIDs := make([]string, 0, len(m.labelIssues))
	for id := range m.labelIssues {
		containerIDs = append(containerIDs, id)
	}
	sort.Slice(containerIDs, func(i, j int) bool {
		a, b := m.labelIssues[containerIDs[i]][0], m.labelIssues[containerIDs[j]][0]
		if a.ContainerName != b.ContainerName {
			return a.ContainerName < b.ContainerName
		}
		return a.ContainerID < b.ContainerID
	})

	result := []ContainerLabelIssue{}
	for _, id := range containerIDs {
		result = append(result, m.labelIssues[id]...)
	}
	return result
}

// RetainLabelIssues drops the issues of containers that are no longer
// running, e.g. after the Docker event stream was down.
func (m *CronJobManager) RetainLabelIssues(running map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.labelIssues {
		if !running[id] {
			delete(m.labelIssues, id)
		}
	}
}
