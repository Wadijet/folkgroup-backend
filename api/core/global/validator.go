package global

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// InitValidator khởi tạo và đăng ký các custom validator
func InitValidator() {
	// Khởi tạo validator
	Validate = validator.New()

	// Đăng ký các custom validator
	_ = Validate.RegisterValidation("no_xss", validateNoXSS)
	_ = Validate.RegisterValidation("no_sql_injection", validateNoSQLInjection)
	_ = Validate.RegisterValidation("strong_password", validateStrongPassword)
	_ = Validate.RegisterValidation("exists", validateExists)
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
