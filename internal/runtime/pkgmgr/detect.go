package pkgmgr

import (
	"os/exec"

	"github.com/shreyansh-shankar/getitback/internal/runtime/executor"
)

func Detect() PackageManager {
	execEng := executor.NewExecutor(executor.Options{})
	if _, err := exec.LookPath("apt-get"); err == nil {
		return NewApt(execEng)
	}
	return NewApt(execEng)
}

func withSudo(args []string) []string {
	if executor.IsRoot() {
		return args
	}
	return append([]string{"sudo"}, args...)
}
