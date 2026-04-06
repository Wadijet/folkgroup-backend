// Package service — Ghi crm_activity_history sau CIX terminal (khung intelligence mục 4: timeline + metricsSnapshot).
package service

import (
	"context"
	"strings"

	cixmodels "meta_commerce/internal/api/cix/models"
	crmmodels "meta_commerce/internal/api/crm/models"
	crmvc "meta_commerce/internal/api/crm/service"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// buildCixConversationIntelMetrics snapshot CIX đồng dạng raw / layer1 / layer2 / layer3 trong metricsSnapshot (lồng dưới cixConversationIntel).
func buildCixConversationIntelMetrics(r *cixmodels.CixAnalysisResult) map[string]interface{} {
	if r == nil {
		return nil
	}
	flags := make([]map[string]interface{}, 0, len(r.Flags))
	for _, f := range r.Flags {
		flags = append(flags, map[string]interface{}{
			"name": f.Name, "severity": f.Severity, "triggeredByRule": f.TriggeredByRule,
		})
	}
	out := map[string]interface{}{
		"raw": map[string]interface{}{
			"turnCount":  r.RawFacts.TurnCount,
			"firstMsgAt": r.RawFacts.FirstMsgAt,
			"lastMsgAt":  r.RawFacts.LastMsgAt,
		},
		"layer1": map[string]interface{}{"stage": r.Layer1.Stage},
		"layer2": map[string]interface{}{
			"intentStage":      r.Layer2.IntentStage,
			"urgencyLevel":     r.Layer2.UrgencyLevel,
			"riskLevelRaw":     r.Layer2.RiskLevelRaw,
			"riskLevelAdj":     r.Layer2.RiskLevelAdj,
			"adjustmentRule":   r.Layer2.AdjustmentRule,
			"adjustmentReason": r.Layer2.AdjustmentReason,
		},
		"layer3": map[string]interface{}{
			"buyingIntent":   r.Layer3.BuyingIntent,
			"objectionLevel": r.Layer3.ObjectionLevel,
			"sentiment":      r.Layer3.Sentiment,
		},
		"flags":             flags,
		"actionSuggestions": r.ActionSuggestions,
		"status":            r.Status,
		"sessionUid":        r.SessionUid,
		"analysisResultId":  r.ID.Hex(),
		"causalOrderingAt":  r.CausalOrderingAt,
		"cixIntelSequence":  r.CixIntelSequence,
	}
	if !r.ParentJobID.IsZero() {
		out["parentJobId"] = r.ParentJobID.Hex()
	}
	return out
}

func activityUnifiedID(c *crmmodels.CrmCustomer) string {
	if c == nil {
		return ""
	}
	if strings.TrimSpace(c.UnifiedId) != "" {
		return c.UnifiedId
	}
	return strings.TrimSpace(c.Uid)
}

// logCixIntelActivityAfterSuccess ghi activity cix_conversation_intel lên crm_activity_history (có khách CRM).
func logCixIntelActivityAfterSuccess(ctx context.Context, ownerOrgID primitive.ObjectID, customerLookup, sessionUid string, result *cixmodels.CixAnalysisResult, activityAt int64) {
	if result == nil || customerLookup == "" || ownerOrgID.IsZero() {
		return
	}
	crmSvc, err := crmvc.NewCrmCustomerService()
	if err != nil {
		return
	}
	cust, err := crmSvc.FindOne(ctx, bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$or": []bson.M{
			{"uid": customerLookup},
			{"unifiedId": customerLookup},
		},
	}, nil)
	if err != nil {
		return
	}
	unified := activityUnifiedID(&cust)
	if unified == "" {
		return
	}
	if activityAt <= 0 {
		activityAt = result.ComputedAt
	}
	if activityAt <= 0 {
		activityAt = result.CreatedAt
	}

	snap := crmvc.BuildSnapshotForNewCustomer(ctx, &cust, activityAt, false, nil)
	metadata := map[string]interface{}{
		"trigger":          "cix_intel_compute",
		"analysisResultId": result.ID.Hex(),
		"sessionUid":       sessionUid,
	}
	if snap != nil {
		crmvc.MergeSnapshotIntoMetadata(metadata, snap)
	}
	if ms, ok := metadata["metricsSnapshot"].(map[string]interface{}); ok {
		ms["cixConversationIntel"] = buildCixConversationIntelMetrics(result)
	} else {
		metadata["metricsSnapshot"] = map[string]interface{}{
			"cixConversationIntel": buildCixConversationIntelMetrics(result),
		}
	}

	actSvc, err := crmvc.NewCrmActivityService()
	if err != nil {
		return
	}
	_ = actSvc.LogActivity(ctx, crmvc.LogActivityInput{
		UnifiedId:    unified,
		OwnerOrgID:   ownerOrgID,
		Domain:       crmmodels.ActivityDomainConversation,
		ActivityType: "cix_conversation_intel",
		Source:       "cix_intel_compute",
		Metadata:     metadata,
		DisplayLabel: "Phân tích hội thoại CIX",
		DisplayIcon:  "chat-intelligence",
		DisplaySubtext: sessionUid,
		ActivityAt:   activityAt,
	})
}

// logCixIntelActivityAfterFailure ghi activity khi CIX terminal lỗi (sau hết retry) — vẫn timeline trên khách nếu resolve được CRM.
func logCixIntelActivityAfterFailure(ctx context.Context, ownerOrgID primitive.ObjectID, customerLookup, sessionUid string, failed *cixmodels.CixAnalysisResult, activityAt int64) {
	if failed == nil || customerLookup == "" || ownerOrgID.IsZero() {
		return
	}
	crmSvc, err := crmvc.NewCrmCustomerService()
	if err != nil {
		return
	}
	cust, err := crmSvc.FindOne(ctx, bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$or": []bson.M{
			{"uid": customerLookup},
			{"unifiedId": customerLookup},
		},
	}, nil)
	if err != nil {
		return
	}
	unified := activityUnifiedID(&cust)
	if unified == "" {
		return
	}
	if activityAt <= 0 {
		activityAt = failed.FailedAt
	}
	if activityAt <= 0 {
		activityAt = failed.CreatedAt
	}
	metadata := map[string]interface{}{
		"trigger":           "cix_intel_compute",
		"analysisResultId":  failed.ID.Hex(),
		"sessionUid":        sessionUid,
		"metricsSnapshot": map[string]interface{}{
			"cixConversationIntel": map[string]interface{}{
				"status":       failed.Status,
				"errorCode":    failed.ErrorCode,
				"errorMessage": failed.ErrorMessage,
				"sessionUid":   sessionUid,
			},
		},
	}
	actSvc, err := crmvc.NewCrmActivityService()
	if err != nil {
		return
	}
	_ = actSvc.LogActivity(ctx, crmvc.LogActivityInput{
		UnifiedId:      unified,
		OwnerOrgID:     ownerOrgID,
		Domain:         crmmodels.ActivityDomainConversation,
		ActivityType:   "cix_conversation_intel_failed",
		Source:         "cix_intel_compute",
		Metadata:       metadata,
		DisplayLabel:   "Phân tích CIX thất bại",
		DisplayIcon:    "chat-intelligence",
		DisplaySubtext: sessionUid,
		ActivityAt:     activityAt,
	})
}

