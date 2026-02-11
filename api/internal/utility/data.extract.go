package utility

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// extractTagConfig chứa cấu hình được parse từ tag extract
type extractTagConfig struct {
	SourcePath    []string // Path đến source field và nested path (đã split bằng \.)
	Converter     string   // Converter name (time, number, int64, bool, string, array_first)
	Format        string   // Format cho time converter
	Default       string   // Giá trị mặc định
	Optional      bool     // Flag optional
	Required      bool     // Flag required
	Priority      int      // Độ ưu tiên (số càng nhỏ càng ưu tiên, 0 = mặc định = ưu tiên thấp nhất)
	MergeStrategy string   // Chiến lược merge: "merge_array", "keep_existing", "overwrite", "priority"
}

// parseExtractTag parse tag extract thành config
// Format mới: "Source1\\.path,options|Source2\\.path,options" (multi-source)
// Format cũ: "[<source_field>][\.<nested_path>][,converter=<name>][,format=<value>][,default=<value>][,optional|required][,priority=<number>][,merge=<strategy>]"
// Nếu có dấu | → parse multi-source, trả về []*extractTagConfig
// Nếu không có dấu | → parse single source, trả về []*extractTagConfig với 1 phần tử (backward compatible)
func parseExtractTag(tag string) ([]*extractTagConfig, error) {
	// Kiểm tra xem có nhiều nguồn không (có dấu |)
	if !strings.Contains(tag, "|") {
		// Single source - backward compatible
		config, err := parseSingleSourceTag(tag)
		if err != nil {
			return nil, err
		}
		return []*extractTagConfig{config}, nil
	}

	// Multi-source: Split bằng | để tách các nguồn
	sources := strings.Split(tag, "|")
	configs := make([]*extractTagConfig, 0, len(sources))

	for _, sourceTag := range sources {
		sourceTag = strings.TrimSpace(sourceTag)
		if sourceTag == "" {
			continue
		}

		config, err := parseSingleSourceTag(sourceTag)
		if err != nil {
			return nil, fmt.Errorf("parse source tag '%s': %w", sourceTag, err)
		}
		configs = append(configs, config)
	}

	return configs, nil
}

// parseSingleSourceTag parse một extract tag từ một nguồn
// Format: "SourceField\\.path[,converter=<name>][,format=<value>][,default=<value>][,optional|required][,priority=<number>][,merge=<strategy>]"
func parseSingleSourceTag(tag string) (*extractTagConfig, error) {
	config := &extractTagConfig{
		Converter:     "string", // Default converter
		Format:        "2006-01-02T15:04:05",
		Priority:      0,           // Mặc định = ưu tiên thấp nhất
		MergeStrategy: "overwrite", // Mặc định: ghi đè
	}

	// Tách phần path và options
	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return nil, fmt.Errorf("extract tag không hợp lệ: %s", tag)
	}

	// Phần đầu tiên là source path
	pathStr := strings.TrimSpace(parts[0])
	if pathStr == "" {
		return nil, fmt.Errorf("extract tag không có source path: %s", tag)
	}

	// Parse source path: split bằng \. (backslash + dot)
	config.SourcePath = parsePath(pathStr)

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
			case "converter":
				config.Converter = value
			case "format":
				config.Format = value
			case "default":
				config.Default = value
			case "priority":
				// Parse priority (số càng nhỏ càng ưu tiên)
				if priority, err := strconv.Atoi(value); err == nil {
					config.Priority = priority
				}
			case "merge":
				// Merge strategy: merge_array, keep_existing, overwrite, priority
				config.MergeStrategy = value
			}
		}
	}

	return config, nil
}

