// Package database — Index bổ sung cho decision_cases_runtime (merge case, lookup theo PLATFORM_L1 §4.6).
package database

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EnsureDecisionCaseRuntimeExtraIndexes tạo index compound phục vụ merge/reopen + FindCaseByOrder (không thay thế CreateIndexes từ model).
func EnsureDecisionCaseRuntimeExtraIndexes(ctx context.Context, coll *mongo.Collection) error {
	if coll == nil {
		return nil
	}
	models := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "orgId", Value: 1},
				{Key: "caseType", Value: 1},
				{Key: "entityRefs.orderId", Value: 1},
				{Key: "ownerOrganizationId", Value: 1},
			},
			Options: options.Index().SetName("idx_order_risk_entity").SetPartialFilterExpression(bson.M{
				"caseType": "order_risk_decision",
			}),
		},
		{
			Keys: bson.D{
				{Key: "orgId", Value: 1},
				{Key: "caseType", Value: 1},
				{Key: "entityRefs.conversationId", Value: 1},
				{Key: "entityRefs.customerId", Value: 1},
			},
			Options: options.Index().SetName("idx_conversation_entity").SetPartialFilterExpression(bson.M{
				"caseType": "conversation_response_decision",
			}),
		},
	}
	_, err := coll.Indexes().CreateMany(ctx, models)
	return err
}
