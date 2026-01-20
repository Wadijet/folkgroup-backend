# Logic Chọn Context Làm Việc

## 📋 Tổng Quan

Hệ thống sử dụng **Context Switching** để quản lý quyền truy cập dữ liệu theo organization. 

**QUAN TRỌNG:** Context làm việc là **ROLE**, không phải organization. User phải chọn một **ROLE** để làm việc. Từ role, hệ thống tự động xác định organization tương ứng. Context này được lưu và áp dụng cho tất cả requests.

## 🔄 Flow Chọn Context

### 1. User Đăng Nhập

User đăng nhập thành công → Lấy danh sách roles của user

**Endpoint:** `GET /api/v1/auth/roles`

**Response:**
```json
{
  "status": "success",
  "data": [
    {
      "roleId": "role-id-1",
      "roleName": "Manager",
      "organizationId": "org-id-1",
      "organizationName": "Company A",
      "organizationCode": "COMPANY_A",
      "organizationType": "company",
      "organizationLevel": 1
    },
    {
      "roleId": "role-id-2",
      "roleName": "Employee",
      "organizationId": "org-id-2",
      "organizationName": "Company B",
      "organizationCode": "COMPANY_B",
      "organizationType": "company",
      "organizationLevel": 1
    }
  ]
}
```

### 2. User Chọn Context (Role)

**Logic:**
- Nếu user có **1 role** → Tự động chọn role đó
- Nếu user có **nhiều roles** → User phải chọn một role để làm việc

**Frontend Implementation:**
```javascript
// Lấy danh sách roles
const roles = await api.get('/auth/roles');

// Nếu có nhiều roles, hiển thị cho user chọn
if (roles.length > 1) {
  const selectedRole = await showRoleSelector(roles);
  // Lưu vào localStorage
  localStorage.setItem('activeRoleId', selectedRole.roleId);
  localStorage.setItem('activeOrganizationId', selectedRole.organizationId);
} else if (roles.length === 1) {
  // Tự động chọn role duy nhất
  localStorage.setItem('activeRoleId', roles[0].roleId);
  localStorage.setItem('activeOrganizationId', roles[0].organizationId);
}
```

### 3. Mỗi Request Gửi Kèm Context

**Frontend gửi header:**
```javascript
// Mỗi request gửi kèm header - CONTEXT LÀ ROLE ID
axios.defaults.headers.common['X-Active-Role-ID'] = localStorage.getItem('activeRoleId');
```

**Backend xử lý:**
- Middleware `OrganizationContextMiddleware` đọc header `X-Active-Role-ID` (ROLE ID)
- Validate user có role này không
- Từ role, lấy organization ID tương ứng
- Lưu `active_role_id` (PRIMARY) và `active_organization_id` (DERIVED) vào context
- Tất cả các operations sau đó tự động dùng `active_organization_id` để filter dữ liệu

**Lưu ý quan trọng:**
- ✅ Context làm việc = **ROLE ID** (được gửi trong header)
- ✅ Organization ID được **tự động suy ra** từ role
- ❌ KHÔNG gửi organization ID trực tiếp trong header

## ⚠️ Vấn Đề: "Role Làm Việc Lại Ra Cả Cây"

### Vấn Đề

Khi frontend user lấy list context (roles để làm việc), nó lại trả về **cả cây organization** thay vì chỉ các **role làm việc**.

### Nguyên Nhân

1. **Endpoint `/auth/roles` chỉ trả về danh sách phẳng các role** - KHÔNG trả về cây
2. **Frontend có thể đang tự build tree** từ danh sách organizations
3. **Logic build tree có thể đang lấy cả children/parents** thay vì chỉ organization trực tiếp của role

### Giải Pháp

**Endpoint `/auth/roles` CHỈ trả về:**
- ✅ Các role **trực tiếp** của user
- ✅ Organization **trực tiếp** của mỗi role
- ❌ **KHÔNG** bao gồm children organizations
- ❌ **KHÔNG** bao gồm parent organizations
- ❌ **KHÔNG** build tree structure

**Mỗi role trong response = một context làm việc**

