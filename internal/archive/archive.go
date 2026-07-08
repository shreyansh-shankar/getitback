package archive

import (
	"archive/tar"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/klauspost/compress/zstd"
)

// TarZstdReader provides streaming read access to a .tar.zst archive.
// Callers iterate entries via Next() and read each entry's content from the
// ReadCloser returned by Next(). Close() must be called when done.
type TarZstdReader struct {
	src    *os.File
	zr     *zstd.Decoder
	tr     *tar.Reader
	closer io.ReadCloser
}

// OpenReader opens a .tar.zst file for streaming entry-by-entry access.
func OpenReader(srcPath string) (*TarZstdReader, error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return nil, err
	}
	zr, err := zstd.NewReader(src)
	if err != nil {
		src.Close()
		return nil, err
	}
	return &TarZstdReader{
		src: src,
		zr:  zr,
		tr:  tar.NewReader(zr),
	}, nil
}

// Next advances to the next entry and returns its header.
// Callers must read the entry body from r before calling Next again.
// Returns io.EOF when no more entries remain.
func (r *TarZstdReader) Next() (*tar.Header, error) {
	return r.tr.Next()
}

// Read reads from the current entry's body.
func (r *TarZstdReader) Read(p []byte) (int, error) {
	return r.tr.Read(p)
}

