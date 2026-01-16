package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/felixgeelhaar/agent-go/application"
	"github.com/felixgeelhaar/agent-go/domain/pattern"
	"github.com/felixgeelhaar/agent-go/domain/proposal"
	"github.com/felixgeelhaar/agent-go/domain/suggestion"
	infraProposal "github.com/felixgeelhaar/agent-go/infrastructure/proposal"
)

// mockSuggestionGenerator implements suggestion.Generator for testing.
type mockSuggestionGenerator struct {
	generateFn func(ctx context.Context, patterns []pattern.Pattern) ([]suggestion.Suggestion, error)
	typesFn    func() []suggestion.SuggestionType
}

func (m *mockSuggestionGenerator) Generate(ctx context.Context, patterns []pattern.Pattern) ([]suggestion.Suggestion, error) {
	if m.generateFn != nil {
		return m.generateFn(ctx, patterns)
	}
	return []suggestion.Suggestion{}, nil
}

func (m *mockSuggestionGenerator) Types() []suggestion.SuggestionType {
	if m.typesFn != nil {
		return m.typesFn()
	}
	return []suggestion.SuggestionType{}
}

// mockSuggestionStore implements suggestion.Store for testing.
type mockSuggestionStore struct {
	saveFn   func(ctx context.Context, s *suggestion.Suggestion) error
	getFn    func(ctx context.Context, id string) (*suggestion.Suggestion, error)
	listFn   func(ctx context.Context, filter suggestion.ListFilter) ([]*suggestion.Suggestion, error)
	deleteFn func(ctx context.Context, id string) error
	updateFn func(ctx context.Context, s *suggestion.Suggestion) error
}

func (m *mockSuggestionStore) Save(ctx context.Context, s *suggestion.Suggestion) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, s)
	}
	return nil
}

func (m *mockSuggestionStore) Get(ctx context.Context, id string) (*suggestion.Suggestion, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return nil, suggestion.ErrSuggestionNotFound
}

func (m *mockSuggestionStore) List(ctx context.Context, filter suggestion.ListFilter) ([]*suggestion.Suggestion, error) {
	if m.listFn != nil {
		return m.listFn(ctx, filter)
	}
	return []*suggestion.Suggestion{}, nil
}

func (m *mockSuggestionStore) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockSuggestionStore) Update(ctx context.Context, s *suggestion.Suggestion) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, s)
	}
	return nil
}

func TestNewEvolutionService(t *testing.T) {
	t.Parallel()

	generator := &mockSuggestionGenerator{}
	suggestionStore := &mockSuggestionStore{}
	patternStore := &mockPatternStore{}

	service := application.NewEvolutionService(generator, suggestionStore, nil, patternStore)
	if service == nil {
		t.Error("NewEvolutionService should return non-nil service")
	}
}

