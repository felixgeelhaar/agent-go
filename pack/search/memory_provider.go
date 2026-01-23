package search

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryProvider is an in-memory implementation of Provider for testing.
type MemoryProvider struct {
	mu      sync.RWMutex
	indices map[string]*memoryIndex
}

type memoryIndex struct {
	name      string
	documents map[string]map[string]interface{}
}

// NewMemoryProvider creates a new in-memory search provider.
func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{
		indices: make(map[string]*memoryIndex),
	}
}

// Name returns the provider name.
func (p *MemoryProvider) Name() string {
	return "memory"
}

// Search executes a search query.
func (p *MemoryProvider) Search(ctx context.Context, index string, query Query) (*SearchResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	start := time.Now()

	p.mu.RLock()
	defer p.mu.RUnlock()

	idx, ok := p.indices[index]
	if !ok {
		return &SearchResult{
			TotalHits: 0,
			Hits:      []Hit{},
			Took:      time.Since(start).Milliseconds(),
		}, nil
	}

	// Simple text matching
	queryLower := strings.ToLower(query.Text)
	hits := make([]Hit, 0)

	for id, doc := range idx.documents {
		// Check if query matches any field
		matched := false
		highlights := make(map[string][]string)

		for field, value := range doc {
			// If fields are specified, only search those fields
			if len(query.Fields) > 0 {
				found := false
				for _, f := range query.Fields {
					if f == field {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			strValue := fmt.Sprintf("%v", value)
			if strings.Contains(strings.ToLower(strValue), queryLower) {
				matched = true
				if query.Highlight {
					highlights[field] = []string{strValue}
				}
			}
		}

		if matched {
			hit := Hit{
				ID:     id,
				Index:  index,
				Score:  1.0,
				Source: doc,
			}
			if query.Highlight && len(highlights) > 0 {
				hit.Highlights = highlights
			}
			hits = append(hits, hit)
		}
	}

	// Apply pagination
	totalHits := int64(len(hits))
	if query.From > 0 && query.From < len(hits) {
		hits = hits[query.From:]
	} else if query.From >= len(hits) {
		hits = []Hit{}
	}

	if query.Size > 0 && query.Size < len(hits) {
		hits = hits[:query.Size]
	}

	// Build facets if requested
	facets := make(FacetMap)
	if len(query.FacetFields) > 0 {
		for _, field := range query.FacetFields {
			counts := make(map[string]int64)
			for _, doc := range idx.documents {
				if value, ok := doc[field]; ok {
					strValue := fmt.Sprintf("%v", value)
					counts[strValue]++
				}
			}
			values := make([]FacetValue, 0, len(counts))
			for val, count := range counts {
				values = append(values, FacetValue{Value: val, Count: count})
			}
			facets[field] = values
		}
	}

	return &SearchResult{
		TotalHits: totalHits,
		MaxScore:  1.0,
		Hits:      hits,
		Facets:    facets,
		Took:      time.Since(start).Milliseconds(),
	}, nil
}

// Index adds or updates a document.
func (p *MemoryProvider) Index(ctx context.Context, index, docID string, document map[string]interface{}) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	idx, ok := p.indices[index]
	if !ok {
		idx = &memoryIndex{
			name:      index,
			documents: make(map[string]map[string]interface{}),
		}
		p.indices[index] = idx
	}

	if docID == "" {
		docID = uuid.New().String()
	}

	// Copy document to avoid external mutations
	docCopy := make(map[string]interface{})
	for k, v := range document {
		docCopy[k] = v
	}

	idx.documents[docID] = docCopy
	return nil
}

// Delete removes a document.
func (p *MemoryProvider) Delete(ctx context.Context, index, docID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	idx, ok := p.indices[index]
	if !ok {
		return fmt.Errorf("index %s not found", index)
	}

	if _, ok := idx.documents[docID]; !ok {
		return fmt.Errorf("document %s not found", docID)
	}

	delete(idx.documents, docID)
	return nil
}

// GetDocument retrieves a document by ID.
func (p *MemoryProvider) GetDocument(ctx context.Context, index, docID string) (map[string]interface{}, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	idx, ok := p.indices[index]
	if !ok {
		return nil, fmt.Errorf("index %s not found", index)
	}

	doc, ok := idx.documents[docID]
	if !ok {
		return nil, fmt.Errorf("document %s not found", docID)
	}

	// Return a copy
	result := make(map[string]interface{})
	for k, v := range doc {
		result[k] = v
	}
	return result, nil
}

// ListIndices returns available indices.
func (p *MemoryProvider) ListIndices(ctx context.Context) ([]IndexInfo, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]IndexInfo, 0, len(p.indices))
	for name, idx := range p.indices {
		result = append(result, IndexInfo{
			Name:     name,
			DocCount: int64(len(idx.documents)),
		})
	}
	return result, nil
}

// IndexExists checks if an index exists.
func (p *MemoryProvider) IndexExists(ctx context.Context, index string) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	_, ok := p.indices[index]
	return ok, nil
}

// Close releases provider resources.
func (p *MemoryProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.indices = make(map[string]*memoryIndex)
	return nil
}

// CreateIndex creates an index for testing.
func (p *MemoryProvider) CreateIndex(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.indices[name]; !ok {
		p.indices[name] = &memoryIndex{
			name:      name,
			documents: make(map[string]map[string]interface{}),
		}
	}
}

// DocumentCount returns the number of documents in an index for testing.
func (p *MemoryProvider) DocumentCount(index string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if idx, ok := p.indices[index]; ok {
		return len(idx.documents)
	}
	return 0
}
