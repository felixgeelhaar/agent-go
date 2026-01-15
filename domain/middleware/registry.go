package middleware

// Registry manages an ordered collection of middleware.
type Registry struct {
	middlewares []Middleware
}

// NewRegistry creates an empty middleware registry.
func NewRegistry() *Registry {
	return &Registry{
		middlewares: make([]Middleware, 0),
	}
}

// Use adds middleware to the registry.
// Middleware are executed in the order they are added.
func (r *Registry) Use(m Middleware) *Registry {
	r.middlewares = append(r.middlewares, m)
	return r
}

// UseMany adds multiple middleware to the registry.
func (r *Registry) UseMany(ms ...Middleware) *Registry {
	r.middlewares = append(r.middlewares, ms...)
	return r
}

// Chain returns the complete middleware chain.
// If no middleware have been added, returns Noop.
func (r *Registry) Chain() Middleware {
	if len(r.middlewares) == 0 {
		return Noop()
	}
	return Chain(r.middlewares...)
}

// Len returns the number of middleware in the registry.
func (r *Registry) Len() int {
	return len(r.middlewares)
}

// Clone creates a copy of the registry.
func (r *Registry) Clone() *Registry {
	clone := NewRegistry()
	clone.middlewares = make([]Middleware, len(r.middlewares))
	copy(clone.middlewares, r.middlewares)
	return clone
}
