# Tóm Tắt: Những Gì Sẽ Thêm Vào Code

## 📁 Files Mới Sẽ Tạo

### 1. `api/core/notification/constants.go`
**Mục đích**: Định nghĩa constants cho Domain và Severity

**Nội dung**:
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
```

### 2. `api/core/notification/classifier.go`
**Mục đích**: Functions để infer Domain và Severity từ EventType

**Nội dung**:
- `GetDomainFromEventType(eventType string) string`
- `GetSeverityFromEventType(eventType string) string`

### 3. `api/core/notification/rules.go`
**Mục đích**: Rules xử lý (Priority, MaxRetries, Throttle)

**Nội dung**:
- `SeverityPriority map[string]int` - Mapping severity → priority
- `SeverityMaxRetries map[string]int` - Mapping severity → maxRetries
- `SeverityThrottleSeconds map[string]int` - Mapping severity → throttle
- `GetPriorityFromSeverity(severity string) int`
- `GetMaxRetriesFromSeverity(severity string) int`
- `GetRecommendedChannels(severity string) []string`

## 🔧 Models Sẽ Cập Nhật

### 1. `DeliveryQueueItem` (model.delivery.queue.go)

**Thêm fields**:
```go
type DeliveryQueueItem struct {
    // ... existing fields ...
    
    // NEW FIELDS:
    Domain   string `json:"domain" bson:"domain" index:"single:1"`        // system, conversation, order, ...
    Severity string `json:"severity" bson:"severity" index:"single:1"`     // critical, high, medium, low, info
    Priority int    `json:"priority" bson:"priority" index:"single:1"`    // 1=critical, 2=high, 3=medium, 4=low, 5=info
}
```

**Cập nhật**:
- Thêm index cho `domain`, `severity`, `priority`
- `MaxRetries` sẽ được tính từ Severity (thay vì hardcode 3)

### 2. `NotificationRoutingRule` (model.notification.routing.go)

**Thêm fields**:
```go
type NotificationRoutingRule struct {
    // ... existing fields ...
    
    // NEW FIELDS:
    EventType *string  `json:"eventType,omitempty" bson:"eventType,omitempty" index:"single:1"` // Optional, null = dùng Domain
    Domain    *string  `json:"domain,omitempty" bson:"domain,omitempty" index:"single:1"`       // Optional, null = dùng EventType
    Severities []string `json:"severities,omitempty" bson:"severities,omitempty"`              // Filter theo severity
}
```

**Lưu ý**: 
- `EventType` đổi từ `string` → `*string` (optional)
- Có thể routing theo Domain hoặc EventType (ưu tiên Domain nếu có)

### 3. `DeliveryHistory` (model.delivery.history.go)

**Thêm fields** (optional, để query/reporting):
```go
type DeliveryHistory struct {
    // ... existing fields ...
    
    // NEW FIELDS (optional, có thể lấy từ QueueItem):
    Domain   string `json:"domain,omitempty" bson:"domain,omitempty" index:"single:1"`
    Severity string `json:"severity,omitempty" bson:"severity,omitempty" index:"single:1"`
}
```

## 🔄 Services Sẽ Cập Nhật

### 1. `NotificationRoutingService` (service.notification.routing.go)

**Thêm methods**:
```go
// FindByDomain tìm rules theo domain
func (s *NotificationRoutingService) FindByDomain(ctx context.Context, domain string) ([]models.NotificationRoutingRule, error)

// FindByDomainAndSeverity tìm rules theo domain và severity
func (s *NotificationRoutingService) FindByDomainAndSeverity(ctx context.Context, domain string, severity string) ([]models.NotificationRoutingRule, error)
```

### 2. `DeliveryQueueService` (service.delivery.queue.go)

**Cập nhật methods**:
```go
// FindPending - Thêm sort theo Priority
func (s *DeliveryQueueService) FindPending(ctx context.Context, limit int) ([]models.DeliveryQueueItem, error) {
    // Sort theo Priority (1 = critical, xử lý trước)
    opts := options.Find().
        SetSort(bson.M{"priority": 1, "createdAt": 1}).
        SetLimit(int64(limit))
    // ...
}
```

## 🔄 Handlers Sẽ Cập Nhật

### 1. `NotificationTriggerHandler` (handler.notification.trigger.go)

**Cập nhật logic**:
```go
// Trong HandleTriggerNotification, khi tạo DeliveryQueueItem:
for _, recipient := range recipients {
    // NEW: Infer Domain và Severity
    domain := notification.GetDomainFromEventType(req.EventType)
    severity := notification.GetSeverityFromEventType(req.EventType)
    priority := notification.GetPriorityFromSeverity(severity)
    maxRetries := notification.GetMaxRetriesFromSeverity(severity)
    
    queueItems = append(queueItems, &models.DeliveryQueueItem{
        // ... existing fields ...
        Domain:    domain,      // NEW
        Severity:  severity,    // NEW
        Priority:  priority,     // NEW
        MaxRetries: maxRetries, // UPDATED (từ severity thay vì hardcode 3)
    })
}
```

**Cập nhật FindRoutes call**:
```go
// OLD:
routes, err := h.router.FindRoutes(c.Context(), req.EventType)

