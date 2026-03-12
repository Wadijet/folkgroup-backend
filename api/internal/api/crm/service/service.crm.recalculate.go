// Package crmvc - RecalculateCustomerFromAllSources: cập nhật toàn bộ thông tin khách hàng từ tất cả nguồn.
// Dùng khi cần chủ động refresh profile + metrics + classification (sync lỗi, data nguồn thay đổi).
package crmvc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	crmdto "meta_commerce/internal/api/crm/dto"
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

// RecalculateCustomerResult kết quả recalculate — trả về cho handler.
type RecalculateCustomerResult struct {
	UnifiedId             string `json:"unifiedId"`
	UpdatedAt             int64  `json:"updatedAt"`
	ProfileUpdated        bool   `json:"profileUpdated"`
	MetricsUpdated        bool   `json:"metricsUpdated"`
	ClassificationUpdated bool   `json:"classificationUpdated"`
	ActivitiesBackfilled  int    `json:"activitiesBackfilled"` // Số conversation_started đã ghi bổ sung
	OrdersBackfilled      int    `json:"ordersBackfilled"`     // Số order_created/order_completed đã ghi bổ sung
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

	// 3b. Lấy conversationId: từ pc_pos_customers.posData.fb_id (POS) + query fb_conversations match ids (FB).
	// Đảm bảo có conversationIds cho cả POS và FB — tránh aggregate bỏ sót do path BSON khác.
	conversationIds := s.getConversationIdsFromPosCustomers(ctx, []string{customer.SourceIds.Pos}, ownerOrgID)
	convIdsFromFb := s.getConversationIdsFromFbMatch(ctx, ids, ownerOrgID)
	for _, cid := range convIdsFromFb {
		if cid != "" {
			conversationIds = appendUnique(conversationIds, cid)
		}
	}

	// 4. Aggregate metrics (orders + conversations)
	metrics := s.aggregateOrderMetricsForCustomer(ctx, ids, ownerOrgID, phones, 0)
	convMetrics := s.aggregateConversationMetricsForCustomer(ctx, ids, conversationIds, ownerOrgID, 0)
	hasConv := convMetrics.ConversationCount > 0 || s.checkHasConversation(ctx, ids, ownerOrgID, conversationIds)

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
	class := ComputeClassificationFromMetrics(metrics.TotalSpent, metrics.OrderCount, metrics.LastOrderAt, metrics.RevenueLast30d, metrics.RevenueLast90d, metrics.OrderCountOnline, metrics.OrderCountOffline, hasConv, convMetrics.ConversationTags)

	// 7. Update crm_customers
	now := time.Now().UnixMilli()
	setFields := bson.M{
		"profile":            profile,
		"sourceIds":          sourceIds,
		"totalSpent":         metrics.TotalSpent,
		"orderCount":         metrics.OrderCount,
		"lastOrderAt":        metrics.LastOrderAt,
		"ownedSkuQuantities": metrics.OwnedSkuQuantities,
		"conversationTags":   convMetrics.ConversationTags,
		"mergedAt":           now,
		"updatedAt":          now,
		"currentMetrics":     cm,
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
	// Truyền cm (metrics vừa lưu) — đảm bảo snapshot = currentMetrics, tránh lệch do GetMetricsForSnapshotAt thiếu expandIds/checkHasConversation.
	s.logRecalculateActivity(ctx, unifiedId, ownerOrgID, now, cm)

	// 8. Backfill conversation_started và order activities thiếu — để lịch sử hiển thị đầy đủ
	activitiesBackfilled := s.backfillConversationActivitiesForCustomer(ctx, unifiedId, ids, conversationIds, ownerOrgID)
	ordersBackfilled := s.backfillOrderActivitiesForCustomer(ctx, ids, phones, ownerOrgID)

	return &RecalculateCustomerResult{
		UnifiedId:             unifiedId,
		UpdatedAt:             now,
		ProfileUpdated:        true,
		MetricsUpdated:        true,
		ClassificationUpdated: true,
		ActivitiesBackfilled:  activitiesBackfilled,
		OrdersBackfilled:      ordersBackfilled,
	}, nil
}

// RecalculateMismatchCustomers recalculate chỉ các khách bị lỗi: engaged trong crm nhưng visitor trong activity snapshot.
// Dùng để sửa chênh lệch currentMetrics vs metricsSnapshot trong hành trình khách hàng.
// limit <= 0: xử lý tất cả mismatch; limit > 0: giới hạn số khách.
// poolSize <= 0: dùng default 10; truyền worker.GetEffectivePoolSize(10, worker.PriorityLow) để giảm khi CPU/RAM cao.
func (s *CrmCustomerService) RecalculateMismatchCustomers(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, poolSize int) (*crmdto.CrmRecalculateAllResult, error) {
	actColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmActivityHistory)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.CrmActivityHistory)
	}

	nowMs := time.Now().UnixMilli()
	// 1. Lấy last metricsSnapshot per customer từ activity
	pipe := []bson.M{
		{"$match": bson.M{
			"ownerOrganizationId":      ownerOrgID,
			"activityAt":               bson.M{"$lte": nowMs},
			"metadata.metricsSnapshot": bson.M{"$exists": true, "$ne": nil},
		}},
		{"$sort": bson.M{"activityAt": -1}},
		{"$group": bson.M{
			"_id":             "$unifiedId",
			"metricsSnapshot": bson.M{"$first": "$metadata.metricsSnapshot"},
		}},
	}
	cursor, err := actColl.Aggregate(ctx, pipe)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	activitySnapshot := make(map[string]string)
	for cursor.Next(ctx) {
		var doc struct {
			ID              string                 `bson:"_id"`
			MetricsSnapshot map[string]interface{} `bson:"metricsSnapshot"`
		}
		if cursor.Decode(&doc) != nil || doc.MetricsSnapshot == nil {
			continue
		}
		stage := extractJourneyStageFromMetrics(doc.MetricsSnapshot)
		activitySnapshot[doc.ID] = stage
	}
	cursor.Close(ctx)

	// 2. Lấy engaged trong crm_customers
	engaged, err := s.Find(ctx, bson.M{
		"ownerOrganizationId": ownerOrgID,
		"journeyStage":        "engaged",
	}, mongoopts.Find().SetProjection(bson.M{"unifiedId": 1}))
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	// 3. Lọc mismatch: engaged crm nhưng visitor trong activity
	var mismatchIds []string
	for _, c := range engaged {
		snapStage := activitySnapshot[c.UnifiedId]
		if snapStage == "visitor" {
			mismatchIds = append(mismatchIds, c.UnifiedId)
		}
	}

	useLimit := limit > 0
	if useLimit && len(mismatchIds) > limit {
		mismatchIds = mismatchIds[:limit]
	}

	result := &crmdto.CrmRecalculateAllResult{}
	total := len(mismatchIds)
	if total == 0 {
		return result, nil
	}

	// Worker pool: xử lý song song để tăng tốc, mỗi customer độc lập (unifiedId khác nhau).
	const basePool = 10
	workers := basePool
	if poolSize > 0 {
		workers = poolSize
	}
	if total < workers {
		workers = total
	}

	jobs := make(chan string, total)
	for _, id := range mismatchIds {
		jobs <- id
	}
	close(jobs)

	var mu sync.Mutex
	const maxFailedIds = 10

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					// Bắt panic để không làm dừng toàn bộ tiến trình (vd: panic từ logger)
					// Dùng fmt thay vì logger để tránh panic lặp
					fmt.Fprintf(os.Stderr, "[CRM] RecalcMismatch: Worker panic recovered: %v\n", r)
				}
			}()
			for unifiedId := range jobs {
				_, err := s.RecalculateCustomerFromAllSources(ctx, unifiedId, ownerOrgID)
				mu.Lock()
				if err != nil {
					result.TotalFailed++
					if len(result.FailedIds) < maxFailedIds {
						result.FailedIds = append(result.FailedIds, unifiedId)
					}
				} else {
					result.TotalProcessed++
					pct := float64(result.TotalProcessed) * 100 / float64(total)
					logger.GetAppLogger().Infof("[CRM] RecalcMismatch: Fix thành công unifiedId=%s processed=%d total=%d progressPct=%.1f%%",
						unifiedId, result.TotalProcessed, total, pct)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	return result, nil
}

// RecalculateOrderCountMismatchCustomers recalculate tất cả khách first/repeat/promoter (đã mua).
// orderCount>0 chưa chắc đúng — recalc lại toàn bộ để đảm bảo metrics khớp DB.
// inactive bỏ khỏi journey — dùng lifecycleStage.
// limit <= 0: xử lý tất cả; limit > 0: giới hạn số khách.
// poolSize <= 0: dùng default 12; truyền worker.GetEffectivePoolSize(12, worker.PriorityLow) để giảm khi CPU/RAM cao.
func (s *CrmCustomerService) RecalculateOrderCountMismatchCustomers(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, poolSize int) (*crmdto.CrmRecalculateAllResult, error) {
	stages := []string{"first", "repeat", "promoter"}
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"journeyStage":        bson.M{"$in": stages},
	}
	opts := mongoopts.Find().SetProjection(bson.M{"unifiedId": 1})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}
	customers, err := s.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	ids := make([]string, 0, len(customers))
	for _, c := range customers {
		ids = append(ids, c.UnifiedId)
	}

	result := &crmdto.CrmRecalculateAllResult{}
	total := len(ids)
	if total == 0 {
		return result, nil
	}

	const basePool = 12
	workers := basePool
	if poolSize > 0 {
		workers = poolSize
	}
	if total < workers {
		workers = total
	}
	jobs := make(chan string, total)
	for _, id := range ids {
		jobs <- id
	}
	close(jobs)

	var mu sync.Mutex
	const maxFailedIds = 10
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "[CRM] RecalcOrderMismatch: Worker panic recovered: %v\n", r)
				}
			}()
			for unifiedId := range jobs {
				_, err := s.RecalculateCustomerFromAllSources(ctx, unifiedId, ownerOrgID)
				mu.Lock()
				if err != nil {
					result.TotalFailed++
					if len(result.FailedIds) < maxFailedIds {
						result.FailedIds = append(result.FailedIds, unifiedId)
					}
				} else {
					result.TotalProcessed++
					pct := float64(result.TotalProcessed) * 100 / float64(total)
					logger.GetAppLogger().Infof("[CRM] RecalcOrderMismatch: Fix unifiedId=%s processed=%d total=%d progressPct=%.1f%%",
						unifiedId, result.TotalProcessed, total, pct)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return result, nil
}

// getOrderCountFromCurrentMetrics đọc orderCount từ currentMetrics (raw/layer1/layer2).
func getOrderCountFromCurrentMetrics(cm map[string]interface{}) int {
	if cm == nil {
		return 0
	}
	for _, layer := range []string{"layer2", "layer1", "raw"} {
		if sub, ok := cm[layer].(map[string]interface{}); ok {
			if v, ok := sub["orderCount"]; ok && v != nil {
				switch x := v.(type) {
				case int:
					return x
				case int64:
					return int(x)
				case float64:
					return int(x)
				}
			}
		}
	}
	return 0
}

// extractJourneyStageFromMetrics lấy journeyStage từ metricsSnapshot (layer1, layer2, raw).
func extractJourneyStageFromMetrics(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	for _, layer := range []string{"layer1", "layer2", "raw"} {
		if sub, ok := m[layer].(map[string]interface{}); ok {
			if v, ok := sub["journeyStage"]; ok && v != nil {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// RecalculateAllCustomers tính toán lại tất cả khách hàng hiện có của org (ngược với backfill).
// Dùng worker pool xử lý song song để tăng tốc, mỗi customer độc lập (unifiedId khác nhau).
// limit <= 0: xử lý tất cả; limit > 0: giới hạn số khách.
// poolSize <= 0: dùng default 12; truyền worker.GetEffectivePoolSize(12, worker.PriorityLow) để giảm khi CPU/RAM cao.
func (s *CrmCustomerService) RecalculateAllCustomers(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, poolSize int) (*crmdto.CrmRecalculateAllResult, error) {
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
	ids := make([]string, 0, len(customers))
	for _, c := range customers {
		ids = append(ids, c.UnifiedId)
	}

	result := &crmdto.CrmRecalculateAllResult{}
	total := len(ids)
	if total == 0 {
		return result, nil
	}

	// Worker pool: xử lý song song để tăng tốc, mỗi customer độc lập (unifiedId khác nhau).
	const basePool = 12
	workers := basePool
	if poolSize > 0 {
		workers = poolSize
	}
	if total < workers {
		workers = total
	}
	jobs := make(chan string, total)
	for _, id := range ids {
		jobs <- id
	}
	close(jobs)

	var mu sync.Mutex
	const maxFailedIds = 10
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "[CRM] RecalcAll: Worker panic recovered: %v\n", r)
				}
			}()
			for unifiedId := range jobs {
				_, err := s.RecalculateCustomerFromAllSources(ctx, unifiedId, ownerOrgID)
				mu.Lock()
				if err != nil {
					result.TotalFailed++
					if len(result.FailedIds) < maxFailedIds {
						result.FailedIds = append(result.FailedIds, unifiedId)
					}
				} else {
					result.TotalProcessed++
					pct := float64(result.TotalProcessed) * 100 / float64(total)
					logger.GetAppLogger().Infof("[CRM] RecalcAll: unifiedId=%s processed=%d total=%d progressPct=%.1f%%",
						unifiedId, result.TotalProcessed, total, pct)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
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
	convIds := s.getConversationIdsFromPosCustomers(ctx, []string{c.SourceIds.Pos}, c.OwnerOrganizationID)
	if convDoc := s.fetchLatestConversationForCustomer(ctx, buildCustomerIdsForRecalculate(c), convIds, c.OwnerOrganizationID); convDoc != nil {
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

// getConversationIdsFromFbMatch query fb_conversations theo customerIds, trả về conversationId của các conv match.
// Dùng cho FB customers — đảm bảo có conversationIds trong filter (fallback khi path BSON khác).
func (s *CrmCustomerService) getConversationIdsFromFbMatch(ctx context.Context, customerIds []string, ownerOrgID primitive.ObjectID) []string {
	if len(customerIds) == 0 {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok {
		return nil
	}
	filter := buildConversationFilterForCustomerIds(customerIds, ownerOrgID, nil)
	if filter["customerId"] == "__NO_MATCH__" {
		return nil
	}
	cursor, err := coll.Find(ctx, filter, mongoopts.Find().SetProjection(bson.M{"conversationId": 1}).SetLimit(50))
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)
	var result []string
	seen := make(map[string]bool)
	for cursor.Next(ctx) {
		var doc struct {
			ConversationId string `bson:"conversationId"`
		}
		if cursor.Decode(&doc) != nil || doc.ConversationId == "" {
			continue
		}
		if !seen[doc.ConversationId] {
			seen[doc.ConversationId] = true
			result = append(result, doc.ConversationId)
		}
	}
	return result
}

func appendUnique(slice []string, item string) []string {
	for _, v := range slice {
		if v == item {
			return slice
		}
	}
	return append(slice, item)
}

// getConversationIdsFromPosCustomers lấy conversationId (pageId_psid) từ pc_pos_customers.posData.fb_id.
// Dùng khi aggregate conversation — conv có customerId khác (Pancake format) nhưng posData.fb_id = conversationId.
func (s *CrmCustomerService) getConversationIdsFromPosCustomers(ctx context.Context, posCustomerIds []string, ownerOrgID primitive.ObjectID) []string {
	if len(posCustomerIds) == 0 {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCustomers)
	if !ok {
		return nil
	}
	cursor, err := coll.Find(ctx, bson.M{
		"ownerOrganizationId": ownerOrgID,
		"customerId":          bson.M{"$in": posCustomerIds},
		"posData.fb_id":       bson.M{"$exists": true, "$ne": ""},
	}, nil)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)
	var result []string
	seen := make(map[string]bool)
	for cursor.Next(ctx) {
		var doc struct {
			PosData map[string]interface{} `bson:"posData"`
		}
		if cursor.Decode(&doc) != nil || doc.PosData == nil {
			continue
		}
		fbId := ""
		if v, ok := doc.PosData["fb_id"].(string); ok && v != "" {
			fbId = v
		} else if n, ok := doc.PosData["fb_id"].(float64); ok {
			fbId = fmt.Sprintf("%.0f", n)
		}
		if fbId != "" && !seen[fbId] {
			seen[fbId] = true
			result = append(result, fbId)
		}
	}
	return result
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
func (s *CrmCustomerService) fetchLatestConversationForCustomer(ctx context.Context, customerIds []string, conversationIds []string, ownerOrgID primitive.ObjectID) *fbmodels.FbConversation {
	if len(customerIds) == 0 && len(conversationIds) == 0 {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok {
		return nil
	}
	filter := buildConversationFilterForCustomerIds(customerIds, ownerOrgID, conversationIds)
	opts := mongoopts.FindOne().SetSort(bson.D{{Key: "panCakeUpdatedAt", Value: -1}})
	var doc fbmodels.FbConversation
	if err := coll.FindOne(ctx, filter, opts).Decode(&doc); err != nil {
		return nil
	}
	return &doc
}

const (
	recalculateConversationActivityLimit = 100 // Giới hạn số conversation backfill mỗi lần recalculate
	recalculateOrderActivityLimit        = 100 // Giới hạn số order backfill mỗi lần recalculate
)

// backfillConversationActivitiesForCustomer ghi conversation_started cho các hội thoại chưa có activity.
// conversationIds: từ pc_pos_customers.posData.fb_id — link POS customer với conv.
func (s *CrmCustomerService) backfillConversationActivitiesForCustomer(ctx context.Context, unifiedId string, customerIds []string, conversationIds []string, ownerOrgID primitive.ObjectID) int {
	if unifiedId == "" || (len(customerIds) == 0 && len(conversationIds) == 0) {
		return 0
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok {
		return 0
	}
	filter := buildConversationFilterForCustomerIds(customerIds, ownerOrgID, conversationIds)
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

// backfillOrderActivitiesForCustomer ghi order_created/order_completed cho các đơn hàng chưa có activity.
// Filter đồng nhất với aggregateOrderMetricsForCustomer: customerId, posData.customer.id, posData.customer_id, billPhoneNumber.
func (s *CrmCustomerService) backfillOrderActivitiesForCustomer(ctx context.Context, customerIds []string, phoneNumbers []string, ownerOrgID primitive.ObjectID) int {
	var ids []string
	for _, id := range customerIds {
		if id != "" {
			ids = append(ids, id)
		}
	}
	var phoneVariants []string
	for _, p := range phoneNumbers {
		if p != "" {
			phoneVariants = append(phoneVariants, p)
			if len(p) >= 3 && p[:2] == "84" {
				phoneVariants = append(phoneVariants, "0"+p[2:])
			} else if len(p) >= 10 && p[0] == '0' {
				phoneVariants = append(phoneVariants, "84"+p[1:])
			}
		}
	}
	if len(ids) == 0 && len(phoneVariants) == 0 {
		return 0
	}

	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return 0
	}

	var orConditions []bson.M
	if len(ids) > 0 {
		orConditions = append(orConditions,
			bson.M{"customerId": bson.M{"$in": ids}},
			bson.M{"posData.customer.id": bson.M{"$in": ids}},
			bson.M{"posData.customer_id": bson.M{"$in": ids}},
		)
	}
	if len(phoneVariants) > 0 {
		orConditions = append(orConditions,
			bson.M{"billPhoneNumber": bson.M{"$in": phoneVariants}},
			bson.M{"posData.bill_phone_number": bson.M{"$in": phoneVariants}},
		)
	}
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$and": []bson.M{
			{"$or": orConditions},
			{"status": bson.M{"$nin": []int{6}}},
			{"posData.status": bson.M{"$nin": []int{6}}},
		},
	}
	opts := mongoopts.Find().
		SetSort(bson.D{{Key: "posData.inserted_at", Value: 1}, {Key: "posData.updated_at", Value: 1}, {Key: "_id", Value: 1}}).
		SetLimit(recalculateOrderActivityLimit)
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)

	backfilled := 0
	for cursor.Next(ctx) {
		var doc pcmodels.PcPosOrder
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		customerId := doc.CustomerId
		if customerId == "" && doc.PosData != nil {
			if m, ok := doc.PosData["customer"].(map[string]interface{}); ok {
				if id, ok := m["id"].(string); ok {
					customerId = id
				}
			}
		}
		if customerId == "" && doc.PosData != nil {
			if id, ok := doc.PosData["customer_id"].(string); ok {
				customerId = id
			}
		}
		if customerId == "" {
			continue
		}
		channel := "offline"
		if doc.PageId != "" {
			channel = "online"
		} else if doc.PosData != nil {
			if pid, ok := doc.PosData["page_id"].(string); ok && pid != "" {
				channel = "online"
			}
		}
		_ = s.IngestOrderTouchpoint(ctx, customerId, ownerOrgID, doc.OrderId, true, channel, true, &doc)
		backfilled++
	}
	return backfilled
}

func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

// logRecalculateActivity ghi activity customer_updated với metricsSnapshot mới sau recalculate.
// Giúp report (ComputeCustomerReport) lấy được snapshot có layer3 qua GetLastSnapshotPerCustomerBeforeEndMs.
// metricsOverride: metrics vừa cập nhật vào crm_customers — dùng trực tiếp để đảm bảo snapshot = currentMetrics.
// Tránh lệch visitor/engaged do GetMetricsForSnapshotAt thiếu expandCustomerIdsForAggregation và checkHasConversation.
func (s *CrmCustomerService) logRecalculateActivity(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID, activityAt int64, metricsOverride map[string]interface{}) {
	actSvc, err := NewCrmActivityService()
	if err != nil {
		return
	}
	cust, err := s.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}, nil)
	if err != nil {
		return
	}
	metadata := map[string]interface{}{"trigger": "recalculate"}
	// Dùng metricsOverride (chính xác metrics vừa lưu) thay vì GetMetricsForSnapshotAt — tránh chênh lệch.
	snap := BuildSnapshotForNewCustomer(&cust, activityAt, false, metricsOverride)
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
