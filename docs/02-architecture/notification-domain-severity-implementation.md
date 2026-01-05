# Phương Án Triển Khai: Domain và Severity cho Notification System

## 🎯 Mục Tiêu

Thêm Domain và Severity vào Notification System để:
1. **Tham khảo**: Biết notification thuộc domain nào, mức độ nghiêm trọng ra sao
2. **Rules xử lý**: Tự động quyết định routing, retry, priority dựa trên domain/severity
3. **Báo cáo**: Filter và phân tích notification theo domain/severity

## 📋 Phân Loại

### Domain (Lĩnh Vực)
```go
const (
    DomainSystem      = "system"      // Hệ thống, database, API errors
    DomainConversation = "conversation" // Chat, message, reply
    DomainOrder       = "order"       // Đơn hàng, payment
    DomainUser        = "user"        // User management, authentication
    DomainSecurity    = "security"    // Security alerts, login failed
    DomainPayment     = "payment"     // Payment processing
    DomainAnalytics   = "analytics"   // Analytics, reports
)
```

### Severity (Mức Độ Nghiêm Trọng)
```go
const (
    SeverityCritical = "critical" // Cực kỳ nghiêm trọng - xử lý ngay
    SeverityHigh     = "high"     // Cao - xử lý sớm
    SeverityMedium   = "medium"   // Trung bình - xử lý trong giờ làm việc
    SeverityLow      = "low"      // Thấp - xử lý khi có thời gian
    SeverityInfo     = "info"     // Thông tin - chỉ log/ghi nhận
)
```

## 🔧 Rules Xử Lý

### Rule 1: Infer Domain và Severity từ EventType

**Mục đích**: Tự động phân loại khi trigger notification

**Implementation**:
```go
// api/core/notification/classifier.go
package notification

import "strings"

// GetDomainFromEventType infer domain từ eventType
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
    return DomainSystem // Default
}

// GetSeverityFromEventType infer severity từ eventType
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
```

### Rule 2: Tính Priority và MaxRetries từ Severity

**Mục đích**: Xác định ưu tiên xử lý và số lần retry

**Implementation**:
```go
// api/core/notification/rules.go
package notification

// SeverityPriority mapping (1 = highest priority)
var SeverityPriority = map[string]int{
    SeverityCritical: 1,
    SeverityHigh:     2,
    SeverityMedium:   3,
    SeverityLow:      4,
    SeverityInfo:     5,
}

// SeverityMaxRetries mapping
var SeverityMaxRetries = map[string]int{
    SeverityCritical: 10, // Critical: retry nhiều hơn
    SeverityHigh:     5,
    SeverityMedium:   3,
    SeverityLow:      2,
    SeverityInfo:     1, // Info: retry ít nhất
}

// GetPriorityFromSeverity tính priority từ severity
func GetPriorityFromSeverity(severity string) int {
    priority := SeverityPriority[severity]
    if priority == 0 {
        return 3 // Default medium
    }
    return priority
}

// GetMaxRetriesFromSeverity tính maxRetries từ severity
func GetMaxRetriesFromSeverity(severity string) int {
    maxRetries := SeverityMaxRetries[severity]
    if maxRetries == 0 {
        return 3 // Default
    }
    return maxRetries
}
```

### Rule 3: Routing Rules theo Domain và Severity

**Mục đích**: Routing thông minh dựa trên domain và severity

**Ví dụ Rules**:
```go
// Rule 1: Tất cả event security → gửi cho security team
{
    Domain: "security",
    OrganizationIDs: [securityTeamID],
    ChannelTypes: ["email", "telegram"], // Critical → nhiều kênh
    Severities: ["critical", "high"],    // Chỉ nhận critical và high
}

// Rule 2: System errors → gửi cho devops team
{
    Domain: "system",
    OrganizationIDs: [devopsTeamID],
    ChannelTypes: ["email", "telegram", "webhook"], // Critical → tất cả kênh
    Severities: ["critical"], // Chỉ nhận critical
}

// Rule 3: Conversation unreplied → gửi cho support team
{
    EventType: "conversation_unreplied", // Có thể dùng EventType cụ thể
    OrganizationIDs: [supportTeamID],
    ChannelTypes: ["email", "telegram"],
    Severities: ["high", "medium"], // Không nhận info
}

// Rule 4: Order events → gửi cho sales team (chỉ info)
{
    Domain: "order",
    OrganizationIDs: [salesTeamID],
    ChannelTypes: ["email"], // Info → chỉ email
    Severities: ["info"], // Chỉ nhận info
}
```

**Implementation**:
```go
// Cập nhật NotificationRoutingRule model
type NotificationRoutingRule struct {
    // ... existing fields ...
    EventType string `json:"eventType,omitempty" bson:"eventType,omitempty" index:"single:1"`
    
    // NEW: Có thể routing theo Domain hoặc EventType (ưu tiên Domain nếu có)
    Domain *string `json:"domain,omitempty" bson:"domain,omitempty" index:"single:1"`
    
    // NEW: Filter theo Severity
    Severities []string `json:"severities,omitempty" bson:"severities,omitempty"`
    
    // ... existing fields ...
}

// Cập nhật Router.FindRoutes
func (r *Router) FindRoutes(ctx context.Context, eventType string, domain string, severity string) ([]Route, error) {
    // 1. Tìm rules theo EventType (nếu có)
    rules, _ := r.routingService.FindByEventType(ctx, eventType)
    
    // 2. Tìm rules theo Domain (nếu có)
    domainRules, _ := r.routingService.FindByDomain(ctx, domain)
    rules = append(rules, domainRules...)
    
    // 3. Filter theo Severity
    filteredRules := []models.NotificationRoutingRule{}
    for _, rule := range rules {
        if !rule.IsActive {
            continue
        }
        
        // Nếu rule có filter Severity, kiểm tra
        if len(rule.Severities) > 0 {
            severityMatched := false
            for _, s := range rule.Severities {
                if s == severity {
                    severityMatched = true
                    break
                }
            }
            if !severityMatched {
                continue // Bỏ qua rule này
            }
        }
        
        filteredRules = append(filteredRules, rule)
    }
    
    // 4. Tạo routes từ filtered rules
    // ... rest of logic
}
```