// parsePath parse path string với escape handling
// Split bằng \. (backslash + dot) để phân tách level
// Dấu chấm (.) mặc định là literal trong field name
func parsePath(pathStr string) []string {
	if pathStr == "" {
		return []string{}
	}

	var parts []string
	var current strings.Builder
	i := 0

	for i < len(pathStr) {
		// Kiểm tra escape sequence \.
		if i < len(pathStr)-1 && pathStr[i] == '\\' && pathStr[i+1] == '.' {
			// Đây là separator \.
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			i += 2 // Bỏ qua \.
			continue
		}

		// Kiểm tra escape sequence \\
		if i < len(pathStr)-1 && pathStr[i] == '\\' && pathStr[i+1] == '\\' {
			current.WriteByte('\\')
			i += 2
			continue
		}

		// Ký tự bình thường
		current.WriteByte(pathStr[i])
		i++
	}

	// Thêm phần cuối cùng
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// ExtractDataIfExists extract data từ source fields vào typed fields dựa trên tag extract
func ExtractDataIfExists(s interface{}) error {
	if s == nil {
		return nil
	}

	val := reflect.ValueOf(s)
	typ := reflect.TypeOf(s)

	// Xác định struct value và type
	var structVal reflect.Value
	var structType reflect.Type

	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil // Nil pointer, không cần extract
		}
		structVal = val.Elem()
		structType = typ.Elem()
	} else {
		// Nếu là value, không thể modify trực tiếp
		// Extract chỉ có ý nghĩa khi struct được modify qua pointer
		// Vì không thể modify value, nên skip extract
		return nil
	}

	// Phải là struct
	if structVal.Kind() != reflect.Struct {
		return nil // Không phải struct, không cần extract
	}

	// Duyệt qua tất cả các field của struct
	for i := 0; i < structVal.NumField(); i++ {
		field := structVal.Field(i)
		fieldType := structType.Field(i)

		// Lấy tag extract
		extractTag := fieldType.Tag.Get("extract")
		if extractTag == "" {
			continue // Không có tag extract, bỏ qua
		}

		// Parse tag - có thể trả về nhiều configs (multi-source)
		configs, err := parseExtractTag(extractTag)
		if err != nil {
			return fmt.Errorf("parse extract tag cho field %s: %w", fieldType.Name, err)
		}

		// Nếu chỉ có 1 config, xử lý như cũ (backward compatible)
		if len(configs) == 1 {
			if err := extractFieldValue(structVal, field, configs[0]); err != nil {
				// Nếu là optional và không tìm thấy, bỏ qua
				if configs[0].Optional && strings.Contains(err.Error(), "không tìm thấy") {
					continue
				}
				// Nếu là required và không tìm thấy, return error
				if configs[0].Required && strings.Contains(err.Error(), "không tìm thấy") {
					return fmt.Errorf("field %s là required nhưng không tìm thấy giá trị: %w", fieldType.Name, err)
				}
				// Nếu có default và không tìm thấy, dùng default
				if configs[0].Default != "" && strings.Contains(err.Error(), "không tìm thấy") {
					if err := setFieldValue(field, configs[0].Default, configs[0]); err != nil {
						return fmt.Errorf("set default value cho field %s: %w", fieldType.Name, err)
					}
					continue
				}
				// Nếu là optional và có lỗi convert (ví dụ parse time), bỏ qua field này
				if configs[0].Optional {
					continue
				}
				// Nếu không phải optional và có lỗi, return error
				return fmt.Errorf("extract field %s: %w", fieldType.Name, err)
			}
			continue
		}

		// Nếu có nhiều configs (multi-source), xử lý conflict
		if err := extractFieldValueMultiSource(structVal, field, configs); err != nil {
			// Nếu tất cả đều optional, bỏ qua
			allOptional := true
			for _, cfg := range configs {
				if !cfg.Optional {
					allOptional = false
					break
				}
			}
			if allOptional && strings.Contains(err.Error(), "không tìm thấy") {
				continue
			}
			return fmt.Errorf("extract field %s (multi-source): %w", fieldType.Name, err)
		}
	}

	return nil
}

