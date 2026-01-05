package notification

import "strings"

// GetDomainFromEventType infer domain từ eventType
// Sử dụng pattern matching để phân loại event theo domain
func GetDomainFromEventType(eventType string) string {
	// Pattern matching
	if strings.HasPrefix(eventType, "system_") {
		return DomainSystem
	}
	if strings.HasPrefix(eventType, "conversation_") {
		return DomainConversation
	}
	if strings.HasPrefix(eventType, "order_") {
		return DomainOrder
	}
	if strings.HasPrefix(eventType, "user_") {
		return DomainUser
	}
	if strings.HasPrefix(eventType, "security_") || strings.Contains(eventType, "_alert") {
		return DomainSecurity
	}
	if strings.HasPrefix(eventType, "payment_") {
		return DomainPayment
	}
	if strings.HasPrefix(eventType, "analytics_") {
		return DomainAnalytics
	}
	return DomainSystem // Default
}

// GetSeverityFromEventType infer severity từ eventType
// Sử dụng pattern matching để xác định mức độ nghiêm trọng
func GetSeverityFromEventType(eventType string) string {
	// Pattern matching
	if strings.Contains(eventType, "_error") ||
		strings.Contains(eventType, "_critical") ||
		strings.Contains(eventType, "_down") {
		return SeverityCritical
	}
	if strings.Contains(eventType, "_failed") ||
		strings.Contains(eventType, "_alert") ||
		strings.Contains(eventType, "_timeout") {
		return SeverityHigh
	}
	if strings.Contains(eventType, "_warning") ||
		strings.Contains(eventType, "_unreplied") {
		return SeverityMedium
	}
	if strings.Contains(eventType, "_completed") ||
		strings.Contains(eventType, "_created") ||
		strings.Contains(eventType, "_updated") {
		return SeverityInfo
	}
	return SeverityMedium // Default
}
