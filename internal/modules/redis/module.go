package redis

import (
	"context"
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

type RedisModule struct{}

func NewModule() *RedisModule { return &RedisModule{} }

func (m *RedisModule) Name() string        { return "redis" }
func (m *RedisModule) Description() string { return "Redis key-value store" }

func (m *RedisModule) Detect() (bool, error) {
	_, err := exec.LookPath("redis-cli")
	return err == nil, nil
}

func (m *RedisModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	if ver, err := exec.Command("redis-cli", "--version").Output(); err == nil {
		result.Version = strings.TrimSpace(string(ver))
	}

	ping, err := exec.Command("redis-cli", "PING").Output()
	if err == nil && strings.TrimSpace(string(ping)) == "PONG" {
		meta := map[string]any{"server": "running"}

		if info, err := exec.Command("redis-cli", "INFO", "server").Output(); err == nil {
			for _, line := range strings.Split(string(info), "\n") {
				if strings.HasPrefix(line, "redis_version:") {
					meta["redis_version"] = strings.TrimSpace(strings.TrimPrefix(line, "redis_version:"))
				}
				if strings.HasPrefix(line, "os:") {
					meta["host_os"] = strings.TrimSpace(strings.TrimPrefix(line, "os:"))
				}
			}
		}

		result.Metadata = meta
	}

	return result, nil
}

func (m *RedisModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	exec.Command("redis-cli", "SAVE").Run()

	possibleDumps := []string{
		"/var/lib/redis/dump.rdb",
		"/var/lib/redis/6379/dump.rdb",
	}

	var dumpFile string
	for _, path := range possibleDumps {
		if _, err := os.Stat(path); err == nil {
			dumpFile = path
			break
		}
	}

	if dumpFile == "" {
		return nil, nil
	}

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), []archive.Entry{
		{Source: dumpFile, ArchivePath: "dump.rdb"},
	})
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	return &module.BackupResult{
		Module: m.Name(),
		Snapshots: []module.Snapshot{{
			Module: m.Name(), Path: snapshot.Path, Size: snapshot.Size, Checksum: snapshot.Checksum,
			OriginalSize: snapshot.OriginalSize, FileCount: snapshot.FileCount,
		}},
	}, nil
}

func (m *RedisModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	tmpDir, err := os.MkdirTemp(opts.WorkDir, "getitback-restore-redis-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return fmt.Errorf("extract snapshot: %w", err)
	}

	var rdbFile string
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".rdb") {
			rdbFile = path
		}
		return nil
	})

	if rdbFile == "" {
		return fmt.Errorf("no RDB file found in snapshot")
	}

	data, err := os.ReadFile(rdbFile)
	if err != nil {
		return fmt.Errorf("read rdb file: %w", err)
	}

	redisDirs := []string{
		"/var/lib/redis",
		"/var/lib/redis/6379",
	}
	var destDir string
	for _, dir := range redisDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			destDir = dir
			break
		}
	}
	if destDir == "" {
		return fmt.Errorf("no redis data directory found; searched: %v", redisDirs)
	}

	destPath := filepath.Join(destDir, "dump.rdb")
	if _, err := os.Stat(destPath); err == nil {
		os.Rename(destPath, destPath+".getitback-bak")
	}

	if err := os.WriteFile(destPath, data, 0640); err != nil {
		return fmt.Errorf("write rdb to %s: %w", destPath, err)
	}

	return nil
}

func (m *RedisModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}

func (m *RedisModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *RedisModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "redis-tools", Hint: "Redis CLI tools"},
	}
}

func (m *RedisModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("redis-tools")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "redis-tools").Run()
}

func (m *RedisModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	dirs := []string{"/var/lib/redis", "/var/lib/redis/6379"}
	for _, dir := range dirs {
		os.MkdirAll(dir, 0755)
	}
	return nil
}

func (m *RedisModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("redis")
	if restoreutil.CommandExists("redis-cli") {
		ver, err := restoreutil.CheckExecOutput("redis-cli", "--version")
		if err == nil {
			v.Version(strings.TrimSpace(ver))
		}
	}
	v.Check(restoreutil.CommandExists("redis-cli"), "redis-cli installed")
	if restoreutil.CommandExists("redis-cli") {
		v.Recovered("Redis CLI tools")
	}
	redisDirs := []string{"/var/lib/redis", "/var/lib/redis/6379"}
	var hasDir bool
	for _, dir := range redisDirs {
		if restoreutil.DirExists(dir) {
			hasDir = true
			if restoreutil.FileExists(filepath.Join(dir, "dump.rdb")) {
				v.Recovered("dump.rdb in " + dir)
			}
		}
	}
	v.Check(hasDir, "Redis data directory exists")
	v.Confidence(80)
	return v.Result(), nil
}

func (m *RedisModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	tmpDir := filepath.Join(os.TempDir(), "getitback-restore-redis")
	return []actions.Action{
		&actions.CreateDirectory{Path: tmpDir, Mode: 0755},
		&actions.ExtractArchive{Source: snap.Path, Destination: tmpDir},
		&restoreUtilAction{
			name: "redis_restore",
			desc: "Restore Redis RDB dump",
			fn: func(ctx *runtime.RestoreContext) error {
				var rdbFile string
				filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() && strings.HasSuffix(path, ".rdb") {
						rdbFile = path
					}
					return nil
				})
				if rdbFile == "" {
					return fmt.Errorf("no RDB file found in snapshot")
				}
				data, err := os.ReadFile(rdbFile)
				if err != nil {
					return fmt.Errorf("read rdb file: %w", err)
				}
				redisDirs := []string{"/var/lib/redis", "/var/lib/redis/6379"}
				var destDir string
				for _, dir := range redisDirs {
					if info, err := os.Stat(dir); err == nil && info.IsDir() {
						destDir = dir
						break
					}
				}
				if destDir == "" {
					return fmt.Errorf("no redis data directory found; searched: %v", redisDirs)
				}
				destPath := filepath.Join(destDir, "dump.rdb")
				if _, err := os.Stat(destPath); err == nil {
					os.Rename(destPath, destPath+".getitback-bak")
				}
				if err := os.WriteFile(destPath, data, 0640); err != nil {
					return fmt.Errorf("write rdb to %s: %w", destPath, err)
				}
				return nil
			},
		},
	}, nil
}

type restoreUtilAction struct {
	actions.BaseAction
	name string
	desc string
	fn   func(ctx *runtime.RestoreContext) error
}

func (a *restoreUtilAction) Name() string        { return a.name }
func (a *restoreUtilAction) Description() string  { return a.desc }
func (a *restoreUtilAction) Execute(ctx *runtime.RestoreContext) error { return a.fn(ctx) }

var _ actions.Provider = (*RedisModule)(nil)
