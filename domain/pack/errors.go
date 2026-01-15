package pack

import "errors"

// Domain errors for pack operations.
var (
	// ErrPackNotFound is returned when a pack does not exist.
	ErrPackNotFound = errors.New("pack not found")

	// ErrPackExists is returned when a pack already exists.
	ErrPackExists = errors.New("pack already exists")

	// ErrInvalidPack is returned when a pack is invalid.
	ErrInvalidPack = errors.New("invalid pack")

	// ErrDependencyNotFound is returned when a required dependency is missing.
	ErrDependencyNotFound = errors.New("pack dependency not found")

	// ErrCircularDependency is returned when packs have circular dependencies.
	ErrCircularDependency = errors.New("circular pack dependency detected")
)
