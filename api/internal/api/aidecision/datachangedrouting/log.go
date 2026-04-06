package datachangedrouting

import (
	"context"
	"os"
	"strings"

	"meta_commerce/internal/api/aidecision/eventintake"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/api/aidecision/routecontract"
	"meta_commerce/internal/logger"
)

// envDatachangedRoutingInfo — đặt "1"/"true" để log mức Info (mặc định chỉ Debug, tránh spam).
const envDatachangedRoutingInfo = "AI_DECISION_DATACHANGED_ROUTING_LOG"

// LogApplied ghi log quyết định định tuyến + policy hiệu lực (sau EvaluateDatachangedSideEffects).
func LogApplied(ctx context.Context, evt *aidecisionmodels.DecisionEvent, orgHex string, dec eventintake.SideEffectDecision, route routecontract.Decision) {
	_ = ctx
	wantInfo := routingLogInfoEnabled()
	fields := map[string]interface{}{
		"topic":                        "datachanged_routing",
		"routingConfigVersion":         route.Version,
		"routingRuleId":                route.RuleID,
		"sourceCollection":             route.Collection,
		"emitToDecisionQueuePlan":      route.EmitToDecisionQueue,
		"pipelineCrmPendingMerge":      route.CrmPendingMergeCollection,
		"pipelineReportTouch":          route.ReportTouchPipeline,
		"pipelineAdsProfile":           route.AdsProfilePipeline,
		"pipelineCixIntel":             route.CixIntelPipeline,
		"pipelineOrderIntel":           route.OrderIntelPipeline,
		"pipelineCrmIntelRefreshDefer": route.CrmIntelRefreshDeferPipeline,
		"policyAllowCrmMergeQueue":     dec.AllowCrmMergeQueue,
		"policyAllowReport":            dec.AllowReport,
		"policyAllowAds":               dec.AllowAds,
		"policyReasonsSkipped":         dec.ReasonsSkipped,
	}
	log := logger.GetAppLogger().WithFields(fields)
	if evt != nil {
		if id := strings.TrimSpace(evt.EventID); id != "" {
			log = log.WithField("eventId", id)
		}
		if et := strings.TrimSpace(evt.EventType); et != "" {
			log = log.WithField("eventType", et)
		}
	}
	if h := strings.TrimSpace(orgHex); h != "" {
		log = log.WithField("ownerOrgIdHex", h)
	}
	msg := "📋 [DATACHANGED_ROUTING] Quyết định định tuyến (bảng phiên bản + policy hiệu lực)"
	if wantInfo {
		log.Info(msg)
		return
	}
	log.Debug(msg)
}

func routingLogInfoEnabled() bool {
	s := strings.TrimSpace(strings.ToLower(os.Getenv(envDatachangedRoutingInfo)))
	return s == "1" || s == "true" || s == "yes"
}
