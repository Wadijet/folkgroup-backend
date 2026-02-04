package global

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"regexp"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ConfigConstraints cấu trúc parse từ JSON string (field constraints của org config).
// Dùng cho validator "config_value" — kiểm tra value theo dataType và constraints.
type ConfigConstraints struct {
	Enum       []interface{} `json:"enum,omitempty"`
	Minimum    *float64      `json:"minimum,omitempty"`
	Maximum    *float64      `json:"maximum,omitempty"`
	MultipleOf *float64      `json:"multipleOf,omitempty"`
	MinLength  *int          `json:"minLength,omitempty"`
	MaxLength  *int          `json:"maxLength,omitempty"`
	Pattern    string        `json:"pattern,omitempty"`
	MinItems   *int          `json:"minItems,omitempty"`
	MaxItems   *int          `json:"maxItems,omitempty"`
}

// InitValidator khởi tạo và đăng ký các custom validator
func InitValidator() {
	// Khởi tạo validator
	Validate = validator.New()

	// Đăng ký các custom validator
	_ = Validate.RegisterValidation("no_xss", validateNoXSS)
	_ = Validate.RegisterValidation("no_sql_injection", validateNoSQLInjection)
	_ = Validate.RegisterValidation("strong_password", validateStrongPassword)
	_ = Validate.RegisterValidation("exists", validateExists)
	_ = Validate.RegisterValidation("config_value", validateConfigValue)
}

// validateNoXSS kiểm tra XSS
func validateNoXSS(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	dangerousPatterns := []string{
		"<script",
		"javascript:",
		"onerror=",
		"onload=",
		"onclick=",
		"onmouseover=",
		"eval(",
		"document.cookie",
		"document.write",
		"innerHTML",
		"fromCharCode",
		"window.location",
		"<iframe",
		"<object",
		"<embed",
	}

	value = strings.ToLower(value)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(value, pattern) {
			return false
		}
	}
	return true
}

// validateNoSQLInjection kiểm tra SQL Injection
func validateNoSQLInjection(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	sqlPatterns := []string{
		"'",
		";",
		"--",
		"/*",
		"*/",
		"xp_",
		"SELECT",
		"DROP",
		"DELETE",
		"UPDATE",
		"INSERT",
		"UNION",
		"OR 1=1",
		"OR '1'='1",
		"OR 'a'='a",
		"OR 1 = 1",
		"WAITFOR",
		"DELAY",
		"BENCHMARK",
	}

	value = strings.ToUpper(value)
	for _, pattern := range sqlPatterns {
		if strings.Contains(value, strings.ToUpper(pattern)) {
			return false
		}
	}
	return true
}