// extractFieldValue extract giá trị từ source field vào target field
func extractFieldValue(structVal reflect.Value, targetField reflect.Value, config *extractTagConfig) error {
	if len(config.SourcePath) == 0 {
		return fmt.Errorf("source path rỗng")
	}

	// Xác định source field name và nested path
	// Logic: Path phải đầy đủ, không có mặc định
	// - Phần đầu tiên luôn là field name (ví dụ: "PanCakeData", "FacebookData")
	// - Phần còn lại là nested path (ví dụ: ["id"], ["user", "name"])
	if len(config.SourcePath) < 2 {
		return fmt.Errorf("source path phải có ít nhất 2 phần: field_name\\.key (ví dụ: PanCakeData\\.id), nhận được: %s", strings.Join(config.SourcePath, " -> "))
	}

	// Phần đầu luôn là field name
	sourceFieldName := config.SourcePath[0]
	if sourceFieldName == "" {
		return fmt.Errorf("source field name không được rỗng")
	}

	// Phần còn lại là nested path
	pathParts := config.SourcePath[1:]

	// Tìm source field trong struct
	sourceField := structVal.FieldByName(sourceFieldName)
	if !sourceField.IsValid() {
		return fmt.Errorf("không tìm thấy source field: %s", sourceFieldName)
	}

	// Source field phải là map[string]interface{}
	if sourceField.Kind() != reflect.Map {
		return fmt.Errorf("source field %s không phải là map", sourceFieldName)
	}

	// Convert sang map[string]interface{}
	sourceMap, ok := sourceField.Interface().(map[string]interface{})
	if !ok {
		return fmt.Errorf("source field %s không phải là map[string]interface{}", sourceFieldName)
	}

	// Nếu source map rỗng, kiểm tra default hoặc required
	if len(sourceMap) == 0 {
		if config.Default != "" {
			return setFieldValue(targetField, config.Default, config)
		}
		if config.Required {
			return fmt.Errorf("source field %s rỗng và field là required", sourceFieldName)
		}
		return fmt.Errorf("source field %s rỗng", sourceFieldName)
	}

	// Traverse nested path
	var value interface{} = sourceMap

	for _, part := range pathParts {
		if value == nil {
			break
		}

		// Kiểm tra xem value có phải là map không
		mapValue, ok := value.(map[string]interface{})
		if !ok {
			return fmt.Errorf("không thể traverse path, giá trị tại '%s' không phải là map", part)
		}

		// Lấy giá trị từ map
		nextValue, exists := mapValue[part]
		if !exists {
			// Không tìm thấy, kiểm tra default hoặc required
			if config.Default != "" {
				return setFieldValue(targetField, config.Default, config)
			}
			if config.Required {
				return fmt.Errorf("không tìm thấy path '%s' trong source và field là required", strings.Join(config.SourcePath, " -> "))
			}
			return fmt.Errorf("không tìm thấy path '%s' trong source", strings.Join(config.SourcePath, " -> "))
		}

		value = nextValue
	}

	// Nếu value là nil và có default, dùng default
	if value == nil {
		if config.Default != "" {
			return setFieldValue(targetField, config.Default, config)
		}
		if config.Required {
			return fmt.Errorf("giá trị tại path '%s' là nil và field là required", strings.Join(config.SourcePath, " -> "))
		}
		return fmt.Errorf("giá trị tại path '%s' là nil", strings.Join(config.SourcePath, " -> "))
	}

	// Set giá trị vào target field với converter
	return setFieldValue(targetField, value, config)
}

