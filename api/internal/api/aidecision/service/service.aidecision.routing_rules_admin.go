// Package aidecisionsvc — CRUD decision_routing_rules theo org (API quản trị).
package aidecisionsvc

import (
	"context"
	"errors"
	"strings"
	"time"

	aidecisiondto "meta_commerce/internal/api/aidecision/dto"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ListRoutingRules liệt kê rule theo org, sort eventType.
func (s *AIDecisionService) ListRoutingRules(ctx context.Context, ownerOrgID primitive.ObjectID) ([]aidecisiondto.RoutingRuleItem, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionRoutingRules)
	if !ok {
		return nil, errors.New("collection decision_routing_rules chưa đăng ký")
	}
	cur, err := coll.Find(ctx, bson.M{"ownerOrganizationId": ownerOrgID}, options.Find().SetSort(bson.D{{Key: "eventType", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []aidecisiondto.RoutingRuleItem
	for cur.Next(ctx) {
		var doc aidecisionmodels.DecisionRoutingRule
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		out = append(out, routingRuleFromModel(doc))
	}
	return out, cur.Err()
}

// UpsertRoutingRule tạo hoặc cập nhật theo (org, eventType).
func (s *AIDecisionService) UpsertRoutingRule(ctx context.Context, ownerOrgID primitive.ObjectID, req aidecisiondto.RoutingRuleUpsertRequest) (*aidecisiondto.RoutingRuleItem, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionRoutingRules)
	if !ok {
		return nil, errors.New("collection decision_routing_rules chưa đăng ký")
	}
	et := strings.TrimSpace(req.EventType)
	if et == "" {
		return nil, errors.New("eventType là bắt buộc")
	}
	b := strings.TrimSpace(strings.ToLower(req.Behavior))
	if b != aidecisionmodels.RoutingBehaviorNoop && b != aidecisionmodels.RoutingBehaviorPassThrough {
		return nil, errors.New("behavior phải là noop hoặc pass_through")
	}
	now := time.Now().UnixMilli()
	filter := bson.M{"ownerOrganizationId": ownerOrgID, "eventType": et}
	update := bson.M{
		"$set": bson.M{
			"behavior":  b,
			"enabled":   req.Enabled,
			"note":      strings.TrimSpace(req.Note),
			"updatedAt": now,
		},
		"$setOnInsert": bson.M{
			"ownerOrganizationId": ownerOrgID,
			"eventType":           et,
			"createdAt":           now,
		},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var doc aidecisionmodels.DecisionRoutingRule
	err := coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&doc)
	if err != nil {
		return nil, err
	}
	item := routingRuleFromModel(doc)
	return &item, nil
}

// DeleteRoutingRule xóa theo _id (Mongo) thuộc org.
func (s *AIDecisionService) DeleteRoutingRule(ctx context.Context, ownerOrgID primitive.ObjectID, ruleIDHex string) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionRoutingRules)
	if !ok {
		return errors.New("collection decision_routing_rules chưa đăng ký")
	}
	oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(ruleIDHex))
	if err != nil {
		return errors.New("id không hợp lệ")
	}
	res, err := coll.DeleteOne(ctx, bson.M{"_id": oid, "ownerOrganizationId": ownerOrgID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func routingRuleFromModel(doc aidecisionmodels.DecisionRoutingRule) aidecisiondto.RoutingRuleItem {
	return aidecisiondto.RoutingRuleItem{
		ID:                  doc.ID.Hex(),
		OwnerOrganizationID: doc.OwnerOrganizationID.Hex(),
		EventType:           doc.EventType,
		Behavior:            doc.Behavior,
		Enabled:             doc.Enabled,
		Note:                doc.Note,
		UpdatedAt:           doc.UpdatedAt,
		CreatedAt:           doc.CreatedAt,
	}
}
