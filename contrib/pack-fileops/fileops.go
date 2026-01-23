// Package fileops provides file operation tools for agent-go.
//
// This pack includes high-level tools for file operations:
//   - fileops_read: Read file contents with encoding detection
//   - fileops_write: Write content to a file with backup
//   - fileops_append: Append content to a file
//   - fileops_search: Search for text patterns in files
//   - fileops_replace: Find and replace text in files
//   - fileops_diff: Compare two files and show differences
//   - fileops_archive: Create archives (zip, tar, tar.gz)
//   - fileops_extract: Extract archives
//   - fileops_checksum: Calculate file checksums (MD5, SHA256)
//
// Supports text encoding detection and conversion.
// All operations can create automatic backups.
package fileops

import (
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the file operations tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("fileops").
		WithDescription("High-level file operation tools").
		WithVersion("0.1.0").
		AddTools(
			fileopsRead(),
			fileopsWrite(),
			fileopsAppend(),
			fileopsSearch(),
			fileopsReplace(),
			fileopsDiff(),
			fileopsArchive(),
			fileopsExtract(),
			fileopsChecksum(),
		).
		AllowInState(agent.StateExplore, "fileops_read", "fileops_search", "fileops_diff", "fileops_checksum").
		AllowInState(agent.StateAct, "fileops_read", "fileops_write", "fileops_append", "fileops_search", "fileops_replace", "fileops_diff", "fileops_archive", "fileops_extract", "fileops_checksum").
		AllowInState(agent.StateValidate, "fileops_read", "fileops_diff", "fileops_checksum").
		Build()
}

func fileopsRead() tool.Tool {
	return tool.NewBuilder("fileops_read").
		WithDescription("Read file contents with automatic encoding detection").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func fileopsWrite() tool.Tool {
	return tool.NewBuilder("fileops_write").
		WithDescription("Write content to a file with optional backup").
		Idempotent().
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func fileopsAppend() tool.Tool {
	return tool.NewBuilder("fileops_append").
		WithDescription("Append content to the end of a file").
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func fileopsSearch() tool.Tool {
	return tool.NewBuilder("fileops_search").
		WithDescription("Search for text patterns in files using regex").
		ReadOnly().
		MustBuild()
}

func fileopsReplace() tool.Tool {
	return tool.NewBuilder("fileops_replace").
		WithDescription("Find and replace text patterns in files").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func fileopsDiff() tool.Tool {
	return tool.NewBuilder("fileops_diff").
		WithDescription("Compare two files and show differences").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func fileopsArchive() tool.Tool {
	return tool.NewBuilder("fileops_archive").
		WithDescription("Create archives (zip, tar, tar.gz) from files").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func fileopsExtract() tool.Tool {
	return tool.NewBuilder("fileops_extract").
		WithDescription("Extract archives to a directory").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func fileopsChecksum() tool.Tool {
	return tool.NewBuilder("fileops_checksum").
		WithDescription("Calculate file checksums (MD5, SHA256, SHA512)").
		ReadOnly().
		Cacheable().
		MustBuild()
}
