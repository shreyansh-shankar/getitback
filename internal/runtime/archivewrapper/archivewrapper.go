package archivewrapper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/archive"
)

type ArchiveManager struct{}

func New() ArchiveManager {
	return ArchiveManager{}
}

func (a ArchiveManager) Extract(archivePath, destDir string) error {
	return archive.Extract(archivePath, destDir)
}

func (a ArchiveManager) ExtractWithOptions(archivePath, destDir string, stripComponents int) error {
	if stripComponents <= 0 {
		return archive.Extract(archivePath, destDir)
	}
	tmpDir, err := os.MkdirTemp("", "archive-strip-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(archivePath, tmpDir); err != nil {
		return fmt.Errorf("extract to temp: %w", err)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		src := filepath.Join(tmpDir, entry.Name())
		dst := filepath.Join(destDir, entry.Name())
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("move %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func (a ArchiveManager) Verify(archivePath string) error {
	ext := strings.ToLower(filepath.Ext(archivePath))
	switch ext {
	case ".zst", ".zstd":
		return nil
	case ".gz", ".gzip":
		return nil
	default:
		return fmt.Errorf("unsupported archive format: %s", ext)
	}
}
