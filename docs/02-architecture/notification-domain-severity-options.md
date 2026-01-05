# Phân Tích: Có Cần Thêm Domain/Severity Vào DeliveryQueueItem?

## 🤔 Câu Hỏi

Có cần thiết thêm Domain/Severity vào `DeliveryQueueItem` không, hay chỉ cần thêm vào `NotificationRoutingRule`?

## 📊 Phân Tích 2 Phương Án

### Phương Án 1: Chỉ Thêm Vào RoutingRule

**Thêm vào**:
- ✅ `NotificationRoutingRule`: Domain, Severities (để filter routing)

**Không thêm vào**:
- ❌ `DeliveryQueueItem`: Không có Domain/Severity
- ✅ `DeliveryHistory`: Có thể thêm (optional, để reporting)

**Cách hoạt động**:
```go
// 1. Trigger notification
eventType := "system_error"

// 2. Infer Domain/Severity (không lưu)
domain := notification.GetDomainFromEventType(eventType)   // "system"
severity := notification.GetSeverityFromEventType(eventType) // "critical"

// 3. Tìm routes với Domain/Severity
routes := router.FindRoutes(ctx, eventType, domain, severity)

// 4. Tính Priority và MaxRetries từ Severity (không lưu)
priority := notification.GetPriorityFromSeverity(severity)  // 1
maxRetries := notification.GetMaxRetriesFromSeverity(severity) // 10

// 5. Tạo DeliveryQueueItem (KHÔNG có Domain/Severity)
queueItem := &DeliveryQueueItem{
    EventType: eventType,
    Priority: priority,      // Lưu Priority (tính từ Severity)
    MaxRetries: maxRetries, // Lưu MaxRetries (tính từ Severity)
    // KHÔNG có Domain, Severity
}

// 6. Khi tạo DeliveryHistory, có thể lưu Domain/Severity (optional)
history := &DeliveryHistory{
    EventType: eventType,
    Domain: domain,    // Lưu để reporting
    Severity: severity, // Lưu để reporting
}
```

**Ưu điểm**:
- ✅ Đơn giản hơn, ít thay đổi
- ✅ DeliveryQueueItem vẫn "dumb" (chỉ có Priority, MaxRetries)
- ✅ Domain/Severity chỉ dùng cho routing (không cần lưu)
- ✅ Có thể lưu vào DeliveryHistory để reporting

**Nhược điểm**:
- ⚠️ Không thể query queue theo Domain/Severity (nhưng queue thường không cần query)
- ⚠️ Priority queue phải tính từ EventType mỗi lần (nhưng chỉ khi dequeue, không ảnh hưởng nhiều)

### Phương Án 2: Thêm Vào Cả DeliveryQueueItem

**Thêm vào**:
- ✅ `NotificationRoutingRule`: Domain, Severities
- ✅ `DeliveryQueueItem`: Domain, Severity, Priority
- ✅ `DeliveryHistory`: Domain, Severity

**Cách hoạt động**:
```go
// 1. Trigger notification
eventType := "system_error"

// 2. Infer Domain/Severity
domain := notification.GetDomainFromEventType(eventType)
severity := notification.GetSeverityFromEventType(eventType)

// 3. Tính Priority và MaxRetries
priority := notification.GetPriorityFromSeverity(severity)
maxRetries := notification.GetMaxRetriesFromSeverity(severity)

// 4. Tạo DeliveryQueueItem (CÓ Domain/Severity)
queueItem := &DeliveryQueueItem{
    EventType: eventType,
    Domain: domain,      // Lưu
    Severity: severity,   // Lưu
    Priority: priority,   // Lưu
    MaxRetries: maxRetries,
}
```

**Ưu điểm**:
- ✅ Có thể query queue theo Domain/Severity
- ✅ Priority queue sort trực tiếp (không cần tính lại)
- ✅ Có thể debug dễ hơn (thấy Domain/Severity trong queue)

**Nhược điểm**:
- ⚠️ Thêm fields vào DeliveryQueueItem (tăng storage)
- ⚠️ Phức tạp hơn một chút

## 🎯 So Sánh Use Cases

