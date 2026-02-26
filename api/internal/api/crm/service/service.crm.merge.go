// Package crmvc - Logic merge khách hàng POS + FB vào crm_customers.
package crmvc

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	crmmodels "meta_commerce/internal/api/crm/models"
	fbmodels "meta_commerce/internal/api/fb/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// MergeFromPosCustomer xử lý khi pc_pos_customers thay đổi — upsert crm_customers.
func (s *CrmCustomerService) MergeFromPosCustomer(ctx context.Context, doc *pcmodels.PcPosCustomer) error {
	if doc == nil {
		return nil
	}
	ownerOrgID := doc.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}
	customerId := strings.TrimSpace(doc.CustomerId)
	if customerId == "" {
		return nil
	}

	// Tìm crm_customer hiện có (theo sourceIds.pos hoặc unifiedId)
	existing, errExisting := s.findByPosId(ctx, customerId, ownerOrgID)
	if errExisting != nil && !errors.Is(errExisting, common.ErrNotFound) {
		return errExisting
	}
	now := time.Now().UnixMilli()

	// Merge name, phone, emails từ POS
	name := doc.Name
	if name == "" {
		if n, ok := getStringFromMap(doc.PosData, "name"); ok {
			name = n
		}
	}
	phones := normalizePhones(doc.PhoneNumbers)
	if len(phones) == 0 {
		if arr := getStringArrayFromMap(doc.PosData, "phone_numbers"); len(arr) > 0 {
			phones = normalizePhones(arr)
		}
	}
	emails := uniqueStrings(doc.Emails)
	if len(emails) == 0 {
		emails = uniqueStrings(getStringArrayFromMap(doc.PosData, "emails"))
	}
	// Thông tin bổ sung từ PosData
	birthday, _ := getStringFromMap(doc.PosData, "date_of_birth")
	if birthday == "" {
		birthday, _ = getStringFromMap(doc.PosData, "birthday")
	}
	gender, _ := getStringFromMap(doc.PosData, "gender")
	addresses := getAddressesFromMap(doc.PosData, "shop_customer_addresses", "shop_customer_address", "addresses")
	referralCode, _ := getStringFromMap(doc.PosData, "referral_code")

	// Xác định merge method và có merge với FB không
	mergeMethod := "single_source"
	sourceIds := crmmodels.CrmCustomerSourceIds{Pos: customerId}
	primarySource := "pos"
	unifiedId := customerId

	// Thử merge qua posData.fb_id (format pageId_psid)
	if fbId, ok := getStringFromMap(doc.PosData, "fb_id"); ok && fbId != "" {
		parts := strings.SplitN(fbId, "_", 2)
		if len(parts) == 2 {
			fbCustomerId := s.findFbCustomerByPagePsid(ctx, parts[0], parts[1], ownerOrgID)
			if fbCustomerId != "" {
				sourceIds.Fb = fbCustomerId
				mergeMethod = "fb_id"
			}
		}
	}

	// Thử merge qua phone nếu chưa có FB
	if sourceIds.Fb == "" && len(phones) > 0 {
		fbByPhone := s.findFbCustomerByPhone(ctx, phones[0], ownerOrgID)
		if fbByPhone != "" {
			if existing != nil && existing.UnifiedId != "" {
				// Đã có crm_customer — cập nhật thêm fb
				sourceIds.Fb = fbByPhone
				sourceIds.Pos = customerId
				mergeMethod = "phone"
				unifiedId = existing.UnifiedId
			} else {
				sourceIds.Fb = fbByPhone
				mergeMethod = "phone"
				unifiedId = customerId // Ưu tiên POS làm unified
			}
		}
	}

	if existing != nil {
		unifiedId = existing.UnifiedId
		// Giữ sourceIds.fb nếu đã có
		if existing.SourceIds.Fb != "" {
			sourceIds.Fb = existing.SourceIds.Fb
		}
	}

	ids := []string{customerId, sourceIds.Fb, unifiedId}
	metrics := s.aggregateOrderMetricsForCustomer(ctx, ids, ownerOrgID, phones, 0)
	convMetrics := s.aggregateConversationMetricsForCustomer(ctx, ids, ownerOrgID, 0)
	hasConv := convMetrics.ConversationCount > 0 || s.checkHasConversation(ctx, ids, ownerOrgID)

	avgOrderValue := 0.0
	if metrics.OrderCount > 0 {
		avgOrderValue = metrics.TotalSpent / float64(metrics.OrderCount)
	}

	filter := bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}
	setFields := bson.M{
		"sourceIds": sourceIds, "primarySource": primarySource, "name": name,
		"phoneNumbers": phones, "emails": emails,
		"hasConversation": hasConv, "hasOrder": metrics.OrderCount > 0,
		"orderCountOnline": metrics.OrderCountOnline, "orderCountOffline": metrics.OrderCountOffline,
		"firstOrderChannel": metrics.FirstOrderChannel, "lastOrderChannel": metrics.LastOrderChannel,
		"isOmnichannel": metrics.OrderCountOnline > 0 && metrics.OrderCountOffline > 0,
		"totalSpent": metrics.TotalSpent, "orderCount": metrics.OrderCount,
		"lastOrderAt": metrics.LastOrderAt, "secondLastOrderAt": metrics.SecondLastOrderAt,
		"revenueLast30d": metrics.RevenueLast30d, "revenueLast90d": metrics.RevenueLast90d,
		"avgOrderValue": avgOrderValue, "cancelledOrderCount": metrics.CancelledOrderCount,
		"ordersLast30d": metrics.OrdersLast30d, "ordersLast90d": metrics.OrdersLast90d,
		"ordersFromAds": metrics.OrdersFromAds, "ordersFromOrganic": metrics.OrdersFromOrganic,
		"ordersFromDirect": metrics.OrdersFromDirect, "ownedSkuQuantities": metrics.OwnedSkuQuantities,
		"conversationCount": convMetrics.ConversationCount,
		"conversationCountByInbox": convMetrics.ConversationCountByInbox,
		"conversationCountByComment": convMetrics.ConversationCountByComment,
		"lastConversationAt": convMetrics.LastConversationAt,
		"firstConversationAt": convMetrics.FirstConversationAt,
		"totalMessages": convMetrics.TotalMessages,
		"lastMessageFromCustomer": convMetrics.LastMessageFromCustomer,
		"conversationFromAds": convMetrics.ConversationFromAds,
		"conversationTags": convMetrics.ConversationTags,
		"mergeMethod": mergeMethod, "mergedAt": now, "updatedAt": now,
	}
	if birthday != "" {
		setFields["birthday"] = birthday
	}
	if gender != "" {
		setFields["gender"] = gender
	}
	if len(addresses) > 0 {
		setFields["addresses"] = addresses
	}
	if referralCode != "" {
		setFields["referralCode"] = referralCode
	}
	// Cập nhật phân loại hiện tại (classification) theo metrics đã aggregate.
	for k, v := range ComputeClassificationFromMetrics(metrics.TotalSpent, metrics.OrderCount, metrics.LastOrderAt, metrics.RevenueLast30d, metrics.RevenueLast90d, metrics.OrderCountOnline, metrics.OrderCountOffline, hasConv) {
		setFields[k] = v
	}
	update := bson.M{
		"$set":         setFields,
		"$setOnInsert": bson.M{"createdAt": now},
	}
	opts := mongoopts.Update().SetUpsert(true)
	result, err := s.Collection().UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return common.ConvertMongoError(err)
	}
	// Ghi lịch sử khởi tạo khách khi tạo mới (ưu tiên created_at/inserted_at từ PosData API)
	if result.UpsertedCount > 0 {
		activityAt := getSourceCustomerTimestamp(doc.PosData)
		if activityAt <= 0 {
			activityAt = now
		}
		actSvc, errAct := NewCrmActivityService()
		if errAct != nil {
			logger.GetAppLogger().WithError(errAct).WithFields(map[string]interface{}{
				"unifiedId": unifiedId, "source": "pos", "customerId": customerId,
			}).Warn("[CRM] MergeFromPosCustomer: không thể tạo CrmActivityService, bỏ qua ghi customer_created")
		} else {
			metadata := map[string]interface{}{"source": "pos", "name": name, "mergeMethod": mergeMethod}
			if cust, err := s.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, nil); err == nil {
				MergeSnapshotIntoMetadata(metadata, BuildSnapshotForNewCustomer(&cust, activityAt, true))
			}
			errLog := actSvc.LogActivity(ctx, LogActivityInput{
				UnifiedId:    unifiedId,
				OwnerOrgID:   ownerOrgID,
				Domain:       crmmodels.ActivityDomainCustomer,
				ActivityType: "customer_created",
				Source:       "pos",
				SourceRef:    map[string]interface{}{"sourceCustomerId": customerId},
				Metadata:     metadata,
				DisplayLabel: "Khởi tạo khách từ POS - " + defaultName(name),
				DisplayIcon:  "person_add",
				ActivityAt:   activityAt,
			})
			if errLog != nil {
				logger.GetAppLogger().WithError(errLog).WithFields(map[string]interface{}{
					"unifiedId": unifiedId, "source": "pos",
				}).Warn("[CRM] MergeFromPosCustomer: LogActivity customer_created lỗi")
			}
		}
	} else if result.ModifiedCount > 0 {
		s.logCustomerUpdatedIfThrottled(ctx, unifiedId, ownerOrgID, "pos", "Cập nhật thông tin từ POS - "+defaultName(name))
	}
	return nil
}

