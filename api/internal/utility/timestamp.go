package utility

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ParseTimestampFromMap lấy Unix ms từ map (hỗ trợ string ISO, primitive.DateTime, float64, int64).
// Dùng cho panCakeData/posData khi BSON decode - inserted_at/updated_at có thể là primitive.DateTime.
// Giá trị số < 1e12 được coi là Unix seconds (Pancake API có thể trả seconds) → nhân 1000.
//
// Tham số:
//   - m: map chứa dữ liệu (vd: panCakeData, posData)
//   - key: tên key cần lấy (vd: "updated_at", "inserted_at")
//
// Trả về: Unix milliseconds, hoặc 0 nếu không parse được.
func ParseTimestampFromMap(m map[string]interface{}, key string) int64 {
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
			ms *= 1000 // Unix seconds → milliseconds
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
