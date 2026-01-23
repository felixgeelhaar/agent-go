package mongodb

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/run"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// runDocument is the MongoDB document representation of a run.
type runDocument struct {
	ID           string                 `bson:"_id"`
	Goal         string                 `bson:"goal"`
	CurrentState string                 `bson:"current_state"`
	Vars         map[string]interface{} `bson:"vars"`
	Evidence     []agent.Evidence       `bson:"evidence"`
	Status       string                 `bson:"status"`
	StartTime    time.Time              `bson:"start_time"`
	EndTime      *time.Time             `bson:"end_time,omitempty"`
	Result       json.RawMessage        `bson:"result,omitempty"`
	Error        string                 `bson:"error,omitempty"`
}

// RunStore is a MongoDB-backed implementation of run.Store.
type RunStore struct {
	collection   *mongo.Collection
	queryTimeout time.Duration
}

// NewRunStore creates a new MongoDB run store.
func NewRunStore(client *Client, collectionName string) *RunStore {
	if collectionName == "" {
		collectionName = "runs"
	}
	return &RunStore{
		collection:   client.Collection(collectionName),
		queryTimeout: client.config.QueryTimeout,
	}
}

// Save persists a new run.
func (s *RunStore) Save(ctx context.Context, r *agent.Run) error {
	if r.ID == "" {
		return run.ErrInvalidRunID
	}

	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	doc := s.toDocument(r)

	_, err := s.collection.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return run.ErrRunExists
		}
		return s.wrapError(err)
	}

	return nil
}

// Get retrieves a run by ID.
func (s *RunStore) Get(ctx context.Context, id string) (*agent.Run, error) {
	if id == "" {
		return nil, run.ErrInvalidRunID
	}

	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	var doc runDocument
	err := s.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, run.ErrRunNotFound
		}
		return nil, s.wrapError(err)
	}

	return s.fromDocument(&doc), nil
}

// Update updates an existing run.
func (s *RunStore) Update(ctx context.Context, r *agent.Run) error {
	if r.ID == "" {
		return run.ErrInvalidRunID
	}

	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	doc := s.toDocument(r)

	result, err := s.collection.ReplaceOne(ctx, bson.M{"_id": r.ID}, doc)
	if err != nil {
		return s.wrapError(err)
	}

	if result.MatchedCount == 0 {
		return run.ErrRunNotFound
	}

	return nil
}

// Delete removes a run by ID.
func (s *RunStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return run.ErrInvalidRunID
	}

	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	result, err := s.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return s.wrapError(err)
	}

	if result.DeletedCount == 0 {
		return run.ErrRunNotFound
	}

	return nil
}

// List returns runs matching the filter.
func (s *RunStore) List(ctx context.Context, filter run.ListFilter) ([]*agent.Run, error) {
	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	mongoFilter := s.buildFilter(filter)
	opts := s.buildFindOptions(filter)

	cursor, err := s.collection.Find(ctx, mongoFilter, opts)
	if err != nil {
		return nil, s.wrapError(err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var runs []*agent.Run
	for cursor.Next(ctx) {
		var doc runDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, s.wrapError(err)
		}
		runs = append(runs, s.fromDocument(&doc))
	}

	if err := cursor.Err(); err != nil {
		return nil, s.wrapError(err)
	}

	return runs, nil
}

// Count returns the number of runs matching the filter.
func (s *RunStore) Count(ctx context.Context, filter run.ListFilter) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	mongoFilter := s.buildFilter(filter)

	count, err := s.collection.CountDocuments(ctx, mongoFilter)
	if err != nil {
		return 0, s.wrapError(err)
	}

	return count, nil
}

// Summary returns aggregate statistics.
func (s *RunStore) Summary(ctx context.Context, filter run.ListFilter) (run.Summary, error) {
	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	matchStage := bson.D{{Key: "$match", Value: s.buildFilter(filter)}}

	pipeline := mongo.Pipeline{
		matchStage,
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "completed", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: bson.A{bson.D{{Key: "$eq", Value: bson.A{"$status", "completed"}}}, 1, 0}},
			}}}},
			{Key: "failed", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: bson.A{bson.D{{Key: "$eq", Value: bson.A{"$status", "failed"}}}, 1, 0}},
			}}}},
			{Key: "running", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: bson.A{bson.D{{Key: "$eq", Value: bson.A{"$status", "running"}}}, 1, 0}},
			}}}},
			{Key: "avg_duration", Value: bson.D{{Key: "$avg", Value: bson.D{
				{Key: "$cond", Value: bson.A{
					bson.D{{Key: "$ne", Value: bson.A{"$end_time", nil}}},
					bson.D{{Key: "$subtract", Value: bson.A{"$end_time", "$start_time"}}},
					nil,
				}},
			}}}},
		}}},
	}

	cursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return run.Summary{}, s.wrapError(err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var summary run.Summary
	if cursor.Next(ctx) {
		var result struct {
			Total       int64   `bson:"total"`
			Completed   int64   `bson:"completed"`
			Failed      int64   `bson:"failed"`
			Running     int64   `bson:"running"`
			AvgDuration float64 `bson:"avg_duration"`
		}
		if err := cursor.Decode(&result); err != nil {
			return run.Summary{}, s.wrapError(err)
		}

		summary.TotalRuns = result.Total
		summary.CompletedRuns = result.Completed
		summary.FailedRuns = result.Failed
		summary.RunningRuns = result.Running
		if result.AvgDuration > 0 {
			summary.AverageDuration = time.Duration(result.AvgDuration * float64(time.Millisecond))
		}
	}

	return summary, nil
}