// MergeFromFbCustomer xử lý khi fb_customers thay đổi.
// sourceEventAt: thời điểm sự kiện nguồn (Unix ms); khi > 0 dùng cho activity. Khi 0, lấy từ PanCakeData (created_at/inserted_at).
// Không dùng doc.CreatedAt/UpdatedAt vì là thời gian đồng bộ.
func (s *CrmCustomerService) MergeFromFbCustomer(ctx context.Context, doc *fbmodels.FbCustomer, sourceEventAt int64) error {
	if doc == nil {
		return nil
	}
	ownerOrgID := doc.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}
	fbCustomerId := strings.TrimSpace(doc.CustomerId)
	if fbCustomerId == "" {
		return nil
	}

	existing, errExisting := s.findByFbId(ctx, fbCustomerId, ownerOrgID)
	if errExisting != nil && !errors.Is(errExisting, common.ErrNotFound) {
		return errExisting
	}
	now := time.Now().UnixMilli()

	name := doc.Name
	if name == "" {
		if n, ok := getStringFromMap(doc.PanCakeData, "name"); ok {
			name = n
		}
	}
	phones := normalizePhones(doc.PhoneNumbers)
	if len(phones) == 0 {
		phones = normalizePhones(getStringArrayFromMap(doc.PanCakeData, "phone_numbers"))
	}
	emails := uniqueStrings([]string{doc.Email})
	if len(emails) == 0 {
		if arr := getStringArrayFromMap(doc.PanCakeData, "emails"); len(arr) > 0 {
			emails = uniqueStrings(arr)
		} else if s, ok := getStringFromMap(doc.PanCakeData, "email"); ok && s != "" {
			emails = []string{s}
		}
	}
	// Thông tin bổ sung từ PanCakeData (FB)
	fbBirthday, _ := getStringFromMap(doc.PanCakeData, "birthday")
	if fbBirthday == "" {
		fbBirthday, _ = getStringFromMap(doc.PanCakeData, "date_of_birth")
	}
	fbGender, _ := getStringFromMap(doc.PanCakeData, "gender")
	fbLivesIn, _ := getStringFromMap(doc.PanCakeData, "lives_in")
	fbAddresses := getAddressesFromMap(doc.PanCakeData, "addresses", "shop_customer_addresses", "shop_customer_address")

	mergeMethod := "single_source"
	sourceIds := crmmodels.CrmCustomerSourceIds{Fb: fbCustomerId}
	primarySource := "fb"
	unifiedId := fbCustomerId

	// Thử tìm POS qua pageId_psid (fb_id trong pos)
	posId := s.findPosCustomerByFbPagePsid(ctx, doc.PageId, doc.Psid, ownerOrgID)
	if posId != "" {
		sourceIds.Pos = posId
		mergeMethod = "fb_id"
		unifiedId = posId
		primarySource = "pos"
	}

	if sourceIds.Pos == "" && len(phones) > 0 {
		posByPhone := s.findPosCustomerByPhone(ctx, phones[0], ownerOrgID)
		if posByPhone != "" {
			sourceIds.Pos = posByPhone
			mergeMethod = "phone"
			unifiedId = posByPhone
			primarySource = "pos"
		}
	}

	if existing != nil {
		unifiedId = existing.UnifiedId
		if existing.SourceIds.Pos != "" {
			sourceIds.Pos = existing.SourceIds.Pos
		}
	}

	fbIds := []string{sourceIds.Pos, fbCustomerId, unifiedId}
	metrics := s.aggregateOrderMetricsForCustomer(ctx, fbIds, ownerOrgID, phones, 0)
	convMetrics := s.aggregateConversationMetricsForCustomer(ctx, fbIds, ownerOrgID, 0)
	hasConv := convMetrics.ConversationCount > 0 || s.checkHasConversation(ctx, fbIds, ownerOrgID)

	avgOrderValue := 0.0
	if metrics.OrderCount > 0 {
		avgOrderValue = metrics.TotalSpent / float64(metrics.OrderCount)
	}

	filter := bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}
	fbSetFields := bson.M{
		"sourceIds": sourceIds, "primarySource": primarySource, "name": name,
		"phoneNumbers": phones, "emails": emails,
		"hasConversation": hasConv, "hasOrder": metrics.OrderCount > 0,
		"orderCountOnline": metrics.OrderCountOnline, "orderCountOffline": metrics.OrderCountOffline,
		"firstOrderChannel": metrics.FirstOrderChannel, "lastOrderChannel": metrics.LastOrderChannel,
		"isOmnichannel": metrics.OrderCountOnline > 0 && metrics.OrderCountOffline > 0,
		"totalSpent": metrics.TotalSpent, "orderCount": metrics.OrderCount,
		"lastOrderAt": metrics.LastOrderAt, "secondLastOrderAt": metrics.SecondLastOrderAt,
		"revenueLast30d": metrics.RevenueLast30d, "revenueLast90d": metrics.RevenueLast90d,
		"avgOrderValue": avgOrderValue, "cancelledOrderCount": metrics.CancelledOrderCount,
		"ordersLast30d": metrics.OrdersLast30d, "ordersLast90d": metrics.OrdersLast90d,
		"ordersFromAds": metrics.OrdersFromAds, "ordersFromOrganic": metrics.OrdersFromOrganic,
		"ordersFromDirect": metrics.OrdersFromDirect, "ownedSkuQuantities": metrics.OwnedSkuQuantities,
		"conversationCount": convMetrics.ConversationCount,
		"conversationCountByInbox": convMetrics.ConversationCountByInbox,
		"conversationCountByComment": convMetrics.ConversationCountByComment,
		"lastConversationAt": convMetrics.LastConversationAt,
		"firstConversationAt": convMetrics.FirstConversationAt,
		"totalMessages": convMetrics.TotalMessages,
		"lastMessageFromCustomer": convMetrics.LastMessageFromCustomer,
		"conversationFromAds": convMetrics.ConversationFromAds,
		"conversationTags": convMetrics.ConversationTags,
		"mergeMethod": mergeMethod, "mergedAt": now, "updatedAt": now,
	}
	if fbBirthday != "" {
		fbSetFields["birthday"] = fbBirthday
	}
	if fbGender != "" {
		fbSetFields["gender"] = fbGender
	}
	if fbLivesIn != "" {
		fbSetFields["livesIn"] = fbLivesIn
	}
	if len(fbAddresses) > 0 {
		fbSetFields["addresses"] = fbAddresses
	}
	// Cập nhật phân loại hiện tại (classification) theo metrics đã aggregate.
	for k, v := range ComputeClassificationFromMetrics(metrics.TotalSpent, metrics.OrderCount, metrics.LastOrderAt, metrics.RevenueLast30d, metrics.RevenueLast90d, metrics.OrderCountOnline, metrics.OrderCountOffline, hasConv) {
		fbSetFields[k] = v
	}
	update := bson.M{
		"$set":         fbSetFields,
		"$setOnInsert": bson.M{"createdAt": now},
	}
	opts := mongoopts.Update().SetUpsert(true)
	result, err := s.Collection().UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return common.ConvertMongoError(err)
	}
	if result.UpsertedCount > 0 {
		activityAt := sourceEventAt
		if activityAt <= 0 {
			activityAt = getSourceCustomerTimestamp(doc.PanCakeData)
		}
		if activityAt <= 0 {
			activityAt = now
		}
		actSvc, errAct := NewCrmActivityService()
		if errAct != nil {
			logger.GetAppLogger().WithError(errAct).WithFields(map[string]interface{}{
				"unifiedId": unifiedId, "source": "fb", "fbCustomerId": fbCustomerId,
			}).Warn("[CRM] MergeFromFbCustomer: không thể tạo CrmActivityService, bỏ qua ghi customer_created")
		} else {
			metadata := map[string]interface{}{"source": "fb", "name": name, "mergeMethod": mergeMethod}
			if cust, err := s.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, nil); err == nil {
				MergeSnapshotIntoMetadata(metadata, BuildSnapshotForNewCustomer(&cust, activityAt, true))
			}
			errLog := actSvc.LogActivity(ctx, LogActivityInput{
				UnifiedId:    unifiedId,
				OwnerOrgID:   ownerOrgID,
				Domain:       crmmodels.ActivityDomainCustomer,
				ActivityType: "customer_created",
				Source:       "fb",
				SourceRef:    map[string]interface{}{"sourceCustomerId": fbCustomerId},
				Metadata:     metadata,
				DisplayLabel: "Khởi tạo khách từ Facebook - " + defaultName(name),
				DisplayIcon:  "person_add",
				ActivityAt:   activityAt,
			})
			if errLog != nil {
				logger.GetAppLogger().WithError(errLog).WithFields(map[string]interface{}{
					"unifiedId": unifiedId, "source": "fb",
				}).Warn("[CRM] MergeFromFbCustomer: LogActivity customer_created lỗi")
			}
		}
	} else if result.ModifiedCount > 0 {
		s.logCustomerUpdatedIfThrottled(ctx, unifiedId, ownerOrgID, "fb", "Cập nhật thông tin từ Facebook - "+defaultName(name))
	}
	return nil
}

