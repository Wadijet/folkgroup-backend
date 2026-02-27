// Package crmvc - Snapshot profile và metrics cho activity history.
// Chỉ lưu khi profile hoặc metrics thay đổi. Snapshot chứa profileSnapshot, metricsSnapshot và snapshotChanges (cái gì thay đổi, cũ → mới).
package crmvc

import (
	"reflect"
	"time"

	crmmodels "meta_commerce/internal/api/crm/models"
	"meta_commerce/internal/api/report/layer3"
)

const (
	maxOwnedSkusInSnapshot = 50
)

// snapshotChange mô tả một thay đổi: field, oldValue, newValue.
type snapshotChange struct {
	Field    string      `json:"field" bson:"field"`
	OldValue interface{} `json:"oldValue,omitempty" bson:"oldValue,omitempty"`
	NewValue interface{} `json:"newValue,omitempty" bson:"newValue,omitempty"`
}

// BuildSnapshotWithChanges tạo snapshot chỉ khi có thay đổi so với lastSnapshot.
// metricsOverride: khi != nil dùng thay cho buildMetricsSnapshot(c) — cho snapshot đúng timeline (metrics as of activityAt).
// profileOverride: khi != nil dùng thay cho buildProfileSnapshot(c) — cho snapshot đúng timeline (profile as of activityAt).
func BuildSnapshotWithChanges(c *crmmodels.CrmCustomer, lastProfile, lastMetrics map[string]interface{}, snapshotAt int64, metricsOverride, profileOverride map[string]interface{}) map[string]interface{} {
	if c == nil {
		return nil
	}
	var profile map[string]interface{}
	if profileOverride != nil {
		profile = profileOverride
	} else {
		profile = buildProfileSnapshot(c)
	}
	var metrics map[string]interface{}
	if metricsOverride != nil {
		metrics = metricsOverride
	} else {
		metrics = buildMetricsSnapshot(c)
	}
	changes := diffSnapshot(profile, metrics, lastProfile, lastMetrics)
	if len(changes) == 0 {
		return nil
	}
	if snapshotAt <= 0 {
		snapshotAt = time.Now().UnixMilli()
	}
	out := map[string]interface{}{
		"profileSnapshot": profile,
		"metricsSnapshot": metrics,
		"snapshotChanges": changes,
		"snapshotAt":      snapshotAt,
	}
	return out
}

// BuildSnapshotForNewCustomer snapshot cho khách mới (customer_created).
// useEmptyMetrics: true = metrics = 0 — đúng timeline (metrics tăng dần theo mỗi order/chat); false = metrics từ customer hiện tại.
// metricsOverride: khi != nil dùng thay cho buildEmpty/buildMetrics — cho metrics as-of activityAt (timeline đúng, có Lớp 3).
func BuildSnapshotForNewCustomer(c *crmmodels.CrmCustomer, snapshotAt int64, useEmptyMetrics bool, metricsOverride map[string]interface{}) map[string]interface{} {
	if c == nil {
		return nil
	}
	profile := buildProfileSnapshot(c)
	var metrics map[string]interface{}
	if metricsOverride != nil {
		metrics = metricsOverride
	} else if useEmptyMetrics {
		metrics = buildEmptyMetricsSnapshot()
	} else {
		metrics = buildMetricsSnapshot(c)
	}
	changes := buildChangesForNewCustomer(profile, metrics)
	if snapshotAt <= 0 {
		snapshotAt = time.Now().UnixMilli()
	}
	out := map[string]interface{}{
		"profileSnapshot": profile,
		"metricsSnapshot": metrics,
		"snapshotChanges": changes,
		"snapshotAt":      snapshotAt,
	}
	return out
}

