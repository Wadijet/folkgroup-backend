// Package activity — helpers cho Activity framework.
package activity

import (
	"encoding/json"
)

// TruncateMetadata giới hạn kích thước metadata (bytes).
// maxBytes <= 0: không giới hạn, trả về data gốc.
// Hiện tại: nếu vượt maxBytes thì trả về data gốc (chưa truncate).
// TODO: Implement truncate bằng cách loại bỏ key ít quan trọng hoặc cắt string values.
func TruncateMetadata(data map[string]interface{}, maxBytes int) map[string]interface{} {
	if data == nil || maxBytes <= 0 {
		return data
	}
	b, err := json.Marshal(data)
	if err != nil {
		return data
	}
	if len(b) <= maxBytes {
		return data
	}
	// Chưa implement truncate — trả về gốc. Caller nên giới hạn payload trước khi gọi.
	return data
}