// UpsertMinimalFromPosId tạo crm_customer tối thiểu nếu chưa có (từ order backfill).
// Sync (MergeFromPosCustomer) sẽ cập nhật đầy đủ khi có dữ liệu từ pc_pos_customers.
// custData: thông tin trích từ order (posData.customer, bill_*); nil nếu không có.
// activityAt: thời điểm từ nguồn order (0 = dùng now).
func (s *CrmCustomerService) UpsertMinimalFromPosId(ctx context.Context, customerId string, ownerOrgID primitive.ObjectID, custData *convCustomerData, activityAt int64) (unifiedId string, created bool) {
	customerId = strings.TrimSpace(customerId)
	if customerId == "" {
		return "", false
	}
	existing, err := s.findByPosId(ctx, customerId, ownerOrgID)
	if err == nil && existing != nil {
		return existing.UnifiedId, false
	}
	now := time.Now().UnixMilli()
	filter := bson.M{"unifiedId": customerId, "ownerOrganizationId": ownerOrgID}
	setOnInsert := bson.M{
		"unifiedId":           customerId,
		"sourceIds":           crmmodels.CrmCustomerSourceIds{Pos: customerId},
		"primarySource":       "pos",
		"mergeMethod":         "single_source",
		"ownerOrganizationId": ownerOrgID,
		"createdAt":           now,
		"mergedAt":            now,
	}
	if custData != nil {
		if custData.Name != "" {
			setOnInsert["name"] = custData.Name
		}
		if len(custData.Phones) > 0 {
			setOnInsert["phoneNumbers"] = normalizePhones(custData.Phones)
		}
		if len(custData.Emails) > 0 {
			setOnInsert["emails"] = uniqueStrings(custData.Emails)
		}
		if custData.Birthday != "" {
			setOnInsert["birthday"] = custData.Birthday
		}
		if custData.Gender != "" {
			setOnInsert["gender"] = custData.Gender
		}
		if custData.LivesIn != "" {
			setOnInsert["livesIn"] = custData.LivesIn
		}
		if len(custData.Addresses) > 0 {
			setOnInsert["addresses"] = custData.Addresses
		}
		if custData.ReferralCode != "" {
			setOnInsert["referralCode"] = custData.ReferralCode
		}
	}
	update := bson.M{
		"$setOnInsert": setOnInsert,
		"$set":         bson.M{"updatedAt": now},
	}
	opts := mongoopts.Update().SetUpsert(true)
	result, err := s.Collection().UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return "", false
	}
	if result.UpsertedCount > 0 {
		displayLabel := "Khởi tạo khách từ đơn hàng"
		if custData != nil && custData.Name != "" {
			displayLabel = "Khởi tạo khách từ đơn hàng - " + custData.Name
		}
		s.logCustomerCreatedFromSource(ctx, customerId, ownerOrgID, "order", displayLabel, activityAt)
	}
	return customerId, result.UpsertedCount > 0
}

