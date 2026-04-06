// Package adsautop — Xử lý sự kiện «ngữ cảnh Ads đã sẵn sàng» (ads.context_ready): đọc số liệu phân tích đã lưu,
// áp quy tắc hành động, lưu kết quả và xếp hàng đề xuất chờ duyệt (chuẩn AI Decision).
package adsautop

import (
	"context"
	"fmt"
	"strings"

	adssvc "meta_commerce/internal/api/ads_meta/service"
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

// RunAdsProposeFromContextReady — Chạy sau job «ngữ cảnh Ads đã sẵn sàng»: đọc số liệu phân tích đã lưu trên chiến dịch,
// áp quy tắc hành động, lưu kết quả đánh giá và phát yêu cầu đề xuất chờ duyệt (chuẩn RunAutoPropose).
// Ghi các mốc lên timeline live và cập nhật trạng thái hồ sơ ads_optimization cho giao diện.
// Thiếu dữ liệu → đóng incomplete; rule không chọn hành động → đóng no_action; lỗi kỹ thuật → đóng failed (lý do trong outcomeSummary).
// queueEvt: envelope job hiện tại — truyền vào mọi Publish để audit/timeline có đủ eventId, trace, tham chiếu entity.
func RunAdsProposeFromContextReady(ctx context.Context, svc *aidecisionsvc.AIDecisionService, ownerOrgID primitive.ObjectID, orgID, campaignID, adAccountID, baseURL string, queueEvt *aidecisionmodels.DecisionEvent) error {
	caseDoc, _ := svc.FindCaseByAdsCampaign(ctx, campaignID, orgID, ownerOrgID)
	traceID := resolveAdsTraceID(caseDoc)

	if campaignID == "" || adAccountID == "" || ownerOrgID.IsZero() {
		reason := "Không thể đánh giá tối ưu quảng cáo: thiếu mã chiến dịch, mã tài khoản quảng cáo hoặc tổ chức. Kiểm tra nội dung sự kiện «ngữ cảnh Ads đã sẵn sàng» (ads.context_ready)."
		publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseEmpty, decisionlive.SeverityWarn,
			reason,
			[]string{"Cần đủ: mã chiến dịch (campaign), mã tài khoản quảng cáo và tổ chức sở hữu trên envelope job."},
			"Thiếu dữ liệu đầu vào", nil, campaignID, adAccountID)
		closeAdsCaseOutcome(svc, ctx, caseDoc, aidecisionmodels.ClosureIncomplete, reason)
		return nil
	}

	publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseAdsEvaluate, decisionlive.SeverityInfo,
		"Đang đánh giá chiến dịch theo quy tắc hành động và số liệu phân tích đã lưu…",
		[]string{
			"Dữ liệu đối chiếu: bản tóm tắt số liệu chiến dịch trên hệ thống (cảnh báo, lớp phân tích).",
			"Bước tiếp: chọn hành động đề xuất hoặc kết thúc nếu không có hành động phù hợp.",
		},
		"Bắt đầu đánh giá", nil, campaignID, adAccountID)

	campaignsColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return fmt.Errorf("không tìm thấy collection meta_campaigns")
	}
	currentMetrics := adssvc.GetCampaignCurrentMetrics(ctx, campaignsColl, campaignID, ownerOrgID)
	if currentMetrics == nil {
		reason := "Chưa có số liệu phân tích trên chiến dịch — không thể áp quy tắc. Hãy đồng bộ Meta / chờ job tính lại intelligence, rồi thử lại."
		publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseEmpty, decisionlive.SeverityWarn,
			reason,
			[]string{"Kiểm tra bản ghi chiến dịch trên hệ thống và phần dữ liệu phân tích (intelligence) đã được cập nhật chưa."},
			"Thiếu số liệu phân tích", nil, campaignID, adAccountID)
		closeAdsCaseOutcome(svc, ctx, caseDoc, aidecisionmodels.ClosureIncomplete, reason)
		return nil
	}

	actions, report := metasvc.ComputeFinalActionsFromCurrentMetrics(ctx, campaignID, adAccountID, ownerOrgID, currentMetrics)
	if len(actions) == 0 {
		bullets := []string{"Quy tắc đã chạy nhưng không chọn được hành động (chưa đạt ngưỡng hoặc chưa có điều kiện kích hoạt)."}
		if report != nil {
			bullets = append(bullets, fmt.Sprintf("Chi tiết từ quy tắc: %v", report))
		}
		reason := "Không tạo đề xuất: sau khi đối chiếu số liệu, không có hành động nào được quy tắc chọn."
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
		reason := fmt.Sprintf("Lỗi khi đọc đề xuất đang chờ duyệt: %v — không thể tiếp tục luồng tối ưu Ads.", pendingErr)
		publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseError, decisionlive.SeverityError,
			reason,
			[]string{"Kiểm tra kết nối DB và bảng lưu đề xuất chờ duyệt (approval)."},
			"Lỗi hệ thống", nil, campaignID, adAccountID)
		closeAdsCaseOutcome(svc, ctx, caseDoc, aidecisionmodels.ClosureFailed, reason)
		return pendingErr
	}
	hasPending := pendingInfo != nil
	if hasPending && pendingInfo.ActionType == actionType && pendingInfo.RuleCode == ruleCode {
		publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseDone, decisionlive.SeverityInfo,
			"Đề xuất trùng với bản đang chờ duyệt — giữ nguyên, không tạo bản mới.",
			[]string{
				fmt.Sprintf("Loại hành động: %s — mã quy tắc: %s.", actionType, ruleCode),
				"Không cần thao tác thêm trên hồ sơ xử lý.",
			},
			"Giữ nguyên đề xuất", nil, campaignID, adAccountID)
		return nil
	}
	if hasPending {
		_, _ = approval.Cancel(ctx, pendingInfo.ID.Hex(), ownerOrgID)
	}

	if err := metasvc.PersistCampaignEvaluatedActions(ctx, campaignID, adAccountID, ownerOrgID, currentMetrics, actions, report); err != nil {
		reason := fmt.Sprintf("Lỗi khi lưu kết quả đánh giá vào cơ sở dữ liệu: %v — luồng Ads dừng tại đây.", err)
		publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseError, decisionlive.SeverityError,
			reason,
			[]string{"Kiểm tra bản ghi chiến dịch và quyền ghi Mongo."},
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
		reasonFail := fmt.Sprintf("Lỗi khi chuẩn bị đề xuất chờ duyệt: %v — chưa thể xếp hàng bước tiếp theo.", err)
		publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseError, decisionlive.SeverityError,
			reasonFail,
			[]string{"Kiểm tra dữ liệu số liệu gửi kèm và cấu hình bước duyệt (approval)."},
			"Lỗi chuẩn bị đề xuất", nil, campaignID, adAccountID)
		closeAdsCaseOutcome(svc, ctx, caseDoc, aidecisionmodels.ClosureFailed, reasonFail)
		return fmt.Errorf("chuẩn bị propose sau context_ready: %w", err)
	}

	publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhasePropose, decisionlive.SeverityInfo,
		fmt.Sprintf("Đã chọn hành động đề xuất: %s (quy tắc %s). Đang xếp hàng bước duyệt…", actionType, ruleCode),
		[]string{
			"Hành động được tính từ quy tắc trên số liệu chiến dịch hiện tại.",
			"Bước tiếp: hệ thống phát yêu cầu tạo đề xuất cho executor (miền Ads).",
		},
		"Tạo đề xuất", nil, campaignID, adAccountID)

	eventID, err := aidecisionsvc.EmitAdsProposeRequest(ctx, inp, ownerOrgID, baseURL)
	if err != nil {
		reasonFail := fmt.Sprintf("Lỗi khi ghi job yêu cầu đề xuất lên hàng đợi: %v — đề xuất chưa được xếp hàng.", err)
		publishAdsOptimizationLive(ownerOrgID, traceID, caseDoc, queueEvt, decisionlive.PhaseError, decisionlive.SeverityError,
			reasonFail,
			[]string{"Kiểm tra collection hàng đợi sự kiện và worker AI Decision có đang chạy không."},
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
		"Đã xếp hàng đề xuất tối ưu quảng cáo — chờ duyệt hoặc thực thi.",
		[]string{
			fmt.Sprintf("Mã job đề xuất trên hàng đợi: %s", eventID),
			"Hồ sơ runtime đã đóng ở trạng thái «đã đề xuất» — các bước sau do Executor xử lý.",
		},
		"Hoàn tất luồng Ads", map[string]string{"proposeEmitEventId": eventID}, campaignID, adAccountID)

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

// publishAdsOptimizationLive — Đẩy mốc timeline tối ưu quảng cáo (tóm tắt + gạch đầu dòng + tham chiếu job queue).
func publishAdsOptimizationLive(ownerOrgID primitive.ObjectID, traceID string, caseDoc *aidecisionmodels.DecisionCase, queueEvt *aidecisionmodels.DecisionEvent, phase, severity, summary string, detailBullets []string, stepTitle string, extraRefs map[string]string, campaignID, adAccountID string) {
	if ownerOrgID.IsZero() || traceID == "" {
		return
	}
	ev := livecopy.BuildAdsOptimizationLiveEvent(caseDoc, queueEvt, phase, severity, summary, detailBullets, stepTitle, extraRefs, campaignID, adAccountID)
	decisionlive.Publish(ownerOrgID, traceID, ev)
}
