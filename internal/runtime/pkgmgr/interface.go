package pkgmgr

type PackageManager interface {
	Install(packages ...string) error
	Remove(packages ...string) error
	IsInstalled(pkg string) bool
	Update() error
	AddRepository(name, url, keyFile string) error
	Name() string
}

type Type string

const (
	TypeApt Type = "apt"
)
