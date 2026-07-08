package restore

import (
	"context"
	"fmt"

	"github.com/shreyansh-shankar/getitback/internal/module"
)

type DependencyResolver struct {
	manager  *module.Manager
	selected map[string]bool
}

func NewDependencyResolver(manager *module.Manager, selected []string) *DependencyResolver {
	s := make(map[string]bool, len(selected))
	for _, name := range selected {
		s[name] = true
	}
	return &DependencyResolver{manager: manager, selected: s}
}

func (r *DependencyResolver) Resolve(ctx context.Context) ([]string, []module.Dependency, []string, error) {
	visited := make(map[string]bool)
	order := make([]string, 0)
	allDeps := make([]module.Dependency, 0)
	manualSteps := make([]string, 0)
	inProgress := make(map[string]bool)

	// Deduplication sets
	seenPkgs := make(map[string]bool)
	seenDeps := make(map[string]bool)

	var resolve func(name string) error
	resolve = func(name string) error {
		if visited[name] {
			return nil
		}
		if inProgress[name] {
			return fmt.Errorf("dependency cycle detected involving module %q", name)
		}
		inProgress[name] = true

		mod, ok := r.manager.Get(name)
		if !ok {
			inProgress[name] = false
			visited[name] = true
			return nil
		}

		dp, ok := mod.(module.DependencyProvider)
		if ok {
			deps := dp.Dependencies(ctx)
			for _, dep := range deps {
				// Deduplicate system packages
				if dep.Type == module.DepSystemPkg {
					if seenPkgs[dep.Package] {
						continue
					}
					seenPkgs[dep.Package] = true
				}
				// Deduplicate downloads by URL
				if dep.Type == module.DepDownload {
					key := dep.URL + dep.Hint
					if seenDeps[key] {
						continue
					}
					seenDeps[key] = true
				}
				// Deduplicate manual steps by message
				if dep.Type == module.DepManual && seenDeps[dep.Message] {
					continue
				}
				if dep.Type == module.DepManual {
					seenDeps[dep.Message] = true
				}

				allDeps = append(allDeps, dep)
				switch dep.Type {
				case module.DepModule:
					if err := resolve(dep.Module); err != nil {
						inProgress[name] = false
						return err
					}
				case module.DepManual:
					if !dep.Optional {
						manualSteps = append(manualSteps, dep.Message)
					}
				}
			}
		}

		inProgress[name] = false
		visited[name] = true
		order = append(order, name)
		return nil
	}

	for name := range r.selected {
		if err := resolve(name); err != nil {
			return nil, nil, nil, err
		}
	}

	return order, allDeps, manualSteps, nil
}
