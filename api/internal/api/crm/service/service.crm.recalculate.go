// Package crmvc - RecalculateCustomerFromAllSources: cập nhật toàn bộ thông tin khách hàng từ tất cả nguồn.
// Dùng khi cần chủ động refresh profile + metrics + classification (sync lỗi, data nguồn thay đổi).
package crmvc

import (
	"context"
	"errors"
	"strings"
	"time"

	crmdto "meta_commerce/internal/api/crm/dto"
	crmmodels "meta_commerce/internal/api/crm/models"
	fbmodels "meta_commerce/internal/api/fb/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// RecalculateCustomerResult kết quả recalculate — trả về cho handler.
type RecalculateCustomerResult struct {
	UnifiedId             string `json:"unifiedId"`
	UpdatedAt             int64  `json:"updatedAt"`
	ProfileUpdated        bool   `json:"profileUpdated"`
	MetricsUpdated        bool   `json:"metricsUpdated"`
	ClassificationUpdated bool   `json:"classificationUpdated"`
	ActivitiesBackfilled  int    `json:"activitiesBackfilled"` // Số conversation_started đã ghi bổ sung
}

// RecalculateCustomerFromAllSources tính toán lại toàn bộ thông tin khách hàng từ tất cả nguồn.
//
// Luồng:
// 1. Lấy crm_customer hiện có (unifiedId, sourceIds, primarySource)
// 2. Rebuild profile: merge từ POS (nếu có) + FB (nếu có), ưu tiên primarySource, fill gaps từ orders/conversations
// 3. Aggregate metrics từ pc_pos_orders + fb_conversations
// 4. Compute classification
// 5. Update crm_customers
//
// Tham số:
// - ctx: context
// - unifiedId: ID thống nhất của khách (crm_customers.unifiedId)
// - ownerOrgID: ID tổ chức sở hữu
//
// Trả về:
// - *RecalculateCustomerResult: kết quả cập nhật
// - error: lỗi nếu có (ErrNotFound khi không tìm thấy khách)
func (s *CrmCustomerService) RecalculateCustomerFromAllSources(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID) (*RecalculateCustomerResult, error) {
	unifiedId = trimSpace(unifiedId)
	if unifiedId == "" {
		return nil, common.NewError(common.ErrCodeValidationInput, "unifiedId không được để trống", common.StatusBadRequest, nil)
	}

	// 1. Lấy crm_customer hiện có
	customer, err := s.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, nil)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, common.ErrNotFound
		}
		return nil, err
	}

	// 2. Rebuild profile từ tất cả nguồn
	profile := s.rebuildProfileFromAllSources(ctx, &customer)

	// 3. Mở rộng ids cho conversation/order: thêm FB/POS customer tìm qua phone khi chưa merge
	ids := buildCustomerIdsForRecalculate(&customer)
	phones := GetPhoneNumbersFromCustomer(&customer)
	if len(profile.PhoneNumbers) > 0 {
		phones = profile.PhoneNumbers
	}
	ids = s.expandCustomerIdsForAggregation(ctx, &customer, ids, phones, ownerOrgID)

	// 4. Aggregate metrics (orders + conversations)
	metrics := s.aggregateOrderMetricsForCustomer(ctx, ids, ownerOrgID, phones, 0)
	convMetrics := s.aggregateConversationMetricsForCustomer(ctx, ids, ownerOrgID, 0)
	hasConv := convMetrics.ConversationCount > 0 || s.checkHasConversation(ctx, ids, ownerOrgID)

	// 5. Cập nhật sourceIds nếu tìm thấy link mới qua phone (để lần sau match conversation)
	sourceIds := customer.SourceIds
	if sourceIds.Fb == "" && len(phones) > 0 {
		if fbId := s.findFbCustomerByPhone(ctx, phones[0], ownerOrgID); fbId != "" {
			sourceIds.Fb = fbId
		}
	}
	if sourceIds.Pos == "" && len(phones) > 0 {
		if posId := s.findPosCustomerByPhone(ctx, phones[0], ownerOrgID); posId != "" {
			sourceIds.Pos = posId
		}
	}

	// 6. Compute classification
	cm := BuildCurrentMetricsFromOrderAndConv(metrics, convMetrics, hasConv)
	class := ComputeClassificationFromMetrics(metrics.TotalSpent, metrics.OrderCount, metrics.LastOrderAt, metrics.RevenueLast30d, metrics.RevenueLast90d, metrics.OrderCountOnline, metrics.OrderCountOffline, hasConv)

	// 7. Update crm_customers
	now := time.Now().UnixMilli()
	setFields := bson.M{
		"profile":             profile,
		"sourceIds":           sourceIds,
		"totalSpent":          metrics.TotalSpent,
		"orderCount":          metrics.OrderCount,
		"lastOrderAt":         metrics.LastOrderAt,
		"ownedSkuQuantities":  metrics.OwnedSkuQuantities,
		"conversationTags":    convMetrics.ConversationTags,
		"mergedAt":            now,
		"updatedAt":           now,
		"currentMetrics":      cm,
	}
	for k, v := range class {
		setFields[k] = v
	}
	unsetAll := make(bson.M)
	for k, v := range unsetRawFields {
		unsetAll[k] = v
	}
	for k, v := range unsetProfileLegacyFields {
		unsetAll[k] = v
	}
	update := bson.M{
		"$set":   setFields,
		"$unset": unsetAll,
	}
	_, err = s.Collection().UpdateOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, update)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	// 7b. Ghi activity customer_updated với metricsSnapshot mới — để report (ComputeCustomerReport) có layer3
	// Report lấy snapshot từ crm_activity_history; nếu không ghi activity thì report dùng snapshot cũ.
	s.logRecalculateActivity(ctx, unifiedId, ownerOrgID, now)

	// 8. Backfill conversation_started activities thiếu — để lịch sử hiển thị đầy đủ
	// Truyền unifiedId để đảm bảo resolve thành công (conversation có thể link qua customer_id=unifiedId
	// nhưng extractConversationCustomerId trả customers[0].id có thể chưa có trong sourceIds.Fb)
	activitiesBackfilled := s.backfillConversationActivitiesForCustomer(ctx, unifiedId, ids, ownerOrgID)

	return &RecalculateCustomerResult{
		UnifiedId:             unifiedId,
		UpdatedAt:             now,
		ProfileUpdated:        true,
		MetricsUpdated:        true,
		ClassificationUpdated: true,
		ActivitiesBackfilled:  activitiesBackfilled,
	}, nil
}

