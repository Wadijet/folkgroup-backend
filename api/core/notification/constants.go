package notification

// Domain constants - Phân loại theo chức năng/lĩnh vực
const (
	DomainSystem      = "system"      // Hệ thống, database, API errors
	DomainConversation = "conversation" // Chat, message, reply
	DomainOrder       = "order"       // Đơn hàng, payment
	DomainUser        = "user"        // User management, authentication
	DomainSecurity    = "security"    // Security alerts, login failed
	DomainPayment     = "payment"     // Payment processing
	DomainAnalytics   = "analytics"   // Analytics, reports
)

// Severity constants - Mức độ nghiêm trọng
const (
	SeverityCritical = "critical" // Cực kỳ nghiêm trọng - xử lý ngay
	SeverityHigh     = "high"     // Cao - xử lý sớm
	SeverityMedium   = "medium"   // Trung bình - xử lý trong giờ làm việc
	SeverityLow      = "low"      // Thấp - xử lý khi có thời gian
	SeverityInfo     = "info"     // Thông tin - chỉ log/ghi nhận
)