func TestEvolutionService_GenerateSuggestions(t *testing.T) {
	t.Parallel()

	t.Run("without generator", func(t *testing.T) {
		t.Parallel()

		service := application.NewEvolutionService(nil, nil, nil, nil)

		_, err := service.GenerateSuggestions(context.Background(), []string{"p1"})
		if !errors.Is(err, suggestion.ErrGenerationFailed) {
			t.Errorf("GenerateSuggestions() error = %v, want %v", err, suggestion.ErrGenerationFailed)
		}
	})

	t.Run("no patterns found", func(t *testing.T) {
		t.Parallel()

		generator := &mockSuggestionGenerator{}
		patternStore := &mockPatternStore{
			getFn: func(ctx context.Context, id string) (*pattern.Pattern, error) {
				return nil, pattern.ErrPatternNotFound
			},
		}
		service := application.NewEvolutionService(generator, nil, nil, patternStore)

		_, err := service.GenerateSuggestions(context.Background(), []string{"p1"})
		if !errors.Is(err, suggestion.ErrNoPatterns) {
			t.Errorf("GenerateSuggestions() error = %v, want %v", err, suggestion.ErrNoPatterns)
		}
	})

	t.Run("generation fails", func(t *testing.T) {
		t.Parallel()

		generator := &mockSuggestionGenerator{
			generateFn: func(ctx context.Context, patterns []pattern.Pattern) ([]suggestion.Suggestion, error) {
				return nil, errors.New("generation error")
			},
		}
		patternStore := &mockPatternStore{
			getFn: func(ctx context.Context, id string) (*pattern.Pattern, error) {
				return &pattern.Pattern{ID: id, Name: "Test Pattern"}, nil
			},
		}
		service := application.NewEvolutionService(generator, nil, nil, patternStore)

		_, err := service.GenerateSuggestions(context.Background(), []string{"p1"})
		if err == nil {
			t.Error("GenerateSuggestions() should return error")
		}
	})

	t.Run("success without store", func(t *testing.T) {
		t.Parallel()

		generator := &mockSuggestionGenerator{
			generateFn: func(ctx context.Context, patterns []pattern.Pattern) ([]suggestion.Suggestion, error) {
				return []suggestion.Suggestion{
					{ID: "s1", Title: "Suggestion 1"},
				}, nil
			},
		}
		patternStore := &mockPatternStore{
			getFn: func(ctx context.Context, id string) (*pattern.Pattern, error) {
				return &pattern.Pattern{ID: id, Name: "Test Pattern"}, nil
			},
		}
		service := application.NewEvolutionService(generator, nil, nil, patternStore)

		suggestions, err := service.GenerateSuggestions(context.Background(), []string{"p1"})
		if err != nil {
			t.Fatalf("GenerateSuggestions() error = %v", err)
		}
		if len(suggestions) != 1 {
			t.Errorf("len(suggestions) = %d, want 1", len(suggestions))
		}
	})

	t.Run("success with store", func(t *testing.T) {
		t.Parallel()

		savedCount := 0
		generator := &mockSuggestionGenerator{
			generateFn: func(ctx context.Context, patterns []pattern.Pattern) ([]suggestion.Suggestion, error) {
				return []suggestion.Suggestion{
					{ID: "s1", Title: "Suggestion 1"},
				}, nil
			},
		}
		suggestionStore := &mockSuggestionStore{
			saveFn: func(ctx context.Context, s *suggestion.Suggestion) error {
				savedCount++
				return nil
			},
		}
		patternStore := &mockPatternStore{
			getFn: func(ctx context.Context, id string) (*pattern.Pattern, error) {
				return &pattern.Pattern{ID: id, Name: "Test Pattern"}, nil
			},
		}
		service := application.NewEvolutionService(generator, suggestionStore, nil, patternStore)

		suggestions, err := service.GenerateSuggestions(context.Background(), []string{"p1"})
		if err != nil {
			t.Fatalf("GenerateSuggestions() error = %v", err)
		}
		if len(suggestions) != 1 {
			t.Errorf("len(suggestions) = %d, want 1", len(suggestions))
		}
		if savedCount != 1 {
			t.Errorf("savedCount = %d, want 1", savedCount)
		}
	})
}

func TestEvolutionService_GetSuggestion(t *testing.T) {
	t.Parallel()

	t.Run("without store", func(t *testing.T) {
		t.Parallel()

		service := application.NewEvolutionService(nil, nil, nil, nil)

		_, err := service.GetSuggestion(context.Background(), "s1")
		if !errors.Is(err, suggestion.ErrSuggestionNotFound) {
			t.Errorf("GetSuggestion() error = %v, want %v", err, suggestion.ErrSuggestionNotFound)
		}
	})

	t.Run("suggestion found", func(t *testing.T) {
		t.Parallel()

		store := &mockSuggestionStore{
			getFn: func(ctx context.Context, id string) (*suggestion.Suggestion, error) {
				return &suggestion.Suggestion{ID: id, Title: "Test Suggestion"}, nil
			},
		}
		service := application.NewEvolutionService(nil, store, nil, nil)

		s, err := service.GetSuggestion(context.Background(), "s1")
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}
		if s.ID != "s1" {
			t.Errorf("Suggestion.ID = %s, want s1", s.ID)
		}
	})
}

