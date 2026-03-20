// Package learningsvc — Phân tích learning cases, tạo rule suggestions (Phase 3: Auto rule generation).
package learningsvc

import (
	"context"
	"fmt"
	"time"

	"meta_commerce/internal/api/learning/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AnalyzeAndSuggestRules phân tích learning_cases theo org, tạo RuleSuggestion khi failure rate cao.
// Gọi từ LearningRuleSuggestionWorker.
func AnalyzeAndSuggestRules(ctx context.Context, ownerOrgID primitive.ObjectID) (created int, err error) {
	lcColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.LearningCases)
	if !ok {
		return 0, fmt.Errorf("không tìm thấy collection learning_cases")
	}
	rsColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.RuleSuggestions)
	if !ok {
		return 0, fmt.Errorf("không tìm thấy collection rule_suggestions")
	}

	// Aggregate: domain, goalCode, result
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"ownerOrganizationId": ownerOrgID}}},
		{{Key: "$group", Value: bson.M{
			"_id":          bson.M{"domain": "$domain", "goalCode": "$goalCode"},
			"total":        bson.M{"$sum": 1},
			"success":     bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$eq": []interface{}{"$result", models.LearningResultSuccess}}, 1, 0}}},
			"failed":      bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$eq": []interface{}{"$result", models.LearningResultFailed}}, 1, 0}}},
			"rejected":   bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$eq": []interface{}{"$result", models.LearningResultRejected}}, 1, 0}}},
		}}},
		{{Key: "$match", Value: bson.M{"total": bson.M{"$gte": 5}}}}, // Ít nhất 5 cases mới suggest
	}

	cursor, err := lcColl.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var groups []struct {
		ID      struct { Domain string `bson:"domain"`; GoalCode string `bson:"goalCode"` } `bson:"_id"`
		Total   int `bson:"total"`
		Success int `bson:"success"`
		Failed  int `bson:"failed"`
		Rejected int `bson:"rejected"`
	}
	if err = cursor.All(ctx, &groups); err != nil {
		return 0, err
	}

	now := time.Now().UnixMilli()
	for _, g := range groups {
		failureRate := 0.0
		if g.Total > 0 {
			failureRate = float64(g.Failed) / float64(g.Total)
		}
		// Chỉ tạo suggestion khi failure rate >= 30%
		if failureRate < 0.3 {
			continue
		}

		// Tránh duplicate: đã có suggestion pending cho domain+goalCode
		var existing models.RuleSuggestion
		_ = rsColl.FindOne(ctx, bson.M{
			"ownerOrganizationId": ownerOrgID,
			"domain":              g.ID.Domain,
			"goalCode":            g.ID.GoalCode,
			"status":              "pending",
		}).Decode(&existing)
		if existing.SuggestionID != "" {
			continue
		}

		suggestionID := utility.GenerateUID("rgs")
		doc := &models.RuleSuggestion{
			SuggestionID:        suggestionID,
			OwnerOrganizationID: ownerOrgID,
			Domain:              g.ID.Domain,
			GoalCode:            g.ID.GoalCode,
			TotalCases:          g.Total,
			SuccessCount:        g.Success,
			FailedCount:         g.Failed,
			RejectedCount:       g.Rejected,
			FailureRate:         failureRate,
			SuggestionType:      "review_rule",
			Message:             fmt.Sprintf("Tỷ lệ thất bại %.1f%% cho %s/%s — nên xem xét điều chỉnh rule", failureRate*100, g.ID.Domain, g.ID.GoalCode),
			Priority:            "normal",
			Status:              "pending",
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if _, err := rsColl.InsertOne(ctx, doc); err != nil {
			continue // Bỏ qua nếu trùng
		}
		created++
	}
	return created, nil
}

// ListRuleSuggestions liệt kê rule suggestions theo org, filter, pagination.
func ListRuleSuggestions(ctx context.Context, ownerOrgID primitive.ObjectID, filter bson.M, limit, skip int, sortField string, sortOrder int) ([]*models.RuleSuggestion, int64, error) {
	rsColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.RuleSuggestions)
	if !ok {
		return nil, 0, fmt.Errorf("không tìm thấy collection rule_suggestions")
	}
	f := bson.M{"ownerOrganizationId": ownerOrgID}
	for k, v := range filter {
		if v != nil && v != "" {
			f[k] = v
		}
	}
	total, err := rsColl.CountDocuments(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetLimit(int64(limit)).SetSkip(int64(skip))
	if sortField != "" {
		opts.SetSort(bson.M{sortField: sortOrder})
	}
	cursor, err := rsColl.Find(ctx, f, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	var list []*models.RuleSuggestion
	if err = cursor.All(ctx, &list); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UpdateRuleSuggestionStatus cập nhật status của rule suggestion (reviewed, applied, dismissed).
func UpdateRuleSuggestionStatus(ctx context.Context, suggestionID string, ownerOrgID primitive.ObjectID, status, reviewedBy string) error {
	if status != "reviewed" && status != "applied" && status != "dismissed" {
		return fmt.Errorf("status không hợp lệ: %s", status)
	}
	rsColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.RuleSuggestions)
	if !ok {
		return fmt.Errorf("không tìm thấy collection rule_suggestions")
	}
	filter := bson.M{"suggestionId": suggestionID, "ownerOrganizationId": ownerOrgID}
	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updatedAt":  now,
			"reviewedAt": now,
			"reviewedBy": reviewedBy,
		},
	}
	result, err := rsColl.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return common.ErrNotFound
	}
	return nil
}
