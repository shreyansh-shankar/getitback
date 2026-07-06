package module

import (
	"context"
	"fmt"
	"sort"
)

type Manager struct {
	modules map[string]Module
	order   []string
}

func NewManager() *Manager {
	return &Manager{modules: make(map[string]Module)}
}

func (m *Manager) Register(mod Module) {
	name := mod.Name()
	if _, ok := m.modules[name]; ok {
		panic(fmt.Sprintf("module %q already registered", name))
	}
	m.modules[name] = mod
	m.order = append(m.order, name)
}

func (m *Manager) Get(name string) (Module, bool) {
	mod, ok := m.modules[name]
	return mod, ok
}

func (m *Manager) All() []Module {
	result := make([]Module, 0, len(m.modules))
	for _, name := range m.order {
		result = append(result, m.modules[name])
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result
}

func (m *Manager) Detect(ctx context.Context) map[string]DetectResult {
	result := make(map[string]DetectResult)
	for name, mod := range m.modules {
		ok, err := mod.Detect()
		r := DetectResult{Detected: ok}
		if err != nil {
			r.Err = err.Error()
		}
		result[name] = r
	}
	return result
}

type DetectResult struct {
	Detected bool   `json:"detected" yaml:"detected"`
	Err      string `json:"error,omitempty" yaml:"error,omitempty"`
}

func (m *Manager) Inventory(ctx context.Context) []*InventoryResult {
	var results []*InventoryResult
	for _, mod := range m.modules {
		ok, err := mod.Detect()
		if err != nil || !ok {
			continue
		}
		result, err := mod.Inventory(ctx)
		if err != nil {
			result = &InventoryResult{
				Module:  mod.Name(),
				Errors:  []string{err.Error()},
			}
		}
		results = append(results, result)
	}
	return results
}

func (m *Manager) Doctor(ctx context.Context) map[string]*DoctorResult {
	results := make(map[string]*DoctorResult)
	for name, mod := range m.modules {
		ok, err := mod.Detect()
		if err != nil || !ok {
			continue
		}
		result, err := mod.Doctor(ctx)
		if err != nil {
			result = &DoctorResult{
				Module: name,
				Status: DoctorStatusError,
				Issues: []DoctorIssue{{Severity: "error", Message: err.Error()}},
			}
		}
		if result == nil {
			result = &DoctorResult{
				Module: name,
				Status: DoctorStatusOK,
			}
		}
		results[name] = result
	}
	return results
}
