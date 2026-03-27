// Package aidecisionsvc — Decision Case: ResolveOrCreate, Update.
//
// Theo PLATFORM_L1_EVENT_DECISION_SUPPLEMENT §4, §3.2.
package aidecisionsvc

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"meta_commerce/internal/api/aidecision/contextpolicy"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const mergeWindowSec = 30 * 60 // 30 phút

// ResolveOrCreateInput input để resolve hoặc tạo case.
type ResolveOrCreateInput struct {
	EventID       string
	EventType     string
	OrgID         string
	OwnerOrgID    primitive.ObjectID
	EntityRefs    aidecisionmodels.DecisionCaseEntityRefs
	CaseType      string
	RequiredCtx   []string
	Priority      string
	Urgency       string
	TraceID       string
	CorrelationID string
}

// ResolveOrCreate tìm case cũ (merge rule) hoặc tạo mới.
func (s *AIDecisionService) ResolveOrCreate(ctx context.Context, input *ResolveOrCreateInput) (*aidecisionmodels.DecisionCase, bool, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return nil, false, errors.New("không tìm thấy collection decision_cases_runtime")
	}

	now := time.Now().UnixMilli()
	mergeCutoff := now - mergeWindowSec*1000

	// Merge rule: cùng org, entity, case_type, chưa closed, trong time window
	filter := bson.M{
		"orgId":    input.OrgID,
		"caseType": input.CaseType,
		"status":   bson.M{"$nin": []string{aidecisionmodels.CaseStatusClosed, "cancelled", "expired", "dropped"}},
		"openedAt": bson.M{"$gte": mergeCutoff},
	}
	if input.CaseType == aidecisionmodels.CaseTypeConversationResponse && input.EntityRefs.ConversationID != "" {
		filter["entityRefs.conversationId"] = input.EntityRefs.ConversationID
	} else if input.CaseType == aidecisionmodels.CaseTypeCustomerState && input.EntityRefs.CustomerID != "" {
		filter["entityRefs.customerId"] = input.EntityRefs.CustomerID
	} else if input.CaseType == aidecisionmodels.CaseTypeAdsOptimization && input.EntityRefs.CampaignID != "" {
		filter["entityRefs.campaignId"] = input.EntityRefs.CampaignID
	} else if input.CaseType == aidecisionmodels.CaseTypeOrderRisk && input.EntityRefs.OrderID != "" {
		filter["entityRefs.orderId"] = input.EntityRefs.OrderID
	}

	var existing aidecisionmodels.DecisionCase
	err := coll.FindOne(ctx, filter).Decode(&existing)
	if err == nil {
		// Cập nhật case cũ — đồng bộ requiredContexts theo matrix/rule hiện tại (merge không giữ snapshot cũ).
		setDoc := bson.M{
			"latestEventId":    input.EventID,
			"status":           aidecisionmodels.CaseStatusContextCollecting,
			"updatedAt":        now,
			"requiredContexts": input.RequiredCtx,
		}
		mergeCaseTraceOntoSetDoc(setDoc, existing.TraceID, existing.CorrelationID, input.TraceID, input.CorrelationID)
		update := bson.M{
			"$set":      setDoc,
			"$addToSet": bson.M{"triggerEventIds": input.EventID},
		}
		_, err = coll.UpdateOne(ctx, bson.M{"_id": existing.ID}, update)
		if err != nil {
			return nil, false, err
		}
		existing.LatestEventID = input.EventID
		existing.Status = aidecisionmodels.CaseStatusContextCollecting
		existing.UpdatedAt = now
		existing.RequiredContexts = append([]string(nil), input.RequiredCtx...)
		if v, ok := setDoc["traceId"].(string); ok && v != "" {
			existing.TraceID = v
		}
		if v, ok := setDoc["correlationId"].(string); ok && v != "" {
			existing.CorrelationID = v
		}
		return &existing, false, nil
	}
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, false, err
	}

	// §4.7 Reopen: case đã closed nhưng trong reopen_window — mở lại thay vì tạo mới.
	closedCase, errReopen := findClosedCaseForReopen(ctx, coll, input, now)
	if errReopen != nil {
		return nil, false, errReopen
	}
	if closedCase != nil {
		setReopen := bson.M{
			"status":           aidecisionmodels.CaseStatusContextCollecting,
			"latestEventId":    input.EventID,
			"updatedAt":        now,
			"requiredContexts": input.RequiredCtx,
		}
		mergeCaseTraceOntoSetDoc(setReopen, closedCase.TraceID, closedCase.CorrelationID, input.TraceID, input.CorrelationID)
		update := bson.M{
			"$set":      setReopen,
			"$unset":    bson.M{"closedAt": "", "closureType": ""},
			"$addToSet": bson.M{"triggerEventIds": input.EventID},
		}
		_, err = coll.UpdateOne(ctx, bson.M{"_id": closedCase.ID}, update)
		if err != nil {
			return nil, false, err
		}
		closedCase.LatestEventID = input.EventID
		closedCase.Status = aidecisionmodels.CaseStatusContextCollecting
		closedCase.UpdatedAt = now
		closedCase.RequiredContexts = append([]string(nil), input.RequiredCtx...)
		closedCase.ClosedAt = nil
		closedCase.ClosureType = ""
		if v, ok := setReopen["traceId"].(string); ok && v != "" {
			closedCase.TraceID = v
		}
		if v, ok := setReopen["correlationId"].(string); ok && v != "" {
			closedCase.CorrelationID = v
		}
		return closedCase, false, nil
	}

	// Tạo case mới
	caseID := utility.GenerateUID(utility.UIDPrefixDecisionCase)
	doc := &aidecisionmodels.DecisionCase{
		DecisionCaseID:      caseID,
		OrgID:               input.OrgID,
		OwnerOrganizationID: input.OwnerOrgID,
		RootEventID:         input.EventID,
		TriggerEventIDs:     []string{input.EventID},
		LatestEventID:       input.EventID,
		TraceID:             strings.TrimSpace(input.TraceID),
		CorrelationID:       strings.TrimSpace(input.CorrelationID),
		EntityRefs:          input.EntityRefs,
		CaseType:            input.CaseType,
		Priority:            input.Priority,
		Urgency:             input.Urgency,
		Status:              aidecisionmodels.CaseStatusContextCollecting,
		RequiredContexts:    input.RequiredCtx,
		ReceivedContexts:    []string{},
		ContextPackets:      make(map[string]interface{}),
		ActionIDs:           []string{},
		ExecutionIDs:        []string{},
		OpenedAt:            now,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	_, err = coll.InsertOne(ctx, doc)
	if err != nil {
		return nil, false, err
	}
	return doc, true, nil
}

