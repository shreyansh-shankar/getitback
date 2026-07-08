package actions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/runtime"
)

// InstallPackage installs system packages via the detected package manager.
type InstallPackage struct {
	BaseAction
	Packages []string
}

func (a *InstallPackage) Name() string { return "install_package" }

func (a *InstallPackage) Description() string {
	return fmt.Sprintf("Install %s", strings.Join(a.Packages, ", "))
}

func (a *InstallPackage) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.Pkg.Install(a.Packages...)
}

func (a *InstallPackage) EstimatedDuration() time.Duration {
	return time.Duration(len(a.Packages)) * 20 * time.Second
}

// RemovePackage removes system packages.
type RemovePackage struct {
	BaseAction
	Packages []string
}

func (a *RemovePackage) Name() string { return "remove_package" }

func (a *RemovePackage) Description() string {
	return fmt.Sprintf("Remove %s", strings.Join(a.Packages, ", "))
}

func (a *RemovePackage) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.Pkg.Remove(a.Packages...)
}

// ExtractArchive extracts an archive to a destination directory.
type ExtractArchive struct {
	BaseAction
	Source           string
	Destination      string
	StripComponents  int
}

func (a *ExtractArchive) Name() string { return "extract_archive" }

func (a *ExtractArchive) Description() string {
	return fmt.Sprintf("Extract %s → %s", filepath.Base(a.Source), a.Destination)
}

func (a *ExtractArchive) Execute(ctx *runtime.RestoreContext) error {
	if a.StripComponents > 0 {
		return ctx.Runtime.Archive.ExtractWithOptions(a.Source, a.Destination, a.StripComponents)
	}
	return ctx.Runtime.Archive.Extract(a.Source, a.Destination)
}

func (a *ExtractArchive) EstimatedDuration() time.Duration { return 15 * time.Second }

// CopyFile copies a single file from source to destination.
type CopyFile struct {
	BaseAction
	Source      string
	Destination string
	Mode        os.FileMode
}

func (a *CopyFile) Name() string { return "copy_file" }

func (a *CopyFile) Description() string {
	return fmt.Sprintf("Copy %s → %s", filepath.Base(a.Source), a.Destination)
}

func (a *CopyFile) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.FS.CopyFile(a.Source, a.Destination)
}

// CopyDirectory recursively copies a directory.
type CopyDirectory struct {
	BaseAction
	Source      string
	Destination string
}

func (a *CopyDirectory) Name() string { return "copy_directory" }

func (a *CopyDirectory) Description() string {
	return fmt.Sprintf("Copy directory %s → %s", filepath.Base(a.Source), a.Destination)
}

func (a *CopyDirectory) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.FS.CopyDir(a.Source, a.Destination)
}

// CreateDirectory creates a directory with the given permissions.
type CreateDirectory struct {
	BaseAction
	Path string
	Mode os.FileMode
}

func (a *CreateDirectory) Name() string { return "create_directory" }

func (a *CreateDirectory) Description() string {
	return fmt.Sprintf("Create directory %s", a.Path)
}

func (a *CreateDirectory) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.FS.MkdirAll(a.Path, a.Mode)
}

func (a *CreateDirectory) EstimatedDuration() time.Duration { return time.Second }

// RemoveDirectory removes a directory and its contents.
type RemoveDirectory struct {
	BaseAction
	Path string
}

func (a *RemoveDirectory) Name() string { return "remove_directory" }

func (a *RemoveDirectory) Description() string {
	return fmt.Sprintf("Remove directory %s", a.Path)
}

func (a *RemoveDirectory) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.FS.RemoveAll(a.Path)
}

// WriteFile writes data to a file with the given permissions.
type WriteFile struct {
	BaseAction
	Path string
	Data []byte
	Mode os.FileMode
}

func (a *WriteFile) Name() string { return "write_file" }

func (a *WriteFile) Description() string {
	return fmt.Sprintf("Write %s", filepath.Base(a.Path))
}

func (a *WriteFile) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.FS.WriteFile(a.Path, a.Data, a.Mode)
}

// CreateSymlink creates a symbolic link.
type CreateSymlink struct {
	BaseAction
	Target string
	Link   string
}

func (a *CreateSymlink) Name() string { return "create_symlink" }

func (a *CreateSymlink) Description() string {
	return fmt.Sprintf("Symlink %s → %s", a.Link, a.Target)
}

func (a *CreateSymlink) Execute(ctx *runtime.RestoreContext) error {
	os.Remove(a.Link)
	return ctx.Runtime.FS.Symlink(a.Target, a.Link)
}

// SetPermission changes file mode on a path.
type SetPermission struct {
	BaseAction
	Path string
	Mode os.FileMode
}

func (a *SetPermission) Name() string { return "set_permission" }

func (a *SetPermission) Description() string {
	return fmt.Sprintf("Set permissions %o on %s", a.Mode, a.Path)
}

func (a *SetPermission) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.FS.Chmod(a.Path, a.Mode)
}

// Chown changes owner and group on a path.
type Chown struct {
	BaseAction
	Path  string
	Owner string
	Group string
}

func (a *Chown) Name() string { return "chown" }

func (a *Chown) Description() string {
	return fmt.Sprintf("Chown %s → %s:%s", a.Path, a.Owner, a.Group)
}

func (a *Chown) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.FS.Chown(a.Path, a.Owner, a.Group)
}
