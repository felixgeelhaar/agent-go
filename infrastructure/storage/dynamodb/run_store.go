package dynamodb

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/run"
)

// runItem represents a run in DynamoDB.
type runItem struct {
	ID           string          `dynamodbav:"id"`
	Goal         string          `dynamodbav:"goal"`
	CurrentState string          `dynamodbav:"current_state"`
	Vars         json.RawMessage `dynamodbav:"vars,omitempty"`
	Evidence     json.RawMessage `dynamodbav:"evidence,omitempty"`
	Status       string          `dynamodbav:"status"`
	StartTime    string          `dynamodbav:"start_time"`
	EndTime      string          `dynamodbav:"end_time,omitempty"`
	Result       []byte          `dynamodbav:"result,omitempty"`
	Error        string          `dynamodbav:"error,omitempty"`
}

// RunStore is a DynamoDB-backed implementation of run.Store.
type RunStore struct {
	client       *dynamodb.Client
	tableName    string
	queryTimeout time.Duration
}

// NewRunStore creates a new DynamoDB run store.
func NewRunStore(client *Client) *RunStore {
	return &RunStore{
		client:       client.DynamoDB(),
		tableName:    client.config.RunsTableName,
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

	item := s.toItem(r)
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}

	// Use condition expression to prevent overwriting existing items
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(s.tableName),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	})
	if err != nil {
		var conditionFailed *types.ConditionalCheckFailedException
		if errors.As(err, &conditionFailed) {
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

	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return nil, s.wrapError(err)
	}

	if result.Item == nil {
		return nil, run.ErrRunNotFound
	}

	var item runItem
	if err := attributevalue.UnmarshalMap(result.Item, &item); err != nil {
		return nil, err
	}

	return s.fromItem(&item), nil
}

// Update updates an existing run.
func (s *RunStore) Update(ctx context.Context, r *agent.Run) error {
	if r.ID == "" {
		return run.ErrInvalidRunID
	}

	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	item := s.toItem(r)
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}

	// Use condition expression to ensure item exists
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(s.tableName),
		Item:                av,
		ConditionExpression: aws.String("attribute_exists(id)"),
	})
	if err != nil {
		var conditionFailed *types.ConditionalCheckFailedException
		if errors.As(err, &conditionFailed) {
			return run.ErrRunNotFound
		}
		return s.wrapError(err)
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

	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
		ConditionExpression: aws.String("attribute_exists(id)"),
	})
	if err != nil {
		var conditionFailed *types.ConditionalCheckFailedException
		if errors.As(err, &conditionFailed) {
			return run.ErrRunNotFound
		}
		return s.wrapError(err)
	}

	return nil
}

// List returns runs matching the filter.
func (s *RunStore) List(ctx context.Context, filter run.ListFilter) ([]*agent.Run, error) {
	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	// Build filter expression
	var scanInput *dynamodb.ScanInput

	if len(filter.Status) > 0 {
		// Use GSI for status-based queries
		statusValues := make([]string, len(filter.Status))
		for i, st := range filter.Status {
			statusValues[i] = string(st)
		}

		// For multiple statuses, we need to scan with filter
		var cond expression.ConditionBuilder
		for i, st := range statusValues {
			if i == 0 {
				cond = expression.Name("status").Equal(expression.Value(st))
			} else {
				cond = cond.Or(expression.Name("status").Equal(expression.Value(st)))
			}
		}

		expr, err := expression.NewBuilder().WithFilter(cond).Build()
		if err != nil {
			return nil, err
		}

		scanInput = &dynamodb.ScanInput{
			TableName:                 aws.String(s.tableName),
			FilterExpression:          expr.Filter(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
		}
	} else {
		scanInput = &dynamodb.ScanInput{
			TableName: aws.String(s.tableName),
		}
	}

	if filter.Limit > 0 {
		scanInput.Limit = aws.Int32(int32(filter.Limit))
	}

	var runs []*agent.Run
	paginator := dynamodb.NewScanPaginator(s.client, scanInput)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, s.wrapError(err)
		}

		for _, item := range page.Items {
			var runItem runItem
			if err := attributevalue.UnmarshalMap(item, &runItem); err != nil {
				return nil, err
			}
			runs = append(runs, s.fromItem(&runItem))
		}

		// Respect limit
		if filter.Limit > 0 && len(runs) >= filter.Limit {
			runs = runs[:filter.Limit]
			break
		}
	}

	// Apply offset
	if filter.Offset > 0 && filter.Offset < len(runs) {
		runs = runs[filter.Offset:]
	} else if filter.Offset >= len(runs) {
		return []*agent.Run{}, nil
	}

	return runs, nil
}