// buildFilter constructs a MongoDB filter from the domain filter.
func (s *RunStore) buildFilter(filter run.ListFilter) bson.M {
	mongoFilter := bson.M{}

	if len(filter.Status) > 0 {
		statuses := make([]string, len(filter.Status))
		for i, status := range filter.Status {
			statuses[i] = string(status)
		}
		mongoFilter["status"] = bson.M{"$in": statuses}
	}

	if len(filter.States) > 0 {
		states := make([]string, len(filter.States))
		for i, state := range filter.States {
			states[i] = string(state)
		}
		mongoFilter["current_state"] = bson.M{"$in": states}
	}

	if !filter.FromTime.IsZero() {
		if _, ok := mongoFilter["start_time"]; !ok {
			mongoFilter["start_time"] = bson.M{}
		}
		mongoFilter["start_time"].(bson.M)["$gte"] = filter.FromTime
	}

	if !filter.ToTime.IsZero() {
		if _, ok := mongoFilter["start_time"]; !ok {
			mongoFilter["start_time"] = bson.M{}
		}
		mongoFilter["start_time"].(bson.M)["$lte"] = filter.ToTime
	}

	if filter.GoalPattern != "" {
		mongoFilter["goal"] = bson.M{"$regex": primitive.Regex{Pattern: filter.GoalPattern, Options: "i"}}
	}

	return mongoFilter
}

// buildFindOptions constructs MongoDB find options from the domain filter.
func (s *RunStore) buildFindOptions(filter run.ListFilter) *options.FindOptions {
	opts := options.Find()

	// Sort order
	sortField := "start_time"
	switch filter.OrderBy {
	case run.OrderByEndTime:
		sortField = "end_time"
	case run.OrderByID:
		sortField = "_id"
	case run.OrderByStatus:
		sortField = "status"
	}

	sortDir := 1
	if filter.Descending {
		sortDir = -1
	}
	opts.SetSort(bson.D{{Key: sortField, Value: sortDir}})

	if filter.Limit > 0 {
		opts.SetLimit(int64(filter.Limit))
	}

	if filter.Offset > 0 {
		opts.SetSkip(int64(filter.Offset))
	}

	return opts
}

// toDocument converts a Run to a MongoDB document.
func (s *RunStore) toDocument(r *agent.Run) *runDocument {
	doc := &runDocument{
		ID:           r.ID,
		Goal:         r.Goal,
		CurrentState: string(r.CurrentState),
		Vars:         r.Vars,
		Evidence:     r.Evidence,
		Status:       string(r.Status),
		StartTime:    r.StartTime,
		Result:       r.Result,
		Error:        r.Error,
	}

	if !r.EndTime.IsZero() {
		doc.EndTime = &r.EndTime
	}

	return doc
}

// fromDocument converts a MongoDB document to a Run.
func (s *RunStore) fromDocument(doc *runDocument) *agent.Run {
	r := &agent.Run{
		ID:           doc.ID,
		Goal:         doc.Goal,
		CurrentState: agent.State(doc.CurrentState),
		Vars:         doc.Vars,
		Evidence:     doc.Evidence,
		Status:       agent.RunStatus(doc.Status),
		StartTime:    doc.StartTime,
		Result:       doc.Result,
		Error:        doc.Error,
	}

	if doc.EndTime != nil {
		r.EndTime = *doc.EndTime
	}

	return r
}

// wrapError wraps MongoDB errors with domain errors.
func (s *RunStore) wrapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(run.ErrOperationTimeout, err)
	}

	return errors.Join(run.ErrConnectionFailed, err)
}

// Ensure RunStore implements run.Store and run.SummaryProvider
var (
	_ run.Store           = (*RunStore)(nil)
	_ run.SummaryProvider = (*RunStore)(nil)
)