// UpsertMinimalFromFbId tạo crm_customer tối thiểu nếu chưa có (từ conversation backfill).
// Sync (MergeFromFbCustomer) sẽ cập nhật đầy đủ khi có dữ liệu từ fb_customers.
// custData: thông tin trích từ panCakeData (customer, customers[0], page_customer); nil nếu không có.
// activityAt: thời điểm từ nguồn conversation (0 = dùng now).
func (s *CrmCustomerService) UpsertMinimalFromFbId(ctx context.Context, customerId string, ownerOrgID primitive.ObjectID, custData *convCustomerData, activityAt int64) (unifiedId string, created bool) {
	customerId = strings.TrimSpace(customerId)
	if customerId == "" {
		return "", false
	}
	existing, err := s.findByFbId(ctx, customerId, ownerOrgID)
	if err == nil && existing != nil {
		return existing.UnifiedId, false
	}
	now := time.Now().UnixMilli()
	filter := bson.M{"unifiedId": customerId, "ownerOrganizationId": ownerOrgID}
	setOnInsert := bson.M{
		"unifiedId":           customerId,
		"sourceIds":           crmmodels.CrmCustomerSourceIds{Fb: customerId},
		"primarySource":       "fb",
		"mergeMethod":         "single_source",
		"ownerOrganizationId": ownerOrgID,
		"createdAt":           now,
		"mergedAt":            now,
	}
	if custData != nil {
		if custData.Name != "" {
			setOnInsert["name"] = custData.Name
		}
		if len(custData.Phones) > 0 {
			setOnInsert["phoneNumbers"] = normalizePhones(custData.Phones)
		}
		if len(custData.Emails) > 0 {
			setOnInsert["emails"] = uniqueStrings(custData.Emails)
		}
		if custData.Birthday != "" {
			setOnInsert["birthday"] = custData.Birthday
		}
		if custData.Gender != "" {
			setOnInsert["gender"] = custData.Gender
		}
		if custData.LivesIn != "" {
			setOnInsert["livesIn"] = custData.LivesIn
		}
		if len(custData.Addresses) > 0 {
			setOnInsert["addresses"] = custData.Addresses
		}
		if custData.ReferralCode != "" {
			setOnInsert["referralCode"] = custData.ReferralCode
		}
	}
	update := bson.M{
		"$setOnInsert": setOnInsert,
		"$set":         bson.M{"updatedAt": now},
	}
	opts := mongoopts.Update().SetUpsert(true)
	result, err := s.Collection().UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return "", false
	}
	if result.UpsertedCount > 0 {
		displayName := "Khách từ hội thoại"
		if custData != nil && custData.Name != "" {
			displayName = custData.Name
		}
		s.logCustomerCreatedFromSource(ctx, customerId, ownerOrgID, "conversation", "Khởi tạo khách từ hội thoại - "+displayName, activityAt)
	}
	return customerId, result.UpsertedCount > 0
}

