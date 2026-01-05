package notification

// SeverityPriority mapping (1 = highest priority)
// Dùng để sort queue items - priority thấp hơn = xử lý trước
var SeverityPriority = map[string]int{
	SeverityCritical: 1, // Critical: xử lý đầu tiên
	SeverityHigh:     2, // High: xử lý thứ 2
	SeverityMedium:   3, // Medium: xử lý thứ 3
	SeverityLow:      4, // Low: xử lý thứ 4
	SeverityInfo:     5, // Info: xử lý cuối cùng
}

// SeverityMaxRetries mapping
// Số lần retry tối đa dựa trên severity
var SeverityMaxRetries = map[string]int{
	SeverityCritical: 10, // Critical: retry nhiều hơn
	SeverityHigh:     5,  // High: retry 5 lần
	SeverityMedium:   3,  // Medium: retry 3 lần (default)
	SeverityLow:      2,  // Low: retry 2 lần
	SeverityInfo:     1,  // Info: retry ít nhất
}

// SeverityThrottleSeconds mapping
// Thời gian throttle (giây) giữa các notification cùng severity
// 0 = không throttle
var SeverityThrottleSeconds = map[string]int{
	SeverityCritical: 0,   // Critical: không throttle
	SeverityHigh:     0,   // High: không throttle
	SeverityMedium:   60,  // Medium: throttle 1 phút
	SeverityLow:      300, // Low: throttle 5 phút
	SeverityInfo:     900, // Info: throttle 15 phút
}

// GetPriorityFromSeverity tính priority từ severity
// Trả về priority (1-5), default = 3 (medium)
func GetPriorityFromSeverity(severity string) int {
	priority := SeverityPriority[severity]
	if priority == 0 {
		return 3 // Default medium
	}
	return priority
}

// GetMaxRetriesFromSeverity tính maxRetries từ severity
// Trả về số lần retry tối đa, default = 3
func GetMaxRetriesFromSeverity(severity string) int {
	maxRetries := SeverityMaxRetries[severity]
	if maxRetries == 0 {
		return 3 // Default
	}
	return maxRetries
}

// GetRecommendedChannels trả về danh sách channels được khuyến nghị cho severity
// Dùng để gợi ý khi tạo routing rules
func GetRecommendedChannels(severity string) []string {
	switch severity {
	case SeverityCritical:
		return []string{"email", "telegram", "webhook"} // Tất cả kênh
	case SeverityHigh:
		return []string{"email", "telegram"} // Email + Telegram
	case SeverityMedium:
		return []string{"email", "telegram"} // Email + Telegram
	case SeverityLow:
		return []string{"email"} // Chỉ email
	case SeverityInfo:
		return []string{"email"} // Chỉ email (có thể throttle)
	default:
		return []string{"email"} // Default
	}
}
