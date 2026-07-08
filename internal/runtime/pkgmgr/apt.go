package pkgmgr

import (
	"os/exec"

	"github.com/shreyansh-shankar/getitback/internal/runtime/executor"
)

type Apt struct {
	Executor executor.Executor
}

func NewApt(exec executor.Executor) *Apt {
	return &Apt{Executor: exec}
}

func (a *Apt) Name() string { return "apt" }

func (a *Apt) Install(packages ...string) error {
	args := append([]string{"install", "-y"}, packages...)
	cmd := a.Executor.Command("apt-get", withSudo(args)...)
	return cmd.Run()
}

func (a *Apt) Remove(packages ...string) error {
	args := append([]string{"remove", "-y"}, packages...)
	cmd := a.Executor.Command("apt-get", withSudo(args)...)
	return cmd.Run()
}

func (a *Apt) IsInstalled(pkg string) bool {
	err := exec.Command("dpkg", "-s", pkg).Run()
	return err == nil
}

func (a *Apt) Update() error {
	cmd := a.Executor.Command("apt-get", withSudo([]string{"update"})...)
	return cmd.Run()
}

func (a *Apt) AddRepository(name, url, keyFile string) error {
	cmd := a.Executor.Command("add-apt-repository", withSudo([]string{"-y", url})...)
	return cmd.Run()
}
