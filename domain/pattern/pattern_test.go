package pattern

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// Pattern Creation Tests

func TestNewPattern_CreatesValidPattern(t *testing.T) {
	p := NewPattern(PatternTypeToolSequence, "test pattern", "test description")

	if p.ID == "" {
		t.Error("expected non-empty ID")
	}
	if p.Type != PatternTypeToolSequence {
		t.Errorf("expected type %s, got %s", PatternTypeToolSequence, p.Type)
	}
	if p.Name != "test pattern" {
		t.Errorf("expected name 'test pattern', got %s", p.Name)
	}
	if p.Description != "test description" {
		t.Errorf("expected description 'test description', got %s", p.Description)
	}
	if p.FirstSeen.IsZero() {
		t.Error("expected FirstSeen to be set")
	}
	if p.LastSeen.IsZero() {
		t.Error("expected LastSeen to be set")
	}
	if p.FirstSeen != p.LastSeen {
		t.Error("expected FirstSeen and LastSeen to be equal for new pattern")
	}
	if p.Frequency != 0 {
		t.Errorf("expected frequency 0, got %d", p.Frequency)
	}
	if len(p.RunIDs) != 0 {
		t.Errorf("expected empty RunIDs, got %d", len(p.RunIDs))
	}
	if len(p.Evidence) != 0 {
		t.Errorf("expected empty Evidence, got %d", len(p.Evidence))
	}
	if p.Metadata == nil {
		t.Error("expected Metadata to be initialized")
	}
}

func TestNewPattern_GeneratesUniqueIDs(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		p := NewPattern(PatternTypeToolSequence, "test", "test")
		if ids[p.ID] {
			t.Errorf("duplicate ID generated: %s", p.ID)
		}
		ids[p.ID] = true
	}
}

func TestNewPattern_AllPatternTypes(t *testing.T) {
	types := []PatternType{
		PatternTypeToolSequence,
		PatternTypeStateLoop,
		PatternTypeToolAffinity,
		PatternTypeRecurringFailure,
		PatternTypeToolFailure,
		PatternTypeBudgetExhaustion,
		PatternTypeSlowTool,
		PatternTypeLongRuns,
	}

	for _, typ := range types {
		p := NewPattern(typ, "test", "test")
		if p.Type != typ {
			t.Errorf("expected type %s, got %s", typ, p.Type)
		}
	}
}

// Evidence Accumulation Tests

func TestAddEvidence_AccumulatesEvidence(t *testing.T) {
	p := NewPattern(PatternTypeToolSequence, "test", "test")

	details := map[string]string{"key": "value"}
	err := p.AddEvidence("run-1", details)
	if err != nil {
		t.Fatalf("failed to add evidence: %v", err)
	}

	if len(p.Evidence) != 1 {
		t.Errorf("expected 1 evidence entry, got %d", len(p.Evidence))
	}
	if p.Frequency != 1 {
		t.Errorf("expected frequency 1, got %d", p.Frequency)
	}
	if len(p.RunIDs) != 1 || p.RunIDs[0] != "run-1" {
		t.Errorf("expected RunIDs to contain 'run-1', got %v", p.RunIDs)
	}
}

func TestAddEvidence_DeduplicatesRunIDs(t *testing.T) {
	p := NewPattern(PatternTypeToolSequence, "test", "test")

	p.AddEvidence("run-1", nil)
	p.AddEvidence("run-1", nil) // Same run ID
	p.AddEvidence("run-2", nil)

	if len(p.RunIDs) != 2 {
		t.Errorf("expected 2 unique RunIDs, got %d", len(p.RunIDs))
	}
	if p.Frequency != 3 {
		t.Errorf("expected frequency 3, got %d", p.Frequency)
	}
}

func TestAddEvidence_UpdatesLastSeen(t *testing.T) {
	p := NewPattern(PatternTypeToolSequence, "test", "test")
	firstSeen := p.FirstSeen

	time.Sleep(1 * time.Millisecond) // Ensure time difference
	p.AddEvidence("run-1", nil)

	if !p.LastSeen.After(firstSeen) {
		t.Error("expected LastSeen to be after FirstSeen")
	}
	if p.FirstSeen != firstSeen {
		t.Error("FirstSeen should not change when adding evidence")
	}
}

func TestAddEvidence_SerializesDetails(t *testing.T) {
	p := NewPattern(PatternTypeToolSequence, "test", "test")

	details := ToolSequenceData{
		Sequence: []string{"tool1", "tool2"},
	}
	err := p.AddEvidence("run-1", details)
	if err != nil {
		t.Fatalf("failed to add evidence: %v", err)
	}

	if p.Evidence[0].Details == nil {
		t.Error("expected evidence details to be set")
	}
}

