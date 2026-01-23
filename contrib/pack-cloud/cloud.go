// Package cloud provides cloud storage tools for agent-go.
//
// This pack includes tools for cloud storage operations:
//   - cloud_upload: Upload a file to cloud storage
//   - cloud_download: Download a file from cloud storage
//   - cloud_list: List objects in a bucket/container
//   - cloud_delete: Delete an object from cloud storage
//   - cloud_copy: Copy an object within or between buckets
//   - cloud_presign: Generate a presigned URL for an object
//   - cloud_metadata: Get object metadata
//
// Supports multiple providers: AWS S3, Google Cloud Storage, Azure Blob Storage.
// Authentication is configured via environment variables or explicit credentials.
package cloud

import (
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the cloud storage tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("cloud").
		WithDescription("Cloud storage tools for S3, GCS, and Azure Blob Storage").
		WithVersion("0.1.0").
		AddTools(
			cloudUpload(),
			cloudDownload(),
			cloudList(),
			cloudDelete(),
			cloudCopy(),
			cloudPresign(),
			cloudMetadata(),
		).
		AllowInState(agent.StateExplore, "cloud_list", "cloud_metadata", "cloud_download").
		AllowInState(agent.StateAct, "cloud_upload", "cloud_download", "cloud_list", "cloud_delete", "cloud_copy", "cloud_presign", "cloud_metadata").
		AllowInState(agent.StateValidate, "cloud_list", "cloud_metadata").
		Build()
}

func cloudUpload() tool.Tool {
	return tool.NewBuilder("cloud_upload").
		WithDescription("Upload a file to cloud storage").
		Idempotent().
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func cloudDownload() tool.Tool {
	return tool.NewBuilder("cloud_download").
		WithDescription("Download a file from cloud storage").
		ReadOnly().
		MustBuild()
}

func cloudList() tool.Tool {
	return tool.NewBuilder("cloud_list").
		WithDescription("List objects in a cloud storage bucket or container").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func cloudDelete() tool.Tool {
	return tool.NewBuilder("cloud_delete").
		WithDescription("Delete an object from cloud storage").
		Destructive().
		MustBuild()
}

func cloudCopy() tool.Tool {
	return tool.NewBuilder("cloud_copy").
		WithDescription("Copy an object within or between buckets").
		Idempotent().
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func cloudPresign() tool.Tool {
	return tool.NewBuilder("cloud_presign").
		WithDescription("Generate a presigned URL for temporary object access").
		ReadOnly().
		MustBuild()
}

func cloudMetadata() tool.Tool {
	return tool.NewBuilder("cloud_metadata").
		WithDescription("Get metadata for a cloud storage object").
		ReadOnly().
		Cacheable().
		MustBuild()
}
