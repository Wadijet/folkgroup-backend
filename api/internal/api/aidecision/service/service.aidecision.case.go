// Package aidecisionsvc — Decision Case: ResolveOrCreate, Update.
//
// Theo PLATFORM_L1_EVENT_DECISION_SUPPLEMENT §4, §3.2.
package aidecisionsvc

import (
	"context"
	"errors"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	}

	var existing aidecisionmodels.DecisionCase
	err := coll.FindOne(ctx, filter).Decode(&existing)
	if err == nil {
		// Cập nhật case cũ
		update := bson.M{
			"$set": bson.M{
				"latestEventId":   input.EventID,
				"status":          aidecisionmodels.CaseStatusContextCollecting,
				"updatedAt":       now,
			},
			"$addToSet": bson.M{"triggerEventIds": input.EventID},
		}
		_, err = coll.UpdateOne(ctx, bson.M{"_id": existing.ID}, update)
		if err != nil {
			return nil, false, err
		}
		existing.LatestEventID = input.EventID
		existing.Status = aidecisionmodels.CaseStatusContextCollecting
		existing.UpdatedAt = now
		return &existing, false, nil
	}

	// Tạo case mới
	caseID := utility.GenerateUID(utility.UIDPrefixDecisionCase)
	doc := &aidecisionmodels.DecisionCase{
		DecisionCaseID:   caseID,
		OrgID:            input.OrgID,
		OwnerOrganizationID: input.OwnerOrgID,
		RootEventID:      input.EventID,
		TriggerEventIDs:  []string{input.EventID},
		LatestEventID:    input.EventID,
		EntityRefs:       input.EntityRefs,
		CaseType:         input.CaseType,
		Priority:         input.Priority,
		Urgency:          input.Urgency,
		Status:           aidecisionmodels.CaseStatusContextCollecting,
		RequiredContexts: input.RequiredCtx,
		ReceivedContexts: []string{},
		ContextPackets:   make(map[string]interface{}),
		ActionIDs:        []string{},
		ExecutionIDs:     []string{},
		OpenedAt:         now,
		CreatedAt:        now,
		UpdatedAt:        now,
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
			"status":            aidecisionmodels.CaseStatusContextCollecting,
			"updatedAt":         now,
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
			"updatedAt":              now,
		},
		"$addToSet": bson.M{"receivedContexts": "customer"},
	}

	_, err := coll.UpdateOne(ctx, filter, update)
	return err
}

// UpdateCaseWithOrderContext cập nhật case với order flags khi nhận order.flags_emitted.
func (s *AIDecisionService) UpdateCaseWithOrderContext(ctx context.Context, orderID, customerID, convID, orgID string, ownerOrgID primitive.ObjectID, orderPayload map[string]interface{}) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return errors.New("không tìm thấy collection decision_cases_runtime")
	}
	filter := bson.M{
		"orgId":    orgID,
		"status":   bson.M{"$nin": []string{aidecisionmodels.CaseStatusClosed, "cancelled", "expired", "dropped"}},
	}
	if convID != "" {
		filter["entityRefs.conversationId"] = convID
	}
	if customerID != "" {
		filter["entityRefs.customerId"] = customerID
	}
	if orderID != "" {
		filter["entityRefs.orderId"] = orderID
	}
	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{
			"contextPackets.order": orderPayload,
			"updatedAt":            now,
		},
		"$addToSet": bson.M{"receivedContexts": "order"},
	}
	_, err := coll.UpdateOne(ctx, filter, update)
	return err
}

// UpdateCaseWithAdsContext cập nhật case với ads context khi nhận ads.context_ready.
func (s *AIDecisionService) UpdateCaseWithAdsContext(ctx context.Context, adAccountID, orgID string, ownerOrgID primitive.ObjectID, adsPayload map[string]interface{}) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return errors.New("không tìm thấy collection decision_cases_runtime")
	}
	filter := bson.M{
		"orgId":               orgID,
		"ownerOrganizationId": ownerOrgID,
		"status":              bson.M{"$nin": []string{aidecisionmodels.CaseStatusClosed, "cancelled", "expired", "dropped"}},
		"requiredContexts":   bson.M{"$in": []string{"ads"}},
	}
	if adAccountID != "" {
		filter["entityRefs.campaignId"] = adAccountID
	}
	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{
			"contextPackets.ads": adsPayload,
			"updatedAt":         now,
		},
		"$addToSet": bson.M{"receivedContexts": "ads"},
	}
	_, err := coll.UpdateOne(ctx, filter, update)
	return err
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

// HasAllRequiredContexts kiểm tra case đã có đủ requiredContexts chưa.
func HasAllRequiredContexts(c *aidecisionmodels.DecisionCase) bool {
	if c == nil || len(c.RequiredContexts) == 0 {
		return true
	}
	received := make(map[string]bool)
	for _, r := range c.ReceivedContexts {
		received[r] = true
	}
	for _, req := range c.RequiredContexts {
		if !received[req] {
			return false
		}
	}
	return true
}