// MergeProfileFromOrder cập nhật profile crm_customer với thông tin từ order (fill gaps).
// Chỉ ghi khi field hiện tại đang trống và order có giá trị.
func (s *CrmCustomerService) MergeProfileFromOrder(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, orderDoc *pcmodels.PcPosOrder) {
	if orderDoc == nil || orderDoc.PosData == nil {
		return
	}
	custData := extractCustomerDataFromOrder(orderDoc)
	existing, err := s.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, nil)
	if err != nil {
		return
	}
	setFields := bson.M{}
	if existing.Name == "" && custData.Name != "" {
		setFields["name"] = custData.Name
	}
	if len(existing.PhoneNumbers) == 0 && len(custData.Phones) > 0 {
		setFields["phoneNumbers"] = normalizePhones(custData.Phones)
	}
	if len(existing.Emails) == 0 && len(custData.Emails) > 0 {
		setFields["emails"] = uniqueStrings(custData.Emails)
	}
	if existing.Birthday == "" && custData.Birthday != "" {
		setFields["birthday"] = custData.Birthday
	}
	if existing.Gender == "" && custData.Gender != "" {
		setFields["gender"] = custData.Gender
	}
	if existing.LivesIn == "" && custData.LivesIn != "" {
		setFields["livesIn"] = custData.LivesIn
	}
	if len(existing.Addresses) == 0 && len(custData.Addresses) > 0 {
		setFields["addresses"] = custData.Addresses
	}
	if existing.ReferralCode == "" && custData.ReferralCode != "" {
		setFields["referralCode"] = custData.ReferralCode
	}
	if len(setFields) == 0 {
		return
	}
	setFields["updatedAt"] = time.Now().UnixMilli()
	setFields["mergedAt"] = time.Now().UnixMilli()
	_, _ = s.Collection().UpdateOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, bson.M{"$set": setFields})
}

