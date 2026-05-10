package deletion

// DeletionProviderRegistry holds all registered deletion providers.
// Providers are registered at startup in the order they should execute.
// The order is significant: auth must be last.
type DeletionProviderRegistry struct {
	providers []DeletionProvider
}

// NewDeletionProviderRegistry creates an empty registry.
func NewDeletionProviderRegistry() *DeletionProviderRegistry {
	return &DeletionProviderRegistry{}
}

// Register adds a provider. Order of registration = order of execution.
func (r *DeletionProviderRegistry) Register(p DeletionProvider) {
	r.providers = append(r.providers, p)
}

// All returns providers in registration order.
func (r *DeletionProviderRegistry) All() []DeletionProvider {
	return r.providers
}
