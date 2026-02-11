# Đề Xuất: Đổi Tên Collection Để Phân Biệt 2 Hệ Thống

## 🔴 Vấn Đề Hiện Tại

Hiện tại tất cả các collection đều có prefix `notification_` gây nhầm lẫn giữa 2 hệ thống:

### Notification System (Hệ thống 2 - Routing/Template)
- ✅ `notification_senders` - Cấu hình sender (email, telegram, webhook)
- ✅ `notification_channels` - Cấu hình kênh nhận (recipients)
- ✅ `notification_templates` - Template thông báo
- ✅ `notification_routing_rules` - Routing rules (Event → Teams → Channels)

### Delivery System (Hệ thống 1 - Gửi)
- ❌ `notification_queue` - **THUỘC Delivery System** nhưng tên có "notification_"
- ❌ `notification_history` - **THUỘC Delivery System** nhưng tên có "notification_"

**Vấn đề**: `notification_queue` và `notification_history` thực chất thuộc về **Delivery System** (hệ thống gửi), không phải Notification System (hệ thống routing/template). Tên collection gây nhầm lẫn về ownership và responsibility.

## ✅ Đề Xuất Đổi Tên

### Option 1: Đổi Thành `delivery_*` (Khuyến Nghị)

**Lý do**: Rõ ràng thuộc về Delivery System

```go
// Trước
notification_queue    → delivery_queue
notification_history → delivery_history

// Sau
Notification System:
- notification_senders
- notification_channels
- notification_templates
- notification_routing_rules

Delivery System:
- delivery_queue      ← Rõ ràng thuộc Delivery
- delivery_history    ← Rõ ràng thuộc Delivery
```

**Ưu điểm**:
- ✅ Phân biệt rõ ràng 2 hệ thống
- ✅ Tên ngắn gọn, dễ hiểu
- ✅ Phù hợp với module structure (`api/internal/delivery/`)

**Nhược điểm**:
- ⚠️ Cần migration script
- ⚠️ Cần update tất cả references

### Option 2: Giữ Prefix `notification_` Nhưng Thêm Suffix

```go
// Trước
notification_queue    → notification_delivery_queue
notification_history → notification_delivery_history
```

**Ưu điểm**:
- ✅ Vẫn giữ prefix "notification_" để dễ tìm
- ✅ Phân biệt được 2 hệ thống

**Nhược điểm**:
- ⚠️ Tên dài hơn
- ⚠️ Vẫn có thể gây nhầm lẫn

### Option 3: Giữ Nguyên Nhưng Thêm Comment/Documentation

**Không khuyến nghị** vì không giải quyết được vấn đề nhầm lẫn.

## 🏗️ Cấu Trúc Đề Xuất (Option 1)

### Global Variables

```go
// api/internal/global/global.vars.go
type MongoDB_Auth_CollectionName struct {
    // ... existing fields ...
    
    // Notification System Collections (Hệ thống 2 - Routing/Template)
    NotificationSenders      string // notification_senders
    NotificationChannels     string // notification_channels
    NotificationTemplates    string // notification_templates
    NotificationRoutingRules string // notification_routing_rules
    
    // Delivery System Collections (Hệ thống 1 - Gửi)
    DeliveryQueue   string // delivery_queue (đổi từ notification_queue)
    DeliveryHistory string // delivery_history (đổi từ notification_history)
}
```

### Init Collections

```go
// api/cmd/server/init.go
// Notification System Collections
global.MongoDB_ColNames.NotificationSenders = "notification_senders"
global.MongoDB_ColNames.NotificationChannels = "notification_channels"
global.MongoDB_ColNames.NotificationTemplates = "notification_templates"
global.MongoDB_ColNames.NotificationRoutingRules = "notification_routing_rules"

// Delivery System Collections
global.MongoDB_ColNames.DeliveryQueue = "delivery_queue"      // Đổi từ notification_queue
global.MongoDB_ColNames.DeliveryHistory = "delivery_history" // Đổi từ notification_history
```

### Registry

```go
// api/cmd/server/init.registry.go
colNames := []string{
    // ... existing ...
    // Notification System
    "notification_senders",
    "notification_channels",
    "notification_templates",
    "notification_routing_rules",
    // Delivery System
    "delivery_queue",    // Đổi từ notification_queue
    "delivery_history",  // Đổi từ notification_history
    // ... existing ...
}
```

### Services

```go
// api/internal/api/services/service.delivery.queue.go (đổi tên từ service.notification.queue.go)
collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.DeliveryQueue)

// api/internal/api/services/service.delivery.history.go (đổi tên từ service.notification.history.go)
collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.DeliveryHistory)
```

### Models

```go
// api/internal/api/models/mongodb/model.delivery.queue.go (đổi tên từ model.notification.queue.go)
type DeliveryQueueItem struct { // Đổi từ NotificationQueueItem
    // ... fields ...
}

// api/internal/api/models/mongodb/model.delivery.history.go (đổi tên từ model.notification.history.go)
type DeliveryHistory struct { // Đổi từ NotificationHistory
    // ... fields ...
}
```