// RecalculateAllCustomers tính toán lại tất cả khách hàng hiện có của org (ngược với backfill).
// Lặp qua crm_customers theo ownerOrganizationId, gọi RecalculateCustomerFromAllSources cho từng khách.
// limit <= 0: xử lý tất cả; limit > 0: giới hạn số khách.
func (s *CrmCustomerService) RecalculateAllCustomers(ctx context.Context, ownerOrgID primitive.ObjectID, limit int) (*crmdto.CrmRecalculateAllResult, error) {
	useLimit := limit > 0
	filter := bson.M{"ownerOrganizationId": ownerOrgID}
	opts := mongoopts.Find().SetProjection(bson.M{"unifiedId": 1})
	if useLimit {
		opts.SetLimit(int64(limit))
	}
	customers, err := s.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	result := &crmdto.CrmRecalculateAllResult{}
	const maxFailedIds = 10
	for _, c := range customers {
		_, err := s.RecalculateCustomerFromAllSources(ctx, c.UnifiedId, ownerOrgID)
		if err != nil {
			result.TotalFailed++
			if len(result.FailedIds) < maxFailedIds {
				result.FailedIds = append(result.FailedIds, c.UnifiedId)
			}
			continue
		}
		result.TotalProcessed++
	}
	return result, nil
}

// rebuildProfileFromAllSources xây dựng profile từ POS + FB + orders + conversations.
// Ưu tiên: primarySource (pos|fb) trước, fill gaps từ nguồn còn lại, rồi từ order/conversation gần nhất.
func (s *CrmCustomerService) rebuildProfileFromAllSources(ctx context.Context, c *crmmodels.CrmCustomer) crmmodels.CrmCustomerProfile {
	var posProfile, fbProfile crmmodels.CrmCustomerProfile

	// Lấy profile từ POS nếu có
	if c.SourceIds.Pos != "" {
		if posDoc := s.fetchPosCustomerById(ctx, c.SourceIds.Pos, c.OwnerOrganizationID); posDoc != nil {
			posProfile = buildProfileFromPosDoc(posDoc)
		}
	}

	// Lấy profile từ FB nếu có
	if c.SourceIds.Fb != "" {
		if fbDoc := s.fetchFbCustomerById(ctx, c.SourceIds.Fb, c.OwnerOrganizationID); fbDoc != nil {
			fbProfile = buildProfileFromFbDoc(fbDoc)
		}
	}

	// Merge: primary trước, fill gaps từ secondary
	var merged crmmodels.CrmCustomerProfile
	if c.PrimarySource == "pos" {
		merged = mergeProfileFillGaps(posProfile, fbProfile)
	} else {
		merged = mergeProfileFillGaps(fbProfile, posProfile)
	}

	// Fill gaps từ order gần nhất
	if orderDoc := s.fetchLatestOrderForCustomer(ctx, buildCustomerIdsForRecalculate(c), c.OwnerOrganizationID); orderDoc != nil {
		custData := extractCustomerDataFromOrder(orderDoc)
		merged = fillProfileGapsFromConvData(merged, custData)
	}

	// Fill gaps từ conversation gần nhất
	if convDoc := s.fetchLatestConversationForCustomer(ctx, buildCustomerIdsForRecalculate(c), c.OwnerOrganizationID); convDoc != nil {
		custData := extractCustomerDataFromConv(convDoc)
		merged = fillProfileGapsFromConvData(merged, custData)
	}

	return merged
}

