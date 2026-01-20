# Notification Processing Rules - Tham Khảo

## 📋 Tổng Quan

Tài liệu này mô tả các rules xử lý notification dựa trên Domain và Severity. Dùng để tham khảo khi:
- Tạo routing rules mới
- Quyết định channel selection
- Cấu hình retry và priority
- Thiết lập escalation rules

## 🎯 Rule Matrix

### Severity → Processing Rules

| Severity | Priority | MaxRetries | Channels | Throttle | Escalation |
|----------|----------|------------|----------|----------|------------|
| **critical** | 1 | 10 | All (email + telegram + webhook) | None | Immediate (SMS, call) |
| **high** | 2 | 5 | Email + Telegram | None | Within 15 minutes |
| **medium** | 3 | 3 | Email + Telegram | 1/min | Within 1 hour |
| **low** | 4 | 2 | Email only | 5/min | Within 24 hours |
| **info** | 5 | 1 | Email only | 15/min | None |

## 📊 Domain-Specific Rules

### 1. System Domain

**Mục đích**: Thông báo về hệ thống, database, API

**Event Types**:
- `system_startup`, `system_shutdown`
- `system_error`, `system_warning`
- `database_error`, `api_error`
- `backup_completed`, `backup_failed`

**Routing Rules**:
```go
// Critical system errors → DevOps team (tất cả kênh)
{
    Domain: "system",
    Severities: ["critical"],
    OrganizationIDs: [devopsTeamID],
    ChannelTypes: ["email", "telegram", "webhook"]
}

// System warnings → DevOps team (email + telegram)
{
    Domain: "system",
    Severities: ["high", "medium"],
    OrganizationIDs: [devopsTeamID],
    ChannelTypes: ["email", "telegram"]
}

// System info → Log only (không gửi notification)
{
    Domain: "system",
    Severities: ["info"],
    // Không có OrganizationIDs → chỉ log
}
```

**Retry Rules**:
- Critical: Retry 10 lần, exponential backoff
- High: Retry 5 lần
- Medium: Retry 3 lần
- Info: Retry 1 lần (hoặc không retry)

### 2. Security Domain

**Mục đích**: Thông báo về bảo mật, authentication

**Event Types**:
- `security_alert`
- `user_login_failed`
- `unauthorized_access`
- `suspicious_activity`

**Routing Rules**:
```go
// Critical security alerts → Security team (tất cả kênh, immediate)
{
    Domain: "security",
    Severities: ["critical"],
    OrganizationIDs: [securityTeamID],
    ChannelTypes: ["email", "telegram", "webhook"]
    // Có thể thêm SMS escalation
}

// Security warnings → Security team (email + telegram)
{
    Domain: "security",
    Severities: ["high", "medium"],
    OrganizationIDs: [securityTeamID],
    ChannelTypes: ["email", "telegram"]
}
```

**Special Rules**:
- Critical security alerts: Không throttle, gửi ngay
- Login failed: Throttle để tránh spam (max 5 notifications/15 phút)
- Unauthorized access: Critical, gửi ngay

### 3. Conversation Domain

**Mục đích**: Thông báo về chat, message, reply

**Event Types**:
- `conversation_unreplied`
- `conversation_new`
- `conversation_closed`
- `message_received`

**Routing Rules**:
```go
// Unreplied conversations → Support team (email + telegram)
{
    EventType: "conversation_unreplied", // Có thể dùng EventType cụ thể
    Severities: ["high", "medium"],
    OrganizationIDs: [supportTeamID],
    ChannelTypes: ["email", "telegram"]
}

// New conversations → Support team (email)
{
    Domain: "conversation",
    Severities: ["medium", "low"],
    OrganizationIDs: [supportTeamID],
    ChannelTypes: ["email"]
}
```

**Throttling Rules**:
- Unreplied: Không throttle (quan trọng)
- New conversation: Throttle 1 notification/phút
- Closed: Info, throttle 15 phút

### 4. Order Domain

**Mục đích**: Thông báo về đơn hàng, payment

**Event Types**:
- `order_created`
- `order_failed`
- `order_cancelled`
- `payment_completed`
- `payment_failed`

**Routing Rules**:
```go
// Order failed → Sales team (email + telegram)
{
    Domain: "order",
    Severities: ["high"],
    OrganizationIDs: [salesTeamID],
    ChannelTypes: ["email", "telegram"]
}

// Order created → Sales team (email only)
{
    Domain: "order",
    Severities: ["info"],
    OrganizationIDs: [salesTeamID],
    ChannelTypes: ["email"]
}
```

**Special Rules**:
- Payment failed: High severity, gửi ngay
- Order created: Info, có thể batch (gửi theo batch hàng giờ)

### 5. User Domain

**Mục đích**: Thông báo về user management

