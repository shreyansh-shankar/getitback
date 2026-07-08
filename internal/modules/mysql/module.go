package mysql

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

type MySQLModule struct{}

func NewModule() *MySQLModule { return &MySQLModule{} }

func (m *MySQLModule) Name() string        { return "mysql" }
func (m *MySQLModule) Description() string { return "MySQL or MariaDB database" }

func (m *MySQLModule) Detect() (bool, error) {
	if _, err := exec.LookPath("mysql"); err == nil {
		return true, nil
	}
	_, err := exec.LookPath("mariadb")
	return err == nil, nil
}

func (m *MySQLModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}
	meta := make(map[string]any)

	if ver, err := exec.Command("mysql", "--version").Output(); err == nil {
		result.Version = strings.TrimSpace(string(ver))
	} else if ver, err := exec.Command("mariadb", "--version").Output(); err == nil {
		result.Version = strings.TrimSpace(string(ver))
	}

	isMariaDB := false
	if out, err := exec.Command("mysql", "--version").Output(); err == nil {
		if strings.Contains(strings.ToLower(string(out)), "mariadb") {
			isMariaDB = true
			meta["flavor"] = "MariaDB"
		} else {
			meta["flavor"] = "MySQL"
		}
	}

	user := os.Getenv("USER")
	mysqlCmd := "mysql"
	if _, err := exec.LookPath("mariadb"); err == nil && !isMariaDB {
		mysqlCmd = "mariadb"
	}

	databases, err := listDatabases(mysqlCmd, user)
	if err == nil && len(databases) > 0 {
		meta["databases"] = databases
	}

	dataDir := findMySQLDataDir(mysqlCmd, user)
	if dataDir != "" {
		meta["dataDir"] = dataDir
		if info, err := os.Stat(dataDir); err == nil {
			meta["storage"] = info.Size()
		}
	}

	configFile := findMySQLConfig()
	if configFile != "" {
		meta["config"] = configFile
		result.Resources = append(result.Resources, module.Resource{
			Name: "my.cnf", Path: configFile, Type: module.ResourceTypeConfig,
		})
	}

	if len(meta) > 0 {
		result.Metadata = meta
	}
	return result, nil
}

func listDatabases(cmd, user string) ([]string, error) {
	out, err := exec.Command(cmd, "-u", user, "-e", "SHOW DATABASES", "--batch", "--skip-column-names").Output()
	if err != nil {
		return nil, err
	}
	var dbs []string
	for _, line := range strings.Split(string(out), "\n") {
		name := strings.TrimSpace(line)
		if name != "" && name != "information_schema" && name != "performance_schema" && name != "mysql" && name != "sys" {
			dbs = append(dbs, name)
		}
	}
	return dbs, nil
}

func findMySQLDataDir(cmd, user string) string {
	out, err := exec.Command(cmd, "-u", user, "-e", "SHOW VARIABLES LIKE 'datadir'", "--batch", "--skip-column-names").Output()
	if err != nil {
		common := []string{"/var/lib/mysql", "/var/lib/mariadb", "/usr/local/var/mysql"}
		for _, d := range common {
			if info, err := os.Stat(d); err == nil && info.IsDir() {
				return d
			}
		}
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			return fields[1]
		}
	}
	return ""
}

func findMySQLConfig() string {
	configs := []string{
		"/etc/mysql/my.cnf",
		"/etc/my.cnf",
		"~/.my.cnf",
	}
	home, _ := os.UserHomeDir()
	for _, c := range configs {
		p := strings.Replace(c, "~", home, 1)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

type mysqlBackupManifest struct {
	Databases  []string `json:"databases"`
	ConfigFile string   `json:"configFile,omitempty"`
	Flavor     string   `json:"flavor,omitempty"`
}

func (m *MySQLModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	user := os.Getenv("USER")

	mysqlCmd := "mysql"
	dumpCmd := "mysqldump"
	if _, err := exec.LookPath("mariadb"); err == nil {
		if _, err := exec.LookPath("mariadb-dump"); err == nil {
			mysqlCmd = "mariadb"
			dumpCmd = "mariadb-dump"
		}
	}

	databases, err := listDatabases(mysqlCmd, user)
	if err != nil {
		return nil, nil
	}
	if len(databases) == 0 {
		return nil, nil
	}

	var manifest mysqlBackupManifest
	manifest.Databases = databases

	if out, err := exec.Command(mysqlCmd, "--version").Output(); err == nil {
		if strings.Contains(strings.ToLower(string(out)), "mariadb") {
			manifest.Flavor = "MariaDB"
		} else {
			manifest.Flavor = "MySQL"
		}
	}

	dumpFile := filepath.Join(os.TempDir(), "getitback-mysql-dump.sql")
	dumpArgs := []string{"-u", user, "--all-databases", "--routines", "--triggers", "--events", "--result-file", dumpFile}
	if err := exec.Command(dumpCmd, dumpArgs...).Run(); err != nil {
		return nil, fmt.Errorf("mysql: dump failed: %w", err)
	}
	defer os.Remove(dumpFile)

	dumpInfo, err := os.Stat(dumpFile)
	if err != nil {
		return nil, fmt.Errorf("mysql: stat dump: %w", err)
	}
	if dumpInfo.Size() == 0 {
		return nil, fmt.Errorf("mysql: dump produced empty file")
	}

	var entries []archive.Entry
	configFile := findMySQLConfig()
	if configFile != "" {
		manifest.ConfigFile = configFile
		entries = append(entries, archive.Entry{
			Source: configFile, ArchivePath: "my.cnf",
		})
	}

	entries = append(entries, archive.Entry{
		Source: dumpFile, ArchivePath: "mysql-dump.sql",
	})

	tmpMeta, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("mysql: marshal manifest: %w", err)
	}
	metaFile := filepath.Join(os.TempDir(), "getitback-mysql-manifest.json")
	if err := os.WriteFile(metaFile, tmpMeta, 0600); err != nil {
		return nil, fmt.Errorf("mysql: write manifest: %w", err)
	}
	defer os.Remove(metaFile)
	entries = append(entries, archive.Entry{
		Source: metaFile, ArchivePath: "manifest.json",
	})

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), entries)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	contents := []string{fmt.Sprintf("database dump (%d databases)", len(databases))}
	if manifest.ConfigFile != "" {
		contents = append(contents, "MySQL config file")
	}
	return &module.BackupResult{
		Module:    m.Name(),
		Snapshots: []module.Snapshot{{
			Module: m.Name(), Path: snapshot.Path, Size: snapshot.Size, Checksum: snapshot.Checksum,
			OriginalSize: snapshot.OriginalSize, FileCount: snapshot.FileCount,
		}},
		Contents: contents,
	}, nil
}