// NEW:
domain := notification.GetDomainFromEventType(req.EventType)
severity := notification.GetSeverityFromEventType(req.EventType)
routes, err := h.router.FindRoutes(c.Context(), req.EventType, domain, severity)
```

## 🔄 Notification Module Sẽ Cập Nhật

### 1. `Router` (notification/router.go)

**Cập nhật FindRoutes**:
```go
// OLD:
func (r *Router) FindRoutes(ctx context.Context, eventType string) ([]Route, error)

// NEW:
func (r *Router) FindRoutes(ctx context.Context, eventType string, domain string, severity string) ([]Route, error) {
    // 1. Tìm rules theo EventType
    // 2. Tìm rules theo Domain
    // 3. Filter theo Severity
    // 4. Tạo routes
}
```

## 📝 DTOs Sẽ Cập Nhật

### 1. `NotificationRoutingRuleCreateInput` (dto.notification.routing.go)

**Thêm fields**:
```go
type NotificationRoutingRuleCreateInput struct {
    // ... existing fields ...
    
    // NEW FIELDS:
    Domain     *string  `json:"domain,omitempty"`      // Optional: routing theo domain
    EventType  *string  `json:"eventType,omitempty"`   // Optional: routing theo eventType cụ thể
    Severities []string `json:"severities,omitempty"`   // Optional: filter theo severity
}
```

**Lưu ý**: 
- `EventType` đổi từ `string` → `*string` (optional)
- Có thể dùng Domain hoặc EventType (không bắt buộc cả 2)

### 2. `NotificationRoutingRuleUpdateInput` (dto.notification.routing.go)

**Thêm fields** (tương tự CreateInput)

## 🔧 Init Scripts Sẽ Cập Nhật

### 1. `service.admin.init.go`

**Thêm default routing rules**:
```go
// Default routing rules theo domain
defaultRules := []struct {
    Domain         string
    Severities     []string
    OrganizationID primitive.ObjectID
    ChannelTypes   []string
}{
    {
        Domain:         "security",
        Severities:     []string{"critical", "high"},
        OrganizationID: securityTeamID,
        ChannelTypes:   []string{"email", "telegram", "webhook"},
    },
    {
        Domain:         "system",
        Severities:     []string{"critical"},
        OrganizationID: devopsTeamID,
        ChannelTypes:   []string{"email", "telegram", "webhook"},
    },
    // ... more rules
}
```

## 📊 Indexes Sẽ Thêm

### MongoDB Indexes

**DeliveryQueue collection**:
```javascript
db.delivery_queue.createIndex({ "domain": 1 })
db.delivery_queue.createIndex({ "severity": 1 })
db.delivery_queue.createIndex({ "priority": 1 })
db.delivery_queue.createIndex({ "priority": 1, "createdAt": 1 }) // Compound index cho priority queue
```

**NotificationRoutingRules collection**:
```javascript
db.notification_routing_rules.createIndex({ "domain": 1 })
db.notification_routing_rules.createIndex({ "domain": 1, "isActive": 1 }) // Compound index
```

**DeliveryHistory collection** (optional):
```javascript
db.delivery_history.createIndex({ "domain": 1 })
db.delivery_history.createIndex({ "severity": 1 })
```

## 📋 Tóm Tắt Thay Đổi

### Files Mới: 3 files
1. `api/core/notification/constants.go`
2. `api/core/notification/classifier.go`
3. `api/core/notification/rules.go`

### Models Cập Nhật: 3 models
1. `DeliveryQueueItem` - Thêm 3 fields (Domain, Severity, Priority)
2. `NotificationRoutingRule` - Thêm 3 fields (Domain, Severities, EventType optional)
3. `DeliveryHistory` - Thêm 2 fields (Domain, Severity) - optional

### Services Cập Nhật: 2 services
1. `NotificationRoutingService` - Thêm 2 methods (FindByDomain, FindByDomainAndSeverity)
2. `DeliveryQueueService` - Cập nhật FindPending (sort theo Priority)

### Handlers Cập Nhật: 1 handler
1. `NotificationTriggerHandler` - Cập nhật logic infer và set domain/severity

### Notification Module Cập Nhật: 1 module
1. `Router` - Cập nhật FindRoutes để support domain và severity

### DTOs Cập Nhật: 1 DTO
1. `NotificationRoutingRuleCreateInput` - Thêm 3 fields
2. `NotificationRoutingRuleUpdateInput` - Thêm 3 fields

### Indexes: 5-7 indexes mới

## ✅ Backward Compatibility

- ✅ EventType vẫn hoạt động như cũ
- ✅ Domain và Severity là optional (có thể infer)
- ✅ Routing rules cũ vẫn hoạt động (không có Domain/Severity filter)
- ✅ Không breaking changes

## 📝 Migration

- ✅ Không cần migration data (domain/severity sẽ được infer khi trigger mới)
- ✅ Có thể tạo script để set domain/severity cho history nếu cần
- ✅ Indexes sẽ được tạo tự động khi init
