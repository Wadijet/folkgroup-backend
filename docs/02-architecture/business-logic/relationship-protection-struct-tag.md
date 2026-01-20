# Bảo Vệ Quan Hệ Tự Động Bằng Struct Tag

## 📋 Tổng Quan

Hệ thống tự động bảo vệ các record có quan hệ bằng cách định nghĩa quan hệ ngay trong struct model thông qua struct tag `relationship`. Khi thực hiện các thao tác xóa (DeleteOne, DeleteById, DeleteMany, FindOneAndDelete), hệ thống sẽ tự động kiểm tra các quan hệ đã định nghĩa và ngăn chặn việc xóa nếu có record khác đang tham chiếu.

## 🎯 Ưu Điểm

1. **Tự động**: Không cần override methods trong service, tự động kiểm tra trong BaseServiceMongoImpl
2. **Declarative**: Định nghĩa quan hệ ngay trong model, dễ đọc và bảo trì
3. **Type-safe**: Sử dụng struct tag, được kiểm tra tại compile time
4. **Tập trung**: Tất cả quan hệ được định nghĩa ở một nơi (model)
5. **Không cần code thủ công**: Không cần viết validateBeforeDelete cho mỗi service

## 📝 Cách Sử Dụng

### Bước 1: Định Nghĩa Quan Hệ Trong Model

Thêm field ẩn `_Relationships` với struct tag `relationship` vào model:

```go
type Role struct {
    _Relationships struct{} `relationship:"collection:user_roles,field:roleId,message:Không thể xóa role vì có %d user đang sử dụng role này. Vui lòng gỡ role khỏi các user trước.|collection:role_permissions,field:roleId,message:Không thể xóa role vì có %d permission đang được gán cho role này. Vui lòng gỡ các permission trước."`
    ID             primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
    // ... các field khác
}
```

### Bước 2: Format Struct Tag

#### Format Cơ Bản

```
relationship:"collection:<tên_collection>,field:<tên_field>,message:<thông_báo_lỗi>"
```

#### Nhiều Quan Hệ

Phân tách nhiều quan hệ bằng dấu `|`:

```
relationship:"collection:user_roles,field:roleId,message:...|collection:role_permissions,field:roleId,message:..."
```

#### Các Tham Số

- **collection** (bắt buộc): Tên collection cần kiểm tra
- **field** (bắt buộc): Tên field trong collection đó trỏ tới record hiện tại
- **message** (tùy chọn): Thông báo lỗi (có thể dùng `%d` để thay thế số lượng)
- **optional** (tùy chọn): `true` nếu collection có thể không tồn tại
- **cascade** (tùy chọn): `true` nếu cho phép xóa cascade (bỏ qua kiểm tra)

### Bước 3: Sử Dụng Collection Name

Sử dụng tên collection từ `global.MongoDB_ColNames`:

```go
relationship:"collection:user_roles,field:roleId,message:..."
```

Các collection names có sẵn:
- `user_roles`
- `role_permissions`
- `roles`
- `permissions`
- `organizations`
- ... (xem `global.MongoDB_ColNames`)

## 📚 Ví Dụ

### Ví Dụ 1: Role Model

Role có quan hệ với:
- UserRole (user_roles collection, field roleId)
- RolePermission (role_permissions collection, field roleId)

```go
type Role struct {
    _Relationships struct{} `relationship:"collection:user_roles,field:roleId,message:Không thể xóa role vì có %d user đang sử dụng role này. Vui lòng gỡ role khỏi các user trước.|collection:role_permissions,field:roleId,message:Không thể xóa role vì có %d permission đang được gán cho role này. Vui lòng gỡ các permission trước."`
    ID             primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
    Name           string             `json:"name" bson:"name"`
    // ... các field khác
}
```

### Ví Dụ 2: Permission Model

Permission có quan hệ với:
- RolePermission (role_permissions collection, field permissionId)

```go
type Permission struct {
    _Relationships struct{} `relationship:"collection:role_permissions,field:permissionId,message:Không thể xóa permission vì có %d role đang sử dụng permission này. Vui lòng gỡ permission khỏi các role trước."`
    ID             primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
    Name           string             `json:"name" bson:"name"`
    // ... các field khác
}
```

### Ví Dụ 3: Organization Model

Organization có quan hệ với:
- Role (roles collection, field organizationId)

```go
type Organization struct {
    _Relationships struct{} `relationship:"collection:roles,field:organizationId,message:Không thể xóa tổ chức vì có %d role trực thuộc. Vui lòng xóa hoặc di chuyển các role trước."`
    ID             primitive.ObjectID  `json:"id,omitempty" bson:"_id,omitempty"`
    Name           string              `json:"name" bson:"name"`
    // ... các field khác
}
```