// validateStrongPassword kiểm tra mật khẩu mạnh
// DEPRECATED: Không còn sử dụng - Firebase quản lý authentication và password
// Giữ lại để tương thích ngược, nhưng không nên sử dụng
func validateStrongPassword(fl validator.FieldLevel) bool {
	value := fl.Field().String()

	// Kiểm tra độ dài tối thiểu
	if len(value) < 8 {
		return false
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range value {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	// Yêu cầu ít nhất 3 trong 4 điều kiện
	conditions := 0
	if hasUpper {
		conditions++
	}
	if hasLower {
		conditions++
	}
	if hasNumber {
		conditions++
	}
	if hasSpecial {
		conditions++
	}

	return conditions >= 3
}

// validateEmail kiểm tra định dạng email
func validateEmail(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(value)
}

// validateExists kiểm tra ObjectID tồn tại trong collection (foreign key validation)
// Format: validate:"exists=<collection_name>"
// Ví dụ: validate:"exists=ai_provider_profiles"
func validateExists(fl validator.FieldLevel) bool {
	value := fl.Field()

	// Lấy collection name từ param
	collectionName := fl.Param()
	if collectionName == "" {
		return false
	}

	// Convert value sang ObjectID
	var objID primitive.ObjectID
	switch v := value.Interface().(type) {
	case string:
		if v == "" {
			return true // Empty string = optional, skip validation (nếu có omitempty)
		}
		var err error
		objID, err = primitive.ObjectIDFromHex(v)
		if err != nil {
			return false
		}
	case primitive.ObjectID:
		if v == primitive.NilObjectID {
			return true // Nil ObjectID = optional, skip validation
		}
		objID = v
	case *primitive.ObjectID:
		if v == nil {
			return true // Nil pointer = optional, skip validation
		}
		objID = *v
	default:
		// Không phải ObjectID → không validate
		return false
	}

	// Lấy collection từ registry
	collection, exist := RegistryCollections.Get(collectionName)
	if !exist {
		// Collection không tồn tại trong registry → không thể validate
		return false
	}

	// Query database để check tồn tại
	ctx := context.Background()
	count, err := collection.CountDocuments(ctx, bson.M{"_id": objID})
	if err != nil {
		return false
	}

	return count > 0
}

// validateConfigValue kiểm tra value theo dataType và constraints (JSON string).
// Struct dùng tag phải có 3 field: Value (tag validate:"config_value"), DataType, Constraints.
// Constraints rỗng = bỏ qua (return true).
func validateConfigValue(fl validator.FieldLevel) bool {
	parent := fl.Parent()
	if !parent.IsValid() {
		return true
	}
	constraintsField := parent.FieldByName("Constraints")
	dataTypeField := parent.FieldByName("DataType")
	if !constraintsField.IsValid() || !dataTypeField.IsValid() {
		return true
	}
	constraintsJSON := constraintsField.String()
	if constraintsJSON == "" {
		return true
	}
	dataType := strings.TrimSpace(strings.ToLower(dataTypeField.String()))
	var c ConfigConstraints
	if err := json.Unmarshal([]byte(constraintsJSON), &c); err != nil {
		return false
	}
	val := fl.Field().Interface()
	return validateConfigValueWithConstraints(val, dataType, &c)
}

// validateConfigValueWithConstraints áp dụng ràng buộc lên value theo dataType.
func validateConfigValueWithConstraints(val interface{}, dataType string, c *ConfigConstraints) bool {
	// enum: giá trị phải nằm trong danh sách
	if len(c.Enum) > 0 {
		ok := false
		for _, e := range c.Enum {
			if reflect.DeepEqual(e, val) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	switch dataType {
	case "string":
		s, ok := val.(string)
		if !ok {
			return false
		}
		if c.MinLength != nil && len(s) < *c.MinLength {
			return false
		}
		if c.MaxLength != nil && len(s) > *c.MaxLength {
			return false
		}
		if c.Pattern != "" {
			re, err := regexp.Compile(c.Pattern)
			if err != nil {
				return false
			}
			if !re.MatchString(s) {
				return false
			}
		}
	case "number", "integer":
		var f float64
		switch v := val.(type) {
		case float64:
			f = v
		case int:
			f = float64(v)
		case int64:
			f = float64(v)
		case int32:
			f = float64(v)
		default:
			return false
		}
		if c.Minimum != nil && f < *c.Minimum {
			return false
		}
		if c.Maximum != nil && f > *c.Maximum {
			return false
		}
		if c.MultipleOf != nil && *c.MultipleOf != 0 {
			q := math.Floor(f / *c.MultipleOf)
			remainder := f - *c.MultipleOf*q
			if math.Abs(remainder) > 1e-9 {
				return false
			}
		}
	case "array":
		rv := reflect.ValueOf(val)
		if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
			return false
		}
		n := rv.Len()
		if c.MinItems != nil && n < *c.MinItems {
			return false
		}
		if c.MaxItems != nil && n > *c.MaxItems {
			return false
		}
	case "boolean", "object":
		// enum đã kiểm tra ở trên; không thêm ràng buộc đặc biệt
	}
	return true
}
