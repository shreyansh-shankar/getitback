package servicemgr

type ServiceManager interface {
	Enable(name string) error
	Disable(name string) error
	Start(name string) error
	Stop(name string) error
	Restart(name string) error
	Status(name string) (active bool, err error)
	Reload() error
	Name() string
}

type Type string

const (
	TypeSystemd Type = "systemd"
)
