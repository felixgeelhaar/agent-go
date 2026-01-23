// Package search provides search engine operation tools.
package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Provider defines the interface for search operations.
// Implementations exist for Elasticsearch and Algolia.
type Provider interface {
	// Name returns the provider name (e.g., "elasticsearch", "algolia").
	Name() string

	// Search executes a search query.
	Search(ctx context.Context, index string, query Query) (*SearchResult, error)

	// Index adds or updates a document.
	Index(ctx context.Context, index, docID string, document map[string]interface{}) error

	// Delete removes a document.
	Delete(ctx context.Context, index, docID string) error

	// GetDocument retrieves a document by ID.
	GetDocument(ctx context.Context, index, docID string) (map[string]interface{}, error)

	// ListIndices returns available indices.
	ListIndices(ctx context.Context) ([]IndexInfo, error)

	// IndexExists checks if an index exists.
	IndexExists(ctx context.Context, index string) (bool, error)

	// Close releases provider resources.
	Close() error
}

// Query represents a search query.
type Query struct {
	// Text is the search query string.
	Text string `json:"text"`

	// Fields to search in (empty means all fields).
	Fields []string `json:"fields,omitempty"`

	// Filters for faceted search.
	Filters map[string]interface{} `json:"filters,omitempty"`

	// Sort order (field:asc or field:desc).
	Sort []string `json:"sort,omitempty"`

	// From is the offset for pagination.
	From int `json:"from,omitempty"`

	// Size is the number of results to return.
	Size int `json:"size,omitempty"`

	// Highlight enables result highlighting.
	Highlight bool `json:"highlight,omitempty"`

	// FacetFields for aggregation.
	FacetFields []string `json:"facet_fields,omitempty"`
}

// SearchResult contains search results.
type SearchResult struct {
	TotalHits int64      `json:"total_hits"`
	MaxScore  float64    `json:"max_score,omitempty"`
	Hits      []Hit      `json:"hits"`
	Facets    FacetMap   `json:"facets,omitempty"`
	Took      int64      `json:"took_ms"`
}

// Hit represents a single search result.
type Hit struct {
	ID         string                 `json:"id"`
	Index      string                 `json:"index"`
	Score      float64                `json:"score,omitempty"`
	Source     map[string]interface{} `json:"source"`
	Highlights map[string][]string    `json:"highlights,omitempty"`
}

// FacetMap contains facet aggregation results.
type FacetMap map[string][]FacetValue

// FacetValue represents a facet bucket.
type FacetValue struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// IndexInfo contains information about a search index.
type IndexInfo struct {
	Name      string `json:"name"`
	DocCount  int64  `json:"doc_count"`
	Size      int64  `json:"size_bytes,omitempty"`
	Health    string `json:"health,omitempty"`
}

// Config configures the search pack.
type Config struct {
	// Provider is the search provider (required).
	Provider Provider

	// ReadOnly disables index/delete operations.
	ReadOnly bool

	// AllowDelete enables delete operations (requires !ReadOnly).
	AllowDelete bool

	// Timeout for operations.
	Timeout time.Duration

	// MaxResultSize limits search results.
	MaxResultSize int
}

// Option configures the search pack.
type Option func(*Config)

// WithWriteAccess enables index operations.
func WithWriteAccess() Option {
	return func(c *Config) {
		c.ReadOnly = false
	}
}

// WithDeleteAccess enables delete operations.
func WithDeleteAccess() Option {
	return func(c *Config) {
		c.AllowDelete = true
	}
}

// WithTimeout sets the operation timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = timeout
	}
}

// WithMaxResultSize sets the maximum results returned.
func WithMaxResultSize(size int) Option {
	return func(c *Config) {
		c.MaxResultSize = size
	}
}

