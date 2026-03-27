// Package adsautop — Xử lý ads.context_ready theo chuẩn AI Decision: đọc Intelligence (flags) từ DB, ACTION_RULE, emit ads.propose_requested.
package adsautop

import (
	"context"
	"fmt"
	"strings"

	adssvc "meta_commerce/internal/api/ads/service"
	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/decisionlive/livecopy"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	metasvc "meta_commerce/internal/api/meta/service"
	"meta_commerce/internal/approval"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// RunAdsProposeFromContextReady sau ads.context_ready: lấy currentMetrics từ meta_campaigns (Intelligence — alertFlags/layers),
// áp ACTION_RULE (metasvc), persist actions + emit ads.propose_requested (cùng chuẩn RunAutoPropose).
// Ghi timeline decisionlive + cập nhật trạng thái case (ads_optimization_decision) cho UI.
// Thiếu dữ liệu / không đủ điều kiện → closed_incomplete; rule không đề xuất hành động → closed_no_action; lỗi kỹ thuật → closed_failed (kèm lý do trong outcomeSummary).
// queueEvt: envelope job ads.context_ready — truyền vào mọi Publish để refs (eventId, trace, entity…) đủ cho audit/timeline nhóm theo job.
func RunAdsProposeFromContextReady(ctx context.Context, svc *aidecisionsvc.AIDecisionService, ownerOrgID primitive.ObjectID, orgID, campaignID, adAccountID, baseURL string, queueEvt *aidecisionmodels.DecisionEvent) error {
	caseDoc, _ := svc.FindCaseByAdsCampaign(ctx, campaignID, orgID, ownerOrgID)
	traceID := resolveAdsTraceID(caseDoc)

	if campaignID == "" || adAccountID == "" || ownerOrgID.IsZero() {
		reason := "Không hoàn thành đánh giá: thiếu campaignId hoặc adAccountId (hoặc org) — không thể ra quyết định tối ưu Ads. Kiểm tra payload sự kiện ads.context_ready."
		publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseEmpty, decisionlive.SeverityWarn,
			reason,
			[]string{"Bắt buộc: campaignId, adAccountId, ownerOrganizationId hợp lệ trên envelope event."},
			"Thiếu dữ liệu đầu vào", nil, campaignID, adAccountID)
		closeAdsCaseOutcome(svc, ctx, caseDoc, aidecisionmodels.ClosureIncomplete, reason)
		return nil
	}

	publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseAdsEvaluate, decisionlive.SeverityInfo,
		"Đang đánh giá chiến dịch theo ACTION_RULE và metrics Intelligence…",
		[]string{
			"Nguồn: meta_campaigns.currentMetrics (cờ cảnh báo / lớp phân tích).",
			"Bước tiếp: tính hành động đề xuất hoặc kết thúc nếu không có hành động.",
		},
		"Bắt đầu đánh giá", nil, campaignID, adAccountID)

	campaignsColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return fmt.Errorf("không tìm thấy collection meta_campaigns")
	}
	currentMetrics := adssvc.GetCampaignCurrentMetrics(ctx, campaignsColl, campaignID, ownerOrgID)
	if currentMetrics == nil {
		reason := "Không hoàn thành đánh giá: không có metrics Intelligence trên campaign (currentMetrics rỗng) — không thể ra quyết định. Đồng bộ Meta/Ads Intelligence hoặc chờ roll-up."
		publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseEmpty, decisionlive.SeverityWarn,
			reason,
			[]string{			"Kiểm tra bản ghi meta_campaigns và trường intelligence cho campaign này."},
			"Thiếu metrics", nil, campaignID, adAccountID)
		closeAdsCaseOutcome(svc, ctx, caseDoc, aidecisionmodels.ClosureIncomplete, reason)
		return nil
	}

	actions, report := metasvc.ComputeFinalActionsFromCurrentMetrics(ctx, campaignID, adAccountID, ownerOrgID, currentMetrics)
	if len(actions) == 0 {
		bullets := []string{"ACTION_RULE đã chạy nhưng không có hành động đề xuất (không đạt ngưỡng / không có cờ kích hoạt)."}
		if report != nil {
			bullets = append(bullets, fmt.Sprintf("Chi tiết rule: %v", report))
		}
		reason := "Không hoàn thành đề xuất từ rule: không có hành động nào được chọn sau khi đánh giá metrics (ACTION_RULE)."
		publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseEmpty, decisionlive.SeverityWarn,
			reason,
			bullets,
			"Không có hành động đề xuất", nil, campaignID, adAccountID)
		closeAdsCaseOutcome(svc, ctx, caseDoc, aidecisionmodels.ClosureNoAction, reason)
		return nil
	}
	action := actions[0]
	actionType, _ := action["actionType"].(string)
	ruleCode, _ := action["ruleCode"].(string)

	pendingInfo, pendingErr := adssvc.GetPendingProposalForCampaign(ctx, campaignID, ownerOrgID)
	if pendingErr != nil {
		reason := fmt.Sprintf("Lỗi khi đọc đề xuất chờ duyệt: %v — không thể hoàn tất pipeline Ads.", pendingErr)
		publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseError, decisionlive.SeverityError,
			reason,
			[]string{"Thử lại sau khi kiểm tra DB approval / action_pending."},
			"Lỗi hệ thống", nil, campaignID, adAccountID)
		closeAdsCaseOutcome(svc, ctx, caseDoc, aidecisionmodels.ClosureFailed, reason)
		return pendingErr
	}
	hasPending := pendingInfo != nil
	if hasPending && pendingInfo.ActionType == actionType && pendingInfo.RuleCode == ruleCode {
		publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseDone, decisionlive.SeverityInfo,
			"Đề xuất trùng với bản chờ duyệt — giữ nguyên, không tạo lại.",
			[]string{
				fmt.Sprintf("actionType=%s, ruleCode=%s.", actionType, ruleCode),
				"Không cần thao tác thêm trên case.",
			},
			"Giữ nguyên đề xuất", nil, campaignID, adAccountID)
		return nil
	}
	if hasPending {
		_, _ = approval.Cancel(ctx, pendingInfo.ID.Hex(), ownerOrgID)
	}

	if err := metasvc.PersistCampaignEvaluatedActions(ctx, campaignID, adAccountID, ownerOrgID, currentMetrics, actions, report); err != nil {
		reason := fmt.Sprintf("Lỗi khi ghi kết quả đánh giá (persist): %v — pipeline Ads không hoàn tất.", err)
		publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseError, decisionlive.SeverityError,
			reason,
			[]string{"Kiểm tra meta_campaigns / quyền ghi DB."},
			"Lỗi ghi dữ liệu", nil, campaignID, adAccountID)
		closeAdsCaseOutcome(svc, ctx, caseDoc, aidecisionmodels.ClosureFailed, reason)
		return err
	}

	campaignName := ""
	var doc struct {
		Name string `bson:"name"`
	}
	_ = campaignsColl.FindOne(ctx, bson.M{
		"campaignId": campaignID, "ownerOrganizationId": ownerOrgID,
	}, mongoopts.FindOne().SetProjection(bson.M{"name": 1})).Decode(&doc)
	campaignName = doc.Name

	metricsPayload := adssvc.BuildMetricsPayloadForNotification(ctx, campaignsColl, campaignID, adAccountID, ownerOrgID, currentMetrics)
	if metricsPayload == nil {
		metricsPayload = make(map[string]interface{})
	}
	if rc := action["result_check"]; rc != nil {
		metricsPayload["result_check"] = rc
	}
	if traceId, _ := action["traceId"].(string); traceId != "" {
		metricsPayload["traceId"] = traceId
	}
	reason, _ := action["reason"].(string)
	value := action["value"]
	traceIDAction, _ := action["traceId"].(string)
	inp, err := adssvc.BuildApprovalProposeInput(ctx, &adssvc.ProposeInput{
		ActionType:   actionType,
		AdAccountId:  adAccountID,
		CampaignId:   campaignID,
		CampaignName: campaignName,
		Reason:       reason,
		Value:        value,
		RuleCode:     ruleCode,
		TraceID:      traceIDAction,
		Payload:      metricsPayload,
	}, ownerOrgID, true)
	if err != nil {
		reasonFail := fmt.Sprintf("Lỗi khi chuẩn bị đề xuất duyệt: %v — không thể đưa vào hàng đợi.", err)
		publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseError, decisionlive.SeverityError,
			reasonFail,
			[]string{"Kiểm tra payload metrics và cấu hình approval."},
			"Lỗi chuẩn bị đề xuất", nil, campaignID, adAccountID)
		closeAdsCaseOutcome(svc, ctx, caseDoc, aidecisionmodels.ClosureFailed, reasonFail)
		return fmt.Errorf("chuẩn bị propose sau context_ready: %w", err)
	}

	publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhasePropose, decisionlive.SeverityInfo,
		fmt.Sprintf("Đã chọn đề xuất: %s (rule %s). Đang gửi vào hàng đợi duyệt…", actionType, ruleCode),
		[]string{
			"Hành động được tính từ ACTION_RULE trên metrics hiện tại.",
			"Sự kiện tiếp theo: executor.propose_requested (domain=ads).",
		},
		"Tạo đề xuất", nil, campaignID, adAccountID)

	eventID, err := aidecisionsvc.EmitAdsProposeRequest(ctx, inp, ownerOrgID, baseURL)
	if err != nil {
		reasonFail := fmt.Sprintf("Lỗi khi ghi queue đề xuất (EmitAdsProposeRequest): %v — đề xuất chưa vào hàng đợi.", err)
		publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseError, decisionlive.SeverityError,
			reasonFail,
			[]string{"Kiểm tra decision_events_queue và worker AI Decision."},
			"Lỗi hàng đợi", nil, campaignID, adAccountID)
		closeAdsCaseOutcome(svc, ctx, caseDoc, aidecisionmodels.ClosureFailed, reasonFail)
		return err
	}

	if caseDoc != nil {
		_ = svc.SetDecisionPacketOnCase(ctx, caseDoc.DecisionCaseID, map[string]interface{}{
			"decision_mode": "ads_action_rule",
			"action_type":   actionType,
			"rule_code":     ruleCode,
			"trace_id":      traceID,
			"event_id":      eventID,
		})
		_ = svc.UpdateCaseStatus(ctx, caseDoc.DecisionCaseID, aidecisionmodels.CaseStatusActionsCreated)
		_ = svc.CloseCase(ctx, caseDoc.DecisionCaseID, aidecisionmodels.ClosureProposed)
	}

	publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseDone, decisionlive.SeverityInfo,
		"Đã xếp hàng đề xuất tối ưu Ads — chờ duyệt / thực thi.",
		[]string{
			fmt.Sprintf("eventId queue đề xuất: %s", eventID),
			"Case runtime đã đóng (closed_proposed) — Executor quản lý bước sau.",
		},
		"Hoàn tất pipeline Ads", map[string]string{"proposeEmitEventId": eventID}, campaignID, adAccountID)

	return nil
}

