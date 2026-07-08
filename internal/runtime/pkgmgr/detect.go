package pkgmgr

import (
	"os/exec"

	"github.com/shreyansh-shankar/getitback/internal/runtime/executor"
)

func Detect() PackageManager {
	if _, err := exec.LookPath("apt-get"); err == nil {
		return NewApt(executor.Executor{})
	}
	return NewApt(executor.Executor{})
}

func withSudo(args []string) []string {
	if executor.IsRoot() {
		return args
	}
	return append([]string{"sudo"}, args...)
}
