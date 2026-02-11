# Notification Domain và Severity System

## 📋 Tổng Quan

Hệ thống phân loại notification theo **Domain** (lĩnh vực) và **Severity** (mức độ nghiêm trọng) để tự động quyết định routing, retry, priority và cách xử lý.

## 🎯 Mục Tiêu

1. **Tham khảo**: Biết notification thuộc domain nào, mức độ nghiêm trọng ra sao
2. **Rules xử lý**: Tự động quyết định routing, retry, priority dựa trên domain/severity
3. **Báo cáo**: Filter và phân tích notification theo domain/severity

---

## 📊 Phân Loại

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

---

## 🏗️ Phân Chia Trách Nhiệm

### Notification Module (`api/internal/notification/`)
**Trách nhiệm**: Xử lý logic nghiệp vụ notification
- ✅ **Infer và set** Domain/Severity từ EventType
- ✅ **Tính toán** Priority và MaxRetries từ Severity
- ✅ **Routing logic** có thể filter theo Domain/Severity
- ✅ **Tạo NotificationQueueItem** với đầy đủ thông tin (Domain, Severity, Priority, MaxRetries)

### Delivery Module (`api/internal/delivery/`)
**Trách nhiệm**: Xử lý việc gửi notification (như "bưu điện")
- ✅ **Chỉ dùng** các field đã được set sẵn (Priority, MaxRetries)
- ✅ **Priority queue**: Sort theo Priority khi dequeue
- ❌ **KHÔNG** infer Domain/Severity (vì đã được set ở Notification module)
- ❌ **KHÔNG** tính MaxRetries từ Severity (vì đã được set sẵn)

**Lý do**: Delivery module là "dumb" service, chỉ cần biết "gửi cái gì, gửi cho ai, gửi như thế nào". Logic nghiệp vụ (domain/severity) nằm ở Notification module.

---

## 🔧 Rules Xử Lý

### Rule 1: Infer Domain và Severity từ EventType

**Mục đích**: Tự động phân loại khi trigger notification

**Implementation**:
```go
// api/internal/notification/classifier.go
func GetDomainFromEventType(eventType string) string {
    if strings.HasPrefix(eventType, "system_") {
        return DomainSystem
    }
    if strings.HasPrefix(eventType, "conversation_") {
        return DomainConversation
    }
    if strings.HasPrefix(eventType, "order_") {
        return DomainOrder
    }
    // ... more patterns
    return DomainSystem // Default
}

func GetSeverityFromEventType(eventType string) string {
    if strings.Contains(eventType, "_error") || 
       strings.Contains(eventType, "_critical") {
        return SeverityCritical
    }
    if strings.Contains(eventType, "_failed") || 
       strings.Contains(eventType, "_alert") {
        return SeverityHigh
    }
    // ... more patterns
    return SeverityMedium // Default
}
```

### Rule 2: Tính Priority và MaxRetries từ Severity

**Mục đích**: Xác định ưu tiên xử lý và số lần retry

**Implementation**:
```go
// api/internal/notification/rules.go
var SeverityPriority = map[string]int{
    SeverityCritical: 1,
    SeverityHigh:     2,
    SeverityMedium:   3,
    SeverityLow:      4,
    SeverityInfo:     5,
}

var SeverityMaxRetries = map[string]int{
    SeverityCritical: 10, // Critical: retry nhiều hơn
    SeverityHigh:     5,
    SeverityMedium:   3,
    SeverityLow:      2,
    SeverityInfo:     1, // Info: retry ít nhất
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
    ChannelTypes: ["email", "telegram"],
    Severities: ["critical", "high"], // Chỉ nhận critical và high
}

// Rule 2: System errors → gửi cho devops team
{
    Domain: "system",
    OrganizationIDs: [devopsTeamID],
    ChannelTypes: ["email", "telegram", "webhook"],
    Severities: ["critical"], // Chỉ nhận critical
}
```

---