// Close releases all underlying resources.
func (r *TarZstdReader) Close() error {
	var errs []error
	if r.closer != nil {
		errs = append(errs, r.closer.Close())
	}
	r.zr.Close()
	errs = append(errs, r.src.Close())
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// WalkEntries iterates all entries in a .tar.zst archive, calling fn for each.
// The fn receives the tar header and a reader for the entry body.
// Return a non-nil error from fn to stop iteration early.
func WalkEntries(srcPath string, fn func(header *tar.Header, r io.Reader) error) error {
	r, err := OpenReader(srcPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for {
		header, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := fn(header, r); err != nil {
			return err
		}
	}
	return nil
}

// ExtractEntry extracts a single named entry from the archive to destDir.
// The entry name is matched against the archive path (header.Name).
// Returns os.ErrNotExist if the entry is not found.
func ExtractEntry(srcPath, entryName string, destDir string) error {
	found := false
	entryName = filepath.ToSlash(entryName)
	err := WalkEntries(srcPath, func(header *tar.Header, r io.Reader) error {
		if filepath.ToSlash(header.Name) != entryName {
			return nil
		}
		found = true
		return WriteEntry(header, r, destDir)
	})
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("entry %s: %w", entryName, os.ErrNotExist)
	}
	return nil
}

// ExtractMatching extracts all entries whose name matches the given prefix.
// For each matched entry, fn is called with the header and the destination path.
func ExtractMatching(srcPath, prefix string, destDir string) error {
	prefix = filepath.ToSlash(prefix)
	return WalkEntries(srcPath, func(header *tar.Header, r io.Reader) error {
		name := filepath.ToSlash(header.Name)
		if !strings.HasPrefix(name, prefix) {
			return nil
		}
		return WriteEntry(header, r, destDir)
	})
}

// ExtractWithPrefix extracts entries under a given prefix, stripping the prefix
// from the destination path. e.g., prefix "images/" with entry "images/foo.tar"
// writes to destDir + "/foo.tar".
func ExtractWithPrefix(srcPath, prefix, destDir string) error {
	prefix = filepath.ToSlash(prefix)
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return WalkEntries(srcPath, func(header *tar.Header, r io.Reader) error {
		name := filepath.ToSlash(header.Name)
		if !strings.HasPrefix(name, prefix) {
			return nil
		}
		rel := strings.TrimPrefix(name, prefix)
		sub := &tar.Header{
			Typeflag: header.Typeflag,
			Name:     rel,
			Mode:     header.Mode,
			Uid:      header.Uid,
			Gid:      header.Gid,
			Size:     header.Size,
			Linkname: header.Linkname,
			ModTime:  header.ModTime,
		}
		return WriteEntry(sub, r, destDir)
	})
}

func WriteEntry(header *tar.Header, r io.Reader, destDir string) error {
	target := filepath.Join(destDir, filepath.FromSlash(header.Name))
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, os.FileMode(header.Mode&07777))
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode&07777))
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, r); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		os.Chtimes(target, time.Now(), header.ModTime)
		return nil
	case tar.TypeSymlink:
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		os.Remove(target)
		return os.Symlink(header.Linkname, target)
	case tar.TypeLink:
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		linkTarget := filepath.Join(destDir, filepath.FromSlash(header.Linkname))
		os.Remove(target)
		return os.Link(linkTarget, target)
	}
	return nil
}

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
			fileCount++
			continue
		}
		if info.IsDir() {
			filepath.Walk(e.Source, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if fi.Mode()&os.ModeSymlink != 0 {
					fileCount++
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
	return extractWithOpts(srcPath, destDir, false)
}

func extractWithOpts(srcPath, destDir string, restoreOwnership bool) error {
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

	// Track created directories for permission/ownership fixup after files
	var dirs []string

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
			if err := os.MkdirAll(target, os.FileMode(header.Mode & 07777)); err != nil {
				return err
			}
			dirs = append(dirs, target)

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode & 07777))
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
			os.Chtimes(target, time.Now(), header.ModTime)

		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}

		case tar.TypeLink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			linkTarget := filepath.Join(destDir, filepath.FromSlash(header.Linkname))
			os.Remove(target)
			if err := os.Link(linkTarget, target); err != nil {
				return err
			}
		}
	}

	// Apply ownership and permissions in reverse order (files before dirs)
	if restoreOwnership && os.Geteuid() == 0 {
		// Re-open to iterate again for ownership
		src.Seek(0, 0)
		zr2, err := zstd.NewReader(src)
		if err != nil {
			return err
		}
		defer zr2.Close()
		tr2 := tar.NewReader(zr2)

		var entries []struct {
			target string
			uid    int
			gid    int
		}

		for {
			header, err := tr2.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			target := filepath.Join(destDir, filepath.FromSlash(header.Name))
			entries = append(entries, struct {
				target string
				uid    int
				gid    int
			}{target: target, uid: header.Uid, gid: header.Gid})
		}

		// Apply ownership from leaf to root
		for i := len(entries) - 1; i >= 0; i-- {
			e := entries[i]
			os.Chown(e.target, e.uid, e.gid)
		}
	} else if restoreOwnership {
		// Not root — try to map ownership intelligently
		src.Seek(0, 0)
		zr2, err := zstd.NewReader(src)
		if err != nil {
			return err
		}
		defer zr2.Close()
		tr2 := tar.NewReader(zr2)
		for {
			header, err := tr2.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			target := filepath.Join(destDir, filepath.FromSlash(header.Name))
			info, err := os.Stat(target)
			if err != nil {
				continue
			}
			// Ensure current user can access
			os.Chmod(target, info.Mode().Perm() | 0600)
		}
	}

	return nil
}

func addToArchive(tw *tar.Writer, source, archivePath string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return filepath.Walk(source, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// Use Lstat for each entry to handle symlinks
			lfi, lerr := os.Lstat(path)
			if lerr != nil {
				return lerr
			}
			rel, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			archiveName := filepath.Join(archivePath, rel)
			if err := writeTarEntry(tw, path, archiveName, lfi); err != nil {
				return err
			}
			return nil
		})
	}

	return writeTarEntry(tw, source, archivePath, info)
}

func writeTarEntry(tw *tar.Writer, source, archivePath string, info os.FileInfo) error {
	var link string
	if info.Mode()&os.ModeSymlink != 0 {
		var err error
		link, err = os.Readlink(source)
		if err != nil {
			return err
		}
	}

	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(archivePath)
	header.Format = tar.FormatPAX
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		header.Uid = int(stat.Uid)
		header.Gid = int(stat.Gid)
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if info.Mode().IsRegular() {
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