**Lưu ý**: Organization cũng có quan hệ với children (organizations con), nhưng quan hệ này phức tạp hơn (cần kiểm tra cả parentId và path), nên vẫn được xử lý bằng logic tùy chỉnh trong OrganizationService.

## 🔧 Cách Hoạt Động

1. **Khi gọi Delete**: BaseServiceMongoImpl tự động gọi `validateRelationshipsDelete`
2. **Parse Tag**: Hàm `ParseRelationshipTag` đọc struct tag từ model
3. **Kiểm Tra Quan Hệ**: Sử dụng `CheckRelationshipExists` để kiểm tra trong database
4. **Trả Về Lỗi**: Nếu tìm thấy quan hệ, trả về lỗi với message đã định nghĩa

## ⚠️ Lưu Ý

### 1. Field `_Relationships`

- Field này **không được export** (bắt đầu bằng `_`)
- Chỉ dùng để chứa struct tag, không lưu vào database
- Có thể đặt ở bất kỳ vị trí nào trong struct

### 2. Collection Names

- Phải sử dụng đúng tên collection từ `global.MongoDB_ColNames`
- Tên collection phải đã được đăng ký trong `RegistryCollections`

### 3. Field Names

- Field name phải đúng với tên field trong collection đích
- Đảm bảo field đã được index để tối ưu performance

### 4. Error Messages

- Có thể dùng `%d` để hiển thị số lượng record đang tham chiếu
- Nên cung cấp hướng dẫn rõ ràng cho người dùng (ví dụ: "Vui lòng gỡ role khỏi các user trước")

### 5. Quan Hệ Phức Tạp

Đối với các quan hệ phức tạp (ví dụ: kiểm tra children trong cây), vẫn cần logic tùy chỉnh trong service:

```go
func (s *OrganizationService) validateBeforeDelete(ctx context.Context, orgID primitive.ObjectID) error {
    // Kiểm tra children (logic tùy chỉnh)
    childrenFilter := bson.M{
        "$or": []bson.M{
            {"parentId": orgID},
            {"path": bson.M{"$regex": "^" + org.Path + "/"}},
        },
    }
    // ... kiểm tra children
    
    // Kiểm tra quan hệ trực tiếp (tự động từ struct tag)
    // BaseServiceMongoImpl sẽ tự động gọi validateRelationshipsDelete
    return nil
}
```

## 🎯 Best Practices

1. **Luôn định nghĩa quan hệ**: Đối với các model có quan hệ, luôn thêm `_Relationships` field
2. **Message rõ ràng**: Cung cấp thông báo lỗi rõ ràng, hướng dẫn người dùng cách xử lý
3. **Index foreign keys**: Đảm bảo các field tham chiếu đã được index
4. **Test kỹ**: Test các trường hợp có và không có quan hệ
5. **Kết hợp với logic tùy chỉnh**: Sử dụng struct tag cho quan hệ đơn giản, logic tùy chỉnh cho quan hệ phức tạp

## 📖 So Sánh Với Cách Cũ

### Cách Cũ (Manual)

```go
// Trong service
func (s *RoleService) validateBeforeDelete(ctx context.Context, roleID primitive.ObjectID) error {
    checks := []RelationshipCheck{
        {
            CollectionName: global.MongoDB_ColNames.UserRoles,
            FieldName:      "roleId",
            ErrorMessage:   "Không thể xóa role vì có %d user đang sử dụng role này.",
        },
    }
    return CheckRelationshipExists(ctx, roleID, checks)
}

// Phải override tất cả delete methods
func (s *RoleService) DeleteOne(ctx context.Context, filter interface{}) error {
    // ... code kiểm tra
}
```

### Cách Mới (Struct Tag)

```go
// Trong model
type Role struct {
    _Relationships struct{} `relationship:"collection:user_roles,field:roleId,message:..."`
    // ... các field khác
}

// Không cần override methods, tự động hoạt động!
```

## 🔍 Debugging

Nếu quan hệ không hoạt động:

1. Kiểm tra struct tag format đúng chưa
2. Kiểm tra collection name có tồn tại trong `global.MongoDB_ColNames` không
3. Kiểm tra field name có đúng với field trong collection đích không
4. Kiểm tra collection đã được đăng ký trong `RegistryCollections` chưa

## 📚 Tài Liệu Liên Quan

- `service.relationship.parser.go`: Parser cho struct tag
- `service.relationship.helper.go`: Helper functions để kiểm tra quan hệ
- `service..base.mongo.go`: BaseServiceMongoImpl với auto-validation
- `model.auth.role.go`: Ví dụ implementation
- `model.auth.permission.go`: Ví dụ implementation
- `model.auth.organization.go`: Ví dụ implementation
