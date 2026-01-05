# Phân Tích và Tối Ưu Endpoints

## 🔴 Vấn Đề: Trùng Lặp Routes

### 1. History Routes - TRÙNG LẶP
```go
// Cả 2 đều dùng cùng handler và model
/notification/history  → NotificationHistoryHandler → DeliveryHistory model
/delivery/history      → DeliveryHistoryHandler     → DeliveryHistory model
```
**Vấn đề**: Cả 2 routes trỏ đến cùng một resource (DeliveryHistory). Không cần thiết có 2 routes.

**Giải pháp**: Xóa `/delivery/history`, chỉ giữ `/notification/history` (vì NotificationHistoryHandler đã là alias)

### 2. Sender Routes - TRÙNG LẶP
```go
// Cả 2 đều dùng cùng service và model
/notification/sender  → NotificationSenderHandler → NotificationSenderService → NotificationChannelSender
/delivery/sender     → DeliverySenderHandler     → NotificationSenderService → NotificationChannelSender
```
**Vấn đề**: Cả 2 routes trỏ đến cùng một resource (NotificationChannelSender). Không cần thiết có 2 routes.

**Giải pháp**: Xóa `/delivery/sender`, chỉ giữ `/notification/sender`

### 3. Tracking Routes - TRÙNG LẶP
```go
// Cả 2 đều làm việc giống hệt nhau
/notification/track/open/:historyId
/notification/track/:historyId/:ctaIndex
/notification/confirm/:historyId

/delivery/track/open/:historyId
/delivery/track/:historyId/:ctaIndex
/delivery/confirm/:historyId
```
**Vấn đề**: Tracking không phụ thuộc vào namespace (notification hay delivery). Cả 2 đều track trên DeliveryHistory. Không cần thiết có 2 bộ routes.

**Giải pháp**: Xóa `/delivery/track/*`, chỉ giữ `/notification/track/*`

## ✅ Endpoints Cần Giữ (Đặc Thù, Không Trùng)

### 1. Notification Trigger - CẦN THIẾT
```go
POST /notification/trigger
```
**Lý do**: Endpoint đặc thù cho Notification System (Hệ thống 2) - trigger notification với routing và template rendering.

### 2. Delivery Send - CẦN THIẾT
```go
POST /delivery/send
```
**Lý do**: Endpoint đặc thù cho Delivery System (Hệ thống 1) - gửi notification trực tiếp không qua routing.

## 📋 Đề Xuất Refactor

### Xóa Các Routes Trùng Lặp

1. **Xóa `/delivery/history`**
   - Lý do: Trùng với `/notification/history`
   - Action: Xóa route và handler `DeliveryHistoryHandler` (hoặc giữ handler nhưng không register route)

2. **Xóa `/delivery/sender`**
   - Lý do: Trùng với `/notification/sender`
   - Action: Xóa route và handler `DeliverySenderHandler`

3. **Xóa `/delivery/track/*`**
   - Lý do: Trùng với `/notification/track/*`
   - Action: Xóa route và handler `DeliveryTrackHandler`

### Giữ Lại Các Routes

✅ **Notification System (Hệ thống 2)**:
- `/notification/sender` - CRUD (dùng base handler)
- `/notification/channel` - CRUD (dùng base handler)
- `/notification/template` - CRUD (dùng base handler)
- `/notification/routing` - CRUD (dùng base handler)
- `/notification/history` - CRUD read-only (dùng base handler)
- `/notification/trigger` - Custom endpoint (cần thiết)

✅ **Delivery System (Hệ thống 1)**:
- `/delivery/send` - Custom endpoint (cần thiết)

✅ **Tracking (Public, không cần auth)**:
- `/notification/track/open/:historyId` - Custom endpoint (cần thiết)
- `/notification/track/:historyId/:ctaIndex` - Custom endpoint (cần thiết)
- `/notification/confirm/:historyId` - Custom endpoint (cần thiết)

## 🎯 Kết Quả Sau Refactor

### Routes Còn Lại (Tối Ưu)

**Notification System**:
```
GET    /notification/sender          → CRUD (base handler)
POST   /notification/sender          → CRUD (base handler)
PUT    /notification/sender/:id      → CRUD (base handler)
DELETE /notification/sender/:id      → CRUD (base handler)
... (tương tự cho channel, template, routing)

GET    /notification/history          → CRUD read-only (base handler)
GET    /notification/history/:id     → CRUD read-only (base handler)
... (các CRUD operations khác)

POST   /notification/trigger        → Custom (cần thiết)
```

**Delivery System**:
```
POST   /delivery/send               → Custom (cần thiết)
```

**Tracking (Public)**:
```
GET    /notification/track/open/:historyId        → Custom (cần thiết)
GET    /notification/track/:historyId/:ctaIndex   → Custom (cần thiết)
GET    /notification/confirm/:historyId           → Custom (cần thiết)
```

## 📝 Implementation Plan

### Step 1: Xóa Routes Trùng Lặp
- [ ] Xóa `/delivery/history` route
- [ ] Xóa `/delivery/sender` route
- [ ] Xóa `/delivery/track/*` routes

### Step 2: Xóa Handlers Không Dùng (Optional)
- [ ] Xóa `DeliveryHistoryHandler` (hoặc giữ lại nhưng không register)
- [ ] Xóa `DeliverySenderHandler`
- [ ] Xóa `DeliveryTrackHandler`

### Step 3: Update Documentation
- [ ] Update API documentation
- [ ] Update endpoint list

## ✅ Lợi Ích

1. **Giảm complexity**: Ít routes hơn, dễ maintain
2. **Tránh nhầm lẫn**: Không còn 2 routes cho cùng 1 resource
3. **Consistency**: Mỗi resource chỉ có 1 route
4. **Dễ hiểu**: Rõ ràng hơn về namespace và responsibility