func buildProfileSnapshot(c *crmmodels.CrmCustomer) map[string]interface{} {
	p := map[string]interface{}{
		"name":          GetNameFromCustomer(c),
		"phoneNumbers":  GetPhoneNumbersFromCustomer(c),
		"emails":        GetEmailsFromCustomer(c),
		"birthday":      GetBirthdayFromCustomer(c),
		"gender":        GetGenderFromCustomer(c),
		"livesIn":       GetLivesInFromCustomer(c),
		"referralCode":  GetReferralCodeFromCustomer(c),
		"primarySource": c.PrimarySource,
	}
	if len(GetAddressesFromCustomer(c)) > 0 {
		p["addresses"] = GetAddressesFromCustomer(c)
	}
	return p
}

// buildMetricsSnapshot trả về metricsSnapshot cấu trúc 3 lớp (raw, layer1, layer2, layer3).
// Nếu customer có currentMetrics (nested) → trả về (đảm bảo có layer3 khi thiếu).
// Nếu không (temp customer từ BuildCurrentMetricsFromOrderAndConv) → build từ top-level.
func buildMetricsSnapshot(c *crmmodels.CrmCustomer) map[string]interface{} {
	if c != nil && c.CurrentMetrics != nil {
		if _, hasRaw := c.CurrentMetrics["raw"]; hasRaw {
			return ensureLayer3InMetrics(c.CurrentMetrics)
		}
	}
	// Build từ top-level (temp customer từ order+conv aggregate)
	totalSpent := GetTotalSpentFromCustomer(c)
	orderCount := GetOrderCountFromCustomer(c)
	lastOrderAt := GetLastOrderAtFromCustomer(c)
	journeyStage := ComputeJourneyStage(c)
	valueTier := ComputeValueTier(totalSpent)
	lifecycleStage := ComputeLifecycleStage(lastOrderAt)
	channel := ComputeChannel(c)
	loyaltyStage := ComputeLoyaltyStage(orderCount)
	momentumStage := ComputeMomentumStage(c)

	avgOrderValue := 0.0
	if orderCount > 0 {
		avgOrderValue = totalSpent / float64(orderCount)
	}
	raw := map[string]interface{}{
		"totalSpent":                totalSpent,
		"orderCount":                orderCount,
		"avgOrderValue":             avgOrderValue,
		"lastOrderAt":               lastOrderAt,
		"secondLastOrderAt":         GetInt64FromCustomer(c, "secondLastOrderAt"),
		"revenueLast30d":            GetFloatFromCustomer(c, "revenueLast30d"),
		"revenueLast90d":            GetFloatFromCustomer(c, "revenueLast90d"),
		"cancelledOrderCount":       GetIntFromCustomer(c, "cancelledOrderCount"),
		"ordersLast30d":             GetIntFromCustomer(c, "ordersLast30d"),
		"ordersLast90d":             GetIntFromCustomer(c, "ordersLast90d"),
		"ordersFromAds":             GetIntFromCustomer(c, "ordersFromAds"),
		"ordersFromOrganic":         GetIntFromCustomer(c, "ordersFromOrganic"),
		"ordersFromDirect":          GetIntFromCustomer(c, "ordersFromDirect"),
		"orderCountOnline":          GetIntFromCustomer(c, "orderCountOnline"),
		"orderCountOffline":         GetIntFromCustomer(c, "orderCountOffline"),
		"firstOrderChannel":         getStrFromCustomer(c, "firstOrderChannel"),
		"lastOrderChannel":          getStrFromCustomer(c, "lastOrderChannel"),
		"isOmnichannel":             GetIntFromCustomer(c, "orderCountOnline") > 0 && GetIntFromCustomer(c, "orderCountOffline") > 0,
		"hasConversation":           GetBoolFromCustomer(c, "hasConversation"),
		"hasOrder":                  GetBoolFromCustomer(c, "hasOrder"),
		"conversationCount":         GetIntFromCustomer(c, "conversationCount"),
		"conversationCountByInbox":  GetIntFromCustomer(c, "conversationCountByInbox"),
		"conversationCountByComment": GetIntFromCustomer(c, "conversationCountByComment"),
		"lastConversationAt":        GetInt64FromCustomer(c, "lastConversationAt"),
		"firstConversationAt":       GetInt64FromCustomer(c, "firstConversationAt"),
		"totalMessages":             GetIntFromCustomer(c, "totalMessages"),
		"lastMessageFromCustomer":   GetBoolFromCustomer(c, "lastMessageFromCustomer"),
		"conversationFromAds":       GetBoolFromCustomer(c, "conversationFromAds"),
	}
	if len(c.ConversationTags) > 0 {
		raw["conversationTags"] = c.ConversationTags
	}
	if len(c.OwnedSkuQuantities) > 0 {
		raw["ownedSkuQuantities"] = truncateOwnedSkuQuantities(c.OwnedSkuQuantities, maxOwnedSkusInSnapshot)
	}

	flatForDerive := make(map[string]interface{})
	for k, v := range raw {
		flatForDerive[k] = v
	}
	flatForDerive["journeyStage"] = journeyStage
	flatForDerive["valueTier"] = valueTier
	flatForDerive["lifecycleStage"] = lifecycleStage
	flatForDerive["channel"] = channel
	flatForDerive["loyaltyStage"] = loyaltyStage
	flatForDerive["momentumStage"] = momentumStage

	layer1 := map[string]interface{}{"journeyStage": journeyStage, "orderCount": orderCount}
	layer2 := map[string]interface{}{
		"valueTier":      valueTier,
		"lifecycleStage": lifecycleStage,
		"channel":        channel,
		"loyaltyStage":   loyaltyStage,
		"momentumStage":  momentumStage,
	}

	agg := layer3.DeriveFromMap(flatForDerive, layer3.NowMs())
	layer3Map := layer3.ToMapForStorage(agg)
	layer3Obj := make(map[string]interface{})
	if layer3Map != nil {
		if v := layer3Map["firstLayer3"]; v != nil {
			layer3Obj["first"] = v
		}
		if v := layer3Map["repeatLayer3"]; v != nil {
			layer3Obj["repeat"] = v
		}
		if v := layer3Map["vipLayer3"]; v != nil {
			layer3Obj["vip"] = v
		}
		if v := layer3Map["inactiveLayer3"]; v != nil {
			layer3Obj["inactive"] = v
		}
		if v := layer3Map["engagedLayer3"]; v != nil {
			layer3Obj["engaged"] = v
		}
	}

	return map[string]interface{}{
		"raw":    raw,
		"layer1": layer1,
		"layer2": layer2,
		"layer3": layer3Obj,
	}
}

