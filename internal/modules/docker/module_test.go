package docker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/runtime/actions"
)

func TestDockerModule_ImplementsModule(t *testing.T) {
	m := &DockerModule{}
	if m.Name() != "docker" {
		t.Errorf("expected name 'docker', got %q", m.Name())
	}
	if m.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestDockerModule_Detect(t *testing.T) {
	m := &DockerModule{}
	detected, err := m.Detect()
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}
	_ = detected
}

func TestDockerModule_Inventory(t *testing.T) {
	m := &DockerModule{}
	result, err := m.Inventory(context.Background())
	if err != nil {
		t.Fatalf("Inventory() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Inventory() returned nil")
	}
	if result.Module != "docker" {
		t.Errorf("expected module name 'docker', got %q", result.Module)
	}
}

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"nginx:latest", "nginx_latest"},
		{"myrepo/myimage:v1.0", "myrepo_myimage_v1_0"},
		{"simple", "simple"},
		{"", ""},
	}
	for _, tt := range tests {
		got := sanitizeFileName(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeFileName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestDockerBackupManifest(t *testing.T) {
	if !dockerAvailableTest() {
		t.Skip("Docker not available, skipping backup test")
	}
	if testing.Short() {
		t.Skip("Skipping backup test in short mode (requires docker save)")
	}

	m := &DockerModule{}
	backupOpts := module.BackupOptions{SnapshotsDir: t.TempDir()}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := m.Backup(ctx, backupOpts)
	if err != nil {
		t.Fatalf("Backup() returned error: %v", err)
	}
	if result != nil {
		t.Logf("Backup result: module=%s, snapshots=%d", result.Module, len(result.Snapshots))
		for _, snap := range result.Snapshots {
			t.Logf("  snapshot: path=%s, size=%d, checksum=%s", snap.Path, snap.Size, snap.Checksum)
		}
	}
}

func dockerAvailableTest() bool {
	err := exec.Command("docker", "ps", "-a", "--format", "{{.ID}}").Run()
	return err == nil
}

func TestDockerRestoreValidateReport(t *testing.T) {
	r := &dockerValidationReport{
		Installed:        true,
		DaemonRunning:    true,
		ComposeInstalled: true,
		RestoredImages:   3,
		MissingImages:    []string{"missing:latest"},
		RestoredVolumes:  2,
	}
	if !r.Installed {
		t.Error("expected installed to be true")
	}
	if len(r.MissingImages) != 1 {
		t.Errorf("expected 1 missing image, got %d", len(r.MissingImages))
	}
}

func TestDockerRestoreAction(t *testing.T) {
	m := &DockerModule{}
	snap := module.Snapshot{
		Path: filepath.Join(t.TempDir(), "test-snapshot.tar"),
	}
	_ = os.WriteFile(snap.Path, []byte("test"), 0644)

	opts := module.RestoreOptions{
		BackupDir: t.TempDir(),
	}
	actions, err := m.Actions(context.Background(), snap, opts)
	if err != nil {
		t.Fatalf("Actions() returned error: %v", err)
	}
	if len(actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(actions))
	}
	if len(actions) > 0 {
		if actions[0].Name() != "extract_archive" {
			t.Errorf("expected first action to be extract_archive, got %q", actions[0].Name())
		}
		if actions[1].Name() != "docker_full_restore" {
			t.Errorf("expected second action to be docker_full_restore, got %q", actions[1].Name())
		}
	}
}

func TestDockerProviderInterface(t *testing.T) {
	m := &DockerModule{}
	if _, ok := interface{}(m).(actions.Provider); !ok {
		t.Error("DockerModule should implement actions.Provider")
	}
}

func TestHomeDir(t *testing.T) {
	home := homeDir(nil)
	if home == "" {
		t.Error("expected non-empty home directory")
	}
	if _, err := os.Stat(home); err != nil {
		t.Errorf("home directory %s is not accessible: %v", home, err)
	}
}