// MergeProfileFromConversation cập nhật profile crm_customer với thông tin từ conversation (fill gaps).
// Chỉ ghi khi field hiện tại đang trống và conversation có giá trị.
func (s *CrmCustomerService) MergeProfileFromConversation(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, convDoc *fbmodels.FbConversation) {
	if convDoc == nil || convDoc.PanCakeData == nil {
		return
	}
	custData := extractCustomerDataFromConv(convDoc)
	existing, err := s.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, nil)
	if err != nil {
		return
	}
	setFields := bson.M{}
	if existing.Name == "" && custData.Name != "" {
		setFields["name"] = custData.Name
	}
	if len(existing.PhoneNumbers) == 0 && len(custData.Phones) > 0 {
		setFields["phoneNumbers"] = normalizePhones(custData.Phones)
	}
	if len(existing.Emails) == 0 && len(custData.Emails) > 0 {
		setFields["emails"] = uniqueStrings(custData.Emails)
	}
	if existing.Birthday == "" && custData.Birthday != "" {
		setFields["birthday"] = custData.Birthday
	}
	if existing.Gender == "" && custData.Gender != "" {
		setFields["gender"] = custData.Gender
	}
	if existing.LivesIn == "" && custData.LivesIn != "" {
		setFields["livesIn"] = custData.LivesIn
	}
	if len(existing.Addresses) == 0 && len(custData.Addresses) > 0 {
		setFields["addresses"] = custData.Addresses
	}
	if len(setFields) == 0 {
		return
	}
	setFields["updatedAt"] = time.Now().UnixMilli()
	setFields["mergedAt"] = time.Now().UnixMilli()
	_, _ = s.Collection().UpdateOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, bson.M{"$set": setFields})
}

