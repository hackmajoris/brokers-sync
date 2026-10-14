package watchlist

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Limits are enforced server-side so the table cannot be used as free storage.
const (
	MaxSymbols   = 50
	MaxNoteLen   = 500
	MaxSymbolLen = 12
	ttlWindow    = 365 * 24 * time.Hour
	// accessWindow is how stale lastAccess may get before a read refreshes it.
	// The table has 1 write unit, so recording every page load would compete
	// with real edits; one write per list per hour is plenty for analytics.
	accessWindow = time.Hour
)

var (
	// ErrNotFound is returned for an unknown portfolio. Callers must not
	// distinguish it from a malformed code in their response.
	ErrNotFound = errors.New("portfolio not found")
	ErrTooMany  = fmt.Errorf("watchlist limited to %d symbols", MaxSymbols)
	ErrInvalid  = errors.New("invalid item")
	// ErrCapReached means the portfolio cap is used up and no new code may be
	// created until it is raised.
	ErrCapReached = errors.New("portfolio cap reached")

	symbolRe = regexp.MustCompile(`^[A-Z0-9.\-]+$`)
)

// API is the subset of the DynamoDB client the store uses, so tests can supply
// a fake without reaching AWS.
type API interface {
	Query(context.Context, *dynamodb.QueryInput, ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	PutItem(context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	UpdateItem(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	DeleteItem(context.Context, *dynamodb.DeleteItemInput, ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
}

// Item is one watchlist entry.
type Item struct {
	Symbol      string  `dynamodbav:"symbol" json:"symbol"`
	Note        string  `dynamodbav:"note" json:"note"`
	TargetPrice float64 `dynamodbav:"targetPrice" json:"targetPrice"`
	Pinned      bool    `dynamodbav:"pinned" json:"pinned"`
	AddedAt     int64   `dynamodbav:"addedAt" json:"addedAt"`
}

type Store struct {
	db    API
	table string
}

func NewStore(db API, table string) *Store {
	return &Store{db: db, table: table}
}

// EnsureMeta creates the portfolio record. It is a no-op if one already exists,
// so an existing portfolio is never reset by a repeated call.
func (s *Store) EnsureMeta(ctx context.Context, pk string) error {
	now := time.Now()
	_, err := s.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item: map[string]types.AttributeValue{
			"PK":        &types.AttributeValueMemberS{Value: pk},
			"SK":        &types.AttributeValueMemberS{Value: "META"},
			"createdAt": num(now.Unix()),
			"lastSeen":  num(now.Unix()),
			"ttl":       num(now.Add(ttlWindow).Unix()),
		},
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
		return nil
	}
	return err
}

// The counter shares the table with portfolios. Portfolio keys are code hashes,
// so this key can never collide with one.
const (
	counterPK = "SYSTEM#codes"
	counterSK = "COUNTER"
)

// Reserve claims one of max portfolio slots, atomically: concurrent callers can
// never push the count past max. It counts every code ever created and is never
// decremented, so expired portfolios do not free their slots. Call it before
// EnsureMeta; a failed create then over-counts by one, which errs on the safe
// side of the cap.
func (s *Store) Reserve(ctx context.Context, max int) error {
	_, err := s.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: counterPK},
			"SK": &types.AttributeValueMemberS{Value: counterSK},
		},
		UpdateExpression:         aws.String("ADD #n :one"),
		ConditionExpression:      aws.String("attribute_not_exists(#n) OR #n < :max"),
		ExpressionAttributeNames: map[string]string{"#n": "created"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one": num(1),
			":max": num(int64(max)),
		},
	})
	if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
		return ErrCapReached
	}
	return err
}

// List returns every watchlist item for a portfolio. It reports ErrNotFound when
// the portfolio does not exist, which callers must surface as a bare 404.
func (s *Store) List(ctx context.Context, pk string) ([]Item, error) {
	items, _, err := s.list(ctx, pk)
	return items, err
}

