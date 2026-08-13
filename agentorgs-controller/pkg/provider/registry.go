package provider

import "fmt"

// Registry routes provider names to concrete implementations.
type Registry struct {
	runtimes       map[string]RuntimeAdapter
	executions     map[string]ExecutionBackend
	collaborations map[string]CollaborationProvider
	storage        map[string]StorageProvider
}

func NewRegistry() *Registry {
	return &Registry{
		runtimes:       map[string]RuntimeAdapter{},
		executions:     map[string]ExecutionBackend{},
		collaborations: map[string]CollaborationProvider{},
		storage:        map[string]StorageProvider{},
	}
}

func (r *Registry) RegisterRuntime(p RuntimeAdapter) {
	r.runtimes[p.Name()] = p
}

func (r *Registry) RegisterExecution(p ExecutionBackend) {
	r.executions[p.Name()] = p
}

func (r *Registry) RegisterCollaboration(p CollaborationProvider) {
	r.collaborations[p.Name()] = p
}

func (r *Registry) RegisterStorage(p StorageProvider) {
	r.storage[p.Name()] = p
}

func (r *Registry) Runtime(name string) (RuntimeAdapter, error) {
	p, ok := r.runtimes[name]
	if !ok {
		return nil, fmt.Errorf("runtime provider %q not registered", name)
	}
	return p, nil
}

func (r *Registry) Execution(name string) (ExecutionBackend, error) {
	p, ok := r.executions[name]
	if !ok {
		return nil, fmt.Errorf("execution provider %q not registered", name)
	}
	return p, nil
}

func (r *Registry) Collaboration(name string) (CollaborationProvider, error) {
	p, ok := r.collaborations[name]
	if !ok {
		return nil, fmt.Errorf("collaboration provider %q not registered", name)
	}
	return p, nil
}

func (r *Registry) Storage(name string) (StorageProvider, error) {
	p, ok := r.storage[name]
	if !ok {
		return nil, fmt.Errorf("storage provider %q not registered", name)
	}
	return p, nil
}

func (r *Registry) DefaultCollaboration() (CollaborationProvider, error) {
	for _, p := range r.collaborations {
		return p, nil
	}
	return nil, fmt.Errorf("no collaboration provider registered")
}

func (r *Registry) DefaultStorage() (StorageProvider, error) {
	for _, p := range r.storage {
		return p, nil
	}
	return nil, fmt.Errorf("no storage provider registered")
}
