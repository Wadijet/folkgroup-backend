# Đề Xuất: Phân Loại Notification Theo Domain và Severity

## 📋 Tổng Quan

Hiện tại hệ thống notification chỉ phân loại theo `EventType`. Đề xuất thêm 2 trường phân loại:
- **Domain**: Phân loại theo chức năng/lĩnh vực
- **Severity**: Mức độ nghiêm trọng

## 🏗️ Phân Chia Trách Nhiệm (Quan Trọng!)

### Notification Module (`api/core/notification/`)
**Trách nhiệm**: Xử lý logic nghiệp vụ notification
- ✅ **Infer và set** Domain/Severity từ EventType
- ✅ **Tính toán** Priority và MaxRetries từ Severity
- ✅ **Routing logic** có thể filter theo Domain/Severity
- ✅ **Tạo NotificationQueueItem** với đầy đủ thông tin (Domain, Severity, Priority, MaxRetries)

### Delivery Module (`api/core/delivery/`)
**Trách nhiệm**: Xử lý việc gửi notification (như "bưu điện")
- ✅ **Chỉ dùng** các field đã được set sẵn (Priority, MaxRetries)
- ✅ **Priority queue**: Sort theo Priority khi dequeue
- ❌ **KHÔNG** infer Domain/Severity (vì đã được set ở Notification module)
- ❌ **KHÔNG** tính MaxRetries từ Severity (vì đã được set sẵn)

**Lý do**: Delivery module là "dumb" service, chỉ cần biết "gửi cái gì, gửi cho ai, gửi như thế nào". Logic nghiệp vụ (domain/severity) nằm ở Notification module.

## 🎯 Lợi Ích

### 1. **Domain** - Phân Loại Theo Chức Năng
- **Mục đích**: Nhóm các event theo lĩnh vực xử lý
- **Ví dụ**: `system`, `conversation`, `order`, `user`, `security`, `payment`, `analytics`
- **Lợi ích**:
  - Dễ dàng filter và báo cáo theo domain
  - Routing rules có thể áp dụng cho cả domain (ví dụ: tất cả event `security` → gửi cho security team)
  - Quản lý permissions theo domain (ví dụ: team chỉ nhận notification của domain `conversation`)

### 2. **Severity** - Mức Độ Nghiêm Trọng
- **Mục đích**: Xác định mức độ ưu tiên và cách xử lý
- **Các mức độ**:
  - `critical`: Cực kỳ nghiêm trọng, cần xử lý ngay lập tức
  - `high`: Cao, cần xử lý sớm
  - `medium`: Trung bình, xử lý trong giờ làm việc
  - `low`: Thấp, xử lý khi có thời gian
  - `info`: Thông tin, chỉ cần log/ghi nhận
- **Lợi ích**:
  - **Routing thông minh**: Critical → nhiều kênh (email + telegram + webhook), Info → chỉ email
  - **Retry logic**: Critical → retry nhiều hơn (5-10 lần), Info → retry ít hơn (1-2 lần)
  - **Priority queue**: Critical → xử lý trước, Info → xử lý sau
  - **Escalation rules**: Critical → gọi ngay, gửi SMS, Info → chỉ log
  - **Throttling**: Critical → không throttle, Info → có thể throttle

## 📊 Ví Dụ Phân Loại

### Domain Mapping
```go
// Ví dụ mapping EventType → Domain
"system_startup"     → Domain: "system"
"system_error"       → Domain: "system", Severity: "critical"
"system_warning"    → Domain: "system", Severity: "medium"
"database_error"    → Domain: "system", Severity: "critical"
"conversation_unreplied" → Domain: "conversation", Severity: "high"
"order_created"      → Domain: "order", Severity: "info"
"order_failed"       → Domain: "order", Severity: "high"
"security_alert"     → Domain: "security", Severity: "critical"
"user_login_failed"  → Domain: "security", Severity: "medium"
```

## 🏗️ Kiến Trúc Đề Xuất

### 1. Cập Nhật Models

#### NotificationQueueItem
```go
type NotificationQueueItem struct {
    // ... existing fields ...
    EventType string `json:"eventType" bson:"eventType" index:"single:1"`
    
    // NEW: Domain và Severity
    Domain   string `json:"domain" bson:"domain" index:"single:1"`        // system, conversation, order, user, security, payment
    Severity string `json:"severity" bson:"severity" index:"single:1"`     // critical, high, medium, low, info
    
    // NEW: Priority (tính từ Severity, để sort queue)
    Priority int `json:"priority" bson:"priority" index:"single:1"`        // 1=critical, 2=high, 3=medium, 4=low, 5=info
    
    // ... existing fields ...
    MaxRetries int `json:"maxRetries" bson:"maxRetries"` // Sẽ được tính từ Severity
}
```