// Count returns the number of runs matching the filter.
func (s *RunStore) Count(ctx context.Context, filter run.ListFilter) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	var scanInput *dynamodb.ScanInput

	if len(filter.Status) > 0 {
		statusValues := make([]string, len(filter.Status))
		for i, st := range filter.Status {
			statusValues[i] = string(st)
		}

		var cond expression.ConditionBuilder
		for i, st := range statusValues {
			if i == 0 {
				cond = expression.Name("status").Equal(expression.Value(st))
			} else {
				cond = cond.Or(expression.Name("status").Equal(expression.Value(st)))
			}
		}

		expr, err := expression.NewBuilder().WithFilter(cond).Build()
		if err != nil {
			return 0, err
		}

		scanInput = &dynamodb.ScanInput{
			TableName:                 aws.String(s.tableName),
			FilterExpression:          expr.Filter(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
			Select:                    types.SelectCount,
		}
	} else {
		scanInput = &dynamodb.ScanInput{
			TableName: aws.String(s.tableName),
			Select:    types.SelectCount,
		}
	}

	var count int64
	paginator := dynamodb.NewScanPaginator(s.client, scanInput)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return 0, s.wrapError(err)
		}
		count += int64(page.Count)
	}

	return count, nil
}

// Summary returns aggregate statistics.
func (s *RunStore) Summary(ctx context.Context, filter run.ListFilter) (run.Summary, error) {
	// DynamoDB doesn't support aggregations natively, so we fetch and compute
	runs, err := s.List(ctx, run.ListFilter{})
	if err != nil {
		return run.Summary{}, err
	}

	var summary run.Summary
	var totalDuration time.Duration
	var completedCount int64

	for _, r := range runs {
		summary.TotalRuns++

		switch r.Status {
		case agent.RunStatusCompleted:
			summary.CompletedRuns++
			if !r.EndTime.IsZero() {
				totalDuration += r.EndTime.Sub(r.StartTime)
				completedCount++
			}
		case agent.RunStatusFailed:
			summary.FailedRuns++
		case agent.RunStatusRunning:
			summary.RunningRuns++
		}
	}

	if completedCount > 0 {
		summary.AverageDuration = totalDuration / time.Duration(completedCount)
	}

	return summary, nil
}

// toItem converts a Run to a DynamoDB item.
func (s *RunStore) toItem(r *agent.Run) *runItem {
	item := &runItem{
		ID:           r.ID,
		Goal:         r.Goal,
		CurrentState: string(r.CurrentState),
		Status:       string(r.Status),
		StartTime:    r.StartTime.Format(time.RFC3339Nano),
		Result:       r.Result,
		Error:        r.Error,
	}

	if !r.EndTime.IsZero() {
		item.EndTime = r.EndTime.Format(time.RFC3339Nano)
	}

	if r.Vars != nil {
		item.Vars, _ = json.Marshal(r.Vars)
	}

	if len(r.Evidence) > 0 {
		item.Evidence, _ = json.Marshal(r.Evidence)
	}

	return item
}

// fromItem converts a DynamoDB item to a Run.
func (s *RunStore) fromItem(item *runItem) *agent.Run {
	r := &agent.Run{
		ID:           item.ID,
		Goal:         item.Goal,
		CurrentState: agent.State(item.CurrentState),
		Status:       agent.RunStatus(item.Status),
		Result:       item.Result,
		Error:        item.Error,
	}

	if item.StartTime != "" {
		r.StartTime, _ = time.Parse(time.RFC3339Nano, item.StartTime)
	}

	if item.EndTime != "" {
		r.EndTime, _ = time.Parse(time.RFC3339Nano, item.EndTime)
	}

	if item.Vars != nil {
		_ = json.Unmarshal(item.Vars, &r.Vars)
	}

	if item.Evidence != nil {
		_ = json.Unmarshal(item.Evidence, &r.Evidence)
	}

	return r
}

// wrapError wraps DynamoDB errors with domain errors.
func (s *RunStore) wrapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(run.ErrOperationTimeout, err)
	}

	// Check for provisioned throughput exceeded
	var throughputExceeded *types.ProvisionedThroughputExceededException
	if errors.As(err, &throughputExceeded) {
		return errors.Join(run.ErrOperationTimeout, err)
	}

	return errors.Join(run.ErrConnectionFailed, err)
}

// Ensure RunStore implements run.Store and run.SummaryProvider
var (
	_ run.Store           = (*RunStore)(nil)
	_ run.SummaryProvider = (*RunStore)(nil)
)