// setFieldValue set giá trị vào field với converter
func setFieldValue(field reflect.Value, value interface{}, config *extractTagConfig) error {
	if !field.CanSet() {
		return fmt.Errorf("field không thể set")
	}

	fieldType := field.Type()

	// Xử lý đặc biệt cho map: nếu target field là map và value cũng là map → gán trực tiếp (ShippingAddress, WarehouseInfo, CustomerInfo)
	if fieldType.Kind() == reflect.Map && value != nil {
		valueVal := reflect.ValueOf(value)
		if valueVal.Kind() == reflect.Map {
			// Kiểm tra type tương thích: map[string]interface{} hoặc map tương tự
			if valueVal.Type().AssignableTo(fieldType) {
				field.Set(valueVal)
				return nil
			}
			// Nếu value là map[string]interface{}, tạo map mới và copy
			if srcMap, ok := value.(map[string]interface{}); ok && fieldType.Key().Kind() == reflect.String && fieldType.Elem().Kind() == reflect.Interface {
				newMap := reflect.MakeMap(fieldType)
				for k, v := range srcMap {
					newMap.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v))
				}
				field.Set(newMap)
				return nil
			}
		}
	}

	// Xử lý đặc biệt cho array/slice: nếu target field là slice và value cũng là array/slice
	// Không cần converter, chỉ cần convert element types
	if fieldType.Kind() == reflect.Slice && value != nil {
		valueVal := reflect.ValueOf(value)
		if valueVal.Kind() == reflect.Slice || valueVal.Kind() == reflect.Array {
			// Tạo slice mới với type của target field
			elemType := fieldType.Elem()
			newSlice := reflect.MakeSlice(fieldType, valueVal.Len(), valueVal.Len())

			// Convert từng element
			for i := 0; i < valueVal.Len(); i++ {
				elemValue := valueVal.Index(i).Interface()
				// Converter array/slice chỉ dùng cho slice gốc (passthrough), không áp dụng cho từng element.
				// Element có thể là map (order item), string, number... — giữ nguyên khi target là []interface{}.
				if config.Converter != "" && config.Converter != "string" && config.Converter != "array" && config.Converter != "slice" {
					convertedElem, err := applyConverter(elemValue, config.Converter, config.Format)
					if err != nil {
						return fmt.Errorf("convert element %d: %w", i, err)
					}
					elemValue = convertedElem
				}
				// Convert element type
				elemVal := reflect.ValueOf(elemValue)
				if elemVal.Type().AssignableTo(elemType) {
					newSlice.Index(i).Set(elemVal)
				} else if elemVal.Type().ConvertibleTo(elemType) {
					newSlice.Index(i).Set(elemVal.Convert(elemType))
				} else {
					return fmt.Errorf("không thể convert element %d từ %v sang %v", i, elemVal.Type(), elemType)
				}
			}

			field.Set(newSlice)
			return nil
		}
	}

	// Apply converter cho non-array values
	convertedValue, err := applyConverter(value, config.Converter, config.Format)
	if err != nil {
		return fmt.Errorf("convert value: %w", err)
	}

	// Set giá trị vào field
	convertedType := reflect.TypeOf(convertedValue)

	// Kiểm tra type compatibility
	if convertedType.AssignableTo(fieldType) {
		field.Set(reflect.ValueOf(convertedValue))
		return nil
	}

	// Nếu không assignable, thử convert
	if convertedType.ConvertibleTo(fieldType) {
		field.Set(reflect.ValueOf(convertedValue).Convert(fieldType))
		return nil
	}

	return fmt.Errorf("không thể convert %v sang %v", convertedType, fieldType)
}

// unwrapExtendedJSONValue gỡ giá trị từ Extended JSON (MongoDB): $numberLong, $numberInt.
// Trả về (value đã gỡ, true) nếu là dạng extended; (value gốc, false) nếu không.
func unwrapExtendedJSONValue(value interface{}) (interface{}, bool) {
	if value == nil {
		return nil, false
	}
	m, ok := value.(map[string]interface{})
	if !ok || len(m) != 1 {
		return value, false
	}
	if s, ok := m["$numberLong"]; ok {
		switch v := s.(type) {
		case string:
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return value, false
			}
			return n, true
		case float64:
			return int64(v), true
		case int64:
			return v, true
		}
	}
	if n, ok := m["$numberInt"]; ok {
		switch v := n.(type) {
		case float64:
			return int64(v), true
		case int64:
			return v, true
		case int:
			return int64(v), true
		case string:
			n64, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return value, false
			}
			return n64, true
		}
	}
	return value, false
}

