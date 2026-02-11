package utility

import (
	"encoding/json"
)

// ConvertStruct chuyển đổi một struct sang struct khác
// Parameters:
//   - source: Struct nguồn cần chuyển đổi
//   - target: Con trỏ đến struct đích
//
// Returns:
//   - interface{}: Struct đích đã được chuyển đổi
//   - error: Lỗi nếu có
func ConvertStruct(source interface{}, target interface{}) (interface{}, error) {
	// Chuyển source thành JSON
	jsonData, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}

	// Chuyển JSON thành target struct
	err = json.Unmarshal(jsonData, target)
	if err != nil {
		return nil, err
	}

	return target, nil
}
