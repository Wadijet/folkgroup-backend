# Database Schema

Tài liệu về cấu trúc database và các collections.

## 📋 Tổng Quan

Hệ thống sử dụng MongoDB với 3 databases:
- **folkform_auth**: Authentication và authorization data
- **folkform_staging**: Staging data
- **folkform_data**: Business data

## 🗄️ Collections

### Auth Collections

#### users

Lưu thông tin người dùng.

**Schema:**
```json
{
  "_id": "ObjectId",
  "firebaseUid": "string (unique)",
  "email": "string (sparse, unique)",
  "phone": "string (sparse, unique)",
  "name": "string",
  "avatarUrl": "string",
  "emailVerified": "boolean",
  "phoneVerified": "boolean",
  "tokens": ["string"],
  "createdAt": "Date",
  "updatedAt": "Date"
}
```

**Indexes:**
- `firebaseUid`: unique
- `email`: sparse, unique
- `phone`: sparse, unique

#### roles

Lưu thông tin vai trò.

**Schema:**
```json
{
  "_id": "ObjectId",
  "name": "string",
  "code": "string",
  "organizationId": "ObjectId",
  "description": "string",
  "createdAt": "Date",
  "updatedAt": "Date"
}
```

**Indexes:**
- `code`: unique
- `organizationId`: index

#### permissions

Lưu thông tin quyền.

**Schema:**
```json
{
  "_id": "ObjectId",
  "name": "string",
  "code": "string",
  "module": "string",
  "action": "string",
  "scope": "number",
  "description": "string",
  "createdAt": "Date",
  "updatedAt": "Date"
}
```

**Indexes:**
- `code`: unique

#### role_permissions

Mapping giữa Role và Permission.

**Schema:**
```json
{
  "_id": "ObjectId",
  "roleId": "ObjectId",
  "permissionId": "ObjectId",
  "createdAt": "Date"
}
```

**Indexes:**
- `roleId`: index
- `permissionId`: index
- `roleId + permissionId`: unique compound

#### user_roles

Mapping giữa User và Role.

**Schema:**
```json
{
  "_id": "ObjectId",
  "userId": "ObjectId",
  "roleId": "ObjectId",
  "createdAt": "Date"
}
```

**Indexes:**
- `userId`: index
- `roleId`: index
- `userId + roleId`: unique compound

#### organizations

Lưu thông tin tổ chức (cấu trúc cây).

**Schema:**
```json
{
  "_id": "ObjectId",
  "name": "string",
  "code": "string",
  "parentId": "ObjectId (nullable)",
  "createdAt": "Date",
  "updatedAt": "Date"
}
```

**Indexes:**
- `code`: unique
- `parentId`: index

#### agents

Lưu thông tin agent.

**Schema:**
```json
{
  "_id": "ObjectId",
  "name": "string",
  "code": "string",
  "status": "string",
  "checkInTime": "Date",
  "checkOutTime": "Date",
  "createdAt": "Date",
  "updatedAt": "Date"
}
```

### Facebook Collections

#### fb_pages

Lưu thông tin Facebook Pages.

#### fb_posts

Lưu thông tin Facebook Posts.

#### fb_conversations

Lưu thông tin Facebook Conversations.

#### fb_messages

Lưu thông tin Facebook Messages.

### Pancake Collections

#### pc_orders

Lưu thông tin Pancake Orders.

#### pc_access_tokens

Lưu thông tin Access Tokens.

## 🔗 Relationships

### User → UserRole → Role → RolePermission → Permission

```
User (1) ──→ (N) UserRole (N) ──→ (1) Role (1) ──→ (N) RolePermission (N) ──→ (1) Permission
```

### Organization → Role

```
Organization (1) ──→ (N) Role
```

## 📝 Indexing Strategy

### Unique Indexes

- `users.firebaseUid`: Đảm bảo mỗi Firebase user chỉ có một record
- `users.email`: Đảm bảo email unique (sparse - cho phép null)
- `users.phone`: Đảm bảo phone unique (sparse - cho phép null)
- `roles.code`: Đảm bảo role code unique
- `permissions.code`: Đảm bảo permission code unique
- `organizations.code`: Đảm bảo organization code unique

### Compound Indexes

- `role_permissions.roleId + permissionId`: Đảm bảo không trùng lặp mapping
- `user_roles.userId + roleId`: Đảm bảo không trùng lặp mapping

### Regular Indexes

- `roles.organizationId`: Tăng tốc query roles theo organization
- `user_roles.userId`: Tăng tốc query roles của user
- `user_roles.roleId`: Tăng tốc query users của role

## 🔍 Query Patterns

### Lấy Permissions của User

```javascript
// 1. Lấy UserRoles của user
userRoles = db.user_roles.find({ userId: userId })

// 2. Lấy Roles
roleIds = userRoles.map(ur => ur.roleId)
roles = db.roles.find({ _id: { $in: roleIds } })

// 3. Lấy RolePermissions
rolePermissions = db.role_permissions.find({ roleId: { $in: roleIds } })

// 4. Lấy Permissions
permissionIds = rolePermissions.map(rp => rp.permissionId)
permissions = db.permissions.find({ _id: { $in: permissionIds } })
```

## 📚 Tài Liệu Liên Quan

- [RBAC System](rbac.md)
- [Organization Structure](organization.md)
- [Tổng Quan Kiến Trúc](tong-quan.md)

