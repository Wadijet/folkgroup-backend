package utility

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// transformTagConfig chứa cấu hình được parse từ tag transform
type transformTagConfig struct {
	Type     string // Transform type: str_objectid, str_objectid_ptr, str_time, str_int64, str_bool, str_number, etc.
	Format   string // Format cho time converter
	Default  string // Giá trị mặc định
	Optional bool   // Flag optional - nếu không có giá trị, bỏ qua
	Required bool   // Flag required - bắt buộc phải có giá trị
	MapTo    string // Map field name: map sang field khác trong Model (ví dụ: map=ParentRefID)
}

// ParseTransformTag parse tag transform thành config (public function)
// Format: "[type][,format=<value>][,default=<value>][,optional|required]"
// Naming convention: <input_type>_<output_type>
// Ví dụ:
//   - transform:"str_objectid" - Convert string → primitive.ObjectID
//   - transform:"str_objectid_ptr" - Convert string → *primitive.ObjectID
//   - transform:"str_time" - Convert string → int64 timestamp
//   - transform:"str_time,format=2006-01-02" - Convert với format cụ thể
//   - transform:"str_int64" - Convert string → int64
//   - transform:"str_bool" - Convert string → bool
//   - transform:"str_number" - Convert string → number (int64 hoặc float64)
//   - transform:"str_objectid,default=" - Với default value (empty = nil)
//   - transform:"str_objectid,optional" - Optional field
func ParseTransformTag(tag string) (*transformTagConfig, error) {
	return parseTransformTag(tag)
}

// parseTransformTag parse tag transform thành config (internal)
func parseTransformTag(tag string) (*transformTagConfig, error) {
	config := &transformTagConfig{
		Type:   "", // Default: không transform (empty = không transform)
		Format: "2006-01-02T15:04:05",
	}

	if tag == "" {
		return config, nil
	}

	// Tách phần type và options
	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return nil, fmt.Errorf("transform tag không hợp lệ: %s", tag)
	}

	// Phần đầu tiên là transform type
	typeStr := strings.TrimSpace(parts[0])
	if typeStr != "" {
		config.Type = typeStr
	}

	// Parse các options từ phần còn lại
	for i := 1; i < len(parts); i++ {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			continue
		}

		// Parse flags
		if part == "optional" {
			config.Optional = true
			continue
		}
		if part == "required" {
			config.Required = true
			continue
		}

		// Parse options với format key=value
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])

			switch key {
			case "format":
				config.Format = value
			case "default":
				config.Default = value
			case "map":
				// Map field name: map sang field khác trong Model
				config.MapTo = value
			}
		}
	}

	return config, nil
}

// TransformFieldValue transform giá trị từ DTO field sang Model field
// Dùng trong transformCreateInputToModel và transformUpdateInputToModel
func TransformFieldValue(value interface{}, config *transformTagConfig, targetFieldType reflect.Type) (interface{}, error) {
	// Nếu value là nil hoặc zero value
	if value == nil {
		if config.Default != "" {
			// Có default value, dùng default
			return applyTransform(config.Default, config, targetFieldType)
		}
		if config.Optional {
			// Optional field, return nil
			return nil, nil
		}
		if config.Required {
			return nil, fmt.Errorf("field là required nhưng không có giá trị")
		}
		// Không có default, không optional, không required → return nil
		return nil, nil
	}

	// Kiểm tra zero value cho string
	if strValue, ok := value.(string); ok {
		if strValue == "" {
			if config.Default != "" {
				return applyTransform(config.Default, config, targetFieldType)
			}
			if config.Optional {
				return nil, nil
			}
			if config.Required {
				return nil, fmt.Errorf("field là required nhưng giá trị rỗng")
			}
			return nil, nil
		}
	}

	// Apply transform
	return applyTransform(value, config, targetFieldType)
}

// applyTransform apply transform type cho value
func applyTransform(value interface{}, config *transformTagConfig, targetFieldType reflect.Type) (interface{}, error) {
	switch config.Type {
	case "str_objectid":
		// Convert string → primitive.ObjectID
		return transformToObjectID(value)
	case "str_objectid_ptr":
		// Convert string → *primitive.ObjectID
		return transformToObjectIDPtr(value)
	case "str_time":
		// Convert string → int64 timestamp
		return transformToTime(value, config.Format)
	case "str_number":
		// Convert string → number (int64 hoặc float64)
		return transformToNumber(value)
	case "str_int64":
		// Convert string → int64
		return transformToInt64(value)
	case "str_bool":
		// Convert string → bool
		return transformToBool(value)
	case "":
		fallthrough
	default:
		// Không transform hoặc type không hợp lệ, return giá trị gốc
		return value, nil
	}
}

// transformToObjectID convert string → primitive.ObjectID
func transformToObjectID(value interface{}) (primitive.ObjectID, error) {
	if value == nil {
		return primitive.NilObjectID, nil
	}

	strValue, ok := value.(string)
	if !ok {
		return primitive.NilObjectID, fmt.Errorf("giá trị không phải là string: %T", value)
	}

	if strValue == "" {
		return primitive.NilObjectID, nil
	}

	objID, err := primitive.ObjectIDFromHex(strValue)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("không thể convert string '%s' sang ObjectID: %w", strValue, err)
	}

	return objID, nil
}

// transformToObjectIDPtr convert string → *primitive.ObjectID
func transformToObjectIDPtr(value interface{}) (*primitive.ObjectID, error) {
	if value == nil {
		return nil, nil
	}

	strValue, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("giá trị không phải là string: %T", value)
	}

	if strValue == "" {
		return nil, nil
	}

	objID, err := primitive.ObjectIDFromHex(strValue)
	if err != nil {
		return nil, fmt.Errorf("không thể convert string '%s' sang ObjectID: %w", strValue, err)
	}

	return &objID, nil
}

// transformToTime convert string → int64 timestamp
func transformToTime(value interface{}, format string) (int64, error) {
	if value == nil {
		return 0, nil
	}

	strValue, ok := value.(string)
	if !ok {
		return 0, fmt.Errorf("giá trị không phải là string: %T", value)
	}

	if strValue == "" {
		return 0, nil
	}

	// Parse time với format
	t, err := time.Parse(format, strValue)
	if err != nil {
		return 0, fmt.Errorf("không thể parse time '%s' với format '%s': %w", strValue, format, err)
	}

	return t.UnixMilli(), nil
}

// transformToNumber convert → number (int64 hoặc float64)
func transformToNumber(value interface{}) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	switch v := value.(type) {
	case string:
		// Thử parse thành int64 trước
		if intVal, err := strconv.ParseInt(v, 10, 64); err == nil {
			return intVal, nil
		}
		// Thử parse thành float64
		if floatVal, err := strconv.ParseFloat(v, 64); err == nil {
			return floatVal, nil
		}
		// Nếu không parse được, return string
		return v, nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		// Nếu là số nguyên, return int64
		if v == float64(int64(v)) {
			return int64(v), nil
		}
		return v, nil
	default:
		// Convert sang string
		return fmt.Sprintf("%v", value), nil
	}
}

// transformToInt64 convert → int64
func transformToInt64(value interface{}) (int64, error) {
	if value == nil {
		return 0, nil
	}

	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case float32:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("không thể convert %T sang int64", value)
	}
}

// transformToBool convert → bool
func transformToBool(value interface{}) (bool, error) {
	if value == nil {
		return false, nil
	}

	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	case int:
		return v != 0, nil
	case int64:
		return v != 0, nil
	case float64:
		return v != 0, nil
	default:
		return false, fmt.Errorf("không thể convert %T sang bool", value)
	}
}