**Event Types**:
- `user_created`
- `user_updated`
- `user_deleted`
- `user_suspended`

**Routing Rules**:
```go
// User suspended → Admin team
{
    Domain: "user",
    Severities: ["high"],
    OrganizationIDs: [adminTeamID],
    ChannelTypes: ["email", "telegram"]
}
```

## 🔄 Escalation Rules

### Escalation Matrix

| Severity | Initial Notification | Escalation (nếu không xử lý) |
|----------|---------------------|------------------------------|
| **critical** | All channels + SMS | Call after 5 minutes |
| **high** | Email + Telegram | SMS after 15 minutes |
| **medium** | Email + Telegram | Email reminder after 1 hour |
| **low** | Email | Email reminder after 24 hours |
| **info** | Email (optional) | None |

### Implementation Example
```go
// Escalation rule cho critical notifications
if severity == SeverityCritical {
    // Gửi ngay qua tất cả kênh
    sendViaChannels(channels)
    
    // Nếu sau 5 phút chưa có response → gọi điện
    scheduleEscalation(5*time.Minute, EscalationTypeCall)
}
```

## 📈 Priority Queue Rules

### Queue Processing Order
1. **Priority 1 (Critical)**: Xử lý ngay, không delay
2. **Priority 2 (High)**: Xử lý trong vòng 1 phút
3. **Priority 3 (Medium)**: Xử lý trong vòng 5 phút
4. **Priority 4 (Low)**: Xử lý trong vòng 15 phút
5. **Priority 5 (Info)**: Xử lý khi có thời gian

### Implementation
```go
// Dequeue với priority sorting
func (q *Queue) Dequeue(ctx context.Context, limit int) ([]*models.DeliveryQueueItem, error) {
    // Sort theo Priority (1 = critical, xử lý trước)
    filter := bson.M{
        "status": "pending",
        "$or": []bson.M{
            {"nextRetryAt": nil},
            {"nextRetryAt": bson.M{"$lte": time.Now().Unix()}},
        },
    }
    
    opts := options.Find().
        SetSort(bson.M{"priority": 1, "createdAt": 1}). // Sort theo priority trước
        SetLimit(int64(limit))
    
    // ... rest of logic
}
```

## 🚫 Throttling Rules

### Throttle Configuration

| Severity | Throttle Window | Max Notifications |
|----------|----------------|-------------------|
| **critical** | None | Unlimited |
| **high** | None | Unlimited |
| **medium** | 1 minute | 1 per minute |
| **low** | 5 minutes | 1 per 5 minutes |
| **info** | 15 minutes | 1 per 15 minutes |

### Implementation
```go
// Throttle check
func ShouldThrottle(severity string, lastSentAt int64) bool {
    throttleSeconds := SeverityThrottleSeconds[severity]
    if throttleSeconds == 0 {
        return false // Không throttle
    }
    
    now := time.Now().Unix()
    return (now - lastSentAt) < int64(throttleSeconds)
}
```

## 📝 Best Practices

### 1. Routing Rules
- ✅ Ưu tiên dùng Domain cho rules tổng quát
- ✅ Dùng EventType cho rules cụ thể
- ✅ Luôn filter theo Severity để tránh spam
- ✅ Critical → nhiều kênh, Info → chỉ email

### 2. Retry Rules
- ✅ Critical → retry nhiều (10 lần)
- ✅ Info → retry ít (1 lần) hoặc không retry
- ✅ Dùng exponential backoff

### 3. Channel Selection
- ✅ Critical → Tất cả kênh (email + telegram + webhook)
- ✅ High → Email + Telegram
- ✅ Medium/Low/Info → Email only

### 4. Throttling
- ✅ Critical/High → Không throttle
- ✅ Medium/Low/Info → Có throttle để tránh spam

## 🔍 Query Examples

### Lấy Notifications theo Domain
```go
// Tất cả security notifications
GET /notification/history?domain=security

// Tất cả critical notifications
GET /notification/history?severity=critical

// Critical security notifications
GET /notification/history?domain=security&severity=critical
```

### Analytics
```go
// Thống kê theo domain
GET /notification/history/analytics?groupBy=domain

// Thống kê theo severity
GET /notification/history/analytics?groupBy=severity

// Thống kê theo domain và severity
GET /notification/history/analytics?groupBy=domain,severity
```

## ✅ Checklist Khi Tạo Routing Rule Mới

- [ ] Xác định Domain của event
- [ ] Xác định Severity của event
- [ ] Chọn OrganizationIDs phù hợp
- [ ] Chọn ChannelTypes dựa trên Severity
- [ ] Set Severities filter (tránh spam)
- [ ] Test với các event types khác nhau
- [ ] Verify throttling hoạt động đúng
- [ ] Verify retry logic hoạt động đúng