// applyConverter apply converter cho value
func applyConverter(value interface{}, converter string, format string) (interface{}, error) {
	// Gỡ Extended JSON ($numberLong, $numberInt) trước khi convert
	if unwrapped, ok := unwrapExtendedJSONValue(value); ok {
		value = unwrapped
	}
	switch converter {
	case "time":
		return convertTime(value, format)
	case "number":
		return convertNumber(value)
	case "int64":
		return convertInt64(value)
	case "int":
		v, err := convertInt64(value)
		if err != nil {
			return nil, err
		}
		return int(v), nil
	case "bool":
		return convertBool(value)
	case "array_first":
		return convertArrayFirst(value)
	case "array", "slice":
		// Passthrough: trả về giá trị slice/array nguyên vẹn (dùng cho OrderItems)
		if value == nil {
			return nil, fmt.Errorf("giá trị là nil")
		}
		val := reflect.ValueOf(value)
		if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
			return value, nil
		}
		return nil, fmt.Errorf("converter array/slice chỉ áp dụng cho slice/array, nhận %T", value)
	case "string":
		fallthrough
	default:
		return convertString(value)
	}
}

// convertArrayFirst lấy phần tử đầu tiên từ array/slice
func convertArrayFirst(value interface{}) (interface{}, error) {
	if value == nil {
		return nil, fmt.Errorf("giá trị là nil")
	}

	val := reflect.ValueOf(value)
	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		if val.Len() == 0 {
			return nil, fmt.Errorf("array rỗng")
		}
		return val.Index(0).Interface(), nil
	}

	// Nếu không phải array, return giá trị gốc
	return value, nil
}

// convertTime convert time string → int64 timestamp
func convertTime(value interface{}, format string) (int64, error) {
	var timeStr string

	switch v := value.(type) {
	case string:
		timeStr = v
	case int64:
		// Nếu đã là timestamp, return luôn
		return v, nil
	case float64:
		// Nếu là số, coi như timestamp (milliseconds)
		return int64(v), nil
	default:
		timeStr = fmt.Sprintf("%v", value)
	}

	if timeStr == "" {
		return 0, fmt.Errorf("time string rỗng")
	}

	// Parse time theo format được chỉ định
	t, err := time.Parse(format, timeStr)
	if err != nil {
		// Thử parse với format mặc định (không có microseconds)
		t, err = time.Parse("2006-01-02T15:04:05", timeStr)
		if err != nil {
			// Thử parse với format có microseconds (6 chữ số)
			t, err = time.Parse("2006-01-02T15:04:05.000000", timeStr)
			if err != nil {
				// Thử parse với format có milliseconds (3 chữ số)
				t, err = time.Parse("2006-01-02T15:04:05.000", timeStr)
				if err != nil {
					// Thử parse với RFC3339 (hỗ trợ fractional seconds)
					t, err = time.Parse(time.RFC3339, timeStr)
					if err != nil {
						// Thử parse với RFC3339Nano (hỗ trợ nanoseconds)
						t, err = time.Parse(time.RFC3339Nano, timeStr)
						if err != nil {
							return 0, fmt.Errorf("không thể parse time '%s' với format '%s': %w", timeStr, format, err)
						}
					}
				}
			}
		}
	}

	// Return timestamp (milliseconds)
	return t.UnixMilli(), nil
}

// convertNumber convert json.Number/string → string/int64
func convertNumber(value interface{}) (interface{}, error) {
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

// convertInt64 convert → int64
func convertInt64(value interface{}) (int64, error) {
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
	case json.Number:
		// json.Number có thể là số nguyên hoặc số thập phân
		// Thử parse thành int64 trước
		if intVal, err := v.Int64(); err == nil {
			return intVal, nil
		}
		// Nếu không được, thử parse thành float64 rồi convert sang int64
		if floatVal, err := v.Float64(); err == nil {
			return int64(floatVal), nil
		}
		return 0, fmt.Errorf("không thể convert json.Number '%s' sang int64", v)
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("không thể convert %T sang int64", value)
	}
}

// convertBool convert → bool
func convertBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	case int64:
		return v != 0, nil
	case int:
		return v != 0, nil
	case float64:
		return v != 0, nil
	default:
		return false, fmt.Errorf("không thể convert %T sang bool", value)
	}
}