### Relationships

```go
// api/internal/api/models/mongodb/model.notification.channel.go
_Relationships struct{} `relationship:"collection:delivery_queue,field:channelId,message:...|collection:delivery_history,field:channelId,message:..."`
```

## 📋 Migration Plan

### Phase 1: Tạo Collections Mới
1. Tạo collections mới: `delivery_queue`, `delivery_history`
2. Copy dữ liệu từ `notification_queue` → `delivery_queue`
3. Copy dữ liệu từ `notification_history` → `delivery_history`

### Phase 2: Update Code
1. Update global variables
2. Update services (đổi tên service files)
3. Update models (đổi tên model files và struct names)
4. Update all references trong codebase

### Phase 3: Dual Write (Optional - Để An Toàn)
1. Code mới write vào cả 2 collections (old + new)
2. Code mới read từ collection mới
3. Monitor để đảm bảo không có lỗi

### Phase 4: Cleanup
1. Drop collections cũ: `notification_queue`, `notification_history`
2. Update documentation

## 🔍 Files Cần Update

### Global & Config
- [ ] `api/internal/global/global.vars.go`
- [ ] `api/cmd/server/init.go`
- [ ] `api/cmd/server/init.registry.go`

### Services (Đổi Tên Files)
- [ ] `api/internal/api/services/service.notification.queue.go` → `service.delivery.queue.go`
- [ ] `api/internal/api/services/service.notification.history.go` → `service.delivery.history.go`

### Models (Đổi Tên Files)
- [ ] `api/internal/api/models/mongodb/model.notification.queue.go` → `model.delivery.queue.go`
- [ ] `api/internal/api/models/mongodb/model.notification.history.go` → `model.delivery.history.go`

### References
- [ ] `api/internal/delivery/queue.go` - Update references
- [ ] `api/internal/delivery/processor.go` - Update references
- [ ] `api/internal/api/handler/handler.notification.trigger.go` - Update references
- [ ] `api/internal/api/handler/handler.delivery.send.go` - Update references
- [ ] `api/internal/api/models/mongodb/model.notification.channel.go` - Update relationships
- [ ] Tất cả files import các models/services này

### Scripts
- [ ] `scripts/migration_recreate_indexes.js` - Update collection names
- [ ] Tạo migration script để copy data

## 📝 Migration Script Example

```javascript
// scripts/migration_notification_to_delivery_collections.js
db = db.getSiblingDB('your_database_name');

print("==========================================");
print("Migration: notification_queue → delivery_queue");
print("Migration: notification_history → delivery_history");
print("==========================================");

// 1. Copy notification_queue → delivery_queue
if (db.notification_queue.count() > 0) {
    print(`Copying ${db.notification_queue.count()} documents from notification_queue to delivery_queue...`);
    db.notification_queue.find().forEach(function(doc) {
        db.delivery_queue.insertOne(doc);
    });
    print("✓ Copied notification_queue → delivery_queue");
} else {
    print("⚠ notification_queue is empty, skipping...");
}

// 2. Copy notification_history → delivery_history
if (db.notification_history.count() > 0) {
    print(`Copying ${db.notification_history.count()} documents from notification_history to delivery_history...`);
    db.notification_history.find().forEach(function(doc) {
        db.delivery_history.insertOne(doc);
    });
    print("✓ Copied notification_history → delivery_history");
} else {
    print("⚠ notification_history is empty, skipping...");
}

// 3. Create indexes cho collections mới
print("Creating indexes for delivery_queue...");
db.delivery_queue.createIndex({ "status": 1 });
db.delivery_queue.createIndex({ "ownerOrganizationId": 1 });
db.delivery_queue.createIndex({ "eventType": 1 });
db.delivery_queue.createIndex({ "channelType": 1 });
db.delivery_queue.createIndex({ "nextRetryAt": 1 });
db.delivery_queue.createIndex({ "priority": 1 }); // Nếu có priority field

print("Creating indexes for delivery_history...");
db.delivery_history.createIndex({ "ownerOrganizationId": 1 });
db.delivery_history.createIndex({ "eventType": 1 });
db.delivery_history.createIndex({ "channelType": 1 });
db.delivery_history.createIndex({ "status": 1 });
db.delivery_history.createIndex({ "sentAt": 1 });

print("==========================================");
print("Migration completed!");
print("==========================================");
print("⚠ IMPORTANT: After verifying the new collections work correctly,");
print("   you can drop the old collections:");
print("   db.notification_queue.drop()");
print("   db.notification_history.drop()");
print("==========================================");
```

## ✅ Kết Luận

**Khuyến nghị**: Đổi tên thành `delivery_queue` và `delivery_history` (Option 1)

**Lý do**:
1. ✅ Phân biệt rõ ràng 2 hệ thống
2. ✅ Phù hợp với module structure
3. ✅ Tên ngắn gọn, dễ hiểu
4. ✅ Dễ maintain và debug

**Lưu ý**:
- ⚠️ Cần migration script cẩn thận
- ⚠️ Cần update tất cả references
- ⚠️ Có thể cần downtime ngắn hoặc dual-write period