// New creates the search pack.
func New(provider Provider, opts ...Option) (*pack.Pack, error) {
	if provider == nil {
		return nil, errors.New("search provider is required")
	}

	cfg := Config{
		Provider:      provider,
		ReadOnly:      true, // Read-only by default for safety
		AllowDelete:   false,
		Timeout:       30 * time.Second,
		MaxResultSize: 100,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	builder := pack.NewBuilder("search").
		WithDescription(fmt.Sprintf("Search operations (%s)", provider.Name())).
		WithVersion("1.0.0").
		AddTools(
			listIndicesTool(&cfg),
			searchTool(&cfg),
			getDocumentTool(&cfg),
		).
		AllowInState(agent.StateExplore, "search_list_indices", "search_query", "search_get_document").
		AllowInState(agent.StateValidate, "search_list_indices", "search_query", "search_get_document")

	readTools := []string{"search_list_indices", "search_query", "search_get_document"}

	// Add write tools if enabled
	if !cfg.ReadOnly {
		builder = builder.AddTools(indexDocumentTool(&cfg))
		allTools := append(readTools, "search_index_document")

		// Add delete tool if enabled
		if cfg.AllowDelete {
			builder = builder.AddTools(deleteDocumentTool(&cfg))
			allTools = append(allTools, "search_delete_document")
		}

		builder = builder.AllowInState(agent.StateAct, allTools...)
	} else {
		builder = builder.AllowInState(agent.StateAct, readTools...)
	}

	return builder.Build(), nil
}

// listIndicesOutput is the output for the search_list_indices tool.
type listIndicesOutput struct {
	Provider string      `json:"provider"`
	Indices  []IndexInfo `json:"indices"`
	Count    int         `json:"count"`
}

func listIndicesTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("search_list_indices").
		WithDescription("List all available search indices").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			indices, err := cfg.Provider.ListIndices(ctx)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list indices: %w", err)
			}

			out := listIndicesOutput{
				Provider: cfg.Provider.Name(),
				Indices:  indices,
				Count:    len(indices),
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// searchInput is the input for the search_query tool.
type searchInput struct {
	Index       string                 `json:"index"`
	Query       string                 `json:"query"`
	Fields      []string               `json:"fields,omitempty"`
	Filters     map[string]interface{} `json:"filters,omitempty"`
	Sort        []string               `json:"sort,omitempty"`
	From        int                    `json:"from,omitempty"`
	Size        int                    `json:"size,omitempty"`
	Highlight   bool                   `json:"highlight,omitempty"`
	FacetFields []string               `json:"facet_fields,omitempty"`
}

// searchOutput is the output for the search_query tool.
type searchOutput struct {
	Index     string       `json:"index"`
	Query     string       `json:"query"`
	TotalHits int64        `json:"total_hits"`
	MaxScore  float64      `json:"max_score,omitempty"`
	Hits      []Hit        `json:"hits"`
	Facets    FacetMap     `json:"facets,omitempty"`
	TookMs    int64        `json:"took_ms"`
}

func searchTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("search_query").
		WithDescription("Execute a search query against an index").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in searchInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Index == "" {
				return tool.Result{}, errors.New("index is required")
			}
			if in.Query == "" {
				return tool.Result{}, errors.New("query is required")
			}

			size := in.Size
			if size == 0 || size > cfg.MaxResultSize {
				size = cfg.MaxResultSize
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			query := Query{
				Text:        in.Query,
				Fields:      in.Fields,
				Filters:     in.Filters,
				Sort:        in.Sort,
				From:        in.From,
				Size:        size,
				Highlight:   in.Highlight,
				FacetFields: in.FacetFields,
			}

			result, err := cfg.Provider.Search(ctx, in.Index, query)
			if err != nil {
				return tool.Result{}, fmt.Errorf("search failed: %w", err)
			}

			out := searchOutput{
				Index:     in.Index,
				Query:     in.Query,
				TotalHits: result.TotalHits,
				MaxScore:  result.MaxScore,
				Hits:      result.Hits,
				Facets:    result.Facets,
				TookMs:    result.Took,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// getDocumentInput is the input for the search_get_document tool.
type getDocumentInput struct {
	Index string `json:"index"`
	ID    string `json:"id"`
}

// getDocumentOutput is the output for the search_get_document tool.
type getDocumentOutput struct {
	Index    string                 `json:"index"`
	ID       string                 `json:"id"`
	Found    bool                   `json:"found"`
	Document map[string]interface{} `json:"document,omitempty"`
}

func getDocumentTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("search_get_document").
		WithDescription("Retrieve a document by ID").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in getDocumentInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Index == "" {
				return tool.Result{}, errors.New("index is required")
			}
			if in.ID == "" {
				return tool.Result{}, errors.New("id is required")
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			doc, err := cfg.Provider.GetDocument(ctx, in.Index, in.ID)
			if err != nil {
				// Document not found is not an error
				out := getDocumentOutput{
					Index: in.Index,
					ID:    in.ID,
					Found: false,
				}
				data, _ := json.Marshal(out)
				return tool.Result{Output: data}, nil
			}

			out := getDocumentOutput{
				Index:    in.Index,
				ID:       in.ID,
				Found:    true,
				Document: doc,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// indexDocumentInput is the input for the search_index_document tool.
type indexDocumentInput struct {
	Index    string                 `json:"index"`
	ID       string                 `json:"id,omitempty"`
	Document map[string]interface{} `json:"document"`
}

// indexDocumentOutput is the output for the search_index_document tool.
type indexDocumentOutput struct {
	Index   string `json:"index"`
	ID      string `json:"id"`
	Created bool   `json:"created"`
}

func indexDocumentTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("search_index_document").
		WithDescription("Index (add or update) a document").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in indexDocumentInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Index == "" {
				return tool.Result{}, errors.New("index is required")
			}
			if in.Document == nil {
				return tool.Result{}, errors.New("document is required")
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			err := cfg.Provider.Index(ctx, in.Index, in.ID, in.Document)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to index document: %w", err)
			}

			out := indexDocumentOutput{
				Index:   in.Index,
				ID:      in.ID,
				Created: true,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// deleteDocumentInput is the input for the search_delete_document tool.
type deleteDocumentInput struct {
	Index string `json:"index"`
	ID    string `json:"id"`
}

// deleteDocumentOutput is the output for the search_delete_document tool.
type deleteDocumentOutput struct {
	Index   string `json:"index"`
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func deleteDocumentTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("search_delete_document").
		WithDescription("Delete a document from an index").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in deleteDocumentInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Index == "" {
				return tool.Result{}, errors.New("index is required")
			}
			if in.ID == "" {
				return tool.Result{}, errors.New("id is required")
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			err := cfg.Provider.Delete(ctx, in.Index, in.ID)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to delete document: %w", err)
			}

			out := deleteDocumentOutput{
				Index:   in.Index,
				ID:      in.ID,
				Deleted: true,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}
