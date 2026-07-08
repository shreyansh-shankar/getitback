package servicemgr

import (
	"github.com/shreyansh-shankar/getitback/internal/runtime/executor"
)

type Systemd struct {
	exec executor.Executor
}

func NewSystemd(e executor.Executor) *Systemd {
	return &Systemd{exec: e}
}

func (s *Systemd) Name() string { return "systemd" }

func (s *Systemd) Enable(name string) error {
	return s.exec.Run("systemctl", s.args("enable", name)...)
}

func (s *Systemd) Disable(name string) error {
	return s.exec.Run("systemctl", s.args("disable", name)...)
}

func (s *Systemd) Start(name string) error {
	return s.exec.Run("systemctl", s.args("start", name)...)
}

func (s *Systemd) Stop(name string) error {
	return s.exec.Run("systemctl", s.args("stop", name)...)
}

func (s *Systemd) Restart(name string) error {
	return s.exec.Run("systemctl", s.args("restart", name)...)
}

func (s *Systemd) Status(name string) (bool, error) {
	res := s.exec.RunCapture("systemctl", append(s.prefix(), "is-active", name)...)
	if res.Err != nil {
		return false, nil
	}
	return res.Stdout == "active\n", nil
}

func (s *Systemd) Reload() error {
	return s.exec.Run("systemctl", append(s.prefix(), "daemon-reload")...)
}

func (s *Systemd) prefix() []string {
	if executor.IsRoot() {
		return nil
	}
	return []string{"sudo"}
}

func (s *Systemd) args(action, unit string) []string {
	return append(s.prefix(), action, unit)
}

var _ ServiceManager = (*Systemd)(nil)
