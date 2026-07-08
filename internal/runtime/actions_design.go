// Package runtime — Action Model Design Evaluation
//
// This file evaluates a declarative Action model against the current
// imperative Restore() approach. It is a design document, not production code.
//
// ── Background ──
// Currently each module implements Restore(ctx, snap, opts) and executes
// restore logic imperatively using os/exec, os.*, archive.Extract, etc.
// The new Runtime layer wraps these primitives but still expects modules
// to call them imperatively.
//
// An Action model would have modules return []Action instead.
//
// ── Proposed Action Types ──
//
//	type Action interface {
//	    Execute(ctx RestoreContext) error
//	    Describe() string // for progress display
//	}
//
//	Concrete types:
//	  InstallPackage{pkgs}     — apt/brew install
//	  RemovePackage{pkgs}      — apt/brew remove
//	  ExtractArchive{src, dst, strip}
//	  CreateDir{path, mode}
//	  RemoveDir{path}
//	  CopyFile{src, dst, mode}
//	  CopyDir{src, dst}
//	  WriteFile{path, data, mode}
//	  SetPermission{path, mode}
//	  Chown{path, owner, group}
//	  CreateSymlink{target, link}
//	  RunCommand{cmd, args, sudo, env}
//	  StartService{name}
//	  EnableService{name}
//	  StopService{name}
//	  RestartService{name}
//	  Download{url, dest, checksum}
//	  SetEnv{key, value}
//	  ManualStep{message}
//	  Validate{description, check func}
//	  RunScript{script} — escape hatch for complex logic
//
// Module interface extension:
//
//	type ActionProvider interface {
//	    RestoreActions(ctx context.Context, snap Snapshot, opts RestoreOptions) ([]Action, error)
//	}
//
// ── Comparison ──
//
//	                │ Imperative (current)    │ Declarative (actions)
//	────────────────┼────────────────────────┼─────────────────────────
//	Flexibility     │ Unlimited               │ Constrained by action set
//	Introspection   │ None                    │ Full — engine inspects plan
//	Dry-run         │ Impossible              │ Trivial
//	Progress        │ Module-driven           │ Engine-driven (precise)
//	Parallelism     │ Manual                  │ Automatic (independent actions)
//	Testability     │ Mock filesystem/exec    │ Actions are data, easy to assert
//	Rollback        │ Manual                  │ Compensation actions possible
//	Complex modules │ Easy                    │ Need RunScript escape hatch
//	Conditional     │ Natural                 │ Harder (if → separate code path)
//	Error handling  │ Fine-grained            │ Coarser (failed action)
//	Backward compat │ Baseline                │ Needs optional interface
//
// ── Migration Recommendation ──
//
//	Phase 0: Runtime layer with imperative helpers (DONE — this PR)
//	Phase 1: Add optional ActionProvider interface to module/interface.go
//	Phase 2: Write action executor in runtime/actions.go
//	         (takes []Action, iterates, reports progress)
//	Phase 3: Port simple modules to return []Action
//	         (git, ssh, shell — filesystem-only modules)
//	Phase 4: Add RunScriptAction for modules with complex logic
//	         (docker, browsers, databases)
//	Phase 5: Make ActionProvider the primary interface, deprecate Restore()
//
//	Is it worthwhile? YES, but only as an opt-in layer on top of the
//	current infrastructure. The runtime primitives (Pkg, FS, Archive,
//	Service, Download) serve both models equally well. The action model
//	adds real value for dry-run, progress, and parallel execution, but
//	should never be the only way to write a module.
//
//	Recommendation: Implement Phase 1-2 in the next cycle, then evaluate
//	on 3-4 real modules before committing to Phase 5.
package runtime
