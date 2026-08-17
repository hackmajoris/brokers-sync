package wishlist

import (
	"context"
	"maps"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// FakeAPI is an in-memory API implementation for tests. It lives outside
// _test.go so both this package and cmd/server can use it.
type FakeAPI struct {
	Items map[string]map[string]types.AttributeValue // PK -> SK -> item
	// CondFailed records whether an attribute_not_exists guard ever rejected a write.
	CondFailed bool
}

func NewFakeAPI() *FakeAPI {
	return &FakeAPI{Items: map[string]map[string]types.AttributeValue{}}
}

func (f *FakeAPI) Query(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	pk := in.ExpressionAttributeValues[":pk"].(*types.AttributeValueMemberS).Value
	var out []map[string]types.AttributeValue
	for sk, item := range f.Items[pk] {
		m := map[string]types.AttributeValue{}
		if inner, ok := item.(*types.AttributeValueMemberM); ok {
			maps.Copy(m, inner.Value)
		}
		m["SK"] = &types.AttributeValueMemberS{Value: sk}
		out = append(out, m)
	}
	return &dynamodb.QueryOutput{Items: out}, nil
}

func (f *FakeAPI) PutItem(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	pk := in.Item["PK"].(*types.AttributeValueMemberS).Value
	sk := in.Item["SK"].(*types.AttributeValueMemberS).Value
	if in.ConditionExpression != nil && strings.Contains(*in.ConditionExpression, "attribute_not_exists") {
		if _, ok := f.Items[pk]; ok {
			f.CondFailed = true
			return nil, &types.ConditionalCheckFailedException{}
		}
	}
	if f.Items[pk] == nil {
		f.Items[pk] = map[string]types.AttributeValue{}
	}
	f.Items[pk][sk] = &types.AttributeValueMemberM{Value: in.Item}
	return &dynamodb.PutItemOutput{}, nil
}

func (f *FakeAPI) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	pk := in.Key["PK"].(*types.AttributeValueMemberS).Value
	sk := in.Key["SK"].(*types.AttributeValueMemberS).Value
	cur, ok := f.Items[pk][sk].(*types.AttributeValueMemberM)
	if !ok {
		return &dynamodb.UpdateItemOutput{}, nil
	}
	for placeholder, v := range in.ExpressionAttributeValues {
		switch placeholder {
		case ":now":
			cur.Value["lastSeen"] = v
		case ":ttl":
			cur.Value["ttl"] = v
		}
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

func (f *FakeAPI) DeleteItem(_ context.Context, in *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	pk := in.Key["PK"].(*types.AttributeValueMemberS).Value
	sk := in.Key["SK"].(*types.AttributeValueMemberS).Value
	delete(f.Items[pk], sk)
	return &dynamodb.DeleteItemOutput{}, nil
}

// Num reads a numeric attribute, returning "" when absent.
func (f *FakeAPI) Num(pk, sk, name string) string {
	m, ok := f.Items[pk][sk].(*types.AttributeValueMemberM)
	if !ok {
		return ""
	}
	n, ok := m.Value[name].(*types.AttributeValueMemberN)
	if !ok {
		return ""
	}
	return n.Value
}