// ensureLayer3InMetrics đảm bảo metrics có layer3 đầy đủ (gồm Engaged khi thiếu).
// Derive từ raw+layer1+layer2 rồi merge vào. Trả về map mới để không mutate input.
func ensureLayer3InMetrics(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	agg := layer3.DeriveFromNested(m, layer3.NowMs())
	layer3Map := layer3.ToMapForStorage(agg)
	if layer3Map == nil {
		return m
	}
	// Merge layer3 vào bản sao (format: first/repeat/vip/inactive từ firstLayer3/repeatLayer3/...)
	out := make(map[string]interface{}, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	layer3Obj := make(map[string]interface{})
	if v := layer3Map["firstLayer3"]; v != nil {
		layer3Obj["first"] = v
	}
	if v := layer3Map["repeatLayer3"]; v != nil {
		layer3Obj["repeat"] = v
	}
	if v := layer3Map["vipLayer3"]; v != nil {
		layer3Obj["vip"] = v
	}
	if v := layer3Map["inactiveLayer3"]; v != nil {
		layer3Obj["inactive"] = v
	}
	if v := layer3Map["engagedLayer3"]; v != nil {
		layer3Obj["engaged"] = v
	}
	if len(layer3Obj) > 0 {
		out["layer3"] = layer3Obj
	}
	return out
}

// GetNameFromCustomer đọc name — ưu tiên Profile, fallback Legacy.
func GetNameFromCustomer(c *crmmodels.CrmCustomer) string {
	if c == nil {
		return ""
	}
	if c.Profile.Name != "" {
		return c.Profile.Name
	}
	return c.LegacyName
}

// GetPhoneNumbersFromCustomer đọc phoneNumbers — ưu tiên Profile, fallback Legacy.
func GetPhoneNumbersFromCustomer(c *crmmodels.CrmCustomer) []string {
	if c == nil {
		return nil
	}
	if len(c.Profile.PhoneNumbers) > 0 {
		return c.Profile.PhoneNumbers
	}
	return c.LegacyPhoneNumbers
}

// GetEmailsFromCustomer đọc emails — ưu tiên Profile, fallback Legacy.
func GetEmailsFromCustomer(c *crmmodels.CrmCustomer) []string {
	if c == nil {
		return nil
	}
	if len(c.Profile.Emails) > 0 {
		return c.Profile.Emails
	}
	return c.LegacyEmails
}

// GetBirthdayFromCustomer đọc birthday — ưu tiên Profile, fallback Legacy.
func GetBirthdayFromCustomer(c *crmmodels.CrmCustomer) string {
	if c == nil {
		return ""
	}
	if c.Profile.Birthday != "" {
		return c.Profile.Birthday
	}
	return c.LegacyBirthday
}

// GetGenderFromCustomer đọc gender — ưu tiên Profile, fallback Legacy.
func GetGenderFromCustomer(c *crmmodels.CrmCustomer) string {
	if c == nil {
		return ""
	}
	if c.Profile.Gender != "" {
		return c.Profile.Gender
	}
	return c.LegacyGender
}

// GetLivesInFromCustomer đọc livesIn — ưu tiên Profile, fallback Legacy.
func GetLivesInFromCustomer(c *crmmodels.CrmCustomer) string {
	if c == nil {
		return ""
	}
	if c.Profile.LivesIn != "" {
		return c.Profile.LivesIn
	}
	return c.LegacyLivesIn
}

// GetAddressesFromCustomer đọc addresses — ưu tiên Profile, fallback Legacy.
func GetAddressesFromCustomer(c *crmmodels.CrmCustomer) []interface{} {
	if c == nil {
		return nil
	}
	if len(c.Profile.Addresses) > 0 {
		return c.Profile.Addresses
	}
	return c.LegacyAddresses
}

// GetReferralCodeFromCustomer đọc referralCode — ưu tiên Profile, fallback Legacy.
func GetReferralCodeFromCustomer(c *crmmodels.CrmCustomer) string {
	if c == nil {
		return ""
	}
	if c.Profile.ReferralCode != "" {
		return c.Profile.ReferralCode
	}
	return c.LegacyReferralCode
}

// GetTotalSpentFromCustomer đọc totalSpent từ customer — ưu tiên currentMetrics, fallback top-level (backward compat).
func GetTotalSpentFromCustomer(c *crmmodels.CrmCustomer) float64 {
	if c != nil && c.CurrentMetrics != nil {
		return GetFloatFromNestedMetrics(c.CurrentMetrics, "totalSpent")
	}
	if c != nil {
		return c.TotalSpent
	}
	return 0
}

// GetOrderCountFromCustomer đọc orderCount từ customer.
func GetOrderCountFromCustomer(c *crmmodels.CrmCustomer) int {
	if c != nil && c.CurrentMetrics != nil {
		return GetIntFromNestedMetrics(c.CurrentMetrics, "orderCount")
	}
	if c != nil {
		return c.OrderCount
	}
	return 0
}

// GetLastOrderAtFromCustomer đọc lastOrderAt từ customer.
func GetLastOrderAtFromCustomer(c *crmmodels.CrmCustomer) int64 {
	if c != nil && c.CurrentMetrics != nil {
		return GetInt64FromNestedMetrics(c.CurrentMetrics, "lastOrderAt")
	}
	if c != nil {
		return c.LastOrderAt
	}
	return 0
}

// GetInt64FromCustomer đọc int64 từ customer (secondLastOrderAt, lastConversationAt, ...).
func GetInt64FromCustomer(c *crmmodels.CrmCustomer, key string) int64 {
	if c != nil && c.CurrentMetrics != nil {
		return GetInt64FromNestedMetrics(c.CurrentMetrics, key)
	}
	switch key {
	case "secondLastOrderAt":
		if c != nil {
			return c.SecondLastOrderAt
		}
	case "lastConversationAt":
		if c != nil {
			return c.LastConversationAt
		}
	case "firstConversationAt":
		if c != nil {
			return c.FirstConversationAt
		}
	}
	return 0
}

// GetIntFromCustomer đọc int từ customer (cancelledOrderCount, ordersLast30d, ...).
func GetIntFromCustomer(c *crmmodels.CrmCustomer, key string) int {
	if c != nil && c.CurrentMetrics != nil {
		return GetIntFromNestedMetrics(c.CurrentMetrics, key)
	}
	switch key {
	case "cancelledOrderCount":
		if c != nil {
			return c.CancelledOrderCount
		}
	case "ordersLast30d":
		if c != nil {
			return c.OrdersLast30d
		}
	case "orderCountOnline":
		if c != nil {
			return c.OrderCountOnline
		}
	case "orderCountOffline":
		if c != nil {
			return c.OrderCountOffline
		}
	case "ordersLast90d":
		if c != nil {
			return c.OrdersLast90d
		}
	case "ordersFromAds", "ordersFromOrganic", "ordersFromDirect":
		if c != nil {
			switch key {
			case "ordersFromAds":
				return c.OrdersFromAds
			case "ordersFromOrganic":
				return c.OrdersFromOrganic
			case "ordersFromDirect":
				return c.OrdersFromDirect
			}
		}
	case "conversationCount", "conversationCountByInbox", "conversationCountByComment", "totalMessages":
		if c != nil {
			switch key {
			case "conversationCount":
				return c.ConversationCount
			case "conversationCountByInbox":
				return c.ConversationCountByInbox
			case "conversationCountByComment":
				return c.ConversationCountByComment
			case "totalMessages":
				return c.TotalMessages
			}
		}
	}
	return 0
}

// GetFloatFromCustomer đọc float64 từ customer (revenueLast30d, revenueLast90d, avgOrderValue).
func GetFloatFromCustomer(c *crmmodels.CrmCustomer, key string) float64 {
	if c != nil && c.CurrentMetrics != nil {
		return GetFloatFromNestedMetrics(c.CurrentMetrics, key)
	}
	if c == nil {
		return 0
	}
	switch key {
	case "revenueLast30d":
		return c.RevenueLast30d
	case "revenueLast90d":
		return c.RevenueLast90d
	case "avgOrderValue":
		return c.AvgOrderValue
	}
	return 0
}

// getStrFromCustomer đọc string từ customer (firstOrderChannel, lastOrderChannel) — dùng nội bộ.
func getStrFromCustomer(c *crmmodels.CrmCustomer, key string) string {
	if c != nil && c.CurrentMetrics != nil {
		return GetStrFromNestedMetrics(c.CurrentMetrics, key)
	}
	if c == nil {
		return ""
	}
	switch key {
	case "firstOrderChannel":
		return c.FirstOrderChannel
	case "lastOrderChannel":
		return c.LastOrderChannel
	}
	return ""
}

// GetBoolFromCustomer đọc bool từ customer (hasConversation, hasOrder).
func GetBoolFromCustomer(c *crmmodels.CrmCustomer, key string) bool {
	if c != nil && c.CurrentMetrics != nil {
		v := getFromNestedMetrics(c.CurrentMetrics, key)
		if b, ok := v.(bool); ok {
			return b
		}
		return false
	}
	if c == nil {
		return false
	}
	switch key {
	case "hasConversation":
		return c.HasConversation
	case "hasOrder":
		return c.HasOrder
	case "lastMessageFromCustomer":
		return c.LastMessageFromCustomer
	case "conversationFromAds":
		return c.ConversationFromAds
	}
	return false
}

// GetStrFromNestedMetrics đọc string từ metricsSnapshot nested.
func GetStrFromNestedMetrics(m map[string]interface{}, key string) string {
	v := getFromNestedMetrics(m, key)
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// GetIntFromNestedMetrics đọc int từ metricsSnapshot nested.
func GetIntFromNestedMetrics(m map[string]interface{}, key string) int {
	v := getFromNestedMetrics(m, key)
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case int32:
		return int(x)
	}
	return 0
}

// GetInt64FromNestedMetrics đọc int64 từ metricsSnapshot nested.
func GetInt64FromNestedMetrics(m map[string]interface{}, key string) int64 {
	v := getFromNestedMetrics(m, key)
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}

// GetFloatFromNestedMetrics đọc float64 từ metricsSnapshot nested.
func GetFloatFromNestedMetrics(m map[string]interface{}, key string) float64 {
	v := getFromNestedMetrics(m, key)
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

func getFromNestedMetrics(m map[string]interface{}, key string) interface{} {
	if m == nil {
		return nil
	}
	if _, hasRaw := m["raw"]; !hasRaw {
		return nil
	}
	for _, layer := range []string{"layer2", "layer1", "raw"} {
		if sub, ok := m[layer].(map[string]interface{}); ok {
			if v, ok := sub[key]; ok {
				return v
			}
		}
	}
	if l3, ok := m["layer3"].(map[string]interface{}); ok {
		nestedKey := key
		if key == "firstLayer3" {
			nestedKey = "first"
		} else if key == "repeatLayer3" {
			nestedKey = "repeat"
		} else if key == "vipLayer3" {
			nestedKey = "vip"
		} else if key == "inactiveLayer3" {
			nestedKey = "inactive"
		}
		if v, ok := l3[nestedKey]; ok {
			return v
		}
	}
	return nil
}

// buildEmptyMetricsSnapshot trả về metrics snapshot với tất cả giá trị = 0.
// Dùng cho customer_created để timeline đúng: tại thời điểm khởi tạo, metrics = 0; tăng dần theo mỗi order/chat.
func buildEmptyMetricsSnapshot() map[string]interface{} {
	return buildMetricsSnapshot(&crmmodels.CrmCustomer{})
}

// BuildCurrentMetricsSnapshot trả về metrics map hiện tại từ customer — cùng cấu trúc với metricsSnapshot.
// Dùng cho full profile response: góc nhìn now song song với lịch sử (metadata.metricsSnapshot trong activity).
func BuildCurrentMetricsSnapshot(c *crmmodels.CrmCustomer) map[string]interface{} {
	if c == nil {
		return nil
	}
	return buildMetricsSnapshot(c)
}

// BuildCurrentMetricsFromOrderAndConv tạo currentMetrics nested từ order + conversation metrics.
// Dùng khi merge/refresh — chưa có CrmCustomer đầy đủ trong memory, cần build từ aggregate results.
func BuildCurrentMetricsFromOrderAndConv(om orderMetrics, cm conversationMetrics, hasConv bool) map[string]interface{} {
	avgOrderValue := 0.0
	if om.OrderCount > 0 {
		avgOrderValue = om.TotalSpent / float64(om.OrderCount)
	}
	c := &crmmodels.CrmCustomer{
		TotalSpent:                om.TotalSpent,
		OrderCount:                om.OrderCount,
		AvgOrderValue:             avgOrderValue,
		LastOrderAt:               om.LastOrderAt,
		SecondLastOrderAt:         om.SecondLastOrderAt,
		RevenueLast30d:            om.RevenueLast30d,
		RevenueLast90d:            om.RevenueLast90d,
		CancelledOrderCount:       om.CancelledOrderCount,
		OrdersLast30d:             om.OrdersLast30d,
		OrdersLast90d:             om.OrdersLast90d,
		OrdersFromAds:             om.OrdersFromAds,
		OrdersFromOrganic:         om.OrdersFromOrganic,
		OrdersFromDirect:          om.OrdersFromDirect,
		OrderCountOnline:          om.OrderCountOnline,
		OrderCountOffline:         om.OrderCountOffline,
		FirstOrderChannel:         om.FirstOrderChannel,
		LastOrderChannel:          om.LastOrderChannel,
		IsOmnichannel:             om.OrderCountOnline > 0 && om.OrderCountOffline > 0,
		HasConversation:           hasConv,
		HasOrder:                  om.OrderCount > 0,
		ConversationCount:         cm.ConversationCount,
		ConversationCountByInbox:  cm.ConversationCountByInbox,
		ConversationCountByComment: cm.ConversationCountByComment,
		LastConversationAt:        cm.LastConversationAt,
		FirstConversationAt:       cm.FirstConversationAt,
		TotalMessages:             cm.TotalMessages,
		LastMessageFromCustomer:   cm.LastMessageFromCustomer,
		ConversationFromAds:       cm.ConversationFromAds,
		ConversationTags:          cm.ConversationTags,
		OwnedSkuQuantities:        om.OwnedSkuQuantities,
	}
	return buildMetricsSnapshot(c)
}

// diffSnapshot so sánh profile và metrics mới với cũ, trả về danh sách thay đổi.
// Metrics nested — diff từng layer với prefix metrics.raw., metrics.layer1., metrics.layer2., metrics.layer3.
func diffSnapshot(newProfile, newMetrics, oldProfile, oldMetrics map[string]interface{}) []snapshotChange {
	newNested := ensureNested(newMetrics)
	oldNested := ensureNested(oldMetrics)
	out := make([]snapshotChange, 0, 64)
	out = append(out, diffMap(oldProfile, newProfile, "profile.")...)
	for _, layer := range []struct{ name string }{{"raw"}, {"layer1"}, {"layer2"}, {"layer3"}} {
		oldSub := getSubMap(oldNested, layer.name)
		newSub := getSubMap(newNested, layer.name)
		out = append(out, diffMap(oldSub, newSub, "metrics."+layer.name+".")...)
	}
	return out
}

func ensureNested(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	if _, has := m["raw"]; has {
		return m
	}
	return nil
}

func getSubMap(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return nil
	}
	if sub, ok := m[key].(map[string]interface{}); ok {
		return sub
	}
	return nil
}

// diffMap so sánh hai map, trả về thay đổi. Xử lý: value thay đổi, key mới, key bị xóa.
// oldMap nil: coi như không có dữ liệu cũ (key mới). newMap nil: mọi key cũ coi như bị xóa.
func diffMap(oldMap, newMap map[string]interface{}, prefix string) []snapshotChange {
	if newMap == nil && oldMap == nil {
		return nil
	}
	estimated := len(newMap) + len(oldMap)
	if estimated > 32 {
		estimated = 32
	}
	out := make([]snapshotChange, 0, estimated)

	// Trường hợp 1: key trong newMap — thay đổi giá trị hoặc key mới (old không có)
	for k, newV := range newMap {
		oldV := oldMap[k] // nil map read trả về zero value, không panic
		if !valuesEqual(oldV, newV) {
			out = append(out, snapshotChange{Field: prefix + k, OldValue: oldV, NewValue: newV})
		}
	}
	// Trường hợp 2: key trong oldMap nhưng không trong newMap — key bị xóa
	for k, oldV := range oldMap {
		if _, exists := newMap[k]; !exists {
			out = append(out, snapshotChange{Field: prefix + k, OldValue: oldV, NewValue: nil})
		}
	}
	return out
}

func buildChangesForNewCustomer(profile, metrics map[string]interface{}) []snapshotChange {
	var out []snapshotChange
	for k, v := range profile {
		if !isEmptyValue(v) {
			out = append(out, snapshotChange{Field: "profile." + k, OldValue: nil, NewValue: v})
		}
	}
	metricsNested := ensureNested(metrics)
	for _, layer := range []string{"raw", "layer1", "layer2", "layer3"} {
		sub := getSubMap(metricsNested, layer)
		for k, v := range sub {
			if !isEmptyValue(v) {
				out = append(out, snapshotChange{Field: "metrics." + layer + "." + k, OldValue: nil, NewValue: v})
			}
		}
	}
	return out
}

// valuesEqual so sánh hai giá trị. Chuẩn hóa kiểu số và slice (BSON vs Go) để tránh false positive.
func valuesEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return reflect.DeepEqual(a, b)
	}
	// Số: chuẩn hóa rồi so sánh (BSON int32/int64 vs Go int/float64)
	if na, nb, ok := asFloat64Pair(a, b); ok {
		return na == nb
	}
	// Slice: so sánh nội dung ([]string vs []interface{} từ BSON)
	if sa, sb := toInterfaceSlice(a), toInterfaceSlice(b); sa != nil && sb != nil {
		return slicesEqual(sa, sb)
	}
	return reflect.DeepEqual(a, b)
}

func toInterfaceSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case []interface{}:
		return x
	case []string:
		out := make([]interface{}, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out
	default:
		rv := reflect.ValueOf(v)
		if rv.Kind() != reflect.Slice {
			return nil
		}
		out := make([]interface{}, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = rv.Index(i).Interface()
		}
		return out
	}
}

func slicesEqual(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !valuesEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func asFloat64Pair(a, b interface{}) (float64, float64, bool) {
	fa := toFloat64(a)
	fb := toFloat64(b)
	if !isNumber(a) || !isNumber(b) {
		return 0, 0, false
	}
	return fa, fb, true
}

func isNumber(v interface{}) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	}
	return false
}

func toFloat64(v interface{}) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case float64:
		return x
	case float32:
		return float64(x)
	default:
		return 0
	}
}

func isEmptyValue(v interface{}) bool {
	if v == nil {
		return true
	}
	switch x := v.(type) {
	case string:
		return x == ""
	case []string:
		return len(x) == 0
	case []interface{}:
		return len(x) == 0
	case map[string]int:
		return len(x) == 0
	case map[string]interface{}:
		return len(x) == 0
	case float64:
		return x == 0
	case int:
		return x == 0
	case int64:
		return x == 0
	case bool:
		return false // bool false vẫn là giá trị có nghĩa
	}
	return false
}

// truncateOwnedSkuQuantities giới hạn map SKU->qty, ưu tiên SKU có số lượng cao.
func truncateOwnedSkuQuantities(m map[string]int, max int) map[string]int {
	if len(m) <= max {
		return m
	}
	// Sắp xếp theo qty giảm dần, lấy top max
	type kv struct {
		k string
		v int
	}
	var arr []kv
	for k, v := range m {
		arr = append(arr, kv{k, v})
	}
	for i := 0; i < len(arr)-1; i++ {
		for j := i + 1; j < len(arr); j++ {
			if arr[j].v > arr[i].v {
				arr[i], arr[j] = arr[j], arr[i]
			}
		}
	}
	out := make(map[string]int)
	for i := 0; i < max && i < len(arr); i++ {
		out[arr[i].k] = arr[i].v
	}
	return out
}

// MergeSnapshotIntoMetadata merge snapshot vào metadata. metadata có thể nil.
func MergeSnapshotIntoMetadata(metadata map[string]interface{}, snapshot map[string]interface{}) {
	if metadata == nil || snapshot == nil {
		return
	}
	for k, v := range snapshot {
		metadata[k] = v
	}
}
