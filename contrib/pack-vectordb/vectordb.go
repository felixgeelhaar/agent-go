// Package vectordb provides vector database tools for agent-go.
//
// This pack is a façade over domain/knowledge.Store (in-memory, SQLite,
// PostgreSQL/pgvector, etc.). It exposes upsert/query/delete/fetch/list and
// index describe operations. Multi-index create/delete map to store-level
// semantics: create is a no-op validation, delete clears all vectors.
package vectordb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/knowledge"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns vector database tools backed by store.
// store must be non-nil.
func Pack(store knowledge.Store) *pack.Pack {
	if store == nil {
		panic("vectordb.Pack: store is required")
	}
	p := &packState{store: store}
	return pack.NewBuilder("vectordb").
		WithDescription("Vector database tools for similarity search (knowledge.Store façade)").
		WithVersion("0.1.0").
		AddTools(
			p.vectorUpsert(),
			p.vectorQuery(),
			p.vectorDelete(),
			p.vectorFetch(),
			p.vectorList(),
			p.vectorCreateIndex(),
			p.vectorDeleteIndex(),
			p.vectorDescribeIndex(),
		).
		AllowInState(agent.StateExplore, "vector_query", "vector_fetch", "vector_list", "vector_describe_index").
		AllowInState(agent.StateAct, "vector_upsert", "vector_query", "vector_delete", "vector_fetch", "vector_list", "vector_create_index", "vector_delete_index", "vector_describe_index").
		AllowInState(agent.StateValidate, "vector_query", "vector_fetch", "vector_describe_index").
		Build()
}

type packState struct {
	store knowledge.Store
}

func resultOK(v any) (tool.Result, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return tool.Result{}, err
	}
	return tool.Result{Output: out}, nil
}

func decodeInput[T any](raw json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, fmt.Errorf("%w: %v", tool.ErrInvalidInput, err)
	}
	return v, nil
}

func (p *packState) vectorUpsert() tool.Tool {
	return tool.NewBuilder("vector_upsert").
		WithDescription("Insert or update vectors with metadata").
		Idempotent().
		WithRiskLevel(tool.RiskMedium).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"id":        json.RawMessage(`{"type":"string"}`),
			"embedding": json.RawMessage(`{"type":"array","items":{"type":"number"}}`),
			"text":      json.RawMessage(`{"type":"string"}`),
			"metadata":  json.RawMessage(`{"type":"object"}`),
		}, []string{"id", "embedding"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decodeInput[struct {
				ID        string            `json:"id"`
				Embedding []float32         `json:"embedding"`
				Text      string            `json:"text"`
				Metadata  map[string]string `json:"metadata,omitempty"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if in.ID == "" || len(in.Embedding) == 0 {
				return tool.Result{}, fmt.Errorf("%w: id and embedding are required", tool.ErrInvalidInput)
			}
			v := &knowledge.Vector{
				ID:        in.ID,
				Embedding: in.Embedding,
				Text:      in.Text,
				Metadata:  in.Metadata,
				CreatedAt: time.Now().UTC(),
			}
			if err := p.store.Upsert(ctx, v); err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"id": in.ID, "upserted": true, "dimension": len(in.Embedding)})
		}).
		MustBuild()
}

func (p *packState) vectorQuery() tool.Tool {
	return tool.NewBuilder("vector_query").
		WithDescription("Query for similar vectors using vector similarity search").
		ReadOnly().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"embedding": json.RawMessage(`{"type":"array","items":{"type":"number"}}`),
			"top_k":     json.RawMessage(`{"type":"integer"}`),
		}, []string{"embedding"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decodeInput[struct {
				Embedding []float32 `json:"embedding"`
				TopK      int       `json:"top_k"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if len(in.Embedding) == 0 {
				return tool.Result{}, fmt.Errorf("%w: embedding is required", tool.ErrInvalidInput)
			}
			if in.TopK <= 0 {
				in.TopK = 5
			}
			results, err := p.store.Search(ctx, in.Embedding, in.TopK)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"results": results, "count": len(results)})
		}).
		MustBuild()
}

func (p *packState) vectorDelete() tool.Tool {
	return tool.NewBuilder("vector_delete").
		WithDescription("Delete vectors by ID").
		Destructive().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"id": json.RawMessage(`{"type":"string"}`),
		}, []string{"id"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decodeInput[struct {
				ID string `json:"id"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if in.ID == "" {
				return tool.Result{}, fmt.Errorf("%w: id is required", tool.ErrInvalidInput)
			}
			if err := p.store.Delete(ctx, in.ID); err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"id": in.ID, "deleted": true})
		}).
		MustBuild()
}

func (p *packState) vectorFetch() tool.Tool {
	return tool.NewBuilder("vector_fetch").
		WithDescription("Fetch vectors by their IDs").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"id": json.RawMessage(`{"type":"string"}`),
		}, []string{"id"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decodeInput[struct {
				ID string `json:"id"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			v, err := p.store.Get(ctx, in.ID)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(v)
		}).
		MustBuild()
}

