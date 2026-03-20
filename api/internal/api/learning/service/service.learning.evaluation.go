// Package learningsvc — Evaluation Job: tính outcome_class, error_attribution cho learning_cases chưa có evaluation.
package learningsvc

import (
	"context"
	"strings"
	"time"

	"meta_commerce/internal/api/learning/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// RunEvaluationBatch xử lý batch learning_cases chưa có evaluation.outcomeClass.
// Trả về số case đã cập nhật.
func RunEvaluationBatch(ctx context.Context, limit int) int {
	if limit <= 0 {
		limit = 50
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.LearningCases)
	if !ok {
		return 0
	}
	// Tìm cases có evaluation.outcomeClass chưa set (hoặc evaluation rỗng)
	filter := bson.M{
		"$or": []bson.M{
			{"evaluation.outcomeClass": bson.M{"$exists": false}},
			{"evaluation.outcomeClass": ""},
		},
	}
	opts := options.Find().SetLimit(int64(limit)).SetSort(bson.M{"closedAt": 1})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)

	processed := 0
	for cursor.Next(ctx) {
		var lc models.LearningCase
		if err := cursor.Decode(&lc); err != nil {
			continue
		}
		eval := computeEvaluation(&lc)
		update := bson.M{
			"$set": bson.M{
				"evaluation": eval,
				"updatedAt": time.Now().UnixMilli(),
			},
		}
		_, err := coll.UpdateOne(ctx, bson.M{"_id": lc.ID}, update)
		if err != nil {
			continue
		}
		processed++
	}
	return processed
}

// computeEvaluation tính evaluation từ outcome.
func computeEvaluation(lc *models.LearningCase) models.LearningEvaluation {
	eval := models.LearningEvaluation{}
	// outcome_class từ outcome.technical + result
	tech := lc.Outcome.Technical
	switch tech.Status {
	case "success":
		eval.OutcomeClass = models.LearningResultSuccess
		if lc.Result == models.LearningResultRejected || lc.Result == models.LearningResultFailed {
			eval.OutcomeClass = lc.Result
		}
	case "rejected":
		eval.OutcomeClass = models.LearningResultRejected
	case "fail":
		eval.OutcomeClass = models.LearningResultFailed
		eval.ErrorAttribution = inferErrorAttribution(tech.Error)
	default:
		eval.OutcomeClass = lc.Result
		if eval.OutcomeClass == "" {
			eval.OutcomeClass = "delayed"
		}
	}
	// primary_metric, delta từ ExecuteResponse nếu có
	if lc.ActionExecuted != nil {
		if resp, ok := lc.ActionExecuted["executeResponse"].(map[string]interface{}); ok {
			if pm, ok := resp["primaryMetric"].(string); ok {
				eval.PrimaryMetric = pm
			}
			if bv, ok := toFloat64(resp["baselineValue"]); ok {
				eval.BaselineValue = bv
			}
			if fv, ok := toFloat64(resp["finalValue"]); ok {
				eval.FinalValue = fv
			}
			eval.Delta = eval.FinalValue - eval.BaselineValue
		}
	}
	return eval
}

func toFloat64(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	default:
		return 0, false
	}
}

func inferErrorAttribution(errMsg string) string {
	if errMsg == "" {
		return ""
	}
	// Heuristic: timeout, rate limit → execution; rule, param → logic/param
	lower := strings.ToLower(errMsg)
	if strings.Contains(lower, "timeout") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "429") || strings.Contains(lower, "503") {
		return "execution"
	}
	if strings.Contains(lower, "rule") || strings.Contains(lower, "condition") {
		return "logic"
	}
	return "execution"
}
