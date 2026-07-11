package deletion

// Registry holds all registered deletion providers.
// Providers are registered at startup in the order they should execute.
// The order is significant: auth must be last.
type Registry struct {
	providers []Provider
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a provider. Order of registration = order of execution.
func (r *Registry) Register(p Provider) {
	r.providers = append(r.providers, p)
}

// All returns providers in registration order.
func (r *Registry) All() []Provider {
	return r.providers
}
