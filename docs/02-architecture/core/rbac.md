# RBAC System

Tài liệu về hệ thống phân quyền Role-Based Access Control (RBAC).

## 📋 Tổng Quan

Hệ thống sử dụng RBAC để quản lý quyền truy cập. Cấu trúc bao gồm:

1. **Permission** - Quyền cụ thể
2. **Role** - Vai trò chứa nhiều permissions
3. **RolePermission** - Mapping giữa Role và Permission
4. **UserRole** - Mapping giữa User và Role
5. **Organization** - Tổ chức (roles thuộc về organization)

## 🏗️ Cấu Trúc

```
User
  ├── UserRole (nhiều)
  │     └── Role
  │           ├── RolePermission (nhiều)
  │           │     └── Permission
  │           └── Organization
```

## 🔑 Permission

### Định Nghĩa

Permission là quyền cụ thể trong hệ thống, có format: `<Module>.<Action>`

**Ví dụ:**
- `User.Read` - Đọc thông tin user
- `User.Create` - Tạo user
- `User.Update` - Cập nhật user
- `User.Delete` - Xóa user
- `Role.Read` - Đọc thông tin role
- `Role.Update` - Cập nhật role

### Scope

Mỗi permission có scope (mức độ quyền):
- `0`: Read (Đọc)
- `1`: Write (Ghi)
- `2`: Delete (Xóa)

## 👥 Role

### Định Nghĩa

Role là vai trò chứa nhiều permissions, thuộc về một Organization.

**Ví dụ:**
- `Administrator` - Quản trị viên (có tất cả quyền)
- `Manager` - Quản lý (có quyền quản lý)
- `User` - Người dùng thường (quyền cơ bản)

### Cấu Trúc

```json
{
  "_id": "role-id",
  "name": "Administrator",
  "code": "ADMIN",
  "organizationId": "org-id",
  "description": "Administrator role"
}
```

## 🔗 RolePermission

Mapping giữa Role và Permission.

**Cấu trúc:**
```json
{
  "_id": "mapping-id",
  "roleId": "role-id",
  "permissionId": "permission-id"
}
```

## 👤 UserRole

Mapping giữa User và Role.

**Cấu trúc:**
```json
{
  "_id": "mapping-id",
  "userId": "user-id",
  "roleId": "role-id"
}
```

## 🏢 Organization

Tổ chức theo cấu trúc cây. Roles thuộc về một Organization.

**Cấu trúc:**
```json
{
  "_id": "org-id",
  "name": "Root Organization",
  "code": "ROOT",
  "parentId": null
}
```

## 🔐 Authorization Flow

### 1. User Đăng Nhập

```
User → Firebase Auth → JWT Token (chứa userId)
```

### 2. Request Đến API

```
Request → Middleware → Verify JWT → Lấy userId
```

### 3. Kiểm Tra Permission

```
userId → UserRole → Role → RolePermission → Permission
```

### 4. So Sánh Permission

```
Required Permission vs User Permissions
```

### 5. Quyết Định

- Có permission → Cho phép request
- Không có permission → Trả về 403 Forbidden

## 💻 Implementation

### Middleware

**Vị trí:** `api/core/api/middleware/middleware.auth.go`

```go
func AuthMiddleware(requiredPermission string) fiber.Handler {
    return func(c fiber.Ctx) error {
        // 1. Verify JWT token
        userId := c.Locals("user_id")
        
        // 2. Lấy permissions của user
        permissions := getUserPermissions(userId)
        
        // 3. Kiểm tra permission
        if !hasPermission(permissions, requiredPermission) {
            return c.Status(403).JSON(fiber.Map{
                "error": "Forbidden",
            })
        }
        
        return c.Next()
    }
}
```

### Cache

Permissions của user được cache để tránh query database mỗi request.

## 📝 Best Practices

1. **Principle of Least Privilege**: Chỉ cấp quyền tối thiểu cần thiết
2. **Role-Based**: Sử dụng roles thay vì gán permission trực tiếp cho user
3. **Organization-Based**: Phân quyền theo organization
4. **Permission Naming**: Sử dụng format `<Module>.<Action>`

## 📚 Tài Liệu Liên Quan

- [RBAC APIs](../03-api/rbac.md)
- [Admin APIs](../03-api/admin.md)
- [Organization Structure](organization.md)