func closeAdsCaseOutcome(svc *aidecisionsvc.AIDecisionService, ctx context.Context, caseDoc *aidecisionmodels.DecisionCase, closureType, outcomeSummary string) {
	if svc == nil || caseDoc == nil {
		return
	}
	_ = svc.CloseCaseWithOutcomeSummary(ctx, caseDoc.DecisionCaseID, closureType, outcomeSummary)
}

func resolveAdsTraceID(caseDoc *aidecisionmodels.DecisionCase) string {
	if caseDoc != nil {
		if t := strings.TrimSpace(caseDoc.TraceID); t != "" {
			return t
		}
	}
	return utility.GenerateUID(utility.UIDPrefixTrace)
}

// publishAdsOptimizationLive ghi timeline Ads — dùng livecopy (khung 3 bullet + refs envelope).
func publishAdsOptimizationLive(ownerOrgID primitive.ObjectID, traceID string, caseDoc *aidecisionmodels.DecisionCase, queueEvt *aidecisionmodels.DecisionEvent, phase, severity, summary string, detailBullets []string, stepTitle string, extraRefs map[string]string, campaignID, adAccountID string) {
	if ownerOrgID.IsZero() || traceID == "" {
		return
	}
	ev := livecopy.BuildAdsOptimizationLiveEvent(caseDoc, queueEvt, phase, severity, summary, detailBullets, stepTitle, extraRefs, campaignID, adAccountID)
	decisionlive.Publish(ownerOrgID, traceID, ev)
}