func TestAddEvidence_HandlesUnserializableDetails(t *testing.T) {
	p := NewPattern(PatternTypeToolSequence, "test", "test")

	// Channels can't be serialized to JSON
	details := make(chan int)
	err := p.AddEvidence("run-1", details)
	if err == nil {
		t.Error("expected error for unserializable details")
	}
}

// Data Set/Get Tests

func TestSetData_SerializesData(t *testing.T) {
	p := NewPattern(PatternTypeToolSequence, "test", "test")

	data := ToolSequenceData{
		Sequence:   []string{"read", "write"},
		AverageGap: 100 * time.Millisecond,
	}
	err := p.SetData(data)
	if err != nil {
		t.Fatalf("failed to set data: %v", err)
	}

	if p.Data == nil {
		t.Error("expected Data to be set")
	}
}

func TestGetData_DeserializesData(t *testing.T) {
	p := NewPattern(PatternTypeToolSequence, "test", "test")

	original := ToolSequenceData{
		Sequence:   []string{"read", "write"},
		AverageGap: 100 * time.Millisecond,
	}
	p.SetData(original)

	var retrieved ToolSequenceData
	err := p.GetData(&retrieved)
	if err != nil {
		t.Fatalf("failed to get data: %v", err)
	}

	if len(retrieved.Sequence) != 2 {
		t.Errorf("expected 2 tools in sequence, got %d", len(retrieved.Sequence))
	}
	if retrieved.Sequence[0] != "read" || retrieved.Sequence[1] != "write" {
		t.Errorf("unexpected sequence: %v", retrieved.Sequence)
	}
}

func TestGetData_HandlesNilData(t *testing.T) {
	p := NewPattern(PatternTypeToolSequence, "test", "test")

	var data ToolSequenceData
	err := p.GetData(&data)
	if err != nil {
		t.Errorf("expected no error for nil data, got %v", err)
	}
}

func TestSetData_HandlesUnserializableData(t *testing.T) {
	p := NewPattern(PatternTypeToolSequence, "test", "test")

	// Channels can't be serialized
	err := p.SetData(make(chan int))
	if err == nil {
		t.Error("expected error for unserializable data")
	}
}

// Significance Tests

func TestIsSignificant_ReturnsTrueWhenMeetsThresholds(t *testing.T) {
	p := NewPattern(PatternTypeToolSequence, "test", "test")
	p.Confidence = 0.8
	p.Frequency = 5

	if !p.IsSignificant(0.7, 3) {
		t.Error("expected pattern to be significant")
	}
}

func TestIsSignificant_ReturnsFalseWhenBelowConfidence(t *testing.T) {
	p := NewPattern(PatternTypeToolSequence, "test", "test")
	p.Confidence = 0.5
	p.Frequency = 10

	if p.IsSignificant(0.7, 3) {
		t.Error("expected pattern to not be significant (low confidence)")
	}
}

func TestIsSignificant_ReturnsFalseWhenBelowFrequency(t *testing.T) {
	p := NewPattern(PatternTypeToolSequence, "test", "test")
	p.Confidence = 0.9
	p.Frequency = 1

	if p.IsSignificant(0.5, 3) {
		t.Error("expected pattern to not be significant (low frequency)")
	}
}

func TestIsSignificant_BoundaryConditions(t *testing.T) {
	p := NewPattern(PatternTypeToolSequence, "test", "test")
	p.Confidence = 0.7
	p.Frequency = 3

	// Exactly at thresholds should be significant
	if !p.IsSignificant(0.7, 3) {
		t.Error("expected pattern at exact thresholds to be significant")
	}
}

// DetectionOptions Tests

func TestDefaultDetectionOptions(t *testing.T) {
	opts := DefaultDetectionOptions()

	if opts.MinConfidence != 0.5 {
		t.Errorf("expected MinConfidence 0.5, got %f", opts.MinConfidence)
	}
	if opts.MinFrequency != 2 {
		t.Errorf("expected MinFrequency 2, got %d", opts.MinFrequency)
	}
	if opts.Limit != 100 {
		t.Errorf("expected Limit 100, got %d", opts.Limit)
	}
}

func TestDetectionOptions_TimeFilters(t *testing.T) {
	now := time.Now()
	opts := DetectionOptions{
		FromTime: now.Add(-24 * time.Hour),
		ToTime:   now,
	}

	if opts.FromTime.IsZero() {
		t.Error("expected FromTime to be set")
	}
	if opts.ToTime.IsZero() {
		t.Error("expected ToTime to be set")
	}
	if !opts.FromTime.Before(opts.ToTime) {
		t.Error("expected FromTime to be before ToTime")
	}
}

// DetectorFunc Tests

