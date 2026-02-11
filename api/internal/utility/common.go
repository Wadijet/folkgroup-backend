package utility

import (
	"fmt"
	"regexp"
	"time"

	"encoding/json"

	"meta_commerce/internal/common"
	"meta_commerce/internal/logger"
)

// GoProtect là một hàm bao bọc (wrapper) giúp bảo vệ một hàm khác khỏi bị panic.
// Nếu xảy ra panic trong hàm f(), GoProtect sẽ bắt lại và in ra lỗi thay vì làm chương trình dừng hẳn.
func GoProtect(f func()) {
	defer func() {
		// Sử dụng recover() để bắt lỗi panic nếu có
		if err := recover(); err != nil {
			fmt.Printf("Đã bắt lỗi panic: %v\n", err)
		}
	}()

	// Gọi hàm f() được truyền vào
	f()
}

// Describe mô tả kiểu và giá trị của interface
// @params - interface cần mô tả
func Describe(t interface{}) {
	fmt.Printf("Interface type %T value %v\n", t, t)
}

// PrettyPrint in đẹp một interface dưới dạng JSON
// @params - interface cần in đẹp
// @returns - chuỗi JSON đẹp
func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

// UnixMilli dùng để lấy mili giây của thời gian cho trước
// @params - thời gian
// @returns - mili giây của thời gian cho trước
func UnixMilli(t time.Time) int64 {
	return t.Round(time.Millisecond).UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}

// CurrentTimeInMilli dùng để lấy thời gian hiện tại tính bằng mili giây
// Hàm này sẽ được sử dụng khi cần timestamp hiện tại
// @returns - timestamp hiện tại (tính bằng mili giây)
func CurrentTimeInMilli() int64 {
	return UnixMilli(time.Now())
}

// LogWarning ghi log cảnh báo với các thông tin bổ sung
func LogWarning(msg string, args ...interface{}) {
	// Tạo map fields từ args
	fields := make(map[string]interface{})
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			if key, ok := args[i].(string); ok {
				fields[key] = args[i+1]
			}
		}
	}
	logger.GetAppLogger().WithFields(fields).Warn(msg)
}

// ValidateEmail kiểm tra định dạng email
func ValidateEmail(email string) error {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return common.ErrInvalidEmail
	}
	return nil
}

// ValidatePassword kiểm tra độ mạnh của mật khẩu
// DEPRECATED: Không còn sử dụng - Firebase quản lý authentication và password
// Giữ lại để tương thích ngược, nhưng không nên sử dụng
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return common.ErrWeakPassword
	}
	return nil
}