func (p *packState) vectorList() tool.Tool {
	return tool.NewBuilder("vector_list").
		WithDescription("List vectors in the store with pagination").
		ReadOnly().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"limit":     json.RawMessage(`{"type":"integer"}`),
			"offset":    json.RawMessage(`{"type":"integer"}`),
			"id_prefix": json.RawMessage(`{"type":"string"}`),
		}, nil)).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decodeInput[struct {
				Limit    int    `json:"limit"`
				Offset   int    `json:"offset"`
				IDPrefix string `json:"id_prefix"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if in.Limit <= 0 {
				in.Limit = 50
			}
			vectors, err := p.store.List(ctx, knowledge.ListFilter{
				Limit:    in.Limit,
				Offset:   in.Offset,
				IDPrefix: in.IDPrefix,
			})
			if err != nil {
				return tool.Result{}, err
			}
			// Omit embeddings from list output for brevity.
			type row struct {
				ID       string            `json:"id"`
				Text     string            `json:"text"`
				Metadata map[string]string `json:"metadata,omitempty"`
			}
			rows := make([]row, 0, len(vectors))
			for _, v := range vectors {
				rows = append(rows, row{ID: v.ID, Text: v.Text, Metadata: v.Metadata})
			}
			return resultOK(map[string]any{"vectors": rows, "count": len(rows)})
		}).
		MustBuild()
}

func (p *packState) vectorCreateIndex() tool.Tool {
	return tool.NewBuilder("vector_create_index").
		WithDescription("Validate readiness of the backing knowledge store (single-index façade)").
		WithRiskLevel(tool.RiskMedium).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"name":      json.RawMessage(`{"type":"string"}`),
			"dimension": json.RawMessage(`{"type":"integer"}`),
		}, nil)).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decodeInput[struct {
				Name      string `json:"name"`
				Dimension int    `json:"dimension"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			count, err := p.store.Count(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			name := in.Name
			if name == "" {
				name = "default"
			}
			return resultOK(map[string]any{
				"name":      name,
				"created":   true,
				"note":      "knowledge.Store façade uses a single implicit index",
				"dimension": in.Dimension,
				"count":     count,
			})
		}).
		MustBuild()
}

func (p *packState) vectorDeleteIndex() tool.Tool {
	return tool.NewBuilder("vector_delete_index").
		WithDescription("Delete all vectors in the backing knowledge store").
		Destructive().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"confirm": json.RawMessage(`{"type":"boolean"}`),
		}, []string{"confirm"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decodeInput[struct {
				Confirm bool `json:"confirm"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if !in.Confirm {
				return tool.Result{}, fmt.Errorf("%w: confirm must be true to delete all vectors", tool.ErrInvalidInput)
			}
			vectors, err := p.store.List(ctx, knowledge.ListFilter{Limit: 100000})
			if err != nil {
				return tool.Result{}, err
			}
			deleted := 0
			if batch, ok := p.store.(knowledge.BatchStore); ok {
				ids := make([]string, len(vectors))
				for i, v := range vectors {
					ids[i] = v.ID
				}
				if err := batch.DeleteBatch(ctx, ids); err != nil {
					return tool.Result{}, err
				}
				deleted = len(ids)
			} else {
				for _, v := range vectors {
					if err := p.store.Delete(ctx, v.ID); err != nil {
						return tool.Result{}, err
					}
					deleted++
				}
			}
			return resultOK(map[string]any{"deleted": deleted})
		}).
		MustBuild()
}

func (p *packState) vectorDescribeIndex() tool.Tool {
	return tool.NewBuilder("vector_describe_index").
		WithDescription("Get statistics for the backing knowledge store").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, _ json.RawMessage) (tool.Result, error) {
			if sp, ok := p.store.(knowledge.StatsProvider); ok {
				stats, err := sp.Stats(ctx)
				if err != nil {
					return tool.Result{}, err
				}
				return resultOK(map[string]any{
					"name":         "default",
					"vector_count": stats.VectorCount,
					"dimension":    stats.Dimension,
				})
			}
			count, err := p.store.Count(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{
				"name":         "default",
				"vector_count": count,
			})
		}).
		MustBuild()
}
