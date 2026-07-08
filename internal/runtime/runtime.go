package runtime

import (
	"io"
	"log/slog"

	"github.com/shreyansh-shankar/getitback/internal/runtime/archivewrapper"
	"github.com/shreyansh-shankar/getitback/internal/runtime/download"
	"github.com/shreyansh-shankar/getitback/internal/runtime/env"
	"github.com/shreyansh-shankar/getitback/internal/runtime/executor"
	"github.com/shreyansh-shankar/getitback/internal/runtime/filesystem"
	"github.com/shreyansh-shankar/getitback/internal/runtime/pkgmgr"
	"github.com/shreyansh-shankar/getitback/internal/runtime/servicemgr"
)

type Runtime struct {
	OS       OSInfo
	Pkg      pkgmgr.PackageManager
	Service  servicemgr.ServiceManager
	Exec     executor.Executor
	FS       filesystem.FileSystem
	Archive  archivewrapper.ArchiveManager
	Download download.Downloader
	Env      env.EnvManager
	Progress ProgressWriter
	Logger   *slog.Logger
}

type ProgressWriter interface {
	Write([]byte) (int, error)
	Stage(stage, title string)
	ModuleSuccess(phase, name, detail string)
	ModuleSkip(phase, name, reason string)
	ModuleFailure(phase, name string, err error)
	DetailLine(format string, args ...any)
	InfoLine(format string, args ...any)
}

func New(w io.Writer, logger *slog.Logger) *Runtime {
	if logger == nil {
		logger = slog.Default()
	}

	osi := DetectOS()
	execEng := executor.NewExecutor(executor.Options{Logger: logger})

	return &Runtime{
		OS:       osi,
		Pkg:      pkgmgr.Detect(),
		Service:  servicemgr.Detect(execEng),
		Exec:     execEng,
		FS:       filesystem.NewFileSystem(execEng),
		Archive:  archivewrapper.New(),
		Download: download.New(execEng),
		Env:      env.NewEnvManager(),
		Logger:   logger,
	}
}
