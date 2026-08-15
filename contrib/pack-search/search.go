// Package search provides search engine tools for agent-go.
//
// Tools operate through a SearchEngine backend. MemoryEngine is a simple
// inverted-index implementation for tests and local use; Elasticsearch /
// Meilisearch / Typesense adapters can implement SearchEngine.
package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// ErrIndexNotFound indicates the index does not exist.
var ErrIndexNotFound = errors.New("index not found")

// ErrDocNotFound indicates the document does not exist.
var ErrDocNotFound = errors.New("document not found")

// Document is an indexed document.
type Document struct {
	ID       string         `json:"id"`
	Index    string         `json:"index"`
	Body     string         `json:"body"`
	Fields   map[string]any `json:"fields,omitempty"`
}

// Hit is a search hit.
type Hit struct {
	ID     string         `json:"id"`
	Score  float64        `json:"score"`
	Body   string         `json:"body,omitempty"`
	Fields map[string]any `json:"fields,omitempty"`
}

// IndexInfo describes an index.
type IndexInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// AggregateBucket is a term aggregation bucket.
type AggregateBucket struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// SearchEngine is the search backend.
type SearchEngine interface {
	Index(ctx context.Context, index string, doc Document) error
	BulkIndex(ctx context.Context, index string, docs []Document) (int, error)
	Delete(ctx context.Context, index, id string) error
	Query(ctx context.Context, index, query string, limit int) ([]Hit, error)
	Suggest(ctx context.Context, index, prefix string, limit int) ([]string, error)
	Aggregate(ctx context.Context, index, field string) ([]AggregateBucket, error)
	ListIndices(ctx context.Context) ([]IndexInfo, error)
}

// Pack returns search tools backed by engine.
func Pack(engine SearchEngine) *pack.Pack {
	if engine == nil {
		panic("search.Pack: engine is required")
	}
	p := &searchPack{engine: engine}
	return pack.NewBuilder("search").
		WithDescription("Search engine tools for indexing and querying").
		WithVersion("0.1.0").
		AddTools(
			p.searchQuery(),
			p.searchIndex(),
			p.searchBulkIndex(),
			p.searchDelete(),
			p.searchSuggest(),
			p.searchAggregate(),
			p.searchIndices(),
		).
		AllowInState(agent.StateExplore, "search_query", "search_suggest", "search_aggregate", "search_indices").
		AllowInState(agent.StateAct, "search_query", "search_index", "search_bulk_index", "search_delete", "search_suggest", "search_aggregate", "search_indices").
		AllowInState(agent.StateValidate, "search_query", "search_indices").
		Build()
}

type searchPack struct {
	engine SearchEngine
}

func resultOK(v any) (tool.Result, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return tool.Result{}, err
	}
	return tool.Result{Output: out}, nil
}

func decode[T any](raw json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, fmt.Errorf("%w: %v", tool.ErrInvalidInput, err)
	}
	return v, nil
}

func (p *searchPack) searchQuery() tool.Tool {
	return tool.NewBuilder("search_query").
		WithDescription("Execute a search query and return matching documents").
		ReadOnly().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"index": json.RawMessage(`{"type":"string"}`),
			"query": json.RawMessage(`{"type":"string"}`),
			"limit": json.RawMessage(`{"type":"integer"}`),
		}, []string{"index", "query"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Index string `json:"index"`
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if in.Limit <= 0 {
				in.Limit = 10
			}
			hits, err := p.engine.Query(ctx, in.Index, in.Query, in.Limit)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"hits": hits, "count": len(hits)})
		}).
		MustBuild()
}