**Quan trọng:**
- Context làm việc = **ROLE** (không phải organization)
- Mỗi role tương ứng với một organization
- Khi chọn role, organization được tự động xác định

### Logic Đúng

```go
// HandleGetUserRoles - CHỈ lấy các role trực tiếp của user
func (h *UserHandler) HandleGetUserRoles(c fiber.Ctx) error {
    // 1. Lấy UserRoles của user
    userRoles, err := h.userRoleService.Find(ctx, bson.M{"userId": objID}, nil)
    
    // 2. Với mỗi UserRole, lấy Role và Organization trực tiếp
    for _, userRole := range userRoles {
        role, _ := h.roleService.FindOneById(ctx, userRole.RoleID)
        org, _ := organizationService.FindOneById(ctx, role.OrganizationID)
        
        // 3. Trả về thông tin role và organization trực tiếp
        // KHÔNG lấy children/parents
        result = append(result, map[string]interface{}{
            "roleId": role.ID.Hex(),
            "roleName": role.Name,
            "organizationId": org.ID.Hex(),
            "organizationName": org.Name,
            // ...
        })
    }
}
```

## 📊 So Sánh: Context Làm Việc vs Allowed Organizations

### Context Làm Việc (Working Context)

**Mục đích:** User chọn một role để làm việc

**Endpoint:** `GET /api/v1/auth/roles`

**Trả về:**
- ✅ Chỉ các role **trực tiếp** của user
- ✅ Mỗi role = một context làm việc
- ❌ KHÔNG bao gồm children/parents

**Ví dụ:**
```json
[
  {
    "roleId": "role-1",
    "roleName": "Manager",
    "organizationId": "org-1",
    "organizationName": "Company A"
  }
]
```

### Allowed Organizations (Quyền Truy Cập)

**Mục đích:** Xác định user có thể truy cập dữ liệu của organizations nào

**Function:** `GetUserAllowedOrganizationIDs()`

**Trả về:**
- ✅ Organization của role
- ✅ Children organizations (nếu Scope = 1)
- ✅ Parent organizations (inverse lookup)

**Ví dụ:**
```go
// User có role ở "Company A" với Scope = 1
// Allowed organizations: [Company A, Department A1, Department A2, ...]
allowedOrgIDs := GetUserAllowedOrganizationIDs(ctx, userID, "User.Read")
```

## 🎯 Kết Luận

1. **Endpoint `/auth/roles`** → Trả về danh sách **context làm việc** (chỉ role trực tiếp)
2. **Function `GetUserAllowedOrganizationIDs()`** → Trả về danh sách **organizations được phép truy cập** (bao gồm children/parents)
3. **Frontend không nên build tree** từ endpoint `/auth/roles`
4. **Mỗi role trong response = một context làm việc** - user chọn một role để làm việc

## 📝 Lưu Ý Quan Trọng

### Context Làm Việc = ROLE

- ✅ **Context làm việc** là **ROLE** mà user **chọn** để làm việc
- ✅ Frontend gửi **ROLE ID** trong header `X-Active-Role-ID`
- ✅ Backend từ role tự động suy ra organization tương ứng
- ❌ **KHÔNG** gửi organization ID trực tiếp trong header

### Phân Biệt

- **Context làm việc (Working Context):** ROLE mà user chọn → Một role cụ thể
- **Allowed organizations:** Danh sách organizations mà user có quyền truy cập (dựa trên scope) → Nhiều organizations (bao gồm children/parents)
- **KHÔNG nhầm lẫn** giữa 2 khái niệm này

### Tại Sao Context Là ROLE?

1. **ROLE chứa thông tin đầy đủ:**
   - Role có organization ID
   - Role có permissions
   - Role có scope

2. **ROLE là đơn vị làm việc:**
   - User làm việc với một role cụ thể
   - Mỗi role có quyền hạn riêng
   - Organization chỉ là nơi role thuộc về

3. **ROLE linh hoạt hơn:**
   - Cùng một organization có thể có nhiều roles
   - User có thể chọn role khác nhau trong cùng organization (nếu có)
   - Dễ mở rộng trong tương lai