func (m *MySQLModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	tmpDir, err := os.MkdirTemp(opts.WorkDir, "getitback-restore-mysql-*")
	if err != nil {
		return fmt.Errorf("mysql: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return fmt.Errorf("mysql: extract snapshot: %w", err)
	}

	var dumpFile string
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if strings.HasSuffix(path, ".sql") {
			dumpFile = path
		}
		return nil
	})

	if dumpFile == "" {
		return fmt.Errorf("mysql: no SQL dump found in snapshot")
	}

	user := os.Getenv("USER")
	cmd := exec.Command("mysql", "-u", user, "-f")
	cmd.Stdin, err = os.Open(dumpFile)
	if err != nil {
		return fmt.Errorf("mysql: open dump file: %w", err)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysql: restore failed: %w", err)
	}

	return nil
}

func (m *MySQLModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *MySQLModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}

	user := os.Getenv("USER")
	mysqlCmd := "mysql"
	if _, err := exec.LookPath("mariadb"); err == nil {
		mysqlCmd = "mariadb"
	}

	databases, err := listDatabases(mysqlCmd, user)
	if err != nil {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "error",
			Message:  fmt.Sprintf("Cannot connect to MySQL: %v", err),
			Help:     "Ensure MySQL is running and accessible without password",
		})
		result.Status = module.DoctorStatusError
		return result, nil
	}

	if len(databases) == 0 {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "info",
			Message:  "No user databases found",
			Help:     "Create a database with: CREATE DATABASE ...",
		})
	}

	if _, err := exec.LookPath("mysqldump"); err != nil {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "warning",
			Message:  "mysqldump not found — backups will not work",
			Help:     "Install mysqldump from the MySQL client package",
		})
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}
	return result, nil
}

func (m *MySQLModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "mysql-client", Hint: "MySQL client tools"},
	}
}

func (m *MySQLModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("mysql-client")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "mysql-client").Run()
}

func (m *MySQLModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	os.MkdirAll("/etc/mysql", 0755)
	return nil
}

func (m *MySQLModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("mysql")
	if restoreutil.CommandExists("mysql") {
		ver, err := restoreutil.CheckExecOutput("mysql", "--version")
		if err == nil {
			v.Version(strings.TrimSpace(ver))
		}
	} else if restoreutil.CommandExists("mariadb") {
		ver, err := restoreutil.CheckExecOutput("mariadb", "--version")
		if err == nil {
			v.Version(strings.TrimSpace(ver))
		}
	}
	v.Check(restoreutil.CommandExists("mysql") || restoreutil.CommandExists("mariadb"), "MySQL/MariaDB client installed")
	v.Check(restoreutil.CommandExists("mysqldump") || restoreutil.CommandExists("mariadb-dump"), "mysqldump installed")
	if restoreutil.CommandExists("mysql") {
		v.Recovered("MySQL client tools")
	}
	configFile, _ := restoreutil.CheckExecOutput("bash", "-c", "ls /etc/mysql/my.cnf /etc/my.cnf ~/.my.cnf 2>/dev/null | head -1")
	if configFile != "" {
		v.Recovered("MySQL config: " + configFile)
	}
	v.Confidence(80)
	return v.Result(), nil
}

func (m *MySQLModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	tmpDir := filepath.Join(os.TempDir(), "getitback-restore-mysql")
	return []actions.Action{
		&actions.CreateDirectory{Path: tmpDir, Mode: 0755},
		&actions.ExtractArchive{Source: snap.Path, Destination: tmpDir},
		&restoreUtilAction{
			name: "mysql_restore",
			desc: "Restore MySQL/MariaDB databases from dump",
			fn: func(ctx *runtime.RestoreContext) error {
				var dumpFile string
				filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() {
						return err
					}
					if strings.HasSuffix(path, ".sql") {
						dumpFile = path
					}
					return nil
				})
				if dumpFile == "" {
					return fmt.Errorf("mysql: no SQL dump found in snapshot")
				}
				user := os.Getenv("USER")
				mysqlCmd := "mysql"
				if _, err := exec.LookPath("mariadb"); err == nil {
					mysqlCmd = "mariadb"
				}
				bakFile := filepath.Join(tmpDir, "pre-restore.getitback-bak")
				exec.Command(mysqlCmd, "-u", user, "-e", fmt.Sprintf("mysqldump -u %s --all-databases --routines --triggers --events --result-file %s", user, bakFile)).Run()
				cmd := exec.Command(mysqlCmd, "-u", user, "-f")
				cmd.Stdin, _ = os.Open(dumpFile)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				return cmd.Run()
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

var _ actions.Provider = (*MySQLModule)(nil)
