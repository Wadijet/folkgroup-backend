package database

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EnsureDecisionTrailingDebounceIndexes — index compound phục vụ FindOneAndDelete theo bucket + dueAtMs (không thay thế CreateIndexes từ model).
func EnsureDecisionTrailingDebounceIndexes(ctx context.Context, coll *mongo.Collection) error {
	if coll == nil {
		return fmt.Errorf("collection nil")
	}
	_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "bucket", Value: 1},
			{Key: "dueAtMs", Value: 1},
		},
		Options: options.Index().SetName("bucket_1_dueAtMs_1"),
	})
	return err
}
