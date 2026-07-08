package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/module"
)

const (
	ManifestVersion  = "1.0"
	BackupFormatVersion = "2.0"
	InventoryVersion = "1.0"
	CompressionAlgo  = "zstd"
	CompressionLevel = 3
)

type Manifest struct {
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Hostname  string    `json:"hostname"`
	OS        string    `json:"os"`
	Snapshots []module.Snapshot      `json:"snapshots"`
	Inventory []*module.InventoryResult `json:"inventory,omitempty"`

	// New fields — added without breaking existing field names
	ManifestVersion  string `json:"manifest_version,omitempty"`
	BackupVersion    string `json:"backup_version,omitempty"`
	BackupID         string `json:"backup_id,omitempty"`
	MachineID        string `json:"machine_id,omitempty"`
	Platform         string `json:"platform,omitempty"`
	Architecture     string `json:"architecture,omitempty"`
	Compression      string `json:"compression,omitempty"`
	Encryption       string `json:"encryption,omitempty"`
	BackupSize       int64  `json:"backup_size,omitempty"`
	InventoryVersion string `json:"inventory_version,omitempty"`
}

type BackupEntry struct {
	ID            string    `json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	Hostname      string    `json:"hostname"`
	SnapshotCount int       `json:"snapshot_count"`
	Size          int64     `json:"size"`
}

type ModuleMeta struct {
	Module           string `json:"module"`
	Version          string `json:"version,omitempty"`
	ArchiveFile      string `json:"archiveFile"`
	OriginalSize     int64  `json:"originalSize"`
	CompressedSize   int64  `json:"compressedSize"`
	Checksum         string `json:"checksum"`
	FileCount        int    `json:"fileCount,omitempty"`
	Status           string `json:"status"`
	Duration         string `json:"duration,omitempty"`
	Created          string `json:"created"`
	Compression      string `json:"compression"`
	CompressionLevel int    `json:"compressionLevel"`
	RecoveryValue    string `json:"recoveryValue,omitempty"`
}

type BackupMeta struct {
	BackupID         string `json:"backupId"`
	CreatedAt        string `json:"createdAt"`
	Hostname         string `json:"hostname"`
	MachineID        string `json:"machineId,omitempty"`
	Platform         string `json:"platform"`
	Architecture     string `json:"architecture"`
	ModuleCount      int    `json:"moduleCount"`
	SnapshotCount    int    `json:"snapshotCount"`
	TotalOriginalSize int64 `json:"totalOriginalSize"`
	TotalCompressedSize int64 `json:"totalCompressedSize"`
	Compression      string `json:"compression"`
	CompressionRatio float64 `json:"compressionRatio,omitempty"`
	Encryption       string `json:"encryption"`
	Duration         string `json:"duration"`
}

func WriteManifest(dir string, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0600)
}

func ReadManifest(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func ListBackups(root string) ([]BackupEntry, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var backups []BackupEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m, err := ReadManifest(filepath.Join(root, entry.Name()))
		if err != nil {
			continue
		}

		var totalSize int64
		for _, snap := range m.Snapshots {
			totalSize += snap.Size
		}

		backups = append(backups, BackupEntry{
			ID:            entry.Name(),
			CreatedAt:     m.CreatedAt,
			Hostname:      m.Hostname,
			SnapshotCount: len(m.Snapshots),
			Size:          totalSize,
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

func LatestBackup(root string) (string, error) {
	backups, err := ListBackups(root)
	if err != nil {
		return "", err
	}
	if len(backups) == 0 {
		return "", nil
	}
	return backups[0].ID, nil
}

func WriteModuleMeta(metaDir string, meta *ModuleMeta) error {
	if err := os.MkdirAll(metaDir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(metaDir, meta.Module+".json"), data, 0600)
}

func WriteBackupMeta(metaDir string, meta *BackupMeta) error {
	if err := os.MkdirAll(metaDir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(metaDir, "backup.json"), data, 0600)
}

func WriteChecksums(dir string, snapshots []module.Snapshot) error {
	lines := ""
	for _, snap := range snapshots {
		if snap.Checksum != "" && snap.ArchiveFile != "" {
			lines += snap.Checksum + "  " + snap.ArchiveFile + "\n"
		}
	}
	if lines == "" {
		return nil
	}
	return os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(lines), 0600)
}
