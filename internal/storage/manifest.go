package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/module"
)

const ManifestVersion = "1.0"

type Manifest struct {
	Version   string                 `json:"version"`
	CreatedAt time.Time              `json:"created_at"`
	Hostname  string                 `json:"hostname"`
	OS        string                 `json:"os"`
	Snapshots []module.Snapshot      `json:"snapshots"`
	Inventory []*module.InventoryResult `json:"inventory,omitempty"`
}

type BackupEntry struct {
	ID            string    `json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	Hostname      string    `json:"hostname"`
	SnapshotCount int       `json:"snapshot_count"`
	Size          int64     `json:"size"`
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
