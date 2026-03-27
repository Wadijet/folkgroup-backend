// Package events — Helper lấy mốc thời gian nguồn từ document (posData / panCakeData / root) phục vụ CRM pending ingest delta.
// So sánh updated_at để giảm ghi Mongo thuộc lớp 1 (DoSyncUpsert); hook emit queue không lặp lại so sánh đó.
package events

import (
	"time"

	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MergeRelevantDataKey trả về key nested (posData / panCakeData) để đọc mốc thời gian nguồn trong document, hoặc "" nếu không dùng map lồng.
func MergeRelevantDataKey(collectionName string) string {
	switch collectionName {
	case global.MongoDB_ColNames.PcPosCustomers:
		return "posData"
	case global.MongoDB_ColNames.FbCustomers:
		return "panCakeData"
	case global.MongoDB_ColNames.PcPosOrders:
		return "posData"
	case global.MongoDB_ColNames.FbConvesations:
		return "panCakeData"
	case global.MongoDB_ColNames.CrmNotes:
		return ""
	default:
		return ""
	}
}

// ExtractUpdatedAtFromDoc lấy updated_at (ms) từ document theo collection (crm_pending_ingest delta).
func ExtractUpdatedAtFromDoc(collectionName string, doc interface{}) int64 {
	if doc == nil {
		return 0
	}
	data, err := bson.Marshal(doc)
	if err != nil {
		return 0
	}
	var m map[string]interface{}
	if err := bson.Unmarshal(data, &m); err != nil {
		return 0
	}
	if collectionName == global.MongoDB_ColNames.CrmNotes {
		if t := TimestampFromMap(m, "updatedAt"); t > 0 {
			return t
		}
		return TimestampFromMap(m, "updated_at")
	}
	key := MergeRelevantDataKey(collectionName)
	if key != "" {
		if sub, ok := m[key].(map[string]interface{}); ok && sub != nil {
			if t := TimestampFromMap(sub, "updated_at"); t > 0 {
				return t
			}
			if t := TimestampFromMap(sub, "updatedAt"); t > 0 {
				return t
			}
			if t := TimestampFromMap(sub, "inserted_at"); t > 0 {
				return t
			}
			return TimestampFromMap(sub, "insertedAt")
		}
	}
	if t := TimestampFromMap(m, "updatedAt"); t > 0 {
		return t
	}
	return TimestampFromMap(m, "updated_at")
}

// TimestampFromMap lấy Unix ms từ map (ISO string, DateTime, số — đồng bộ với crm ingest).
func TimestampFromMap(m map[string]interface{}, key string) int64 {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case string:
		layouts := []string{
			"2006-01-02T15:04:05.000000", "2006-01-02T15:04:05.999999",
			"2006-01-02T15:04:05.000", "2006-01-02T15:04:05",
			"2006-01-02 15:04:05.000000", "2006-01-02 15:04:05",
			time.RFC3339, time.RFC3339Nano,
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, x); err == nil {
				return t.UnixMilli()
			}
		}
		return 0
	case primitive.DateTime:
		return x.Time().UnixMilli()
	case float64:
		ms := int64(x)
		if ms > 0 && ms < 1e12 {
			ms *= 1000
		}
		return ms
	case int64:
		if x > 0 && x < 1e12 {
			x *= 1000
		}
		return x
	case int:
		ms := int64(x)
		if ms > 0 && ms < 1e12 {
			ms *= 1000
		}
		return ms
	}
	return 0
}
