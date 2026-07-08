package java

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/archive"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/runtime"
	"github.com/shreyansh-shankar/getitback/internal/runtime/actions"
	"github.com/shreyansh-shankar/getitback/internal/runtime/restoreutil"
)

type JavaModule struct{}

func NewModule() *JavaModule { return &JavaModule{} }

func (m *JavaModule) Name() string        { return "java" }
func (m *JavaModule) Description() string { return "Java runtime and build tools" }

func (m *JavaModule) Detect() (bool, error) {
	return restoreutil.CommandExists("java"), nil
}

func (m *JavaModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true, Metadata: make(map[string]any)}

	if ver, err := restoreutil.CheckExecOutput("java", "-version"); err == nil {
		for _, line := range strings.Split(ver, "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "version") || strings.Contains(line, "openjdk") || strings.Contains(line, "java") {
				result.Version = strings.TrimSpace(line)
				break
			}
		}
	}

	if restoreutil.CommandExists("javac") {
		result.Metadata["jdk"] = true
		if home := javaHome(); home != "" {
			result.Metadata["javaHome"] = home
		}
	} else {
		result.Metadata["jre"] = true
	}

	if restoreutil.CommandExists("gradle") {
		result.Metadata["gradle"] = true
		if home := os.Getenv("GRADLE_HOME"); home != "" {
			result.Metadata["gradleHome"] = home
		}
		gradleCache := filepath.Join(restoreutil.HomeDir(), ".gradle", "caches")
		if restoreutil.DirExists(gradleCache) {
			var size int64
			filepath.Walk(gradleCache, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				size += info.Size()
				return nil
			})
			result.Metadata["gradleCache"] = size
		}
	}

	if restoreutil.CommandExists("mvn") {
		result.Metadata["maven"] = true
		if ver, err := restoreutil.CheckExecOutput("mvn", "--version"); err == nil {
			first := strings.SplitN(ver, "\n", 2)[0]
			result.Metadata["mavenVersion"] = stripANSI(strings.TrimSpace(first))
		}
		mavenRepo := filepath.Join(restoreutil.HomeDir(), ".m2", "repository")
		if restoreutil.DirExists(mavenRepo) {
			var size int64
			filepath.Walk(mavenRepo, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				size += info.Size()
				return nil
			})
			result.Metadata["mavenCache"] = size
		}
	}

	if restoreutil.CommandExists("sbt") {
		result.Metadata["sbt"] = true
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

type javaBackupManifest struct {
	HasJDK      bool   `json:"hasJDK"`
	JAVAHome    string `json:"javaHome,omitempty"`
	HasGradle   bool   `json:"hasGradle"`
	HasMaven    bool   `json:"hasMaven"`
	MavenVer    string `json:"mavenVersion,omitempty"`
	HasSBT      bool   `json:"hasSBT"`
	SettingsXML string `json:"mavenSettingsXML,omitempty"`
	GradleProps string `json:"gradleProperties,omitempty"`
}

func (m *JavaModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	var manifest javaBackupManifest
	var entries []archive.Entry
	home := restoreutil.HomeDir()

	if restoreutil.CommandExists("javac") {
		manifest.HasJDK = true
		manifest.JAVAHome = javaHome()
	}

	mavenDir := filepath.Join(home, ".m2")
	settingsXML := filepath.Join(mavenDir, "settings.xml")
	if restoreutil.FileExists(settingsXML) {
		manifest.SettingsXML = settingsXML
		entries = append(entries, archive.Entry{
			Source: settingsXML, ArchivePath: "maven-settings.xml",
		})
	}

	gradleDir := filepath.Join(home, ".gradle")
	gradleProps := filepath.Join(gradleDir, "gradle.properties")
	if restoreutil.FileExists(gradleProps) {
		manifest.GradleProps = gradleProps
		entries = append(entries, archive.Entry{
			Source: gradleProps, ArchivePath: "gradle.properties",
		})
	}

	gradleConfig := filepath.Join(gradleDir, "init.gradle")
	if restoreutil.FileExists(gradleConfig) {
		entries = append(entries, archive.Entry{
			Source: gradleConfig, ArchivePath: "init.gradle",
		})
	}

	if restoreutil.CommandExists("gradle") {
		manifest.HasGradle = true
	}
	if restoreutil.CommandExists("mvn") {
		manifest.HasMaven = true
		if ver, err := restoreutil.CheckExecOutput("mvn", "--version"); err == nil {
			manifest.MavenVer = stripANSI(strings.TrimSpace(strings.SplitN(ver, "\n", 2)[0]))
		}
	}
	if restoreutil.CommandExists("sbt") {
		manifest.HasSBT = true
		sbtConfig := filepath.Join(home, ".sbt")
		if restoreutil.DirExists(sbtConfig) {
			entries = append(entries, archive.Entry{
				Source: sbtConfig, ArchivePath: "sbt",
			})
		}
	}

	tmpMeta, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("java: marshal manifest: %w", err)
	}
	metaFile := filepath.Join(os.TempDir(), "getitback-java-manifest.json")
	if err := os.WriteFile(metaFile, tmpMeta, 0600); err != nil {
		return nil, fmt.Errorf("java: write manifest: %w", err)
	}
	defer os.Remove(metaFile)
	entries = append(entries, archive.Entry{
		Source: metaFile, ArchivePath: "manifest.json",
	})

	if len(entries) == 0 {
		return nil, nil
	}

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), entries)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	contents := []string{}
	if manifest.HasJDK {
		contents = append(contents, "JDK detection")
	}
	if manifest.SettingsXML != "" {
		contents = append(contents, "Maven settings")
	}
	if manifest.GradleProps != "" {
		contents = append(contents, "Gradle configuration")
	}
	if manifest.HasSBT {
		contents = append(contents, "SBT configuration")
	}
	if manifest.HasGradle {
		contents = append(contents, "Gradle tool")
	}
	if manifest.HasMaven {
		contents = append(contents, "Maven tool")
	}
	return &module.BackupResult{
		Module: m.Name(),
		Snapshots: []module.Snapshot{{
			Module: m.Name(), Path: snapshot.Path, Size: snapshot.Size, Checksum: snapshot.Checksum,
			OriginalSize: snapshot.OriginalSize, FileCount: snapshot.FileCount,
		}},
		Contents: contents,
	}, nil
}