// buildProfileFromPosDoc trích profile từ PcPosCustomer.
func buildProfileFromPosDoc(doc *pcmodels.PcPosCustomer) crmmodels.CrmCustomerProfile {
	name := doc.Name
	if name == "" {
		name, _ = getStringFromMap(doc.PosData, "name")
	}
	phones := normalizePhones(doc.PhoneNumbers)
	if len(phones) == 0 {
		phones = normalizePhones(getStringArrayFromMap(doc.PosData, "phone_numbers"))
	}
	emails := uniqueStrings(doc.Emails)
	if len(emails) == 0 {
		emails = uniqueStrings(getStringArrayFromMap(doc.PosData, "emails"))
	}
	birthday, _ := getStringFromMap(doc.PosData, "date_of_birth")
	if birthday == "" {
		birthday, _ = getStringFromMap(doc.PosData, "birthday")
	}
	gender, _ := getStringFromMap(doc.PosData, "gender")
	addresses := getAddressesFromMap(doc.PosData, "shop_customer_addresses", "shop_customer_address", "addresses")
	referralCode, _ := getStringFromMap(doc.PosData, "referral_code")
	return crmmodels.CrmCustomerProfile{
		Name: name, PhoneNumbers: phones, Emails: emails,
		Birthday: birthday, Gender: gender, LivesIn: "", Addresses: addresses, ReferralCode: referralCode,
	}
}

// buildProfileFromFbDoc trích profile từ FbCustomer.
func buildProfileFromFbDoc(doc *fbmodels.FbCustomer) crmmodels.CrmCustomerProfile {
	name := doc.Name
	if name == "" {
		name, _ = getStringFromMap(doc.PanCakeData, "name")
	}
	phones := normalizePhones(doc.PhoneNumbers)
	if len(phones) == 0 {
		phones = normalizePhones(getStringArrayFromMap(doc.PanCakeData, "phone_numbers"))
	}
	emails := uniqueStrings([]string{doc.Email})
	if len(emails) == 0 {
		if arr := getStringArrayFromMap(doc.PanCakeData, "emails"); len(arr) > 0 {
			emails = uniqueStrings(arr)
		} else if e, ok := getStringFromMap(doc.PanCakeData, "email"); ok && e != "" {
			emails = []string{e}
		}
	}
	birthday, _ := getStringFromMap(doc.PanCakeData, "birthday")
	if birthday == "" {
		birthday, _ = getStringFromMap(doc.PanCakeData, "date_of_birth")
	}
	gender, _ := getStringFromMap(doc.PanCakeData, "gender")
	livesIn, _ := getStringFromMap(doc.PanCakeData, "lives_in")
	addresses := getAddressesFromMap(doc.PanCakeData, "addresses", "shop_customer_addresses", "shop_customer_address")
	return crmmodels.CrmCustomerProfile{
		Name: name, PhoneNumbers: phones, Emails: emails,
		Birthday: birthday, Gender: gender, LivesIn: livesIn, Addresses: addresses, ReferralCode: "",
	}
}

