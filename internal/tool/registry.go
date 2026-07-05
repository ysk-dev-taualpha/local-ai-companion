package tool

import (
	"encoding/json"
	"fmt"
	"sync"
)

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Entry
	defs  []Definition
}

type Entry struct {
	Def  Definition
	Impl Executor
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Entry)}
}

func (r *Registry) Register(def Definition, impl Executor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[def.Name]; exists {
		return fmt.Errorf("tool %q already registered", def.Name)
	}
	r.tools[def.Name] = Entry{Def: def, Impl: impl}
	r.defs = nil
	return nil
}

func (r *Registry) Definitions() []Definition {
	r.mu.RLock()
	if r.defs != nil {
		defs := r.defs
		r.mu.RUnlock()
		return defs
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.defs != nil {
		return r.defs
	}
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	r.defs = make([]Definition, len(names))
	for i, name := range names {
		r.defs[i] = r.tools[name].Def
	}
	return r.defs
}

func (r *Registry) Lookup(name string) (Executor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.tools[name]
	if !ok {
		return nil, false
	}
	return entry.Impl, true
}

func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}

type Policy struct {
	allowlist map[string]bool
}

func NewPolicy() *Policy {
	return &Policy{allowlist: make(map[string]bool)}
}

func AllowAllPolicy() *Policy {
	return &Policy{allowlist: nil}
}

func (p *Policy) Allow(name string) {
	if p.allowlist == nil {
		p.allowlist = make(map[string]bool)
	}
	p.allowlist[name] = true
}

func (p *Policy) IsAllowed(name string) bool {
	if p.allowlist == nil {
		return true
	}
	return p.allowlist[name]
}

type ExecutorService struct {
	registry *Registry
	policy   *Policy
}

func NewExecutor(registry *Registry, policy *Policy) *ExecutorService {
	return &ExecutorService{registry: registry, policy: policy}
}

func (e *ExecutorService) Execute(call Call) Result {
	result := Result{
		ToolCallID: call.ID,
		Name:       call.Name,
	}

	if !e.policy.IsAllowed(call.Name) {
		result.Error = fmt.Sprintf("tool %q is not allowed by policy", call.Name)
		return result
	}

	impl, ok := e.registry.Lookup(call.Name)
	if !ok {
		result.Error = fmt.Sprintf("tool %q is not registered", call.Name)
		return result
	}

	if len(call.Args) > 0 {
		var dummy interface{}
		if err := json.Unmarshal(call.Args, &dummy); err != nil {
			result.Error = fmt.Sprintf("tool %q: malformed arguments: %v", call.Name, err)
			return result
		}
	}

	content, err := impl.Execute(call.Args)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Content = content
	return result
}
