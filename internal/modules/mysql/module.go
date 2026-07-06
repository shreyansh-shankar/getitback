package mysql

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/archive"
	"github.com/shreyansh-shankar/getitback/internal/module"
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

func (m *MySQLModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	user := os.Getenv("USER")
	databases, err := listDatabases("mysql", user)
	if err != nil {
		return nil, nil
	}
	if len(databases) == 0 {
		return nil, nil
	}

	tmpFile := filepath.Join(os.TempDir(), "getitback-mysql-dump.sql")
	args := []string{"-u", user, "--all-databases", "--result-file", tmpFile}
	if err := exec.Command("mysqldump", args...).Run(); err != nil {
		return nil, nil
	}
	defer os.Remove(tmpFile)

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), []archive.Entry{
		{Source: tmpFile, ArchivePath: "mysql-dump.sql"},
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
		}},
	}, nil
}

func (m *MySQLModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	tmpDir, err := os.MkdirTemp("", "getitback-restore-mysql-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return fmt.Errorf("extract snapshot: %w", err)
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
		return fmt.Errorf("no SQL dump found in snapshot")
	}

	user := os.Getenv("USER")
	cmd := exec.Command("mysql", "-u", user, "-f", "<", dumpFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
	return &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}, nil
}