// UpdateCaseWithCixContext cập nhật case với CIX payload khi nhận cix.analysis_completed.
func (s *AIDecisionService) UpdateCaseWithCixContext(ctx context.Context, conversationID, customerID, orgID string, ownerOrgID primitive.ObjectID, cixPayload map[string]interface{}) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return errors.New("không tìm thấy collection decision_cases_runtime")
	}

	filter := bson.M{
		"orgId":    orgID,
		"caseType": aidecisionmodels.CaseTypeConversationResponse,
		"status":   bson.M{"$nin": []string{aidecisionmodels.CaseStatusClosed, "cancelled", "expired", "dropped"}},
	}
	if conversationID != "" {
		filter["entityRefs.conversationId"] = conversationID
	}
	if customerID != "" {
		filter["entityRefs.customerId"] = customerID
	}

	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{
			"contextPackets.cix": cixPayload,
			"status":             aidecisionmodels.CaseStatusContextCollecting,
			"updatedAt":          now,
		},
		"$addToSet": bson.M{"receivedContexts": "cix"},
	}

	res, err := coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		// Không tìm thấy case — có thể case chưa tạo hoặc đã closed. Tiếp tục Execute bình thường.
		return nil
	}
	return nil
}

// UpdateCaseWithCustomerContext cập nhật case với customer payload khi nhận customer.context_ready.
func (s *AIDecisionService) UpdateCaseWithCustomerContext(ctx context.Context, conversationID, customerID, orgID string, ownerOrgID primitive.ObjectID, customerPayload map[string]interface{}) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return errors.New("không tìm thấy collection decision_cases_runtime")
	}

	filter := bson.M{
		"orgId":    orgID,
		"caseType": aidecisionmodels.CaseTypeConversationResponse,
		"status":   bson.M{"$nin": []string{aidecisionmodels.CaseStatusClosed, "cancelled", "expired", "dropped"}},
	}
	if conversationID != "" {
		filter["entityRefs.conversationId"] = conversationID
	}
	if customerID != "" {
		filter["entityRefs.customerId"] = customerID
	}

	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{
			"contextPackets.customer": customerPayload,
			"updatedAt":               now,
		},
		"$addToSet": bson.M{"receivedContexts": "customer"},
	}

	_, err := coll.UpdateOne(ctx, filter, update)
	return err
}

