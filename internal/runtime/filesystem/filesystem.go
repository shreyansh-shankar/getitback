package filesystem

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/runtime/executor"
)

type FileSystem struct {
	exec executor.Executor
}

func NewFileSystem(exec executor.Executor) FileSystem {
	return FileSystem{exec: exec}
}

func (fs FileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (fs FileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (fs FileSystem) CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return dstFile.Sync()
}

func (fs FileSystem) CopyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		return fs.CopyFile(path, target)
	})
}

func (fs FileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

func (fs FileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (fs FileSystem) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (fs FileSystem) IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func (fs FileSystem) Chown(path, owner, group string) error {
	return fs.exec.Run("chown", withSudo(owner+":"+group, path)...)
}

func (fs FileSystem) Chmod(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

func (fs FileSystem) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

func (fs FileSystem) Readlink(name string) (string, error) {
	return os.Readlink(name)
}

func (fs FileSystem) HomeDir() string {
	return os.Getenv("HOME")
}

func (fs FileSystem) ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(fs.HomeDir(), path[2:])
	}
	if path == "~" {
		return fs.HomeDir()
	}
	return path
}

func withSudo(args ...string) []string {
	if executor.IsRoot() {
		return args
	}
	return append([]string{"sudo"}, args...)
}