// ListAndRecordAccess is List for a user opening their list: it also stamps
// lastAccess on the portfolio record, at most once per accessWindow. Unlike
// lastSeen, which only moves on edits, this shows lists that are read but
// rarely changed. The stamp is bookkeeping, so a failed write never fails the
// read; the cost is lastAccess lagging by one more window.
func (s *Store) ListAndRecordAccess(ctx context.Context, pk string, now time.Time) ([]Item, error) {
	items, lastAccess, err := s.list(ctx, pk)
	if err != nil {
		return nil, err
	}
	if now.Sub(time.Unix(lastAccess, 0)) >= accessWindow {
		_ = s.recordAccess(ctx, pk, now)
	}
	return items, nil
}

func (s *Store) recordAccess(ctx context.Context, pk string, now time.Time) error {
	_, err := s.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: "META"},
		},
		UpdateExpression: aws.String("SET lastAccess = :access"),
		// Never recreate a portfolio that expired between the read and here.
		ConditionExpression: aws.String("attribute_exists(PK)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":access": num(now.Unix()),
		},
	})
	return err
}

// list also reports the portfolio's lastAccess (0 when never recorded), which
// the Query returns with the META record at no extra cost.
func (s *Store) list(ctx context.Context, pk string) ([]Item, int64, error) {
	out, err := s.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.table),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
	})
	if err != nil {
		return nil, 0, err
	}

	var meta bool
	var lastAccess int64
	items := make([]Item, 0, len(out.Items))
	for _, raw := range out.Items {
		sk, _ := raw["SK"].(*types.AttributeValueMemberS)
		if sk == nil {
			continue
		}
		if sk.Value == "META" {
			meta = true
			if n, ok := raw["lastAccess"].(*types.AttributeValueMemberN); ok {
				lastAccess, _ = strconv.ParseInt(n.Value, 10, 64)
			}
			continue
		}
		var it Item
		if err := attributevalue.UnmarshalMap(raw, &it); err != nil {
			return nil, 0, err
		}
		items = append(items, it)
	}
	if !meta {
		return nil, 0, ErrNotFound
	}
	return items, lastAccess, nil
}

// Upsert adds or replaces one watchlist entry and refreshes the portfolio TTL so
// an actively used portfolio never expires.
func (s *Store) Upsert(ctx context.Context, pk string, it Item) error {
	it.Symbol = strings.ToUpper(strings.TrimSpace(it.Symbol))
	if it.Symbol == "" || len(it.Symbol) > MaxSymbolLen || !symbolRe.MatchString(it.Symbol) {
		return ErrInvalid
	}
	if len(it.Note) > MaxNoteLen {
		return ErrInvalid
	}

	existing, err := s.List(ctx, pk)
	if err != nil {
		return err
	}
	if len(existing) >= MaxSymbols && !contains(existing, it.Symbol) {
		return ErrTooMany
	}

	if it.AddedAt == 0 {
		it.AddedAt = time.Now().Unix()
	}
	av, err := attributevalue.MarshalMap(it)
	if err != nil {
		return err
	}
	av["PK"] = &types.AttributeValueMemberS{Value: pk}
	av["SK"] = &types.AttributeValueMemberS{Value: "W#" + it.Symbol}

	if _, err := s.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item:      av,
	}); err != nil {
		return err
	}
	return s.touch(ctx, pk)
}

// Delete removes one watchlist entry.
func (s *Store) Delete(ctx context.Context, pk, symbol string) error {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return ErrInvalid
	}
	if _, err := s.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: "W#" + symbol},
		},
	}); err != nil {
		return err
	}
	return s.touch(ctx, pk)
}

// touch pushes the expiry out on write.
func (s *Store) touch(ctx context.Context, pk string) error {
	now := time.Now()
	_, err := s.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: "META"},
		},
		UpdateExpression: aws.String("SET lastSeen = :now, #t = :ttl"),
		ExpressionAttributeNames: map[string]string{
			"#t": "ttl",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now": num(now.Unix()),
			":ttl": num(now.Add(ttlWindow).Unix()),
		},
	})
	return err
}

func contains(items []Item, symbol string) bool {
	for _, it := range items {
		if it.Symbol == symbol {
			return true
		}
	}
	return false
}

func num(v int64) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", v)}
}
