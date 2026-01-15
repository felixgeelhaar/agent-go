package tool

// Registry defines the interface for tool registration and lookup.
// This is a repository interface - implementations are in infrastructure.
type Registry interface {
	// Register adds a tool to the registry.
	Register(tool Tool) error

	// Get retrieves a tool by name.
	Get(name string) (Tool, bool)

	// List returns all registered tools.
	List() []Tool

	// Names returns all registered tool names.
	Names() []string

	// Has checks if a tool is registered.
	Has(name string) bool

	// Unregister removes a tool from the registry.
	Unregister(name string) error
}
