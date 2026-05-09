package engine

// ProviderRegistry holds all registered data providers.
// Providers are registered at startup and iterated during export execution.
type ProviderRegistry struct {
	providers []DataProvider
}

// NewProviderRegistry creates an empty provider registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{}
}

// Register adds a provider to the registry.
func (r *ProviderRegistry) Register(p DataProvider) {
	r.providers = append(r.providers, p)
}

// All returns all registered providers.
func (r *ProviderRegistry) All() []DataProvider {
	return r.providers
}
