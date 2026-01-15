// Package inspector provides types for inspecting and exporting agent data.
package inspector

import "errors"

var (
	// ErrRunNotFound indicates the run was not found.
	ErrRunNotFound = errors.New("run not found")

	// ErrInvalidFormat indicates an unsupported export format.
	ErrInvalidFormat = errors.New("invalid export format")

	// ErrExportFailed indicates the export failed.
	ErrExportFailed = errors.New("export failed")

	// ErrNoData indicates there is no data to export.
	ErrNoData = errors.New("no data to export")
)