// mergeProfileFillGaps merge primary với secondary — chỉ fill gaps (field trống).
func mergeProfileFillGaps(primary, secondary crmmodels.CrmCustomerProfile) crmmodels.CrmCustomerProfile {
	out := primary
	if out.Name == "" && secondary.Name != "" {
		out.Name = secondary.Name
	}
	if len(out.PhoneNumbers) == 0 && len(secondary.PhoneNumbers) > 0 {
		out.PhoneNumbers = secondary.PhoneNumbers
	}
	if len(out.Emails) == 0 && len(secondary.Emails) > 0 {
		out.Emails = secondary.Emails
	}
	if out.Birthday == "" && secondary.Birthday != "" {
		out.Birthday = secondary.Birthday
	}
	if out.Gender == "" && secondary.Gender != "" {
		out.Gender = secondary.Gender
	}
	if out.LivesIn == "" && secondary.LivesIn != "" {
		out.LivesIn = secondary.LivesIn
	}
	if len(out.Addresses) == 0 && len(secondary.Addresses) > 0 {
		out.Addresses = secondary.Addresses
	}
	if out.ReferralCode == "" && secondary.ReferralCode != "" {
		out.ReferralCode = secondary.ReferralCode
	}
	return out
}

// fillProfileGapsFromConvData fill gaps profile từ convCustomerData (order/conversation).
func fillProfileGapsFromConvData(p crmmodels.CrmCustomerProfile, d convCustomerData) crmmodels.CrmCustomerProfile {
	if p.Name == "" && d.Name != "" {
		p.Name = d.Name
	}
	if len(p.PhoneNumbers) == 0 && len(d.Phones) > 0 {
		p.PhoneNumbers = normalizePhones(d.Phones)
	}
	if len(p.Emails) == 0 && len(d.Emails) > 0 {
		p.Emails = uniqueStrings(d.Emails)
	}
	if p.Birthday == "" && d.Birthday != "" {
		p.Birthday = d.Birthday
	}
	if p.Gender == "" && d.Gender != "" {
		p.Gender = d.Gender
	}
	if p.LivesIn == "" && d.LivesIn != "" {
		p.LivesIn = d.LivesIn
	}
	if len(p.Addresses) == 0 && len(d.Addresses) > 0 {
		p.Addresses = d.Addresses
	}
	if p.ReferralCode == "" && d.ReferralCode != "" {
		p.ReferralCode = d.ReferralCode
	}
	return p
}

// buildCustomerIdsForRecalculate tạo danh sách customerId để query (unifiedId + pos + fb).
func buildCustomerIdsForRecalculate(c *crmmodels.CrmCustomer) []string {
	seen := make(map[string]bool)
	var ids []string
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	add(c.UnifiedId)
	add(c.SourceIds.Pos)
	add(c.SourceIds.Fb)
	return ids
}

// expandCustomerIdsForAggregation mở rộng ids để match conversation/order khi khách chưa merge.
// Thêm FB customer ID tìm qua phone (nếu sourceIds.Fb trống), POS customer ID tìm qua phone (nếu sourceIds.Pos trống).
// Giúp aggregateConversationMetricsForCustomer tìm được hội thoại của khách FB có cùng SĐT.
func (s *CrmCustomerService) expandCustomerIdsForAggregation(ctx context.Context, c *crmmodels.CrmCustomer, ids []string, phones []string, ownerOrgID primitive.ObjectID) []string {
	seen := make(map[string]bool)
	for _, id := range ids {
		if id != "" {
			seen[id] = true
		}
	}
	for _, p := range phones {
		if p == "" {
			continue
		}
		norm := normalizePhones([]string{p})
		if len(norm) == 0 {
			continue
		}
		if c.SourceIds.Fb == "" {
			if fbId := s.findFbCustomerByPhone(ctx, norm[0], ownerOrgID); fbId != "" && !seen[fbId] {
				seen[fbId] = true
				ids = append(ids, fbId)
			}
		}
		if c.SourceIds.Pos == "" {
			if posId := s.findPosCustomerByPhone(ctx, norm[0], ownerOrgID); posId != "" && !seen[posId] {
				seen[posId] = true
				ids = append(ids, posId)
			}
		}
	}
	return ids
}

// fetchPosCustomerById lấy PcPosCustomer theo customerId.
func (s *CrmCustomerService) fetchPosCustomerById(ctx context.Context, customerId string, ownerOrgID primitive.ObjectID) *pcmodels.PcPosCustomer {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCustomers)
	if !ok {
		return nil
	}
	var doc pcmodels.PcPosCustomer
	if err := coll.FindOne(ctx, bson.M{"customerId": customerId, "ownerOrganizationId": ownerOrgID}).Decode(&doc); err != nil {
		return nil
	}
	return &doc
}

// fetchFbCustomerById lấy FbCustomer theo customerId.
func (s *CrmCustomerService) fetchFbCustomerById(ctx context.Context, customerId string, ownerOrgID primitive.ObjectID) *fbmodels.FbCustomer {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbCustomers)
	if !ok {
		return nil
	}
	var doc fbmodels.FbCustomer
	if err := coll.FindOne(ctx, bson.M{"customerId": customerId, "ownerOrganizationId": ownerOrgID}).Decode(&doc); err != nil {
		return nil
	}
	return &doc
}

