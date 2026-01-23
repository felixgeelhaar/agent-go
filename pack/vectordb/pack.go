package vectordb

import (
	"context"
	"encoding/json"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// PackConfig configures the vector database pack.
type PackConfig struct {
	// Provider is the vector database provider to use.
	Provider Provider

	// DefaultCollection is the default collection name.
	DefaultCollection string

	// DefaultTopK is the default number of results for queries.
	DefaultTopK int
}

// DefaultPackConfig returns default pack configuration.
func DefaultPackConfig() PackConfig {
	return PackConfig{
		DefaultTopK: 10,
	}
}

// New creates a new vector database pack with the given configuration.
func New(cfg PackConfig) *pack.Pack {
	if cfg.DefaultTopK == 0 {
		cfg.DefaultTopK = 10
	}

	return pack.NewBuilder("vectordb").
		WithDescription("Tools for interacting with vector databases").
		WithVersion("1.0.0").
		AddTools(
			upsertTool(cfg),
			queryTool(cfg),
			deleteTool(cfg),
		).
		AllowInState(agent.StateExplore, "vector_query").
		AllowInState(agent.StateAct, "vector_upsert", "vector_query", "vector_delete").
		Build()
}

// upsertTool creates the vector_upsert tool.
func upsertTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("vector_upsert").
		WithDescription("Insert or update vectors in a collection").
		WithTags("vectordb", "ai", "embeddings").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req UpsertRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if req.Collection == "" {
				req.Collection = cfg.DefaultCollection
			}

			if req.Collection == "" {
				return tool.Result{}, ErrInvalidInput
			}

			if len(req.Vectors) == 0 {
				return tool.Result{}, ErrInvalidInput
			}

			resp, err := cfg.Provider.Upsert(ctx, req)
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// queryTool creates the vector_query tool.
func queryTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("vector_query").
		WithDescription("Search for similar vectors").
		ReadOnly().
		Cacheable().
		WithTags("vectordb", "ai", "search").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req QueryRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if req.Collection == "" {
				req.Collection = cfg.DefaultCollection
			}

			if req.Collection == "" || len(req.Vector) == 0 {
				return tool.Result{}, ErrInvalidInput
			}

			if req.TopK == 0 {
				req.TopK = cfg.DefaultTopK
			}

			resp, err := cfg.Provider.Query(ctx, req)
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{
				Output: output,
				Cached: true,
			}, nil
		}).
		MustBuild()
}

// deleteTool creates the vector_delete tool.
func deleteTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("vector_delete").
		WithDescription("Delete vectors from a collection").
		Destructive().
		WithTags("vectordb", "ai", "delete").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req DeleteRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if req.Collection == "" {
				req.Collection = cfg.DefaultCollection
			}

			if req.Collection == "" {
				return tool.Result{}, ErrInvalidInput
			}

			if len(req.IDs) == 0 && !req.DeleteAll && req.Filter == nil {
				return tool.Result{}, ErrInvalidInput
			}

			resp, err := cfg.Provider.Delete(ctx, req)
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
