package config

import "errors"

// Domain errors for configuration operations.
var (
	// ErrConfigNotFound indicates the configuration file was not found.
	ErrConfigNotFound = errors.New("configuration file not found")

	// ErrInvalidFormat indicates the configuration format is invalid.
	ErrInvalidFormat = errors.New("invalid configuration format")

	// ErrUnsupportedFormat indicates the file format is not supported.
	ErrUnsupportedFormat = errors.New("unsupported configuration format")

	// ErrValidationFailed indicates configuration validation failed.
	ErrValidationFailed = errors.New("configuration validation failed")

	// ErrEnvExpansionFailed indicates environment variable expansion failed.
	ErrEnvExpansionFailed = errors.New("environment variable expansion failed")

	// ErrMissingEnvVar indicates a required environment variable is not set.
	ErrMissingEnvVar = errors.New("required environment variable not set")

	// ErrBuildFailed indicates engine building from config failed.
	ErrBuildFailed = errors.New("failed to build engine from configuration")

	// ErrSchemaGenerationFailed indicates JSON schema generation failed.
	ErrSchemaGenerationFailed = errors.New("failed to generate JSON schema")
)