func (p *searchPack) searchIndex() tool.Tool {
	return tool.NewBuilder("search_index").
		WithDescription("Index a document for searching").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"index":  json.RawMessage(`{"type":"string"}`),
			"id":     json.RawMessage(`{"type":"string"}`),
			"body":   json.RawMessage(`{"type":"string"}`),
			"fields": json.RawMessage(`{"type":"object"}`),
		}, []string{"index", "id", "body"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Index  string         `json:"index"`
				ID     string         `json:"id"`
				Body   string         `json:"body"`
				Fields map[string]any `json:"fields"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			doc := Document{ID: in.ID, Index: in.Index, Body: in.Body, Fields: in.Fields}
			if err := p.engine.Index(ctx, in.Index, doc); err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"indexed": true, "id": in.ID, "index": in.Index})
		}).
		MustBuild()
}

func (p *searchPack) searchBulkIndex() tool.Tool {
	return tool.NewBuilder("search_bulk_index").
		WithDescription("Bulk index multiple documents").
		WithRiskLevel(tool.RiskMedium).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"index": json.RawMessage(`{"type":"string"}`),
			"docs":  json.RawMessage(`{"type":"array"}`),
		}, []string{"index", "docs"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Index string     `json:"index"`
				Docs  []Document `json:"docs"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			n, err := p.engine.BulkIndex(ctx, in.Index, in.Docs)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"indexed": n, "index": in.Index})
		}).
		MustBuild()
}

func (p *searchPack) searchDelete() tool.Tool {
	return tool.NewBuilder("search_delete").
		WithDescription("Delete a document from the search index").
		Destructive().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"index": json.RawMessage(`{"type":"string"}`),
			"id":    json.RawMessage(`{"type":"string"}`),
		}, []string{"index", "id"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Index string `json:"index"`
				ID    string `json:"id"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if err := p.engine.Delete(ctx, in.Index, in.ID); err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"deleted": true, "id": in.ID})
		}).
		MustBuild()
}

func (p *searchPack) searchSuggest() tool.Tool {
	return tool.NewBuilder("search_suggest").
		WithDescription("Get search suggestions and autocomplete results").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"index":  json.RawMessage(`{"type":"string"}`),
			"prefix": json.RawMessage(`{"type":"string"}`),
			"limit":  json.RawMessage(`{"type":"integer"}`),
		}, []string{"index", "prefix"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Index  string `json:"index"`
				Prefix string `json:"prefix"`
				Limit  int    `json:"limit"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if in.Limit <= 0 {
				in.Limit = 5
			}
			sugs, err := p.engine.Suggest(ctx, in.Index, in.Prefix, in.Limit)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"suggestions": sugs, "count": len(sugs)})
		}).
		MustBuild()
}