// logCustomerCreatedFromSource ghi activity customer_created với source tùy chỉnh (order, conversation).
// activityAt: thời điểm từ nguồn (0 = dùng now).
func (s *CrmCustomerService) logCustomerCreatedFromSource(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, source, displayLabel string, activityAt int64) {
	actSvc, err := NewCrmActivityService()
	if err != nil {
		return
	}
	if activityAt <= 0 {
		activityAt = time.Now().UnixMilli()
	}
	metadata := map[string]interface{}{"source": source}
	if cust, err := s.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, nil); err == nil {
		MergeSnapshotIntoMetadata(metadata, BuildSnapshotForNewCustomer(&cust, activityAt, true))
	}
	sourceRef := map[string]interface{}{"sourceCustomerId": unifiedId}
	_ = actSvc.LogActivity(ctx, LogActivityInput{
		UnifiedId:    unifiedId,
		OwnerOrgID:   ownerOrgID,
		Domain:       crmmodels.ActivityDomainCustomer,
		ActivityType: "customer_created",
		Source:       source,
		SourceRef:    sourceRef,
		Metadata:     metadata,
		DisplayLabel: displayLabel,
		DisplayIcon:  "person_add",
		ActivityAt:   activityAt,
	})
}

// logCustomerUpdatedIfThrottled ghi customer_updated khi merge cập nhật thông tin, tối đa 1 lần/customer/ngày.
func (s *CrmCustomerService) logCustomerUpdatedIfThrottled(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, source, displayLabel string) {
	actSvc, err := NewCrmActivityService()
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-24 * time.Hour).UnixMilli()
	activities, _ := actSvc.FindByUnifiedId(ctx, unifiedId, ownerOrgID, []string{crmmodels.ActivityDomainCustomer}, 10)
	for _, a := range activities {
		if a.ActivityType == "customer_updated" && a.ActivityAt >= cutoff {
			return
		}
	}
	metadata := map[string]interface{}{"mergeSource": source}
	activityAt := time.Now().UnixMilli()
	if cust, err := s.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, nil); err == nil {
		lastProfile, lastMetrics, _ := actSvc.GetLastSnapshotForCustomer(ctx, unifiedId, ownerOrgID, "", nil)
		metricsOverride := s.GetMetricsForSnapshotAt(ctx, &cust, activityAt)
		profileOverride := s.GetProfileForSnapshotAt(ctx, &cust, activityAt)
		if snap := BuildSnapshotWithChanges(&cust, lastProfile, lastMetrics, activityAt, metricsOverride, profileOverride); snap != nil {
			MergeSnapshotIntoMetadata(metadata, snap)
		}
	}
	sourceRef := map[string]interface{}{"trigger": "profile_update"}
	_ = actSvc.LogActivity(ctx, LogActivityInput{
		UnifiedId:    unifiedId,
		OwnerOrgID:   ownerOrgID,
		Domain:       crmmodels.ActivityDomainCustomer,
		ActivityType: "customer_updated",
		Source:       "system",
		SourceRef:    sourceRef,
		Metadata:     metadata,
		DisplayLabel: displayLabel,
		DisplayIcon:  "person",
		ActivityAt:   activityAt,
	})
}

// findByPosId tìm crm_customer theo sourceIds.pos hoặc unifiedId.
func (s *CrmCustomerService) findByPosId(ctx context.Context, posId string, ownerOrgID primitive.ObjectID) (*crmmodels.CrmCustomer, error) {
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$or": []bson.M{
			{"sourceIds.pos": posId},
			{"unifiedId": posId},
		},
	}
	c, err := s.FindOne(ctx, filter, nil)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// findByFbId tìm crm_customer theo sourceIds.fb hoặc unifiedId.
func (s *CrmCustomerService) findByFbId(ctx context.Context, fbId string, ownerOrgID primitive.ObjectID) (*crmmodels.CrmCustomer, error) {
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$or": []bson.M{
			{"sourceIds.fb": fbId},
			{"unifiedId": fbId},
		},
	}
	c, err := s.FindOne(ctx, filter, nil)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *CrmCustomerService) findFbCustomerByPagePsid(ctx context.Context, pageId, psid string, ownerOrgID primitive.ObjectID) string {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbCustomers)
	if !ok {
		return ""
	}
	var doc struct {
		CustomerId string `bson:"customerId"`
	}
	err := coll.FindOne(ctx, bson.M{
		"pageId": pageId, "psid": psid, "ownerOrganizationId": ownerOrgID,
	}).Decode(&doc)
	if err != nil {
		return ""
	}
	return doc.CustomerId
}

func (s *CrmCustomerService) findFbCustomerByPhone(ctx context.Context, normalizedPhone string, ownerOrgID primitive.ObjectID) string {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbCustomers)
	if !ok {
		return ""
	}
	// Tạo các biến thể phone để match (84xxx, 0xxx)
	variants := []string{normalizedPhone}
	if len(normalizedPhone) >= 3 && normalizedPhone[:2] == "84" {
		variants = append(variants, "0"+normalizedPhone[2:])
	}
	cursor, err := coll.Find(ctx, bson.M{"ownerOrganizationId": ownerOrgID, "phoneNumbers": bson.M{"$in": variants}}, nil)
	if err != nil {
		return ""
	}
	defer cursor.Close(ctx)
	if cursor.Next(ctx) {
		var doc struct {
			CustomerId string `bson:"customerId"`
		}
		if cursor.Decode(&doc) == nil {
			return doc.CustomerId
		}
	}
	return ""
}