### Rule 4: Channel Selection theo Severity

**Mục đích**: Chọn kênh gửi phù hợp với mức độ nghiêm trọng

**Rules**:
```go
// Critical → Tất cả kênh (email + telegram + webhook)
// High → Email + Telegram
// Medium → Email + Telegram (optional)
// Low → Email
// Info → Email (có thể throttle)
```

**Implementation**:
```go
// api/core/notification/channel_selector.go
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
```

### Rule 5: Throttling theo Severity

**Mục đích**: Giảm spam cho notification không quan trọng

**Rules**:
```go
// Critical → Không throttle
// High → Không throttle
// Medium → Throttle 1 notification/phút
// Low → Throttle 1 notification/5 phút
// Info → Throttle 1 notification/15 phút
```

**Implementation**:
```go
// api/core/notification/throttler.go
var SeverityThrottleSeconds = map[string]int{
    SeverityCritical: 0,  // Không throttle
    SeverityHigh:     0,  // Không throttle
    SeverityMedium:   60, // 1 phút
    SeverityLow:      300, // 5 phút
    SeverityInfo:     900, // 15 phút
}
```

## 📊 Ví Dụ EventType Mapping

### System Domain
```go
"system_startup"     → Domain: "system", Severity: "info"
"system_shutdown"    → Domain: "system", Severity: "high"
"system_error"       → Domain: "system", Severity: "critical"
"system_warning"     → Domain: "system", Severity: "medium"
"database_error"     → Domain: "system", Severity: "critical"
"api_error"          → Domain: "system", Severity: "high"
"backup_completed"   → Domain: "system", Severity: "info"
"backup_failed"      → Domain: "system", Severity: "high"
```

### Conversation Domain
```go
"conversation_unreplied" → Domain: "conversation", Severity: "high"
"conversation_new"       → Domain: "conversation", Severity: "medium"
"conversation_closed"    → Domain: "conversation", Severity: "info"
```

### Order Domain
```go
"order_created"  → Domain: "order", Severity: "info"
"order_failed"   → Domain: "order", Severity: "high"
"order_cancelled" → Domain: "order", Severity: "medium"
```

### Security Domain
```go
"security_alert"        → Domain: "security", Severity: "critical"
"user_login_failed"     → Domain: "security", Severity: "medium"
"unauthorized_access"   → Domain: "security", Severity: "critical"
```

## 🏗️ Implementation Plan

### Phase 1: Thêm Constants và Helpers
- [ ] Tạo `api/core/notification/constants.go` với Domain và Severity constants
- [ ] Tạo `api/core/notification/classifier.go` với functions infer domain/severity
- [ ] Tạo `api/core/notification/rules.go` với priority và retry rules

### Phase 2: Cập Nhật Models
- [ ] Thêm `Domain`, `Severity`, `Priority` vào `DeliveryQueueItem`
- [ ] Thêm `Domain`, `Severities` vào `NotificationRoutingRule`
- [ ] Update indexes

### Phase 3: Cập Nhật Logic
- [ ] Update `handler.notification.trigger.go` để set domain/severity khi tạo queue item
- [ ] Update `notification/router.go` để support routing theo domain và filter theo severity
- [ ] Update `delivery/queue.go` để sort theo priority khi dequeue

### Phase 4: Rules và Configuration
- [ ] Tạo default routing rules theo domain
- [ ] Tạo mapping table cho eventType → domain/severity
- [ ] Update init scripts

## 📝 Usage Examples

### Example 1: Trigger với Auto Classification
```go
// Trigger notification
POST /notification/trigger
{
    "eventType": "system_error",
    "payload": {
        "errorMessage": "Database connection failed"
    }
}

// System tự động:
// - Domain: "system"
// - Severity: "critical"
// - Priority: 1
// - MaxRetries: 10
// - Routing: Tìm rules có Domain="system" và Severities chứa "critical"
```

### Example 2: Routing Rule theo Domain
```go
// Tạo rule: Tất cả event security → gửi cho security team
POST /notification/routing
{
    "domain": "security",
    "organizationIds": ["security_team_id"],
    "channelTypes": ["email", "telegram"],
    "severities": ["critical", "high"]
}
```

### Example 3: Query theo Domain/Severity
```go
// Lấy tất cả critical notifications
GET /notification/history?severity=critical

// Lấy tất cả security notifications
GET /notification/history?domain=security

// Lấy critical security notifications
GET /notification/history?domain=security&severity=critical
```

## ✅ Lợi Ích

1. **Tự động hóa**: Không cần config từng event, system tự infer
2. **Linh hoạt**: Có thể routing theo domain hoặc eventType cụ thể
3. **Thông minh**: Priority và retry tự động dựa trên severity
4. **Báo cáo**: Dễ dàng filter và phân tích theo domain/severity
5. **Mở rộng**: Dễ thêm domain/severity mới

## 🔄 Migration

### Backward Compatibility
- EventType vẫn hoạt động như cũ
- Domain và Severity là optional (có thể infer)
- Routing rules cũ vẫn hoạt động (không có Domain/Severity filter)

### Data Migration
- Không cần migration data (domain/severity sẽ được infer khi trigger mới)
- Có thể tạo script để set domain/severity cho history nếu cần