### Use Case 1: Priority Queue

**Phương Án 1** (không lưu Domain/Severity):
```go
// Dequeue: Phải infer Severity từ EventType để sort
func FindPending() {
    // Sort theo createdAt (không có Priority field)
    // Hoặc phải infer Severity từ EventType mỗi item → chậm
}
```

**Phương Án 2** (có lưu Priority):
```go
// Dequeue: Sort trực tiếp theo Priority
func FindPending() {
    opts := options.Find().
        SetSort(bson.M{"priority": 1, "createdAt": 1}) // Fast
}
```

**Kết luận**: Phương Án 2 tốt hơn cho priority queue

### Use Case 2: Query Queue

**Phương Án 1**: Không thể query theo Domain/Severity
**Phương Án 2**: Có thể query

**Nhưng**: Queue thường không cần query (chỉ dequeue và process). Query thường ở History.

**Kết luận**: Không quan trọng

### Use Case 3: Reporting

**Phương Án 1**: Lưu vào DeliveryHistory
**Phương Án 2**: Lưu vào cả QueueItem và History

**Kết luận**: Cả 2 đều OK, nhưng History là đủ

### Use Case 4: Debug

**Phương Án 1**: Phải infer từ EventType khi debug
**Phương Án 2**: Thấy trực tiếp trong queue item

**Kết luận**: Phương Án 2 tiện hơn

## 💡 Đề Xuất: Phương Án Hybrid

### Chỉ Thêm Priority Vào DeliveryQueueItem

**Lý do**:
- Priority cần cho queue sorting (quan trọng)
- Domain/Severity không cần trong queue (chỉ dùng cho routing)

**Implementation**:
```go
type DeliveryQueueItem struct {
    // ... existing fields ...
    
    // CHỈ THÊM Priority (tính từ Severity)
    Priority int `json:"priority" bson:"priority" index:"single:1"`
    
    // KHÔNG thêm Domain, Severity (infer khi cần)
}

// Khi tạo queue item:
domain := notification.GetDomainFromEventType(eventType)
severity := notification.GetSeverityFromEventType(eventType)
priority := notification.GetPriorityFromSeverity(severity) // Lưu
maxRetries := notification.GetMaxRetriesFromSeverity(severity)

queueItem := &DeliveryQueueItem{
    EventType: eventType,
    Priority: priority,      // Lưu (cần cho sorting)
    MaxRetries: maxRetries, // Lưu (cần cho retry)
    // Domain, Severity: Không lưu (infer khi cần)
}
```

**RoutingRule**:
```go
type NotificationRoutingRule struct {
    // ... existing fields ...
    
    Domain     *string  `json:"domain,omitempty"`      // Routing theo domain
    Severities []string `json:"severities,omitempty"`   // Filter theo severity
}
```

**DeliveryHistory** (optional, để reporting):
```go
type DeliveryHistory struct {
    // ... existing fields ...
    
    Domain   string `json:"domain,omitempty"`    // Lưu để reporting
    Severity string `json:"severity,omitempty"`  // Lưu để reporting
}
```

## ✅ Kết Luận

### Khuyến Nghị: Phương Án Hybrid

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

## 📝 Implementation

### Minimal Changes

1. **RoutingRule**: Thêm Domain, Severities
2. **DeliveryQueueItem**: Chỉ thêm Priority
3. **DeliveryHistory**: Thêm Domain, Severity (optional)
4. **Notification Module**: Infer Domain/Severity khi routing
5. **Queue Service**: Sort theo Priority

### Files Cần Thay Đổi

- ✅ `model.notification.routing.go` - Thêm Domain, Severities
- ✅ `model.delivery.queue.go` - Chỉ thêm Priority
- ✅ `model.delivery.history.go` - Thêm Domain, Severity (optional)
- ✅ `notification/router.go` - Infer Domain/Severity
- ✅ `notification/classifier.go` - Functions infer
- ✅ `notification/rules.go` - Priority rules
- ✅ `handler.notification.trigger.go` - Set Priority
- ✅ `service.delivery.queue.go` - Sort theo Priority