func TestDetectorFunc_ImplementsDetector(t *testing.T) {
	called := false
	expectedPatterns := []Pattern{
		*NewPattern(PatternTypeToolSequence, "test", "test"),
	}

	detector := DetectorFunc(func(ctx context.Context, opts DetectionOptions) ([]Pattern, error) {
		called = true
		return expectedPatterns, nil
	})

	ctx := context.Background()
	patterns, err := detector.Detect(ctx, DetectionOptions{})

	if !called {
		t.Error("expected detector function to be called")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(patterns) != 1 {
		t.Errorf("expected 1 pattern, got %d", len(patterns))
	}
}

func TestDetectorFunc_TypesReturnsNil(t *testing.T) {
	detector := DetectorFunc(func(ctx context.Context, opts DetectionOptions) ([]Pattern, error) {
		return nil, nil
	})

	types := detector.Types()
	if types != nil {
		t.Errorf("expected nil Types(), got %v", types)
	}
}

// ListFilter Tests

func TestListFilter_DefaultValues(t *testing.T) {
	filter := ListFilter{}

	if filter.MinConfidence != 0 {
		t.Error("expected zero MinConfidence by default")
	}
	if filter.MinFrequency != 0 {
		t.Error("expected zero MinFrequency by default")
	}
	if filter.Limit != 0 {
		t.Error("expected zero Limit by default")
	}
	if filter.OrderBy != "" {
		t.Error("expected empty OrderBy by default")
	}
}

func TestListFilter_TypeFiltering(t *testing.T) {
	filter := ListFilter{
		Types: []PatternType{PatternTypeToolSequence, PatternTypeToolFailure},
	}

	if len(filter.Types) != 2 {
		t.Errorf("expected 2 types, got %d", len(filter.Types))
	}
}

func TestListFilter_Pagination(t *testing.T) {
	filter := ListFilter{
		Limit:  10,
		Offset: 20,
	}

	if filter.Limit != 10 {
		t.Errorf("expected Limit 10, got %d", filter.Limit)
	}
	if filter.Offset != 20 {
		t.Errorf("expected Offset 20, got %d", filter.Offset)
	}
}

func TestListFilter_Ordering(t *testing.T) {
	testCases := []OrderBy{
		OrderByFirstSeen,
		OrderByLastSeen,
		OrderByFrequency,
		OrderByConfidence,
	}

	for _, order := range testCases {
		filter := ListFilter{OrderBy: order, Descending: true}
		if filter.OrderBy != order {
			t.Errorf("expected OrderBy %s, got %s", order, filter.OrderBy)
		}
		if !filter.Descending {
			t.Error("expected Descending to be true")
		}
	}
}

// Pattern Data Type Tests

func TestToolSequenceData_Serialization(t *testing.T) {
	data := ToolSequenceData{
		Sequence:   []string{"read", "process", "write"},
		AverageGap: 500 * time.Millisecond,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled ToolSequenceData
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(unmarshaled.Sequence) != 3 {
		t.Errorf("expected 3 tools, got %d", len(unmarshaled.Sequence))
	}
}

func TestToolAffinityData_Serialization(t *testing.T) {
	data := ToolAffinityData{
		Tools:       []string{"tool_a", "tool_b"},
		Correlation: 0.85,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled ToolAffinityData
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.Correlation != 0.85 {
		t.Errorf("expected correlation 0.85, got %f", unmarshaled.Correlation)
	}
}

func TestFailureData_Serialization(t *testing.T) {
	data := FailureData{
		FailureType:  "timeout",
		ToolName:     "slow_tool",
		ErrorPattern: "context deadline exceeded",
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled FailureData
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.FailureType != "timeout" {
		t.Errorf("expected failure type 'timeout', got %s", unmarshaled.FailureType)
	}
}

// Error Sentinel Tests

func TestErrorSentinels_Defined(t *testing.T) {
	if ErrPatternNotFound == nil {
		t.Error("ErrPatternNotFound should not be nil")
	}
	if ErrPatternExists == nil {
		t.Error("ErrPatternExists should not be nil")
	}
	if ErrInvalidPattern == nil {
		t.Error("ErrInvalidPattern should not be nil")
	}
	if ErrInvalidPatternType == nil {
		t.Error("ErrInvalidPatternType should not be nil")
	}
	if ErrInsufficientData == nil {
		t.Error("ErrInsufficientData should not be nil")
	}
	if ErrDetectionFailed == nil {
		t.Error("ErrDetectionFailed should not be nil")
	}
}

func TestErrorSentinels_HaveMessages(t *testing.T) {
	errors := []error{
		ErrPatternNotFound,
		ErrPatternExists,
		ErrInvalidPattern,
		ErrInvalidPatternType,
		ErrInsufficientData,
		ErrDetectionFailed,
	}

	for _, err := range errors {
		if err.Error() == "" {
			t.Errorf("error %v should have a message", err)
		}
	}
}