// UpdateCaseWithOrderContext cập nhật case với order flags khi nhận order.flags_emitted.
// Cập nhật tối đa hai case: (1) conversation_response theo conv+cust (2) order_risk theo orderUid.
func (s *AIDecisionService) UpdateCaseWithOrderContext(ctx context.Context, orderID, customerID, convID, orgID string, ownerOrgID primitive.ObjectID, orderPayload map[string]interface{}) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return errors.New("không tìm thấy collection decision_cases_runtime")
	}
	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{
			"contextPackets.order": orderPayload,
			"updatedAt":            now,
		},
		"$addToSet": bson.M{"receivedContexts": "order"},
	}
	closed := bson.M{"$nin": []string{aidecisionmodels.CaseStatusClosed, "cancelled", "expired", "dropped"}}

	// 1) Case hội thoại — khớp conv + customer (không bắt buộc orderId trên entity lúc tạo case)
	if convID != "" && customerID != "" {
		filter := bson.M{
			"orgId":                     orgID,
			"caseType":                  aidecisionmodels.CaseTypeConversationResponse,
			"status":                    closed,
			"entityRefs.conversationId": convID,
			"entityRefs.customerId":     customerID,
		}
		if _, err := coll.UpdateOne(ctx, filter, update); err != nil {
			return err
		}
	}
	// 2) Case order_risk — merge theo orderUid (PLATFORM_L1 §4.5)
	if orderID != "" {
		filter := bson.M{
			"orgId":              orgID,
			"caseType":           aidecisionmodels.CaseTypeOrderRisk,
			"status":             closed,
			"entityRefs.orderId": orderID,
		}
		_, err := coll.UpdateOne(ctx, filter, update)
		return err
	}
	return nil
}

