// Package crmvc - Snapshot profile và metrics cho activity history.
// Chỉ lưu khi profile hoặc metrics thay đổi. Snapshot chứa profileSnapshot, metricsSnapshot và snapshotChanges (cái gì thay đổi, cũ → mới).
package crmvc

import (
	"reflect"
	"time"

	crmmodels "meta_commerce/internal/api/crm/models"
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
func BuildSnapshotForNewCustomer(c *crmmodels.CrmCustomer, snapshotAt int64, useEmptyMetrics bool) map[string]interface{} {
	if c == nil {
		return nil
	}
	profile := buildProfileSnapshot(c)
	var metrics map[string]interface{}
	if useEmptyMetrics {
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
		"name":          c.Name,
		"phoneNumbers":  c.PhoneNumbers,
		"emails":        c.Emails,
		"birthday":      c.Birthday,
		"gender":        c.Gender,
		"livesIn":       c.LivesIn,
		"referralCode":  c.ReferralCode,
		"primarySource": c.PrimarySource,
	}
	if len(c.Addresses) > 0 {
		p["addresses"] = c.Addresses
	}
	return p
}

func buildMetricsSnapshot(c *crmmodels.CrmCustomer) map[string]interface{} {
	m := map[string]interface{}{
		"totalSpent":                 c.TotalSpent,
		"orderCount":                 c.OrderCount,
		"avgOrderValue":              c.AvgOrderValue,
		"lastOrderAt":                c.LastOrderAt,
		"secondLastOrderAt":          c.SecondLastOrderAt,
		"revenueLast30d":             c.RevenueLast30d,
		"revenueLast90d":             c.RevenueLast90d,
		"cancelledOrderCount":        c.CancelledOrderCount,
		"ordersLast30d":              c.OrdersLast30d,
		"ordersLast90d":              c.OrdersLast90d,
		"ordersFromAds":              c.OrdersFromAds,
		"ordersFromOrganic":          c.OrdersFromOrganic,
		"ordersFromDirect":           c.OrdersFromDirect,
		"orderCountOnline":           c.OrderCountOnline,
		"orderCountOffline":          c.OrderCountOffline,
		"firstOrderChannel":          c.FirstOrderChannel,
		"lastOrderChannel":           c.LastOrderChannel,
		"isOmnichannel":              c.IsOmnichannel,
		"hasConversation":            c.HasConversation,
		"hasOrder":                   c.HasOrder,
		"conversationCount":          c.ConversationCount,
		"conversationCountByInbox":   c.ConversationCountByInbox,
		"conversationCountByComment": c.ConversationCountByComment,
		"lastConversationAt":         c.LastConversationAt,
		"firstConversationAt":        c.FirstConversationAt,
		"totalMessages":              c.TotalMessages,
		"lastMessageFromCustomer":    c.LastMessageFromCustomer,
		"conversationFromAds":        c.ConversationFromAds,
		"valueTier":                  ComputeValueTier(c.TotalSpent),
		"lifecycleStage":             ComputeLifecycleStage(c.LastOrderAt),
		"journeyStage":               ComputeJourneyStage(c),
		"channel":                    ComputeChannel(c),
		"loyaltyStage":               ComputeLoyaltyStage(c.OrderCount),
		"momentumStage":              ComputeMomentumStage(c),
	}
	if len(c.ConversationTags) > 0 {
		m["conversationTags"] = c.ConversationTags
	}
	if len(c.OwnedSkuQuantities) > 0 {
		m["ownedSkuQuantities"] = truncateOwnedSkuQuantities(c.OwnedSkuQuantities, maxOwnedSkusInSnapshot)
	}
	return m
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

// diffSnapshot so sánh profile và metrics mới với cũ, trả về danh sách thay đổi.
// Phát hiện: giá trị thay đổi, key mới (trong new không có trong old), key bị xóa (trong old không có trong new).
func diffSnapshot(newProfile, newMetrics, oldProfile, oldMetrics map[string]interface{}) []snapshotChange {
	// Pre-allocate: profile ~9 fields, metrics ~35 fields; ít khi tất cả đều thay đổi
	capProfile := len(newProfile) + len(oldProfile)
	capMetrics := len(newMetrics) + len(oldMetrics)
	if capProfile > 16 {
		capProfile = 16
	}
	if capMetrics > 40 {
		capMetrics = 40
	}
	out := make([]snapshotChange, 0, capProfile+capMetrics)

	out = append(out, diffMap(oldProfile, newProfile, "profile.")...)
	out = append(out, diffMap(oldMetrics, newMetrics, "metrics.")...)
	return out
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
	for k, v := range metrics {
		if !isEmptyValue(v) {
			out = append(out, snapshotChange{Field: "metrics." + k, OldValue: nil, NewValue: v})
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