#### NotificationRoutingRule
```go
type NotificationRoutingRule struct {
    // ... existing fields ...
    EventType string `json:"eventType" bson:"eventType" index:"single:1"`
    
    // NEW: Có thể routing theo Domain hoặc EventType
    Domain *string `json:"domain,omitempty" bson:"domain,omitempty" index:"single:1"` // null = dùng EventType
    
    // NEW: Filter theo Severity
    Severities []string `json:"severities,omitempty" bson:"severities,omitempty"` // ["critical", "high"] - chỉ nhận các severity này
    
    // ... existing fields ...
}
```

#### NotificationTemplate
```go
type NotificationTemplate struct {
    // ... existing fields ...
    EventType string `json:"eventType" bson:"eventType" index:"single:1"`
    
    // NEW: Domain và Severity (optional, có thể infer từ EventType)
    Domain   *string `json:"domain,omitempty" bson:"domain,omitempty" index:"single:1"`
    Severity *string `json:"severity,omitempty" bson:"severity,omitempty" index:"single:1"`
    
    // ... existing fields ...
}
```

### 2. Constants và Helpers

#### notification/constants.go
```go
package notification

// Domain constants
const (
    DomainSystem      = "system"
    DomainConversation = "conversation"
    DomainOrder       = "order"
    DomainUser        = "user"
    DomainSecurity    = "security"
    DomainPayment     = "payment"
    DomainAnalytics   = "analytics"
)

// Severity constants
const (
    SeverityCritical = "critical"
    SeverityHigh     = "high"
    SeverityMedium   = "medium"
    SeverityLow      = "low"
    SeverityInfo     = "info"
)

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

// GetDomainFromEventType infer domain từ eventType
func GetDomainFromEventType(eventType string) string {
    // Logic mapping eventType → domain
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
    // Logic mapping eventType → severity
    if strings.Contains(eventType, "_error") || strings.Contains(eventType, "_critical") {
        return SeverityCritical
    }
    if strings.Contains(eventType, "_failed") || strings.Contains(eventType, "_alert") {
        return SeverityHigh
    }
    if strings.Contains(eventType, "_warning") {
        return SeverityMedium
    }
    if strings.Contains(eventType, "_completed") || strings.Contains(eventType, "_created") {
        return SeverityInfo
    }
    return SeverityMedium // Default
}
```

### 3. Cập Nhật Logic

#### ⚠️ QUAN TRỌNG: Phân Chia Trách Nhiệm

**Notification Module** (`api/core/notification/`):
- ✅ **Set Domain và Severity** khi tạo NotificationQueueItem
- ✅ **Infer Domain/Severity** từ EventType (helper functions)
- ✅ **Set Priority và MaxRetries** dựa trên Severity
- ✅ **Routing logic** có thể filter theo Domain/Severity

**Delivery Module** (`api/core/delivery/`):
- ✅ **Chỉ dùng** các field đã được set sẵn (Priority, MaxRetries, Domain, Severity)
- ✅ **Priority queue**: Sort theo Priority khi dequeue
- ❌ **KHÔNG** infer Domain/Severity (vì đã được set ở Notification module)
- ❌ **KHÔNG** tính MaxRetries từ Severity (vì đã được set sẵn)

#### Notification Module - Set Domain/Severity khi tạo QueueItem

```go
// Trong handler.notification.trigger.go (dòng 241-258)
for _, recipient := range recipients {
    // Infer Domain và Severity từ EventType
    domain := notification.GetDomainFromEventType(req.EventType)
    severity := notification.GetSeverityFromEventType(req.EventType)
    
    // Set Priority và MaxRetries dựa trên Severity
    priority := notification.SeverityPriority[severity]
    if priority == 0 {
        priority = 3 // Default medium
    }
    
    maxRetries := notification.SeverityMaxRetries[severity]
    if maxRetries == 0 {
        maxRetries = 3 // Default
    }
    
    queueItems = append(queueItems, &models.NotificationQueueItem{
        ID:                  primitive.NewObjectID(),
        EventType:           req.EventType,
        Domain:              domain,      // ✅ Set ở đây
        Severity:            severity,   // ✅ Set ở đây
        Priority:            priority,   // ✅ Set ở đây
        OwnerOrganizationID: route.OrganizationID,
        SenderID:            senderID,
        SenderConfig:        encryptedSenderConfig,
        ChannelType:         channel.ChannelType,
        Recipient:           recipient,
        Subject:             rendered.Subject,
        Content:             rendered.Content,
        CTAs:                ctaJSONs,
        Payload:             req.Payload,
        Status:              "pending",
        RetryCount:          0,
        MaxRetries:          maxRetries, // ✅ Set ở đây (từ Severity)
        CreatedAt:           time.Now().Unix(),
        UpdatedAt:           time.Now().Unix(),
    })
}
```

#### Notification Module - Routing với Domain/Severity

