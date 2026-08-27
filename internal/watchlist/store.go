package watchlist

import (
	"context"
	"errors"
	"fmt"
	"regexp"
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
)

var (
	// ErrNotFound is returned for an unknown portfolio. Callers must not
	// distinguish it from a malformed code in their response.
	ErrNotFound = errors.New("portfolio not found")
	ErrTooMany  = fmt.Errorf("watchlist limited to %d symbols", MaxSymbols)
	ErrInvalid  = errors.New("invalid item")

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

// List returns every watchlist item for a portfolio. It reports ErrNotFound when
// the portfolio does not exist, which callers must surface as a bare 404.
func (s *Store) List(ctx context.Context, pk string) ([]Item, error) {
	out, err := s.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.table),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
	})
	if err != nil {
		return nil, err
	}

	var meta bool
	items := make([]Item, 0, len(out.Items))
	for _, raw := range out.Items {
		sk, _ := raw["SK"].(*types.AttributeValueMemberS)
		if sk == nil {
			continue
		}
		if sk.Value == "META" {
			meta = true
			continue
		}
		var it Item
		if err := attributevalue.UnmarshalMap(raw, &it); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if !meta {
		return nil, ErrNotFound
	}
	return items, nil
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