// convertString convert bất kỳ → string
func convertString(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	return fmt.Sprintf("%v", value), nil
}

// extractedValue chứa giá trị đã extract từ một nguồn
type extractedValue struct {
	value  interface{}
	config *extractTagConfig
}

// extractFieldValueMultiSource extract giá trị từ nhiều nguồn với conflict resolution
func extractFieldValueMultiSource(structVal reflect.Value, targetField reflect.Value, configs []*extractTagConfig) error {
	if len(configs) == 0 {
		return fmt.Errorf("không có config nào")
	}

	// Extract giá trị từ tất cả các nguồn (mỗi nguồn có converter riêng)
	values := make([]extractedValue, 0, len(configs))

	for _, config := range configs {
		// Kiểm tra xem nguồn này có data không
		sourceFieldName := config.SourcePath[0]
		sourceField := structVal.FieldByName(sourceFieldName)
		if !sourceField.IsValid() {
			continue // Nguồn không tồn tại, bỏ qua
		}

		// Kiểm tra source field có data không
		if sourceField.Kind() != reflect.Map {
			continue // Không phải map, bỏ qua
		}

		sourceMap, ok := sourceField.Interface().(map[string]interface{})
		if !ok || sourceMap == nil || len(sourceMap) == 0 {
			// Nguồn không có data, bỏ qua (nếu optional)
			if config.Optional {
				continue
			}
			// Nếu required và không có data, return error
			if config.Required {
				return fmt.Errorf("source field %s là required nhưng không có data", sourceFieldName)
			}
			continue
		}

		// Extract giá trị từ nguồn này (với converter riêng của nguồn)
		value, err := extractValueFromSource(structVal, config)
		if err != nil {
			// Nếu optional và không tìm thấy, bỏ qua
			if config.Optional && strings.Contains(err.Error(), "không tìm thấy") {
				continue
			}
			// Nếu required và không tìm thấy, return error
			if config.Required && strings.Contains(err.Error(), "không tìm thấy") {
				return err
			}
			// Nếu optional và có lỗi convert, bỏ qua
			if config.Optional {
				continue
			}
			return err
		}

		values = append(values, extractedValue{
			value:  value,
			config: config,
		})
	}

	if len(values) == 0 {
		// Không có nguồn nào có data, kiểm tra default
		for _, config := range configs {
			if config.Default != "" {
				return setFieldValue(targetField, config.Default, config)
			}
		}
		// Nếu tất cả đều optional, bỏ qua
		allOptional := true
		for _, config := range configs {
			if !config.Optional {
				allOptional = false
				break
			}
		}
		if allOptional {
			return nil // Bỏ qua field này
		}
		return fmt.Errorf("không tìm thấy giá trị từ bất kỳ nguồn nào")
	}

	// Áp dụng merge strategy
	return applyMergeStrategy(targetField, values)
}

// extractValueFromSource extract giá trị từ một nguồn (với converter riêng của nguồn)
func extractValueFromSource(structVal reflect.Value, config *extractTagConfig) (interface{}, error) {
	if len(config.SourcePath) < 2 {
		return nil, fmt.Errorf("source path phải có ít nhất 2 phần")
	}

	sourceFieldName := config.SourcePath[0]
	pathParts := config.SourcePath[1:]

	// Tìm source field
	sourceField := structVal.FieldByName(sourceFieldName)
	if !sourceField.IsValid() {
		return nil, fmt.Errorf("không tìm thấy source field: %s", sourceFieldName)
	}

	// Convert sang map
	sourceMap, ok := sourceField.Interface().(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("source field %s không phải là map[string]interface{}", sourceFieldName)
	}

	// Traverse nested path
	var value interface{} = sourceMap
	for _, part := range pathParts {
		if value == nil {
			return nil, fmt.Errorf("không tìm thấy path '%s' trong source", strings.Join(config.SourcePath, " -> "))
		}

		mapValue, ok := value.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("không thể traverse path, giá trị tại '%s' không phải là map", part)
		}

		nextValue, exists := mapValue[part]
		if !exists {
			return nil, fmt.Errorf("không tìm thấy path '%s' trong source", strings.Join(config.SourcePath, " -> "))
		}

		value = nextValue
	}

	if value == nil {
		return nil, fmt.Errorf("giá trị tại path '%s' là nil", strings.Join(config.SourcePath, " -> "))
	}

	// Apply converter (riêng cho nguồn này)
	convertedValue, err := applyConverter(value, config.Converter, config.Format)
	if err != nil {
		return nil, fmt.Errorf("convert value từ nguồn %s: %w", sourceFieldName, err)
	}

	return convertedValue, nil
}

