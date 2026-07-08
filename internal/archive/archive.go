package archive

import (
	"archive/tar"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/klauspost/compress/zstd"
)

type Entry struct {
	Source      string
	ArchivePath string
}

type SnapshotInfo struct {
	Path         string
	Size         int64
	Checksum     string
	OriginalSize int64
	FileCount    int
}

func CreateSnapshot(snapshotsDir, moduleName string, entries []Entry) (*SnapshotInfo, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	if err := os.MkdirAll(snapshotsDir, 0700); err != nil {
		return nil, err
	}

	originalSize, fileCount := computeEntryStats(entries)

	snapshotPath := filepath.Join(snapshotsDir, moduleName+".tar.zst")
	if err := Create(snapshotPath, entries); err != nil {
		return nil, err
	}

	info, err := os.Stat(snapshotPath)
	if err != nil {
		return nil, err
	}

	checksum, err := fileChecksum(snapshotPath)
	if err != nil {
		return nil, err
	}

	return &SnapshotInfo{
		Path:         snapshotPath,
		Size:         info.Size(),
		Checksum:     checksum,
		OriginalSize: originalSize,
		FileCount:    fileCount,
	}, nil
}

func computeEntryStats(entries []Entry) (totalSize int64, fileCount int) {
	for _, e := range entries {
		info, err := os.Lstat(e.Source)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if info.IsDir() {
			filepath.Walk(e.Source, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if fi.Mode()&os.ModeSymlink != 0 {
					return nil
				}
				if !fi.IsDir() {
					totalSize += fi.Size()
					fileCount++
				}
				return nil
			})
		} else {
			totalSize += info.Size()
			fileCount++
		}
	}
	return
}

func Create(destPath string, entries []Entry) error {
	tmpPath := destPath + ".tmp"
	dst, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	cleanup := func() {
		dst.Close()
		os.Remove(tmpPath)
	}

	zw, err := zstd.NewWriter(dst, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		cleanup()
		return err
	}

	tw := tar.NewWriter(zw)

	for _, entry := range entries {
		if err := addToArchive(tw, entry.Source, entry.ArchivePath); err != nil {
			tw.Close()
			zw.Close()
			cleanup()
			return err
		}
	}

	if err := tw.Close(); err != nil {
		zw.Close()
		cleanup()
		return err
	}
	if err := zw.Close(); err != nil {
		cleanup()
		return err
	}
	if err := dst.Close(); err != nil {
		cleanup()
		return err
	}

	return os.Rename(tmpPath, destPath)
}

func Extract(srcPath, destDir string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	zr, err := zstd.NewReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()

	tr := tar.NewReader(zr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, filepath.FromSlash(header.Name))

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
			if err := os.Chtimes(target, time.Now(), header.ModTime); err != nil {
				return err
			}
		}
	}

	return nil
}

func addToArchive(tw *tar.Writer, source, archivePath string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			archiveName := filepath.Join(archivePath, rel)
			if err := writeTarEntry(tw, path, archiveName, info); err != nil {
				return err
			}
			return nil
		})
	}

	return writeTarEntry(tw, source, archivePath, info)
}

func writeTarEntry(tw *tar.Writer, source, archivePath string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(archivePath)
	header.Format = tar.FormatPAX

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if !info.IsDir() {
		f, err := os.Open(source)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	}

	return nil
}

type VerifyResult struct {
	FileCount    int
	OriginalSize int64
}

func VerifyReadable(path string) (*VerifyResult, error) {
	src, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer src.Close()

	zr, err := zstd.NewReader(src)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	tr := tar.NewReader(zr)
	var res VerifyResult
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag == tar.TypeReg {
			res.FileCount++
			res.OriginalSize += header.Size
		}
	}
	return &res, nil
}

func fileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
