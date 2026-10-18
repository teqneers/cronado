package domain

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/teqneers/cronado/internal/config"
)

func TestParseJobLabels_ImageJob(t *testing.T) {
	container := &Container{ID: "abc123456789", Name: "cronado", Labels: map[string]string{
		"cronado.backup.enabled":         "true",
		"cronado.backup.schedule":        "0 3 * * *",
		"cronado.backup.image":           "restic/restic:0.17.3",
		"cronado.backup.cmd":             "backup /data",
		"cronado.backup.network":         "backend",
		"cronado.backup.volumes":         "/srv/data:/data:ro, restic-cache:/cache",
		"cronado.backup.env.RESTIC_REPO": "s3:example.com/bucket",
		"cronado.backup.env.PASSWORD":    "a=b",
		"cronado.default.enabled":        "true",
		"cronado.default.schedule":       "@daily",
		"cronado.default.image":          "alpine",
	}}

	jobs, issues := ParseJobLabels(container, "cronado", 0, 0)
	if len(issues) != 0 {
		t.Fatalf("issues = %+v", issues)
	}
	backup := jobs["backup"]
	if backup.Image != "restic/restic:0.17.3" || backup.Command != "backup /data" || backup.Network != "backend" {
		t.Errorf("backup = %+v", backup)
	}
	if want := []string{"/srv/data:/data:ro", "restic-cache:/cache"}; !reflect.DeepEqual(backup.Volumes, want) {
		t.Errorf("Volumes = %v, want %v", backup.Volumes, want)
	}
	if want := []string{"PASSWORD=a=b", "RESTIC_REPO=s3:example.com/bucket"}; !reflect.DeepEqual(backup.Env, want) {
		t.Errorf("Env = %v, want %v", backup.Env, want)
	}
	if def, ok := jobs["default"]; !ok || def.Command != "" || !def.IsValid() {
		t.Errorf("image job without cmd = %+v, %v; want valid job running the image's default command", def, ok)
	}
}

func TestParseJobLabels_ImageJobProblems(t *testing.T) {
	base := func(extra map[string]string) *Container {
		labels := map[string]string{
			"cronado.job.enabled":  "true",
			"cronado.job.schedule": "@daily",
		}
		for k, v := range extra {
			labels[k] = v
		}
		return &Container{ID: "abc123456789", Name: "app", Labels: labels}
	}
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{"invalid image", map[string]string{"cronado.job.image": "Not An Image"}, "invalid image"},
		{"invalid volume", map[string]string{"cronado.job.image": "alpine", "cronado.job.volumes": "/data"}, `invalid volume "/data"`},
		{"relative target", map[string]string{"cronado.job.image": "alpine", "cronado.job.volumes": "/a:data"}, `invalid volume "/a:data"`},
		{"invalid env name", map[string]string{"cronado.job.image": "alpine", "cronado.job.env.1X": "y"}, `invalid environment variable name "1X"`},
		{"unterminated quote", map[string]string{"cronado.job.image": "alpine", "cronado.job.cmd": `echo "hi`}, "unterminated quote"},
		{"network without image", map[string]string{"cronado.job.cmd": "true", "cronado.job.network": "backend"}, "need an image"},
		{"env without image", map[string]string{"cronado.job.cmd": "true", "cronado.job.env.X": "y"}, "need an image"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs, issues := ParseJobLabels(base(tt.labels), "cronado", 0, 0)
			if len(jobs) != 0 {
				t.Errorf("jobs = %v, want job rejected", jobs)
			}
			var messages []string
			for _, issue := range issues {
				messages = append(messages, issue.Message)
			}
			if !strings.Contains(strings.Join(messages, "\n"), tt.want) {
				t.Errorf("issues = %q, want one containing %q", messages, tt.want)
			}
		})
	}
}

