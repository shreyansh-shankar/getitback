package module

var global = NewManager()

func Register(mod Module) {
	global.Register(mod)
}

func Global() *Manager {
	return global
}