// FindCaseByOrder tìm case order_risk đang mở theo orderUid (business uid).
func (s *AIDecisionService) FindCaseByOrder(ctx context.Context, orderUID, orgID string, ownerOrgID primitive.ObjectID) (*aidecisionmodels.DecisionCase, error) {
	if orderUID == "" {
		return nil, nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return nil, errors.New("không tìm thấy collection decision_cases_runtime")
	}
	filter := bson.M{
		"orgId":               orgID,
		"ownerOrganizationId": ownerOrgID,
		"caseType":            aidecisionmodels.CaseTypeOrderRisk,
		"status":              bson.M{"$nin": []string{aidecisionmodels.CaseStatusClosed, "cancelled", "expired", "dropped"}},
		"entityRefs.orderId":  orderUID,
	}
	var doc aidecisionmodels.DecisionCase
	err := coll.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

// UpdateCaseWithAdsContext cập nhật case ads_optimization với ads context khi nhận ads.context_ready.
// campaignID: Meta campaignId (entityRefs.campaignId), không phải ad account.
// Ghi nhận đủ context Ads → chuyển ready_for_decision (không lọc requiredContexts — tránh 0 bản ghi khi policy trả rỗng).
func (s *AIDecisionService) UpdateCaseWithAdsContext(ctx context.Context, campaignID, orgID string, ownerOrgID primitive.ObjectID, adsPayload map[string]interface{}) error {
	if campaignID == "" {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return errors.New("không tìm thấy collection decision_cases_runtime")
	}
	filter := bson.M{
		"orgId":                 orgID,
		"ownerOrganizationId":   ownerOrgID,
		"caseType":              aidecisionmodels.CaseTypeAdsOptimization,
		"status":                bson.M{"$nin": []string{aidecisionmodels.CaseStatusClosed, "cancelled", "expired", "dropped"}},
		"entityRefs.campaignId": campaignID,
	}
	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{
			"contextPackets.ads": adsPayload,
			"updatedAt":          now,
			"status":             aidecisionmodels.CaseStatusReadyForDecision,
		},
		"$addToSet": bson.M{"receivedContexts": "ads"},
	}
	_, err := coll.UpdateOne(ctx, filter, update)
	return err
}

// FindCaseByAdsCampaign tìm case ads_optimization đang mở theo Meta campaignId (audit + timeline).
func (s *AIDecisionService) FindCaseByAdsCampaign(ctx context.Context, campaignID, orgID string, ownerOrgID primitive.ObjectID) (*aidecisionmodels.DecisionCase, error) {
	if campaignID == "" || orgID == "" {
		return nil, nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return nil, errors.New("không tìm thấy collection decision_cases_runtime")
	}
	filter := bson.M{
		"orgId":                 orgID,
		"ownerOrganizationId":   ownerOrgID,
		"caseType":              aidecisionmodels.CaseTypeAdsOptimization,
		"status":                bson.M{"$nin": []string{aidecisionmodels.CaseStatusClosed, "cancelled", "expired", "dropped"}},
		"entityRefs.campaignId": campaignID,
	}
	var doc aidecisionmodels.DecisionCase
	err := coll.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

// CloseCaseWithOutcomeSummary đóng case và ghi outcomeSummary (luồng Ads — mô tả ngắn cho người dùng).
func (s *AIDecisionService) CloseCaseWithOutcomeSummary(ctx context.Context, decisionCaseID, closureType, outcomeSummary string) error {
	if decisionCaseID == "" {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return errors.New("không tìm thấy collection decision_cases_runtime")
	}
	now := time.Now().UnixMilli()
	set := bson.M{
		"status":      aidecisionmodels.CaseStatusClosed,
		"closureType": closureType,
		"closedAt":    now,
		"updatedAt":   now,
	}
	if strings.TrimSpace(outcomeSummary) != "" {
		set["outcomeSummary"] = strings.TrimSpace(outcomeSummary)
	}
	_, err := coll.UpdateOne(ctx, bson.M{"decisionCaseId": decisionCaseID}, bson.M{"$set": set})
	return err
}

// AdsContextRequestThrottleCooldownSec đọc ADS_CONTEXT_REQUEST_COOLDOWN_SEC (mặc định 60 giây). ≤0 = tắt cooldown.
func AdsContextRequestThrottleCooldownSec() int64 {
	v := strings.TrimSpace(os.Getenv("ADS_CONTEXT_REQUEST_COOLDOWN_SEC"))
	if v == "" {
		return 60
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return 60
	}
	return n
}

// AcquireAdsContextRequestSlot đặt lastAdsContextRequestedAt nếu đã hết cooldown; trả rollback để gọi khi EmitEvent(ads.context_requested) thất bại.
func (s *AIDecisionService) AcquireAdsContextRequestSlot(ctx context.Context, decisionCaseID string) (rollback func(), allowed bool, err error) {
	rollback = func() {}
	if decisionCaseID == "" {
		return rollback, true, nil
	}
	sec := AdsContextRequestThrottleCooldownSec()
	if sec <= 0 {
		return rollback, true, nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return rollback, true, nil
	}
	var raw bson.M
	if err := coll.FindOne(ctx, bson.M{"decisionCaseId": decisionCaseID}).Decode(&raw); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return rollback, true, nil
		}
		return rollback, false, err
	}
	prev, hadField := bsonRawInt64FromM(raw, "lastAdsContextRequestedAt")
	now := time.Now().UnixMilli()
	cutoff := now - sec*1000
	res, err := coll.UpdateOne(ctx,
		bson.M{
			"decisionCaseId": decisionCaseID,
			"$or": []bson.M{
				{"lastAdsContextRequestedAt": bson.M{"$exists": false}},
				{"lastAdsContextRequestedAt": bson.M{"$lte": cutoff}},
			},
		},
		bson.M{"$set": bson.M{"lastAdsContextRequestedAt": now, "updatedAt": now}},
	)
	if err != nil {
		return rollback, false, err
	}
	if res.MatchedCount == 0 {
		return rollback, false, nil
	}
	rollback = func() {
		rbCtx := context.Background()
		if !hadField {
			_, _ = coll.UpdateOne(rbCtx, bson.M{"decisionCaseId": decisionCaseID}, bson.M{
				"$unset": bson.M{"lastAdsContextRequestedAt": ""},
				"$set":   bson.M{"updatedAt": time.Now().UnixMilli()},
			})
		} else {
			_, _ = coll.UpdateOne(rbCtx, bson.M{"decisionCaseId": decisionCaseID}, bson.M{
				"$set": bson.M{"lastAdsContextRequestedAt": prev, "updatedAt": time.Now().UnixMilli()},
			})
		}
	}
	return rollback, true, nil
}