## 📊 Phân Tích: Có Cần Thêm Domain/Severity Vào DeliveryQueueItem?

### Phương Án 1: Chỉ Thêm Vào RoutingRule

**Thêm vào**:
- ✅ `NotificationRoutingRule`: Domain, Severities (để filter routing)
- ✅ `DeliveryHistory`: Domain, Severity (optional, để reporting)

**Không thêm vào**:
- ❌ `DeliveryQueueItem`: Không có Domain/Severity

**Ưu điểm**:
- ✅ Đơn giản hơn, ít thay đổi
- ✅ DeliveryQueueItem vẫn "dumb" (chỉ có Priority, MaxRetries)

**Nhược điểm**:
- ⚠️ Không thể query queue theo Domain/Severity
- ⚠️ Priority queue phải tính từ EventType mỗi lần

### Phương Án 2: Thêm Vào Cả DeliveryQueueItem

**Thêm vào**:
- ✅ `NotificationRoutingRule`: Domain, Severities
- ✅ `DeliveryQueueItem`: Domain, Severity, Priority
- ✅ `DeliveryHistory`: Domain, Severity

**Ưu điểm**:
- ✅ Có thể query queue theo Domain/Severity
- ✅ Priority queue sort trực tiếp (không cần tính lại)
- ✅ Có thể debug dễ hơn

**Nhược điểm**:
- ⚠️ Thêm fields vào DeliveryQueueItem (tăng storage)
- ⚠️ Phức tạp hơn một chút

### 💡 Đề Xuất: Phương Án Hybrid

**Thêm vào**:
1. ✅ `NotificationRoutingRule`: Domain, Severities (để filter routing)
2. ✅ `DeliveryQueueItem`: **CHỈ Priority** (để priority queue)
3. ✅ `DeliveryHistory`: Domain, Severity (để reporting, optional)

**Không thêm vào**:
- ❌ `DeliveryQueueItem`: Domain, Severity (không cần, có thể infer)

**Lý do**:
- ✅ Priority cần cho queue sorting (quan trọng)
- ✅ Domain/Severity chỉ cần cho routing (infer khi cần)
- ✅ Reporting dùng History (không cần trong queue)
- ✅ Đơn giản hơn, ít thay đổi hơn

---

## 🏗️ Kiến Trúc Triển Khai

### 1. Files Mới Sẽ Tạo

#### `api/internal/notification/constants.go`
Định nghĩa constants cho Domain và Severity

#### `api/internal/notification/classifier.go`
Functions để infer Domain và Severity từ EventType:
- `GetDomainFromEventType(eventType string) string`
- `GetSeverityFromEventType(eventType string) string`

#### `api/internal/notification/rules.go`
Rules xử lý (Priority, MaxRetries, Throttle):
- `SeverityPriority map[string]int` - Mapping severity → priority
- `SeverityMaxRetries map[string]int` - Mapping severity → maxRetries
- `GetPriorityFromSeverity(severity string) int`
- `GetMaxRetriesFromSeverity(severity string) int`

### 2. Models Sẽ Cập Nhật

#### `DeliveryQueueItem`
**Thêm fields**:
```go
type DeliveryQueueItem struct {
    // ... existing fields ...
    Priority int `json:"priority" bson:"priority" index:"single:1"` // 1=critical, 2=high, ...
}
```

#### `NotificationRoutingRule`
**Thêm fields**:
```go
type NotificationRoutingRule struct {
    // ... existing fields ...
    Domain     *string  `json:"domain,omitempty" bson:"domain,omitempty" index:"single:1"`
    Severities []string `json:"severities,omitempty" bson:"severities,omitempty"`
}
```

#### `DeliveryHistory`
**Thêm fields** (optional, để query/reporting):
```go
type DeliveryHistory struct {
    // ... existing fields ...
    Domain   string `json:"domain,omitempty" bson:"domain,omitempty" index:"single:1"`
    Severity string `json:"severity,omitempty" bson:"severity,omitempty" index:"single:1"`
}
```

