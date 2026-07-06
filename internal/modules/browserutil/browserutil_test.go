package browserutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectFirefoxProfiles_RealSystem(t *testing.T) {
	info := DetectFirefoxProfiles()
	// The function should at least not crash
	_ = info
}

func TestDetectChromeProfiles_Standard(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "getitback-chrome-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	os.MkdirAll(filepath.Join(tmpDir, "Default"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "Profile 1"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "Profile 2"), 0755)

	info := DetectChromeProfiles(tmpDir)
	if !info.Available {
		t.Fatal("expected Available=true")
	}
	if info.Count != 3 {
		t.Errorf("expected Count=3, got %d", info.Count)
	}
	if info.Default == "" {
		t.Error("expected a default profile")
	}
}

func TestDetectChromeProfiles_LocalState(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "getitback-chrome-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Local State with profile info
	localState := `{"profile":{"info_cache":{"Default":{"name":"Default"},"Profile 1":{"name":"Personal"},"Profile 2":{"name":"Work"}}}}`
	os.WriteFile(filepath.Join(tmpDir, "Local State"), []byte(localState), 0644)

	// Create profile dirs
	os.MkdirAll(filepath.Join(tmpDir, "Default"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "Profile 1"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "Profile 2"), 0755)

	info := DetectChromeProfiles(tmpDir)
	if !info.Available {
		t.Fatal("expected Available=true")
	}
	if info.Count != 3 {
		t.Errorf("expected Count=3, got %d", info.Count)
	}
}

func TestDetectChromeProfiles_Empty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "getitback-chrome-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	info := DetectChromeProfiles(tmpDir)
	if info.Count != 0 {
		t.Errorf("expected Count=0, got %d", info.Count)
	}
	if info.Error != "directory exists but no profiles found" {
		t.Errorf("expected error about no profiles, got %q", info.Error)
	}
}

func TestDirSize(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "getitback-dirsize-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file with known size
	content := []byte("hello world")
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), content, 0644)

	size := DirSize(tmpDir)
	if size != int64(len(content)) {
		t.Errorf("expected size=%d, got %d", len(content), size)
	}
}
