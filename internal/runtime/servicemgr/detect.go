package servicemgr

import (
	"os/exec"

	"github.com/shreyansh-shankar/getitback/internal/runtime/executor"
)

func Detect(e executor.Executor) ServiceManager {
	if _, err := exec.LookPath("systemctl"); err == nil {
		return NewSystemd(e)
	}
	return NewSystemd(e)
}