### 3. Logic Cập Nhật

#### Notification Module - Set Domain/Severity khi tạo QueueItem
```go
// Trong handler.notification.trigger.go
for _, recipient := range recipients {
    // Infer Domain và Severity từ EventType
    domain := notification.GetDomainFromEventType(req.EventType)
    severity := notification.GetSeverityFromEventType(req.EventType)
    
    // Set Priority và MaxRetries dựa trên Severity
    priority := notification.GetPriorityFromSeverity(severity)
    maxRetries := notification.GetMaxRetriesFromSeverity(severity)
    
    queueItems = append(queueItems, &models.NotificationQueueItem{
        EventType:  req.EventType,
        Priority:   priority,     // ✅ Set ở đây
        MaxRetries: maxRetries,  // ✅ Set ở đây (từ Severity)
        // ...
    })
}
```

#### Delivery Module - Priority Queue (chỉ dùng Priority đã set sẵn)
```go
// Trong delivery/queue.go
// Dequeue - sort theo Priority (đã được set sẵn)
func (q *Queue) Dequeue(ctx context.Context, limit int) ([]*models.NotificationQueueItem, error) {
    // Sort theo Priority (1 = critical, xử lý trước)
    opts := options.Find().
        SetSort(bson.M{"priority": 1, "createdAt": 1}).
        SetLimit(int64(limit))
    // ...
}
```

---

## 📊 Ví Dụ Phân Loại

### Domain Mapping
```go
"system_startup"     → Domain: "system", Severity: "info"
"system_error"       → Domain: "system", Severity: "critical"
"conversation_unreplied" → Domain: "conversation", Severity: "high"
"order_created"      → Domain: "order", Severity: "info"
"order_failed"       → Domain: "order", Severity: "high"
"security_alert"     → Domain: "security", Severity: "critical"
```

---

## ✅ Lợi Ích

1. **Tự động hóa**: Không cần config từng event, system tự infer
2. **Linh hoạt**: Có thể routing theo domain hoặc eventType cụ thể
3. **Thông minh**: Priority và retry tự động dựa trên severity
4. **Báo cáo**: Dễ dàng filter và phân tích theo domain/severity
5. **Mở rộng**: Dễ thêm domain/severity mới

---

## 🔄 Migration Plan

### Phase 1: Thêm Fields (Backward Compatible)
1. Thêm `Priority` vào models (optional fields)
2. Tạo helper functions để infer domain/severity từ eventType
3. Update Enqueue để tự động set priority nếu chưa có

### Phase 2: Update Logic
1. Update Dequeue để sort theo Priority
2. Update Retry logic để dùng SeverityMaxRetries
3. Update Router để support routing theo Domain

### Phase 3: Migration Data
1. Script migration để set priority cho các notification cũ
2. Update templates và routing rules

### Backward Compatibility
- ✅ EventType vẫn hoạt động như cũ
- ✅ Domain và Severity là optional (có thể infer)
- ✅ Routing rules cũ vẫn hoạt động (không có Domain/Severity filter)
- ✅ Không breaking changes

---

## 📝 Tóm Tắt Thay Đổi

### Files Mới: 3 files
1. `api/internal/notification/constants.go`
2. `api/internal/notification/classifier.go`
3. `api/internal/notification/rules.go`

### Models Cập Nhật: 3 models
1. `DeliveryQueueItem` - Thêm Priority field
2. `NotificationRoutingRule` - Thêm Domain, Severities fields
3. `DeliveryHistory` - Thêm Domain, Severity fields (optional)

### Services Cập Nhật: 2 services
1. `NotificationRoutingService` - Thêm FindByDomain methods
2. `DeliveryQueueService` - Cập nhật FindPending (sort theo Priority)

### Handlers Cập Nhật: 1 handler
1. `NotificationTriggerHandler` - Cập nhật logic infer và set priority

### Indexes: 5-7 indexes mới