// bsonRawInt64FromM trả về giá trị int64 và có tồn tại key hay không (kể cả 0).
func bsonRawInt64FromM(raw bson.M, key string) (v int64, exists bool) {
	x, ok := raw[key]
	if !ok {
		return 0, false
	}
	switch t := x.(type) {
	case int64:
		return t, true
	case int32:
		return int64(t), true
	case float64:
		return int64(t), true
	default:
		return 0, true
	}
}

// FindCaseByDecisionCaseID tìm case theo decisionCaseId (org + owner). Không thấy → nil, nil.
func (s *AIDecisionService) FindCaseByDecisionCaseID(ctx context.Context, decisionCaseID string, ownerOrgID primitive.ObjectID) (*aidecisionmodels.DecisionCase, error) {
	if decisionCaseID == "" {
		return nil, nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return nil, errors.New("không tìm thấy collection decision_cases_runtime")
	}
	filter := bson.M{
		"decisionCaseId":      decisionCaseID,
		"ownerOrganizationId": ownerOrgID,
	}
	var doc aidecisionmodels.DecisionCase
	err := coll.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

// FindCaseByConversation tìm case đang mở theo conversation/customer.
func (s *AIDecisionService) FindCaseByConversation(ctx context.Context, conversationID, customerID, orgID string, ownerOrgID primitive.ObjectID) (*aidecisionmodels.DecisionCase, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return nil, errors.New("không tìm thấy collection decision_cases_runtime")
	}

	filter := bson.M{
		"orgId":    orgID,
		"caseType": aidecisionmodels.CaseTypeConversationResponse,
		"status":   bson.M{"$nin": []string{aidecisionmodels.CaseStatusClosed, "cancelled", "expired", "dropped"}},
	}
	if conversationID != "" {
		filter["entityRefs.conversationId"] = conversationID
	}
	if customerID != "" {
		filter["entityRefs.customerId"] = customerID
	}

	var doc aidecisionmodels.DecisionCase
	err := coll.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// AppendActionIDsToCase thêm action IDs vào case sau khi Execute.
func (s *AIDecisionService) AppendActionIDsToCase(ctx context.Context, decisionCaseID string, actionIDs []string) error {
	if decisionCaseID == "" || len(actionIDs) == 0 {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return errors.New("không tìm thấy collection decision_cases_runtime")
	}
	now := time.Now().UnixMilli()
	_, err := coll.UpdateOne(ctx, bson.M{"decisionCaseId": decisionCaseID}, bson.M{
		"$addToSet": bson.M{"actionIds": bson.M{"$each": actionIDs}},
		"$set":      bson.M{"status": aidecisionmodels.CaseStatusActionsCreated, "updatedAt": now},
	})
	return err
}

// SetDecisionPacketOnCase ghi decision_packet (metadata quyết định — vision §11) trước khi đóng case.
func (s *AIDecisionService) SetDecisionPacketOnCase(ctx context.Context, decisionCaseID string, packet map[string]interface{}) error {
	if decisionCaseID == "" || packet == nil || len(packet) == 0 {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return errors.New("không tìm thấy collection decision_cases_runtime")
	}
	now := time.Now().UnixMilli()
	_, err := coll.UpdateOne(ctx, bson.M{"decisionCaseId": decisionCaseID}, bson.M{
		"$set": bson.M{"decisionPacket": packet, "updatedAt": now},
	})
	return err
}

// CloseCaseWithOrgCheck đóng case với kiểm tra ownerOrgID (cho API closed_manual).
func (s *AIDecisionService) CloseCaseWithOrgCheck(ctx context.Context, decisionCaseID, closureType string, ownerOrgID primitive.ObjectID) error {
	if decisionCaseID == "" {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return errors.New("không tìm thấy collection decision_cases_runtime")
	}
	res := coll.FindOne(ctx, bson.M{"decisionCaseId": decisionCaseID, "ownerOrganizationId": ownerOrgID})
	if res.Err() != nil {
		return common.ErrNotFound
	}
	return s.CloseCase(ctx, decisionCaseID, closureType)
}

// CloseCase đóng case với closureType (closed_proposed | closed_complete | closed_timeout | closed_manual).
func (s *AIDecisionService) CloseCase(ctx context.Context, decisionCaseID, closureType string) error {
	if decisionCaseID == "" {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return errors.New("không tìm thấy collection decision_cases_runtime")
	}
	now := time.Now().UnixMilli()
	_, err := coll.UpdateOne(ctx, bson.M{"decisionCaseId": decisionCaseID}, bson.M{
		"$set": bson.M{
			"status":      aidecisionmodels.CaseStatusClosed,
			"closureType": closureType,
			"closedAt":    now,
			"updatedAt":   now,
		},
	})
	return err
}

// CloseStaleCases đóng các case quá hạn (decided/actions_created) với closed_timeout.
// maxAgeHours: case cũ hơn X giờ sẽ bị đóng.
func (s *AIDecisionService) CloseStaleCases(ctx context.Context, maxAgeHours int) (int, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return 0, nil
	}
	if maxAgeHours <= 0 {
		maxAgeHours = 24
	}
	cutoff := time.Now().Add(-time.Duration(maxAgeHours) * time.Hour).UnixMilli()

	filter := bson.M{
		"status": bson.M{"$in": []string{
			aidecisionmodels.CaseStatusDecided,
			aidecisionmodels.CaseStatusActionsCreated,
			aidecisionmodels.CaseStatusExecuting,
			aidecisionmodels.CaseStatusOutcomeWaiting,
		}},
		"updatedAt": bson.M{"$lt": cutoff},
	}

	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var docs []struct {
		DecisionCaseID string `bson:"decisionCaseId"`
	}
	if err = cursor.All(ctx, &docs); err != nil {
		return 0, err
	}

	closed := 0
	for _, d := range docs {
		if err := s.CloseCase(ctx, d.DecisionCaseID, aidecisionmodels.ClosureTimeout); err == nil {
			closed++
		}
	}
	return closed, nil
}

