# Tóm Tắt Relationship Tags Trong Các Model

## 📋 Tổng Quan

Tài liệu này liệt kê tất cả các model đã có relationship tag để bảo vệ khỏi việc xóa khi có quan hệ.

## ✅ Các Model Đã Có Relationship Tag

### 1. Role (`model.auth.role.go`)

**Quan hệ:**
- `user_roles` collection, field `roleId` → UserRole
- `role_permissions` collection, field `roleId` → RolePermission

**Tag:**
```go
_Relationships struct{} `relationship:"collection:user_roles,field:roleId,message:Không thể xóa role vì có %d user đang sử dụng role này. Vui lòng gỡ role khỏi các user trước.|collection:role_permissions,field:roleId,message:Không thể xóa role vì có %d permission đang được gán cho role này. Vui lòng gỡ các permission trước."`
```

### 2. Permission (`model.auth.permission.go`)

**Quan hệ:**
- `role_permissions` collection, field `permissionId` → RolePermission

**Tag:**
```go
_Relationships struct{} `relationship:"collection:role_permissions,field:permissionId,message:Không thể xóa permission vì có %d role đang sử dụng permission này. Vui lòng gỡ permission khỏi các role trước."`
```

### 3. Organization (`model.auth.organization.go`)

**Quan hệ:**
- `roles` collection, field `organizationId` → Role

**Tag:**
```go
_Relationships struct{} `relationship:"collection:roles,field:organizationId,message:Không thể xóa tổ chức vì có %d role trực thuộc. Vui lòng xóa hoặc di chuyển các role trước."`
```

**Lưu ý**: Organization cũng có quan hệ với children (organizations con), nhưng quan hệ này phức tạp (cần kiểm tra cả parentId và path), nên được xử lý bằng logic tùy chỉnh trong OrganizationService.

### 4. User (`model.auth.user.go`)

**Quan hệ:**
- `user_roles` collection, field `userId` → UserRole

**Tag:**
```go
_Relationships struct{} `relationship:"collection:user_roles,field:userId,message:Không thể xóa user vì có %d role đang được gán cho user này. Vui lòng gỡ các role trước."`
```

### 5. NotificationChannel (`model.notification.channel.go`)

**Quan hệ:**
- `notification_queue` collection, field `channelId` → NotificationQueueItem
- `notification_history` collection, field `channelId` → NotificationHistory

**Tag:**
```go
_Relationships struct{} `relationship:"collection:notification_queue,field:channelId,message:Không thể xóa channel vì có %d notification đang trong queue. Vui lòng xử lý hoặc xóa các notification trước.|collection:notification_history,field:channelId,message:Không thể xóa channel vì có %d notification trong lịch sử. Vui lòng xóa lịch sử trước."`
```

## ❌ Các Model Không Có Relationship Tag

### Lý Do: Quan Hệ Không Dùng ObjectID

Các model sau có quan hệ nhưng không thể dùng relationship tag vì quan hệ dùng string hoặc int64 thay vì ObjectID:

1. **FbPage** - Quan hệ với FbPost, FbConversation (dùng `pageId` string)
2. **FbPost** - Quan hệ với FbPage (dùng `pageId` string)
3. **FbConversation** - Quan hệ với FbMessage, FbMessageItem (dùng `conversationId` string)
4. **FbMessage** - Quan hệ với FbMessageItem (dùng `conversationId` string)
5. **PcPosShop** - Quan hệ với PcPosProduct, PcPosOrder, PcPosCategory (dùng `shopId` int64)
6. **PcPosProduct** - Quan hệ với PcPosVariation (dùng `productId` string)
7. **PcPosCategory** - Quan hệ với PcPosProduct (dùng `categoryIds` array, không phải foreign key đơn giản)

**Giải pháp**: Các quan hệ này cần được xử lý bằng logic tùy chỉnh trong service nếu cần bảo vệ.

### Lý Do: Mapping Tables

Các model sau là mapping tables, không cần bảo vệ:

1. **UserRole** - Mapping giữa User và Role
2. **RolePermission** - Mapping giữa Role và Permission

### Lý Do: Không Có Quan Hệ

Các model sau không có quan hệ với các model khác:

1. **Agent** - Không có quan hệ
2. **AccessToken** - Không có quan hệ
3. **FbCustomer** - Không có quan hệ (chỉ có pageId string)
4. **PcPosCustomer** - Không có quan hệ (chỉ có shopId int64)
5. **PcPosWarehouse** - Không có quan hệ (chỉ có shopId int64)
6. **PcPosOrder** - Không có quan hệ (chỉ có shopId, customerId, warehouseId - không phải ObjectID)
7. **NotificationTemplate** - Không có quan hệ trực tiếp (không có templateId trong queue/history)
8. **NotificationSender** - Không có quan hệ trực tiếp (chỉ có trong SenderIDs array của NotificationChannel)
9. **NotificationRoutingRule** - Không có quan hệ trực tiếp (chỉ có OrganizationIDs array)

## 📝 Ghi Chú

### Quan Hệ Phức Tạp

Một số quan hệ phức tạp không thể xử lý bằng relationship tag đơn giản:

1. **Organization → Organization (children)**: Cần kiểm tra cả `parentId` và `path` với regex
2. **PcPosCategory → PcPosProduct**: Quan hệ qua `categoryIds` array, không phải foreign key đơn giản
3. **NotificationChannel → NotificationSender**: Quan hệ qua `SenderIDs` array

Các quan hệ này cần logic tùy chỉnh trong service.

### Hạn Chế Hiện Tại

Hệ thống relationship tag hiện tại chỉ hỗ trợ:
- Quan hệ với ObjectID (primitive.ObjectID)
- Foreign key đơn giản (một field trỏ tới một ObjectID)

Không hỗ trợ:
- Quan hệ với string IDs
- Quan hệ với int64 IDs
- Quan hệ qua array fields
- Quan hệ phức tạp (regex, multiple conditions)

## 🔄 Cập Nhật

Khi thêm model mới có quan hệ với ObjectID, nhớ:
1. Thêm field `_Relationships` với struct tag `relationship`
2. Định nghĩa collection name từ `global.MongoDB_ColNames`
3. Định nghĩa field name trong collection đích
4. Cung cấp error message rõ ràng

## 📚 Tài Liệu Liên Quan

- `relationship-protection-struct-tag.md`: Hướng dẫn sử dụng relationship tag
- `service.relationship.parser.go`: Parser cho struct tag
- `service.relationship.helper.go`: Helper functions để kiểm tra quan hệ