```go
// Trong notification/router.go
func (r *Router) FindRoutes(ctx context.Context, eventType string, domain string, severity string) ([]Route, error) {
    // 1. Tìm rules theo EventType
    rules, _ := r.routingService.FindByEventType(ctx, eventType)
    
    // 2. Tìm rules theo Domain (nếu có)
    domainRules, _ := r.routingService.FindByDomain(ctx, domain)
    rules = append(rules, domainRules...)
    
    // 3. Filter theo Severity (nếu rule có filter)
    filteredRules := []models.NotificationRoutingRule{}
    for _, rule := range rules {
        if len(rule.Severities) == 0 || contains(rule.Severities, severity) {
            filteredRules = append(filteredRules, rule)
        }
    }
    
    // ... rest of logic
}
```

#### Delivery Module - Priority Queue (chỉ dùng Priority đã set sẵn)

```go
// Trong delivery/queue.go
// ⚠️ KHÔNG set Domain/Severity/Priority ở đây, chỉ dùng giá trị đã có

// Enqueue - chỉ set timestamp, không thay đổi Domain/Severity/Priority
func (q *Queue) Enqueue(ctx context.Context, items []*models.NotificationQueueItem) error {
    now := time.Now().Unix()
    for _, item := range items {
        item.Status = "pending"
        item.RetryCount = 0
        // ⚠️ KHÔNG set MaxRetries ở đây (đã được set ở Notification module)
        // ⚠️ KHÔNG set Priority ở đây (đã được set ở Notification module)
        item.CreatedAt = now
        item.UpdatedAt = now
    }
    // ...
}

// Dequeue - sort theo Priority (đã được set sẵn)
func (q *Queue) Dequeue(ctx context.Context, limit int) ([]*models.NotificationQueueItem, error) {
    // Sort theo Priority (1 = critical, xử lý trước)
    // Priority đã được set ở Notification module
    items, err := q.queueService.FindPendingWithPriority(ctx, limit)
    // ...
}
```

#### Routing với Domain và Severity
```go
// Trong notification/router.go
func (r *Router) FindRoutes(ctx context.Context, eventType string, domain string, severity string) ([]Route, error) {
    // 1. Tìm rules theo EventType
    rules, _ := r.routingService.FindByEventType(ctx, eventType)
    
    // 2. Tìm rules theo Domain (nếu có)
    domainRules, _ := r.routingService.FindByDomain(ctx, domain)
    rules = append(rules, domainRules...)
    
    // 3. Filter theo Severity (nếu rule có filter)
    filteredRules := []models.NotificationRoutingRule{}
    for _, rule := range rules {
        if len(rule.Severities) == 0 || contains(rule.Severities, severity) {
            filteredRules = append(filteredRules, rule)
        }
    }
    
    // ... rest of logic
}
```

## 🔄 Migration Plan

### Phase 1: Thêm Fields (Backward Compatible)
1. Thêm `Domain`, `Severity`, `Priority` vào models (optional fields)
2. Tạo helper functions để infer domain/severity từ eventType
3. Update Enqueue để tự động set domain/severity nếu chưa có

### Phase 2: Update Logic
1. Update Dequeue để sort theo Priority
2. Update Retry logic để dùng SeverityMaxRetries
3. Update Router để support routing theo Domain

### Phase 3: Migration Data
1. Script migration để set domain/severity cho các notification cũ
2. Update templates và routing rules

### Phase 4: New Features
1. Escalation rules dựa trên Severity
2. Throttling logic dựa trên Severity
3. Dashboard/reporting theo Domain và Severity

## 📝 Ví Dụ Sử Dụng

### 1. Trigger Notification với Domain và Severity
```go
// Tự động infer từ eventType
triggerReq := TriggerNotificationRequest{
    EventType: "system_error",
    Payload: map[string]interface{}{
        "errorMessage": "Database connection failed",
    },
}
// System sẽ tự động set:
// - Domain: "system"
// - Severity: "critical"
// - Priority: 1
// - MaxRetries: 10
```

### 2. Routing Rule theo Domain
```go
// Rule: Tất cả event security → gửi cho security team
rule := NotificationRoutingRule{
    Domain: "security",
    OrganizationIDs: []primitive.ObjectID{securityTeamID},
    ChannelTypes: []string{"email", "telegram"}, // Critical → nhiều kênh
    Severities: []string{"critical", "high"},    // Chỉ nhận critical và high
}
```

### 3. Priority Queue
```go
// Dequeue sẽ tự động ưu tiên:
// 1. Critical notifications (Priority = 1)
// 2. High notifications (Priority = 2)
// 3. Medium notifications (Priority = 3)
// ...
```

## ✅ Kết Luận

**Nên thêm Domain và Severity** vì:
1. ✅ Tăng tính linh hoạt trong routing và xử lý
2. ✅ Cải thiện hiệu quả với priority queue
3. ✅ Dễ dàng mở rộng với escalation rules
4. ✅ Hỗ trợ tốt hơn cho monitoring và reporting
5. ✅ Backward compatible (có thể infer từ eventType)

**Lưu ý**:
- Cần migration script cho dữ liệu cũ
- Cần update documentation
- Cần test kỹ với các event types hiện có