// findClosedCaseForReopen tìm case đã closed trong cửa sổ reopen (supplement §4.7). Cần khóa entity rõ (conversationId, …).
func findClosedCaseForReopen(ctx context.Context, coll *mongo.Collection, input *ResolveOrCreateInput, now int64) (*aidecisionmodels.DecisionCase, error) {
	if !caseHasEntityKeyForResolve(input) {
		return nil, nil
	}
	sec := reopenWindowSecFromEnv()
	if sec <= 0 {
		return nil, nil
	}
	cutoff := now - sec*1000
	f := bson.M{
		"orgId":    input.OrgID,
		"caseType": input.CaseType,
		"status":   aidecisionmodels.CaseStatusClosed,
		"closedAt": bson.M{"$gte": cutoff},
	}
	switch input.CaseType {
	case aidecisionmodels.CaseTypeConversationResponse:
		f["entityRefs.conversationId"] = input.EntityRefs.ConversationID
	case aidecisionmodels.CaseTypeCustomerState:
		f["entityRefs.customerId"] = input.EntityRefs.CustomerID
	case aidecisionmodels.CaseTypeAdsOptimization:
		f["entityRefs.campaignId"] = input.EntityRefs.CampaignID
	case aidecisionmodels.CaseTypeOrderRisk:
		f["entityRefs.orderId"] = input.EntityRefs.OrderID
	default:
		return nil, nil
	}
	var doc aidecisionmodels.DecisionCase
	err := coll.FindOne(ctx, f, options.FindOne().SetSort(bson.D{{Key: "closedAt", Value: -1}})).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func caseHasEntityKeyForResolve(input *ResolveOrCreateInput) bool {
	switch input.CaseType {
	case aidecisionmodels.CaseTypeConversationResponse:
		return input.EntityRefs.ConversationID != ""
	case aidecisionmodels.CaseTypeCustomerState:
		return input.EntityRefs.CustomerID != ""
	case aidecisionmodels.CaseTypeAdsOptimization:
		return input.EntityRefs.CampaignID != ""
	case aidecisionmodels.CaseTypeOrderRisk:
		return input.EntityRefs.OrderID != ""
	default:
		return false
	}
}

// reopenWindowSecFromEnv cửa sổ cho phép reopen sau khi case closed (mặc định 300 giây). Đặt 0 để tắt reopen.
func reopenWindowSecFromEnv() int64 {
	v := strings.TrimSpace(os.Getenv("AI_DECISION_REOPEN_WINDOW_SEC"))
	if v == "" {
		return 300
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return 300
	}
	return n
}

// mergeCaseTraceOntoSetDoc thêm traceId/correlationId vào $set chỉ khi bản ghi case chưa có (giữ neo gốc).
func mergeCaseTraceOntoSetDoc(setDoc bson.M, existingTraceID, existingCorrID, inTrace, inCorr string) {
	if strings.TrimSpace(existingTraceID) == "" && strings.TrimSpace(inTrace) != "" {
		setDoc["traceId"] = strings.TrimSpace(inTrace)
	}
	if strings.TrimSpace(existingCorrID) == "" && strings.TrimSpace(inCorr) != "" {
		setDoc["correlationId"] = strings.TrimSpace(inCorr)
	}
}

// HasAllRequiredContexts kiểm tra case đã có đủ requiredContexts chưa (Context Policy Matrix §3.4).
func HasAllRequiredContexts(c *aidecisionmodels.DecisionCase) bool {
	return contextpolicy.HasAllRequiredContexts(c)
}
