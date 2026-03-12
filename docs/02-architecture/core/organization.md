# Organization Structure

Tài liệu về cấu trúc tổ chức trong hệ thống.

## 📋 Tổng Quan

Hệ thống sử dụng cấu trúc tổ chức theo cây (tree structure) để quản lý phân quyền theo tổ chức.

## 🏗️ Cấu Trúc

### Tree Structure

```
Root Organization
  ├── Department A
  │     ├── Team A1
  │     └── Team A2
  └── Department B
        ├── Team B1
        └── Team B2
```

### Schema

```json
{
  "_id": "ObjectId",
  "name": "Organization Name",
  "code": "ORG_CODE",
  "parentId": "ObjectId (nullable)",
  "createdAt": "Date",
  "updatedAt": "Date"
}
```

## 🔗 Relationship với Role

Mỗi Role thuộc về một Organization:

```json
{
  "_id": "role-id",
  "name": "Manager",
  "code": "MANAGER",
  "organizationId": "org-id",
  "permissions": ["permission-id-1", "permission-id-2"]
}
```

## 📝 Quy Tắc

1. **Root Organization**: Có `parentId = null`
2. **Child Organizations**: Có `parentId` trỏ đến parent
3. **Roles**: Thuộc về một Organization
4. **Permissions**: Có thể được gán cho nhiều Roles trong nhiều Organizations

## 🏢 Teams mặc định cho định tuyến thông báo

Hệ thống khởi tạo các team mặc định thuộc System Organization để định tuyến thông báo theo domain:

| Team Code | Tên | Domain nhận |
|-----------|-----|-------------|
| `TECH_TEAM` | Tech Team | system, security, user |
| `MARKETING_TEAM` | Marketing Team | ads, analytics |
| `SALES_TEAM` | Sales Team | conversation |
| `OPERATIONS_TEAM` | Vận hành (Operations) | order, payment |

Chi tiết: [Notification Domain và Severity](../systems/notification-domain-severity.md).

## 🔍 Use Cases

### 1. Phân Quyền Theo Tổ Chức

User trong Organization A chỉ có thể quản lý data của Organization A.

### 2. Multi-Tenant

Mỗi Organization là một tenant riêng biệt.

### 3. Hierarchical Permissions

Permissions có thể được kế thừa từ parent organization.

## 📚 Tài Liệu Liên Quan

- [RBAC System](rbac.md)
- [Database Schema](database.md)
- [Organization Structure Analysis](../organization-structure-analysis.md)

