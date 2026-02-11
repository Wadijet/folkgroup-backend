package utility

import (
	"encoding/json"
	"fmt"
)

// MapToJSON chuyển đổi map thành chuỗi JSON
// @params - map cần chuyển đổi
// @returns - chuỗi JSON và lỗi nếu có
func MapToJSON(m map[string]interface{}) (string, error) {
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("lỗi khi chuyển đổi map thành JSON: %v", err)
	}
	return string(jsonBytes), nil
}

// JSONToMap chuyển đổi chuỗi JSON thành map
// @params - chuỗi JSON cần chuyển đổi
// @returns - map và lỗi nếu có
func JSONToMap(jsonStr string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi chuyển đổi JSON thành map: %v", err)
	}
	return result, nil
}

// GetMapValue lấy giá trị từ map theo key
// @params - map cần tìm, key cần tìm
// @returns - giá trị tìm được và lỗi nếu có
func GetMapValue(m map[string]interface{}, key string) (interface{}, error) {
	if m == nil {
		return nil, fmt.Errorf("map không được nil")
	}
	value, exists := m[key]
	if !exists {
		return nil, fmt.Errorf("không tìm thấy key: %s", key)
	}
	return value, nil
}

// SetMapValue đặt giá trị cho key trong map
// @params - map cần cập nhật, key cần đặt, giá trị cần đặt
// @returns - lỗi nếu có
func SetMapValue(m map[string]interface{}, key string, value interface{}) error {
	if m == nil {
		return fmt.Errorf("map không được nil")
	}
	m[key] = value
	return nil
}

// DeleteMapValue xóa một key khỏi map
// @params - map cần xóa, key cần xóa
// @returns - lỗi nếu có
func DeleteMapValue(m map[string]interface{}, key string) error {
	if m == nil {
		return fmt.Errorf("map không được nil")
	}
	delete(m, key)
	return nil
}

// MapContainsKey kiểm tra xem map có chứa key hay không
// @params - map cần kiểm tra, key cần tìm
// @returns - true nếu có key, false nếu không
func MapContainsKey(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	_, exists := m[key]
	return exists
}

// MapIsEmpty kiểm tra xem map có rỗng hay không
// @params - map cần kiểm tra
// @returns - true nếu map rỗng, false nếu không
func MapIsEmpty(m map[string]interface{}) bool {
	return m == nil || len(m) == 0
}
