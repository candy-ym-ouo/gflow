package action

import "fmt"

type Registry struct{ actions map[string]Function }

func NewRegistry() *Registry { return &Registry{actions: map[string]Function{}} }
func (r *Registry) Register(name string, fn Function) error {
	if name == "" || fn == nil {
		return fmt.Errorf("action name and function are required")
	}
	if _, exists := r.actions[name]; exists {
		return fmt.Errorf("action %s already registered", name)
	}
	r.actions[name] = fn
	return nil
}
func (r *Registry) Get(name string) (Function, bool) { fn, ok := r.actions[name]; return fn, ok }
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.actions))
	for name := range r.actions {
		names = append(names, name)
	}
	return names
}