// applyMergeStrategy áp dụng chiến lược merge
func applyMergeStrategy(targetField reflect.Value, values []extractedValue) error {
	if len(values) == 0 {
		return fmt.Errorf("không có giá trị nào")
	}

	// Lấy merge strategy từ config đầu tiên (tất cả configs nên có cùng strategy)
	strategy := values[0].config.MergeStrategy
	if strategy == "" {
		strategy = "overwrite" // Mặc định
	}

	switch strategy {
	case "merge_array":
		// Merge tất cả giá trị vào array (loại bỏ duplicate)
		// Chỉ áp dụng cho slice/array fields
		if targetField.Type().Kind() != reflect.Slice {
			return fmt.Errorf("merge_array chỉ áp dụng cho slice/array fields")
		}

		// Collect tất cả giá trị
		allValues := make([]interface{}, 0)
		for _, v := range values {
			val := reflect.ValueOf(v.value)
			if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
				for i := 0; i < val.Len(); i++ {
					allValues = append(allValues, val.Index(i).Interface())
				}
			} else {
				allValues = append(allValues, v.value)
			}
		}

		// Loại bỏ duplicate
		uniqueValues := removeDuplicates(allValues)

		// Tạo slice mới
		elemType := targetField.Type().Elem()
		newSlice := reflect.MakeSlice(targetField.Type(), len(uniqueValues), len(uniqueValues))
		for i, val := range uniqueValues {
			valVal := reflect.ValueOf(val)
			if valVal.Type().AssignableTo(elemType) {
				newSlice.Index(i).Set(valVal)
			} else if valVal.Type().ConvertibleTo(elemType) {
				newSlice.Index(i).Set(valVal.Convert(elemType))
			}
		}

		targetField.Set(newSlice)
		return nil

	case "keep_existing":
		// Giữ giá trị hiện có nếu đã có, nếu không lấy từ nguồn đầu tiên
		if !targetField.IsZero() {
			return nil // Giữ nguyên giá trị hiện có
		}
		return setFieldValue(targetField, values[0].value, values[0].config)

	case "priority":
		// Chọn giá trị từ nguồn có priority nhỏ nhất (ưu tiên cao nhất)
		priorityValue := values[0]
		for _, v := range values[1:] {
			priority1 := priorityValue.config.Priority
			priority2 := v.config.Priority

			// Priority = 0 → ưu tiên thấp nhất (số lớn)
			if priority1 == 0 {
				priority1 = 999999
			}
			if priority2 == 0 {
				priority2 = 999999
			}

			if priority2 < priority1 {
				priorityValue = v
			}
		}
		return setFieldValue(targetField, priorityValue.value, priorityValue.config)

	case "overwrite":
		fallthrough
	default:
		// Mặc định: ghi đè bằng giá trị từ nguồn đầu tiên
		return setFieldValue(targetField, values[0].value, values[0].config)
	}
}

// removeDuplicates loại bỏ duplicate trong array
func removeDuplicates(values []interface{}) []interface{} {
	seen := make(map[string]bool)
	result := make([]interface{}, 0)

	for _, val := range values {
		key := fmt.Sprintf("%v", val) // Convert sang string để so sánh
		if !seen[key] {
			seen[key] = true
			result = append(result, val)
		}
	}

	return result
}
