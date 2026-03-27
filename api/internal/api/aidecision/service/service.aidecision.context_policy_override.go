// Package aidecisionsvc — CRUD decision_context_policy_overrides + đọc cho merge policy.
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

// loadContextPolicyOverrideDoc đọc override theo org + caseType (enabled).
func loadContextPolicyOverrideDoc(ctx context.Context, ownerOrgID primitive.ObjectID, caseType string) (*aidecisionmodels.DecisionContextPolicyOverride, error) {
	ct := strings.TrimSpace(caseType)
	if ownerOrgID.IsZero() || ct == "" {
		return nil, nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionContextPolicyOverrides)
	if !ok {
		return nil, nil
	}
	var doc aidecisionmodels.DecisionContextPolicyOverride
	err := coll.FindOne(ctx, bson.M{
		"ownerOrganizationId": ownerOrgID,
		"caseType":            ct,
		"enabled":             true,
	}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// ListContextPolicyOverrides liệt kê override theo org.
func (s *AIDecisionService) ListContextPolicyOverrides(ctx context.Context, ownerOrgID primitive.ObjectID) ([]aidecisiondto.ContextPolicyOverrideItem, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionContextPolicyOverrides)
	if !ok {
		return nil, errors.New("collection decision_context_policy_overrides chưa đăng ký")
	}
	cur, err := coll.Find(ctx, bson.M{"ownerOrganizationId": ownerOrgID}, options.Find().SetSort(bson.D{{Key: "caseType", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []aidecisiondto.ContextPolicyOverrideItem
	for cur.Next(ctx) {
		var doc aidecisionmodels.DecisionContextPolicyOverride
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		out = append(out, contextPolicyOverrideFromModel(doc))
	}
	return out, cur.Err()
}

// UpsertContextPolicyOverride tạo hoặc cập nhật theo (org, caseType).
func (s *AIDecisionService) UpsertContextPolicyOverride(ctx context.Context, ownerOrgID primitive.ObjectID, req aidecisiondto.ContextPolicyOverrideUpsertRequest) (*aidecisiondto.ContextPolicyOverrideItem, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionContextPolicyOverrides)
	if !ok {
		return nil, errors.New("collection decision_context_policy_overrides chưa đăng ký")
	}
	ct := strings.TrimSpace(req.CaseType)
	if ct == "" {
		return nil, errors.New("caseType là bắt buộc")
	}
	now := time.Now().UnixMilli()
	reqCtx := normalizeContextKeys(req.RequiredContexts)
	optCtx := normalizeContextKeys(req.OptionalContexts)
	filter := bson.M{"ownerOrganizationId": ownerOrgID, "caseType": ct}
	update := bson.M{
		"$set": bson.M{
			"enabled":           req.Enabled,
			"requiredContexts":  reqCtx,
			"optionalContexts":  optCtx,
			"note":              strings.TrimSpace(req.Note),
			"updatedAt":         now,
		},
		"$setOnInsert": bson.M{
			"ownerOrganizationId": ownerOrgID,
			"caseType":            ct,
			"createdAt":           now,
		},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var doc aidecisionmodels.DecisionContextPolicyOverride
	if err := coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&doc); err != nil {
		return nil, err
	}
	invalidateContextPolicyCache()
	item := contextPolicyOverrideFromModel(doc)
	return &item, nil
}

// DeleteContextPolicyOverride xóa theo _id thuộc org.
func (s *AIDecisionService) DeleteContextPolicyOverride(ctx context.Context, ownerOrgID primitive.ObjectID, ruleIDHex string) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionContextPolicyOverrides)
	if !ok {
		return errors.New("collection decision_context_policy_overrides chưa đăng ký")
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
	invalidateContextPolicyCache()
	return nil
}

func normalizeContextKeys(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, s := range in {
		k := strings.TrimSpace(strings.ToLower(s))
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

func contextPolicyOverrideFromModel(doc aidecisionmodels.DecisionContextPolicyOverride) aidecisiondto.ContextPolicyOverrideItem {
	return aidecisiondto.ContextPolicyOverrideItem{
		ID:                  doc.ID.Hex(),
		OwnerOrganizationID: doc.OwnerOrganizationID.Hex(),
		CaseType:            doc.CaseType,
		Enabled:             doc.Enabled,
		RequiredContexts:    append([]string(nil), doc.RequiredContexts...),
		OptionalContexts:    append([]string(nil), doc.OptionalContexts...),
		Note:                doc.Note,
		UpdatedAt:           doc.UpdatedAt,
		CreatedAt:           doc.CreatedAt,
	}
}