// fetchLatestOrderForCustomer lấy đơn hàng gần nhất của khách.
func (s *CrmCustomerService) fetchLatestOrderForCustomer(ctx context.Context, customerIds []string, ownerOrgID primitive.ObjectID) *pcmodels.PcPosOrder {
	if len(customerIds) == 0 {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return nil
	}
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$or": []bson.M{
			{"customerId": bson.M{"$in": customerIds}},
			{"posData.customer.id": bson.M{"$in": customerIds}},
		},
	}
	opts := mongoopts.FindOne().SetSort(bson.D{{Key: "insertedAt", Value: -1}})
	var doc pcmodels.PcPosOrder
	if err := coll.FindOne(ctx, filter, opts).Decode(&doc); err != nil {
		return nil
	}
	return &doc
}

// fetchLatestConversationForCustomer lấy hội thoại gần nhất của khách.
func (s *CrmCustomerService) fetchLatestConversationForCustomer(ctx context.Context, customerIds []string, ownerOrgID primitive.ObjectID) *fbmodels.FbConversation {
	if len(customerIds) == 0 {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok {
		return nil
	}
	filter := buildConversationFilterForCustomerIds(customerIds, ownerOrgID)
	opts := mongoopts.FindOne().SetSort(bson.D{{Key: "panCakeUpdatedAt", Value: -1}})
	var doc fbmodels.FbConversation
	if err := coll.FindOne(ctx, filter, opts).Decode(&doc); err != nil {
		return nil
	}
	return &doc
}

const recalculateConversationActivityLimit = 100 // Giới hạn số conversation backfill mỗi lần recalculate

// backfillConversationActivitiesForCustomer ghi conversation_started cho các hội thoại chưa có activity.
// unifiedId: ID thống nhất của khách (dùng để resolve — đảm bảo ghi đúng customer khi conversation link qua customer_id khác sourceIds).
// customerIds: danh sách id để filter conversation (unifiedId + sourceIds + expanded).
// Trả về số activity đã ghi bổ sung.
func (s *CrmCustomerService) backfillConversationActivitiesForCustomer(ctx context.Context, unifiedId string, customerIds []string, ownerOrgID primitive.ObjectID) int {
	if unifiedId == "" || len(customerIds) == 0 {
		return 0
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok {
		return 0
	}
	filter := buildConversationFilterForCustomerIds(customerIds, ownerOrgID)
	opts := mongoopts.Find().SetSort(bson.D{{Key: "panCakeUpdatedAt", Value: 1}}).SetLimit(recalculateConversationActivityLimit)
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)

	backfilled := 0
	for cursor.Next(ctx) {
		var doc fbmodels.FbConversation
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		// Dùng unifiedId để đảm bảo resolve thành công — conversation có thể link qua customer_id=unifiedId
		// nhưng extractConversationCustomerId trả customers[0].id có thể chưa có trong sourceIds.Fb
		logged, _ := s.IngestConversationTouchpoint(ctx, unifiedId, ownerOrgID, doc.ConversationId, true, &doc)
		if logged {
			backfilled++
		}
	}
	return backfilled
}

func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

// logRecalculateActivity ghi activity customer_updated với metricsSnapshot mới sau recalculate.
// Giúp report (ComputeCustomerReport) lấy được snapshot có layer3 qua GetLastSnapshotPerCustomerBeforeEndMs.
func (s *CrmCustomerService) logRecalculateActivity(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, activityAt int64) {
	actSvc, err := NewCrmActivityService()
	if err != nil {
		return
	}
	cust, err := s.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, nil)
	if err != nil {
		return
	}
	metadata := map[string]interface{}{"trigger": "recalculate"}
	snap := BuildSnapshotForNewCustomer(&cust, activityAt, false, nil)
	if snap != nil {
		MergeSnapshotIntoMetadata(metadata, snap)
	}
	_ = actSvc.LogActivity(ctx, LogActivityInput{
		UnifiedId:    unifiedId,
		OwnerOrgID:   ownerOrgID,
		Domain:       crmmodels.ActivityDomainCustomer,
		ActivityType: "customer_updated",
		Source:       "system",
		SourceRef:    map[string]interface{}{"trigger": "recalculate"},
		Metadata:     metadata,
		DisplayLabel: "Tính toán lại thông tin khách hàng",
		DisplayIcon:  "refresh",
		ActivityAt:   activityAt,
	})
}