func (s *CrmCustomerService) findPosCustomerByFbPagePsid(ctx context.Context, pageId, psid string, ownerOrgID primitive.ObjectID) string {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCustomers)
	if !ok {
		return ""
	}
	fbId := pageId + "_" + psid
	cursor, err := coll.Find(ctx, bson.M{"ownerOrganizationId": ownerOrgID, "posData.fb_id": fbId}, nil)
	if err != nil {
		return ""
	}
	defer cursor.Close(ctx)
	if cursor.Next(ctx) {
		var doc struct {
			CustomerId string `bson:"customerId"`
		}
		if cursor.Decode(&doc) == nil {
			return doc.CustomerId
		}
	}
	return ""
}

func (s *CrmCustomerService) findPosCustomerByPhone(ctx context.Context, normalizedPhone string, ownerOrgID primitive.ObjectID) string {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCustomers)
	if !ok {
		return ""
	}
	variants := []string{normalizedPhone}
	if len(normalizedPhone) >= 3 && normalizedPhone[:2] == "84" {
		variants = append(variants, "0"+normalizedPhone[2:])
	}
	cursor, err := coll.Find(ctx, bson.M{"ownerOrganizationId": ownerOrgID, "phoneNumbers": bson.M{"$in": variants}}, nil)
	if err != nil {
		return ""
	}
	defer cursor.Close(ctx)
	if cursor.Next(ctx) {
		var doc struct {
			CustomerId string `bson:"customerId"`
		}
		if cursor.Decode(&doc) == nil {
			return doc.CustomerId
		}
	}
	return ""
}

func normalizePhones(phones []string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, p := range phones {
		n := normalizePhone(p)
		if n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

var phoneRegex = regexp.MustCompile(`\D`)

func normalizePhone(s string) string {
	s = phoneRegex.ReplaceAllString(s, "")
	if len(s) >= 9 {
		if strings.HasPrefix(s, "0") {
			s = "84" + s[1:]
		} else if !strings.HasPrefix(s, "84") {
			s = "84" + s
		}
		return s
	}
	return ""
}

func uniqueStrings(arr []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range arr {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// parseTimeFromMap lấy timestamp (Unix ms) từ map — hỗ trợ panCakeData.inserted_at, created_at từ Pancake API.
// Ưu tiên format "2006-02-14T13:03:30" (không fractional, không timezone) theo sample-data.
func parseTimeFromMap(m map[string]interface{}, keys ...string) int64 {
	if m == nil {
		return 0
	}
	// Thử format đơn giản trước (theo sample: "2026-02-14T13:03:30"), rồi có fractional, RFC3339
	formats := []string{"2006-01-02T15:04:05", "2006-01-02T15:04:05.000000", "2006-01-02T15:04:05.000", "2006-01-02T15:04:05Z", time.RFC3339, time.RFC3339Nano}
	for _, key := range keys {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case string:
			for _, f := range formats {
				if t, err := time.Parse(f, x); err == nil {
					return t.UnixMilli()
				}
			}
		case primitive.DateTime:
			return x.Time().UnixMilli()
		case float64:
			if x > 0 {
				return int64(x)
			}
		case int64:
			if x > 0 {
				return x
			}
		case int:
			if x > 0 {
				return int64(x)
			}
		}
	}
	return 0
}

// getSourceCustomerTimestamp trả về thời gian sự kiện gốc từ sourceData (panCakeData/posData).
// BẮT BUỘC lấy từ inserted_at/created_at trong ...Data; không dùng doc.CreatedAt/UpdatedAt (thời gian đồng bộ).
func getSourceCustomerTimestamp(sourceData map[string]interface{}) int64 {
	// Ưu tiên inserted_at (ví dụ "2026-02-14T13:03:30" từ Pancake)
	if v := parseTimeFromMap(sourceData, "inserted_at", "created_at"); v > 0 {
		return v
	}
	if v := parseTimeFromMap(sourceData, "updated_at"); v > 0 {
		return v
	}
	return 0
}

func defaultName(name string) string {
	if name == "" {
		return "Khách mới"
	}
	return name
}

func getStringFromMap(m map[string]interface{}, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// getAddressesFromMap lấy mảng địa chỉ từ map, thử lần lượt các key.
func getAddressesFromMap(m map[string]interface{}, keys ...string) []interface{} {
	if m == nil {
		return nil
	}
	for _, key := range keys {
		v, ok := m[key]
		if !ok {
			continue
		}
		if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
			return arr
		}
	}
	return nil
}

func getStringArrayFromMap(m map[string]interface{}, key string) []string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch arr := v.(type) {
	case []interface{}:
		var out []string
		for _, x := range arr {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return arr
	}
	return nil
}