func (p *searchPack) searchAggregate() tool.Tool {
	return tool.NewBuilder("search_aggregate").
		WithDescription("Run term aggregation on a field").
		ReadOnly().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"index": json.RawMessage(`{"type":"string"}`),
			"field": json.RawMessage(`{"type":"string"}`),
		}, []string{"index", "field"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Index string `json:"index"`
				Field string `json:"field"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			buckets, err := p.engine.Aggregate(ctx, in.Index, in.Field)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"buckets": buckets, "count": len(buckets)})
		}).
		MustBuild()
}

func (p *searchPack) searchIndices() tool.Tool {
	return tool.NewBuilder("search_indices").
		WithDescription("List available search indices").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, _ json.RawMessage) (tool.Result, error) {
			idxs, err := p.engine.ListIndices(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"indices": idxs, "count": len(idxs)})
		}).
		MustBuild()
}

// ---------------------------------------------------------------------------
// MemoryEngine
// ---------------------------------------------------------------------------

type memIndex struct {
	docs  map[string]Document
	inv   map[string]map[string]int // term -> docID -> tf
	terms map[string]struct{}
}

// MemoryEngine is an in-process inverted-index SearchEngine.
type MemoryEngine struct {
	mu      sync.RWMutex
	indexes map[string]*memIndex
}

// NewMemoryEngine creates an empty in-memory search engine.
func NewMemoryEngine() *MemoryEngine {
	return &MemoryEngine{indexes: make(map[string]*memIndex)}
}

func (e *MemoryEngine) ensure(name string) *memIndex {
	idx, ok := e.indexes[name]
	if !ok {
		idx = &memIndex{
			docs:  make(map[string]Document),
			inv:   make(map[string]map[string]int),
			terms: make(map[string]struct{}),
		}
		e.indexes[name] = idx
	}
	return idx
}

func tokenize(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func (e *MemoryEngine) indexLocked(idx *memIndex, doc Document) {
	if old, ok := idx.docs[doc.ID]; ok {
		for _, t := range tokenize(old.Body) {
			if m, ok := idx.inv[t]; ok {
				delete(m, doc.ID)
				if len(m) == 0 {
					delete(idx.inv, t)
					delete(idx.terms, t)
				}
			}
		}
	}
	idx.docs[doc.ID] = doc
	for _, t := range tokenize(doc.Body) {
		m := idx.inv[t]
		if m == nil {
			m = make(map[string]int)
			idx.inv[t] = m
			idx.terms[t] = struct{}{}
		}
		m[doc.ID]++
	}
}

func (e *MemoryEngine) Index(_ context.Context, index string, doc Document) error {
	if index == "" || doc.ID == "" {
		return fmt.Errorf("%w: index and id required", tool.ErrInvalidInput)
	}
	doc.Index = index
	e.mu.Lock()
	defer e.mu.Unlock()
	e.indexLocked(e.ensure(index), doc)
	return nil
}

func (e *MemoryEngine) BulkIndex(ctx context.Context, index string, docs []Document) (int, error) {
	n := 0
	for _, d := range docs {
		if d.ID == "" {
			continue
		}
		if err := e.Index(ctx, index, d); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (e *MemoryEngine) Delete(_ context.Context, index, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	idx, ok := e.indexes[index]
	if !ok {
		return ErrIndexNotFound
	}
	doc, ok := idx.docs[id]
	if !ok {
		return ErrDocNotFound
	}
	for _, t := range tokenize(doc.Body) {
		if m, ok := idx.inv[t]; ok {
			delete(m, id)
			if len(m) == 0 {
				delete(idx.inv, t)
				delete(idx.terms, t)
			}
		}
	}
	delete(idx.docs, id)
	return nil
}

func (e *MemoryEngine) Query(_ context.Context, index, query string, limit int) ([]Hit, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	idx, ok := e.indexes[index]
	if !ok {
		return nil, ErrIndexNotFound
	}
	scores := map[string]float64{}
	for _, t := range tokenize(query) {
		for docID, tf := range idx.inv[t] {
			scores[docID] += float64(tf)
		}
	}
	type pair struct {
		id    string
		score float64
	}
	pairs := make([]pair, 0, len(scores))
	for id, sc := range scores {
		pairs = append(pairs, pair{id, sc})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].score == pairs[j].score {
			return pairs[i].id < pairs[j].id
		}
		return pairs[i].score > pairs[j].score
	})
	if limit <= 0 {
		limit = 10
	}
	if limit > len(pairs) {
		limit = len(pairs)
	}
	hits := make([]Hit, 0, limit)
	for i := 0; i < limit; i++ {
		doc := idx.docs[pairs[i].id]
		hits = append(hits, Hit{ID: doc.ID, Score: pairs[i].score, Body: doc.Body, Fields: doc.Fields})
	}
	return hits, nil
}

func (e *MemoryEngine) Suggest(_ context.Context, index, prefix string, limit int) ([]string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	idx, ok := e.indexes[index]
	if !ok {
		return nil, ErrIndexNotFound
	}
	prefix = strings.ToLower(prefix)
	out := make([]string, 0)
	for t := range idx.terms {
		if strings.HasPrefix(t, prefix) {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	if limit <= 0 {
		limit = 5
	}
	if limit > len(out) {
		limit = len(out)
	}
	return out[:limit], nil
}

func (e *MemoryEngine) Aggregate(_ context.Context, index, field string) ([]AggregateBucket, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	idx, ok := e.indexes[index]
	if !ok {
		return nil, ErrIndexNotFound
	}
	counts := map[string]int{}
	for _, doc := range idx.docs {
		if doc.Fields == nil {
			continue
		}
		v, ok := doc.Fields[field]
		if !ok {
			continue
		}
		key := fmt.Sprint(v)
		counts[key]++
	}
	buckets := make([]AggregateBucket, 0, len(counts))
	for k, c := range counts {
		buckets = append(buckets, AggregateBucket{Key: k, Count: c})
	}
	sort.Slice(buckets, func(i, j int) bool {
		if buckets[i].Count == buckets[j].Count {
			return buckets[i].Key < buckets[j].Key
		}
		return buckets[i].Count > buckets[j].Count
	})
	return buckets, nil
}

func (e *MemoryEngine) ListIndices(_ context.Context) ([]IndexInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]IndexInfo, 0, len(e.indexes))
	for name, idx := range e.indexes {
		out = append(out, IndexInfo{Name: name, Count: len(idx.docs)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