func (m *JavaModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}

	tmpDir, err := os.MkdirTemp(opts.WorkDir, "getitback-restore-java-*")
	if err != nil {
		return fmt.Errorf("java: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if rt != nil {
		rt.Archive.Extract(snap.Path, tmpDir)
	} else {
		archive.Extract(snap.Path, tmpDir)
	}

	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(tmpDir, path)
		switch rel {
		case "maven-settings.xml":
			dst := filepath.Join(home, ".m2", "settings.xml")
			os.MkdirAll(filepath.Dir(dst), 0755)
			if restoreutil.FileExists(dst) {
				os.Rename(dst, dst+".getitback-bak")
			}
			data, _ := os.ReadFile(path)
			os.WriteFile(dst, data, 0644)
		case "gradle.properties":
			dst := filepath.Join(home, ".gradle", "gradle.properties")
			os.MkdirAll(filepath.Dir(dst), 0755)
			if restoreutil.FileExists(dst) {
				os.Rename(dst, dst+".getitback-bak")
			}
			data, _ := os.ReadFile(path)
			os.WriteFile(dst, data, 0644)
		case "init.gradle":
			dst := filepath.Join(home, ".gradle", "init.gradle")
			os.MkdirAll(filepath.Dir(dst), 0755)
			data, _ := os.ReadFile(path)
			os.WriteFile(dst, data, 0644)
		default:
			if strings.HasPrefix(rel, "sbt") {
				dst := filepath.Join(home, rel)
				os.MkdirAll(filepath.Dir(dst), 0755)
				data, _ := os.ReadFile(path)
				os.WriteFile(dst, data, 0644)
			}
		}
		return nil
	})

	return nil
}

func (m *JavaModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *JavaModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}

	if !restoreutil.CommandExists("java") {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "error",
			Message:  "Java not installed",
			Help:     "Install JDK from https://adoptium.net or your package manager",
		})
		result.Status = module.DoctorStatusError
		return result, nil
	}

	home := os.Getenv("JAVA_HOME")
	if home == "" {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "info",
			Message:  "JAVA_HOME is not set",
			Help:     "Set JAVA_HOME to your JDK installation directory",
		})
	}

	homeDir := restoreutil.HomeDir()
	mavenSettings := filepath.Join(homeDir, ".m2", "settings.xml")
	if !restoreutil.FileExists(mavenSettings) {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "info",
			Message:  "No Maven settings.xml found",
			Help:     "Create ~/.m2/settings.xml for Maven configuration",
		})
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}
	return result, nil
}

func (m *JavaModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "java", Hint: "Java Runtime"},
		{Type: module.DepSystemPkg, Package: "mvn", Hint: "Maven build tool", Optional: true},
		{Type: module.DepSystemPkg, Package: "gradle", Hint: "Gradle build tool", Optional: true},
	}
}

func (m *JavaModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("default-jdk")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "default-jdk").Run()
}

