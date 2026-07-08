package archivewrapper

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/archive"
)

type ArchiveManager struct{}

func New() ArchiveManager {
	return ArchiveManager{}
}

// Extract decompresses a .tar.zst archive into destDir.
func (a ArchiveManager) Extract(archivePath, destDir string) error {
	return archive.Extract(archivePath, destDir)
}

// ExtractWithOptions extracts an archive, optionally stripping leading components.
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

// WalkEntries iterates archive entries without extracting to disk.
func (a ArchiveManager) WalkEntries(srcPath string, fn func(header *tar.Header, r io.Reader) error) error {
	return archive.WalkEntries(srcPath, fn)
}

// ExtractEntry extracts a single named entry from the archive.
func (a ArchiveManager) ExtractEntry(srcPath, entryName, destDir string) error {
	return archive.ExtractEntry(srcPath, entryName, destDir)
}

// ExtractMatching extracts all entries whose name starts with prefix.
func (a ArchiveManager) ExtractMatching(srcPath, prefix, destDir string) error {
	return archive.ExtractMatching(srcPath, prefix, destDir)
}

// ExtractWithPrefix extracts entries under prefix, stripping prefix from dest path.
func (a ArchiveManager) ExtractWithPrefix(srcPath, prefix, destDir string) error {
	return archive.ExtractWithPrefix(srcPath, prefix, destDir)
}

// OpenReader returns a streaming reader for entry-by-entry access.
func (a ArchiveManager) OpenReader(srcPath string) (*archive.TarZstdReader, error) {
	return archive.OpenReader(srcPath)
}