func TestEvolutionService_ListSuggestions(t *testing.T) {
	t.Parallel()

	t.Run("without store", func(t *testing.T) {
		t.Parallel()

		service := application.NewEvolutionService(nil, nil, nil, nil)

		suggestions, err := service.ListSuggestions(context.Background(), suggestion.ListFilter{})
		if err != nil {
			t.Fatalf("ListSuggestions() error = %v", err)
		}
		if len(suggestions) != 0 {
			t.Errorf("len(suggestions) = %d, want 0", len(suggestions))
		}
	})

	t.Run("with store", func(t *testing.T) {
		t.Parallel()

		store := &mockSuggestionStore{
			listFn: func(ctx context.Context, filter suggestion.ListFilter) ([]*suggestion.Suggestion, error) {
				return []*suggestion.Suggestion{
					{ID: "s1"},
					{ID: "s2"},
				}, nil
			},
		}
		service := application.NewEvolutionService(nil, store, nil, nil)

		suggestions, err := service.ListSuggestions(context.Background(), suggestion.ListFilter{})
		if err != nil {
			t.Fatalf("ListSuggestions() error = %v", err)
		}
		if len(suggestions) != 2 {
			t.Errorf("len(suggestions) = %d, want 2", len(suggestions))
		}
	})
}

func TestEvolutionService_RejectSuggestion(t *testing.T) {
	t.Parallel()

	t.Run("without store", func(t *testing.T) {
		t.Parallel()

		service := application.NewEvolutionService(nil, nil, nil, nil)

		err := service.RejectSuggestion(context.Background(), "s1", "admin", "not needed")
		if !errors.Is(err, suggestion.ErrSuggestionNotFound) {
			t.Errorf("RejectSuggestion() error = %v, want %v", err, suggestion.ErrSuggestionNotFound)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		updated := false
		store := &mockSuggestionStore{
			getFn: func(ctx context.Context, id string) (*suggestion.Suggestion, error) {
				return &suggestion.Suggestion{
					ID:       id,
					Title:    "Test Suggestion",
					Status:   suggestion.SuggestionStatusPending,
					Metadata: make(map[string]any),
				}, nil
			},
			updateFn: func(ctx context.Context, s *suggestion.Suggestion) error {
				updated = true
				return nil
			},
		}
		service := application.NewEvolutionService(nil, store, nil, nil)

		err := service.RejectSuggestion(context.Background(), "s1", "admin", "not needed")
		if err != nil {
			t.Fatalf("RejectSuggestion() error = %v", err)
		}
		if !updated {
			t.Error("Update should have been called")
		}
	})
}

func TestEvolutionService_CreateProposal(t *testing.T) {
	t.Parallel()

	t.Run("without workflow", func(t *testing.T) {
		t.Parallel()

		service := application.NewEvolutionService(nil, nil, nil, nil)

		_, err := service.CreateProposal(context.Background(), "Test", "Description", "admin")
		if !errors.Is(err, proposal.ErrInvalidProposal) {
			t.Errorf("CreateProposal() error = %v, want %v", err, proposal.ErrInvalidProposal)
		}
	})
}

func TestEvolutionService_GetProposal(t *testing.T) {
	t.Parallel()

	t.Run("without workflow", func(t *testing.T) {
		t.Parallel()

		service := application.NewEvolutionService(nil, nil, nil, nil)

		_, err := service.GetProposal(context.Background(), "p1")
		if !errors.Is(err, proposal.ErrProposalNotFound) {
			t.Errorf("GetProposal() error = %v, want %v", err, proposal.ErrProposalNotFound)
		}
	})

	t.Run("with workflow", func(t *testing.T) {
		t.Parallel()

		// Note: GetProposal always returns ErrProposalNotFound in current implementation
		workflow := &infraProposal.WorkflowService{}
		service := application.NewEvolutionService(nil, nil, workflow, nil)

		_, err := service.GetProposal(context.Background(), "p1")
		if !errors.Is(err, proposal.ErrProposalNotFound) {
			t.Errorf("GetProposal() error = %v, want %v", err, proposal.ErrProposalNotFound)
		}
	})
}

func TestEvolutionService_SubmitProposal(t *testing.T) {
	t.Parallel()

	t.Run("without workflow", func(t *testing.T) {
		t.Parallel()

		service := application.NewEvolutionService(nil, nil, nil, nil)

		err := service.SubmitProposal(context.Background(), "p1", "admin")
		if !errors.Is(err, proposal.ErrProposalNotFound) {
			t.Errorf("SubmitProposal() error = %v, want %v", err, proposal.ErrProposalNotFound)
		}
	})
}

func TestEvolutionService_ApproveProposal(t *testing.T) {
	t.Parallel()

	t.Run("without workflow", func(t *testing.T) {
		t.Parallel()

		service := application.NewEvolutionService(nil, nil, nil, nil)

		err := service.ApproveProposal(context.Background(), "p1", "admin", "approved")
		if !errors.Is(err, proposal.ErrProposalNotFound) {
			t.Errorf("ApproveProposal() error = %v, want %v", err, proposal.ErrProposalNotFound)
		}
	})
}

func TestEvolutionService_RejectProposal(t *testing.T) {
	t.Parallel()

	t.Run("without workflow", func(t *testing.T) {
		t.Parallel()

		service := application.NewEvolutionService(nil, nil, nil, nil)

		err := service.RejectProposal(context.Background(), "p1", "admin", "rejected")
		if !errors.Is(err, proposal.ErrProposalNotFound) {
			t.Errorf("RejectProposal() error = %v, want %v", err, proposal.ErrProposalNotFound)
		}
	})
}

func TestEvolutionService_ApplyProposal(t *testing.T) {
	t.Parallel()

	t.Run("without workflow", func(t *testing.T) {
		t.Parallel()

		service := application.NewEvolutionService(nil, nil, nil, nil)

		err := service.ApplyProposal(context.Background(), "p1")
		if !errors.Is(err, proposal.ErrProposalNotFound) {
			t.Errorf("ApplyProposal() error = %v, want %v", err, proposal.ErrProposalNotFound)
		}
	})
}

func TestEvolutionService_RollbackProposal(t *testing.T) {
	t.Parallel()

	t.Run("without workflow", func(t *testing.T) {
		t.Parallel()

		service := application.NewEvolutionService(nil, nil, nil, nil)

		err := service.RollbackProposal(context.Background(), "p1", "rollback reason")
		if !errors.Is(err, proposal.ErrProposalNotFound) {
			t.Errorf("RollbackProposal() error = %v, want %v", err, proposal.ErrProposalNotFound)
		}
	})
}

func TestEvolutionService_AcceptSuggestion(t *testing.T) {
	t.Parallel()

	t.Run("without suggestion store", func(t *testing.T) {
		t.Parallel()

		service := application.NewEvolutionService(nil, nil, nil, nil)

		_, err := service.AcceptSuggestion(context.Background(), "s1", "admin")
		if !errors.Is(err, suggestion.ErrSuggestionNotFound) {
			t.Errorf("AcceptSuggestion() error = %v, want %v", err, suggestion.ErrSuggestionNotFound)
		}
	})

	t.Run("without workflow", func(t *testing.T) {
		t.Parallel()

		store := &mockSuggestionStore{
			getFn: func(ctx context.Context, id string) (*suggestion.Suggestion, error) {
				return &suggestion.Suggestion{
					ID:         id,
					Title:      "Test Suggestion",
					Status:     suggestion.SuggestionStatusPending,
					ChangeData: json.RawMessage(`{}`),
				}, nil
			},
		}
		service := application.NewEvolutionService(nil, store, nil, nil)

		_, err := service.AcceptSuggestion(context.Background(), "s1", "admin")
		if !errors.Is(err, proposal.ErrInvalidProposal) {
			t.Errorf("AcceptSuggestion() error = %v, want %v", err, proposal.ErrInvalidProposal)
		}
	})

	t.Run("suggestion get error", func(t *testing.T) {
		t.Parallel()

		store := &mockSuggestionStore{
			getFn: func(ctx context.Context, id string) (*suggestion.Suggestion, error) {
				return nil, errors.New("database error")
			},
		}
		workflow := &infraProposal.WorkflowService{}
		service := application.NewEvolutionService(nil, store, workflow, nil)

		_, err := service.AcceptSuggestion(context.Background(), "s1", "admin")
		if err == nil {
			t.Error("AcceptSuggestion() should return error")
		}
	})
}