func TestParseContainerJobs_ConfigImageJobs(t *testing.T) {
	jobs := []config.JobConfig{
		{Name: "offsite", Image: "restic/restic:0.17.3", Schedule: "0 3 * * *", Cmd: "backup /data",
			Network: "backend", Volumes: []string{"/srv/data:/data:ro", "cache:/cache"},
			Environment: []string{"RESTIC_PASSWORD=x=y", "EMPTY="}},
		{Name: "backup", Container: "db", Schedule: "0 2 * * *", Cmd: "pg_dump"},
	}

	parsed, issues := ParseContainerJobs(ConfigRunOwner(), "cronado", jobs, time.Minute, time.Hour)
	if len(issues) != 0 {
		t.Fatalf("issues = %+v", issues)
	}
	if len(parsed) != 1 {
		t.Fatalf("jobs = %v, want only offsite", parsed)
	}
	offsite := parsed["offsite"]
	if offsite.ID != "cronado-offsite" || offsite.Container != ConfigRunOwner() || offsite.Source != SourceConfig ||
		offsite.Image != "restic/restic:0.17.3" || offsite.Network != "backend" {
		t.Errorf("offsite = %+v", offsite)
	}
	if want := []string{"/srv/data:/data:ro", "cache:/cache"}; !reflect.DeepEqual(offsite.Volumes, want) {
		t.Errorf("Volumes = %v, want %v", offsite.Volumes, want)
	}
	if want := []string{"EMPTY=", "RESTIC_PASSWORD=x=y"}; !reflect.DeepEqual(offsite.Env, want) {
		t.Errorf("Env = %v, want %v", offsite.Env, want)
	}

	db := &Container{ID: "aaaaaaaaaaaa1", Name: "db"}
	parsed, _ = ParseContainerJobs(db, "cronado", jobs, time.Minute, time.Hour)
	if _, ok := parsed["offsite"]; ok || len(parsed) != 1 {
		t.Errorf("db jobs = %v, want only backup", parsed)
	}
}

