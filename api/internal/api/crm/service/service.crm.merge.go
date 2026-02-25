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

	metrics := s.aggregateOrderMetricsForCustomer(ctx, []string{customerId, sourceIds.Fb, unifiedId}, ownerOrgID, phones)
	hasConv := s.checkHasConversation(ctx, []string{sourceIds.Fb, customerId, unifiedId}, ownerOrgID)

	filter := bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}
	update := bson.M{
		"$set": bson.M{
			"sourceIds": sourceIds, "primarySource": primarySource, "name": name,
			"phoneNumbers": phones, "emails": emails,
			"hasConversation": hasConv, "hasOrder": metrics.OrderCount > 0,
			"orderCountOnline": metrics.OrderCountOnline, "orderCountOffline": metrics.OrderCountOffline,
			"firstOrderChannel": metrics.FirstOrderChannel, "lastOrderChannel": metrics.LastOrderChannel,
			"isOmnichannel": metrics.OrderCountOnline > 0 && metrics.OrderCountOffline > 0,
			"totalSpent": metrics.TotalSpent, "orderCount": metrics.OrderCount,
			"lastOrderAt": metrics.LastOrderAt, "secondLastOrderAt": metrics.SecondLastOrderAt,
			"revenueLast30d": metrics.RevenueLast30d, "revenueLast90d": metrics.RevenueLast90d,
			"mergeMethod": mergeMethod, "mergedAt": now, "updatedAt": now,
		},
		"$setOnInsert": bson.M{"createdAt": now},
	}
	opts := mongoopts.Update().SetUpsert(true)
	_, err := s.Collection().UpdateOne(ctx, filter, update, opts)
	return common.ConvertMongoError(err)
}

// MergeFromFbCustomer xử lý khi fb_customers thay đổi.
func (s *CrmCustomerService) MergeFromFbCustomer(ctx context.Context, doc *fbmodels.FbCustomer) error {
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
		emails = nil
	}

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

	metrics := s.aggregateOrderMetricsForCustomer(ctx, []string{sourceIds.Pos, fbCustomerId, unifiedId}, ownerOrgID, phones)
	hasConv := s.checkHasConversation(ctx, []string{sourceIds.Pos, fbCustomerId, unifiedId}, ownerOrgID)

	filter := bson.M{"unifiedId": unifiedId, "ownerOrganizationId": ownerOrgID}
	update := bson.M{
		"$set": bson.M{
			"sourceIds": sourceIds, "primarySource": primarySource, "name": name,
			"phoneNumbers": phones, "emails": emails,
			"hasConversation": hasConv, "hasOrder": metrics.OrderCount > 0,
			"orderCountOnline": metrics.OrderCountOnline, "orderCountOffline": metrics.OrderCountOffline,
			"firstOrderChannel": metrics.FirstOrderChannel, "lastOrderChannel": metrics.LastOrderChannel,
			"isOmnichannel": metrics.OrderCountOnline > 0 && metrics.OrderCountOffline > 0,
			"totalSpent": metrics.TotalSpent, "orderCount": metrics.OrderCount,
			"lastOrderAt": metrics.LastOrderAt, "secondLastOrderAt": metrics.SecondLastOrderAt,
			"revenueLast30d": metrics.RevenueLast30d, "revenueLast90d": metrics.RevenueLast90d,
			"mergeMethod": mergeMethod, "mergedAt": now, "updatedAt": now,
		},
		"$setOnInsert": bson.M{"createdAt": now},
	}
	opts := mongoopts.Update().SetUpsert(true)
	_, err := s.Collection().UpdateOne(ctx, filter, update, opts)
	return common.ConvertMongoError(err)
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
