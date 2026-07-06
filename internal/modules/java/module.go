package java

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/module"
)

type JavaModule struct{}

func NewModule() *JavaModule { return &JavaModule{} }

func (m *JavaModule) Name() string        { return "java" }
func (m *JavaModule) Description() string { return "Java runtime and build tools" }

func (m *JavaModule) Detect() (bool, error) {
	if _, err := exec.LookPath("java"); err == nil {
		return true, nil
	}
	if _, err := exec.LookPath("javac"); err == nil {
		return true, nil
	}
	return false, nil
}

func (m *JavaModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}
	meta := make(map[string]any)

	if ver, err := exec.Command("java", "-version").CombinedOutput(); err == nil {
		for _, line := range strings.Split(string(ver), "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "version") || strings.Contains(line, "openjdk") || strings.Contains(line, "java") {
				result.Version = strings.TrimSpace(line)
				break
			}
		}
	}

	if _, err := exec.LookPath("javac"); err == nil {
		meta["jdk"] = true
		home := javaHome()
		if home != "" {
			meta["javaHome"] = home
		}
	} else {
		meta["jre"] = true
	}

	if _, err := exec.LookPath("gradle"); err == nil {
		meta["gradle"] = true
		if home := os.Getenv("GRADLE_HOME"); home != "" {
			meta["gradleHome"] = home
		}
		gradleCache := filepath.Join(func() string { h, _ := os.UserHomeDir(); return h }(), ".gradle", "caches")
		if info, err := os.Stat(gradleCache); err == nil && info.IsDir() {
			var size int64
			filepath.Walk(gradleCache, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				size += info.Size()
				return nil
			})
			meta["gradleCache"] = size
		}
	}

	if _, err := exec.LookPath("mvn"); err == nil {
		meta["maven"] = true
		if ver, err := exec.Command("mvn", "--version").Output(); err == nil {
			first := strings.SplitN(string(ver), "\n", 2)[0]
			meta["mavenVersion"] = stripANSI(strings.TrimSpace(first))
		}
		mavenRepo := filepath.Join(func() string { h, _ := os.UserHomeDir(); return h }(), ".m2", "repository")
		if info, err := os.Stat(mavenRepo); err == nil && info.IsDir() {
			var size int64
			filepath.Walk(mavenRepo, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				size += info.Size()
				return nil
			})
			meta["mavenCache"] = size
		}
	}

	if _, err := exec.LookPath("sbt"); err == nil {
		meta["sbt"] = true
	}

	if len(meta) > 0 {
		result.Metadata = meta
	}
	return result, nil
}

func javaHome() string {
	home := os.Getenv("JAVA_HOME")
	if home != "" {
		return home
	}
	out, err := exec.Command("java", "-XshowSettings:properties", "-version").CombinedOutput()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "java.home") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func (m *JavaModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	return nil, nil
}

func (m *JavaModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	return nil
}

func (m *JavaModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *JavaModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}, nil
}

func stripANSI(s string) string {
	var out []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		out = append(out, s[i])
	}
	return string(out)
}