func (m *JavaModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	home := restoreutil.HomeDir()
	os.MkdirAll(filepath.Join(home, ".m2"), 0755)
	os.MkdirAll(filepath.Join(home, ".gradle"), 0755)
	return nil
}

func (m *JavaModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("java")

	ver, err := restoreutil.CheckExecOutput("java", "-version")
	if err == nil {
		for _, line := range strings.Split(ver, "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "version") || strings.Contains(line, "openjdk") || strings.Contains(line, "java") {
				v.Version(strings.TrimSpace(line))
				break
			}
		}
	}
	v.Check(restoreutil.CommandExists("java"), "Java runtime installed")
	v.Check(restoreutil.CommandExists("mvn"), "Maven available")
	v.Check(restoreutil.CommandExists("gradle"), "Gradle available")

	home := restoreutil.HomeDir()
	m2Dir := filepath.Join(home, ".m2")
	if restoreutil.DirExists(m2Dir) {
		v.Recovered("Maven .m2 directory")
	} else {
		v.Missing("Maven .m2 directory")
	}

	settingsXML := filepath.Join(m2Dir, "settings.xml")
	if restoreutil.FileExists(settingsXML) {
		v.Recovered("Maven settings.xml")
	} else {
		v.Missing("Maven settings.xml")
	}

	gradleDir := filepath.Join(home, ".gradle")
	if restoreutil.DirExists(gradleDir) {
		v.Recovered("Gradle .gradle directory")
	} else {
		v.Missing("Gradle .gradle directory")
	}

	gradleProps := filepath.Join(gradleDir, "gradle.properties")
	if restoreutil.FileExists(gradleProps) {
		v.Recovered("Gradle properties")
	} else {
		v.Warn("Gradle properties not restored")
	}

	v.Confidence(90)
	return v.Result(), nil
}

func (m *JavaModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}
	m2Dir := filepath.Join(home, ".m2")
	gradleDir := filepath.Join(home, ".gradle")

	return []actions.Action{
		&actions.CreateDirectory{Path: m2Dir, Mode: 0755},
		&actions.CreateDirectory{Path: gradleDir, Mode: 0755},
		&restoreUtilAction{
			name: "restore_java_configs",
			desc: "Restore Maven and Gradle configuration files",
			fn: func(ctx *runtime.RestoreContext) error {
				tmpDir, err := os.MkdirTemp("", "getitback-restore-java-*")
				if err != nil {
					return fmt.Errorf("java: create temp dir: %w", err)
				}
				defer os.RemoveAll(tmpDir)

				if ctx.Runtime != nil {
					ctx.Runtime.Archive.Extract(snap.Path, tmpDir)
				} else {
					archive.Extract(snap.Path, tmpDir)
				}

				return filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() {
						return err
					}
					rel, _ := filepath.Rel(tmpDir, path)
					switch rel {
					case "maven-settings.xml":
						dst := filepath.Join(home, ".m2", "settings.xml")
						os.MkdirAll(filepath.Dir(dst), 0755)
						if restoreutil.FileExists(dst) {
							os.Rename(dst, dst+".getitback-bak")
						}
						data, _ := os.ReadFile(path)
						os.WriteFile(dst, data, 0644)
					case "gradle.properties":
						dst := filepath.Join(home, ".gradle", "gradle.properties")
						os.MkdirAll(filepath.Dir(dst), 0755)
						if restoreutil.FileExists(dst) {
							os.Rename(dst, dst+".getitback-bak")
						}
						data, _ := os.ReadFile(path)
						os.WriteFile(dst, data, 0644)
					case "init.gradle":
						dst := filepath.Join(home, ".gradle", "init.gradle")
						os.MkdirAll(filepath.Dir(dst), 0755)
						data, _ := os.ReadFile(path)
						os.WriteFile(dst, data, 0644)
					default:
						if strings.HasPrefix(rel, "sbt") {
							dst := filepath.Join(home, rel)
							os.MkdirAll(filepath.Dir(dst), 0755)
							data, _ := os.ReadFile(path)
							os.WriteFile(dst, data, 0644)
						}
					}
					return nil
				})
			},
		},
	}, nil
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

type restoreUtilAction struct {
	actions.BaseAction
	name string
	desc string
	fn   func(ctx *runtime.RestoreContext) error
}

func (a *restoreUtilAction) Name() string                        { return a.name }
func (a *restoreUtilAction) Description() string                  { return a.desc }
func (a *restoreUtilAction) Execute(ctx *runtime.RestoreContext) error { return a.fn(ctx) }

var _ actions.Provider = (*JavaModule)(nil)