func TestCheckConfigJobs_ImageJobs(t *testing.T) {
	jobs := []config.JobConfig{
		{Name: "both", Container: "db", Image: "alpine", Schedule: "@daily"},
		{Name: "badenv", Image: "alpine", Schedule: "@daily", Environment: []string{"NOVALUE"}},
		{Name: "dup", Image: "alpine", Schedule: "@daily"},
		{Name: "dup", Image: "alpine", Schedule: "@daily"},
	}
	got := CheckConfigJobs(jobs)
	want := []string{
		"jobs[0] both: set either container or image, not both",
		`jobs[1] badenv: environment entry "NOVALUE" must be KEY=value`,
		"jobs[3] dup: defined more than once",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q, want %q", got, want)
	}

	parsed, _ := ParseContainerJobs(ConfigRunOwner(), "cronado", jobs, 0, 0)
	if len(parsed) != 1 {
		t.Errorf("jobs = %v, want only the first dup", parsed)
	}
}

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"backup /data", []string{"backup", "/data"}},
		{`sh -c "echo $HOME && ls"`, []string{"sh", "-c", "echo $HOME && ls"}},
		{`echo 'a "b" c'`, []string{"echo", `a "b" c`}},
		{`echo "a \"b\" C:\dir"`, []string{"echo", `a "b" C:\dir`}},
		{`echo a\ b ""`, []string{"echo", "a b", ""}},
		{"  a\t b\n", []string{"a", "b"}},
	}
	for _, tt := range tests {
		got, err := splitArgs(tt.in)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitArgs(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
	for _, in := range []string{`echo "a`, `echo 'a`, `echo a\`} {
		if _, err := splitArgs(in); err == nil {
			t.Errorf("splitArgs(%q) succeeded, want error", in)
		}
	}
}

// mockRunClient implements ContainerRunClient for testing
type mockRunClient struct {
	mu sync.Mutex

	imageMissing bool
	pulled       []string

	createConfig *dockercontainer.Config
	createHost   *dockercontainer.HostConfig
	createName   string
	createErr    error

	exitCode int64
	// block makes the container run until it is killed
	block  bool
	killed chan struct{}

	logs    []byte
	removed []string
	listed  []dockercontainer.Summary
}

func (m *mockRunClient) ContainerList(_ context.Context, _ dockercontainer.ListOptions) ([]dockercontainer.Summary, error) {
	return m.listed, nil
}

func (m *mockRunClient) ImageInspect(_ context.Context, _ string, _ ...client.ImageInspectOption) (image.InspectResponse, error) {
	if m.imageMissing {
		return image.InspectResponse{}, errdefs.ErrNotFound
	}
	return image.InspectResponse{}, nil
}

func (m *mockRunClient) ImagePull(_ context.Context, ref string, _ image.PullOptions) (io.ReadCloser, error) {
	m.pulled = append(m.pulled, ref)
	return io.NopCloser(strings.NewReader(`{"status":"done"}`)), nil
}

func (m *mockRunClient) ContainerCreate(_ context.Context, cfg *dockercontainer.Config, host *dockercontainer.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, name string) (dockercontainer.CreateResponse, error) {
	m.createConfig, m.createHost, m.createName = cfg, host, name
	if m.createErr != nil {
		return dockercontainer.CreateResponse{}, m.createErr
	}
	return dockercontainer.CreateResponse{ID: "run123456789abc"}, nil
}

func (m *mockRunClient) ContainerStart(_ context.Context, _ string, _ dockercontainer.StartOptions) error {
	return nil
}

func (m *mockRunClient) ContainerWait(ctx context.Context, _ string, _ dockercontainer.WaitCondition) (<-chan dockercontainer.WaitResponse, <-chan error) {
	results := make(chan dockercontainer.WaitResponse, 1)
	errs := make(chan error, 1)
	if m.block {
		go func() {
			<-ctx.Done()
			errs <- ctx.Err()
		}()
	} else {
		results <- dockercontainer.WaitResponse{StatusCode: m.exitCode}
	}
	return results, errs
}

func (m *mockRunClient) ContainerLogs(_ context.Context, _ string, _ dockercontainer.LogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(m.logs)), nil
}

func (m *mockRunClient) ContainerKill(_ context.Context, _, _ string) error {
	if m.killed != nil {
		close(m.killed)
	}
	return nil
}

func (m *mockRunClient) ContainerRemove(_ context.Context, id string, _ dockercontainer.RemoveOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removed = append(m.removed, id)
	return nil
}

// muxFrame encodes data as a multiplexed Docker log frame.
func muxFrame(stream byte, data string) []byte {
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:], uint32(len(data)))
	return append(header, data...)
}

func testRunJob() *CronJob {
	return &CronJob{
		ID:      "cronado-backup",
		Name:    "backup",
		Image:   "restic/restic:0.17.3",
		Command: `backup "/data dir"`,
		User:    "1000",
		Network: "backend",
		Volumes: []string{"/srv:/data:ro"},
		Env:     []string{"A=b"},
		Timeout: time.Minute,
	}
}

func TestDockerContainerRunner_Success(t *testing.T) {
	mock := &mockRunClient{imageMissing: true, logs: append(muxFrame(1, "out\n"), muxFrame(2, "err\n")...)}
	stdout, stderr, err := NewDockerContainerRunner(mock).RunContainer(testRunJob())
	if err != nil {
		t.Fatalf("RunContainer: %v", err)
	}
	if stdout != "out\n" || stderr != "err\n" {
		t.Errorf("output = %q, %q", stdout, stderr)
	}
	if !reflect.DeepEqual(mock.pulled, []string{"restic/restic:0.17.3"}) {
		t.Errorf("pulled = %v, want the missing image", mock.pulled)
	}
	cfg := mock.createConfig
	if cfg.Image != "restic/restic:0.17.3" || cfg.User != "1000" || !reflect.DeepEqual([]string(cfg.Cmd), []string{"backup", "/data dir"}) ||
		!reflect.DeepEqual(cfg.Env, []string{"A=b"}) || cfg.Labels[RunJobLabel] != "cronado-backup" {
		t.Errorf("config = %+v", cfg)
	}
	if !reflect.DeepEqual(mock.createHost.Binds, []string{"/srv:/data:ro"}) || mock.createHost.NetworkMode != "backend" {
		t.Errorf("host config = %+v", mock.createHost)
	}
	if !strings.HasPrefix(mock.createName, "cronado-cronado-backup-") {
		t.Errorf("name = %q", mock.createName)
	}
	if !reflect.DeepEqual(mock.removed, []string{"run123456789abc"}) {
		t.Errorf("removed = %v, want the job container", mock.removed)
	}
}

func TestDockerContainerRunner_PresentImageIsNotPulled(t *testing.T) {
	mock := &mockRunClient{}
	job := testRunJob()
	job.Command = ""
	if _, _, err := NewDockerContainerRunner(mock).RunContainer(job); err != nil {
		t.Fatalf("RunContainer: %v", err)
	}
	if len(mock.pulled) != 0 {
		t.Errorf("pulled = %v, want none", mock.pulled)
	}
	if mock.createConfig.Cmd != nil {
		t.Errorf("Cmd = %v, want nil to run the image's default command", mock.createConfig.Cmd)
	}
}

func TestDockerContainerRunner_ExitCode(t *testing.T) {
	mock := &mockRunClient{exitCode: 3, logs: muxFrame(2, "boom\n")}
	_, stderr, err := NewDockerContainerRunner(mock).RunContainer(testRunJob())
	if err == nil || err.Error() != "command exited with code 3" {
		t.Errorf("err = %v", err)
	}
	if stderr != "boom\n" || len(mock.removed) != 1 {
		t.Errorf("stderr = %q, removed = %v", stderr, mock.removed)
	}
}

func TestDockerContainerRunner_Timeout(t *testing.T) {
	mock := &mockRunClient{block: true, killed: make(chan struct{})}
	job := testRunJob()
	job.Timeout = 50 * time.Millisecond
	_, _, err := NewDockerContainerRunner(mock).RunContainer(job)
	if err == nil || !strings.Contains(err.Error(), "timed out after 50ms and was killed") {
		t.Errorf("err = %v", err)
	}
	select {
	case <-mock.killed:
	default:
		t.Error("container was not killed")
	}
	if len(mock.removed) != 1 {
		t.Errorf("removed = %v, want the job container", mock.removed)
	}
}

func TestDockerContainerRunner_CreateError(t *testing.T) {
	mock := &mockRunClient{createErr: errors.New("no such network")}
	_, _, err := NewDockerContainerRunner(mock).RunContainer(testRunJob())
	if err == nil || !strings.Contains(err.Error(), "no such network") {
		t.Errorf("err = %v", err)
	}
	if len(mock.removed) != 0 {
		t.Errorf("removed = %v, want nothing", mock.removed)
	}
}

func TestRemoveLeftoverRunContainers(t *testing.T) {
	mock := &mockRunClient{listed: []dockercontainer.Summary{
		{ID: "exited", State: dockercontainer.StateExited},
		{ID: "running", State: dockercontainer.StateRunning},
		{ID: "created", State: dockercontainer.StateCreated},
	}}
	RemoveLeftoverRunContainers(mock)
	if want := []string{"exited", "created"}; !reflect.DeepEqual(mock.removed, want) {
		t.Errorf("removed = %v, want %v", mock.removed, want)
	}
}

// mockRunner implements ContainerRunner for testing
type mockRunner struct {
	mu   sync.Mutex
	jobs []string
}

func (m *mockRunner) RunContainer(job *CronJob) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = append(m.jobs, job.ID)
	return "", "", nil
}

func TestCronJobManager_ImageJobUsesRunner(t *testing.T) {
	scheduler := NewMockScheduler()
	executor := NewMockCommandExecutor()
	runner := &mockRunner{}
	manager := NewCronJobManager(CronJobManagerOptions{
		Scheduler: scheduler,
		Executor:  executor,
		Runner:    runner,
		Metrics:   NewMockMetricsCollector(),
		Notifier:  NewMockNotificationService(),
	})

	owner := ConfigRunOwner()
	job := CronJob{ID: "cronado-backup", Name: "backup", Container: owner, Enabled: true, Schedule: "@daily", Image: "alpine", Status: StatusIdle}
	if err := manager.Add(owner, job); err != nil {
		t.Fatalf("Add: %v", err)
	}
	stored, _ := manager.GetJobByID(job.ID)
	scheduler.ExecuteJob(stored.SchedulerId)

	if !reflect.DeepEqual(runner.jobs, []string{"cronado-backup"}) {
		t.Errorf("runner jobs = %v", runner.jobs)
	}
	if executor.GetExecuteCallCount() != 0 {
		t.Errorf("executor calls = %d, want 0", executor.GetExecuteCallCount())
	}
	if stored, _ := manager.GetJobByID(job.ID); stored.LastResult != "success" {
		t.Errorf("LastResult = %q", stored.LastResult)
	}
}

func TestIsRunContainer(t *testing.T) {
	if !IsRunContainer(&Container{Labels: map[string]string{RunJobLabel: "x-job"}}) {
		t.Error("job container not detected")
	}
	if IsRunContainer(&Container{Labels: map[string]string{"cronado.job.cmd": "true"}}) {
		t.Error("regular container detected as job container")
	}
}
