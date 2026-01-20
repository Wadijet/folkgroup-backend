# Logic Share Quyền Dữ Liệu

**Mục đích:** Mô tả chi tiết logic chia sẻ quyền dữ liệu giữa các tổ chức trong hệ thống.

---

## 📋 Tổng Quan

Hệ thống sử dụng cơ chế **Organization-Level Sharing** để cho phép tổ chức này chia sẻ dữ liệu với tổ chức khác. Logic này hoạt động song song với cơ chế phân quyền dữ liệu dựa trên `OwnerOrganizationID` và `Scope` của permission.

---

## 🏗️ Kiến Trúc

### 1. **Mô Hình Dữ Liệu**

#### **OrganizationShare Model**

```go
type OrganizationShare struct {
    ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
    OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId"` // Tổ chức sở hữu dữ liệu (share data với ToOrgID)
    ToOrgID             primitive.ObjectID `json:"toOrgId" bson:"toOrgId"`                           // Tổ chức nhận data
    PermissionNames     []string          `json:"permissionNames,omitempty" bson:"permissionNames,omitempty"` // [] hoặc nil = tất cả permissions
    CreatedAt           int64             `json:"createdAt" bson:"createdAt"`
    CreatedBy           primitive.ObjectID `json:"createdBy" bson:"createdBy"`
}
```

**Ý nghĩa các trường:**
- `OwnerOrganizationID`: Tổ chức sở hữu dữ liệu, muốn chia sẻ với `ToOrgID`
- `ToOrgID`: Tổ chức nhận dữ liệu được chia sẻ
- `PermissionNames`: 
  - `[]` hoặc `nil` → Share với **tất cả permissions**
  - `["Order.Read", "Order.Create"]` → Chỉ share với permissions cụ thể

**Collection:** `auth_organization_shares`

---

## 🔄 Luồng Xử Lý

### **Bước 1: Tính Toán Allowed Organizations (Từ Scope)**

Khi user thực hiện query, hệ thống tính toán các organizations mà user được phép truy cập dựa trên:

1. **Roles của user**
2. **Permissions trong mỗi role**
3. **Scope của permission** (0 hoặc 1)

```go
// service.organization.helper.go:GetUserAllowedOrganizationIDs()

// 1. Lấy tất cả roles của user
userRoles := GetUserRoles(userID)

// 2. Duyệt qua từng role
for _, userRole := range userRoles {
    role := GetRole(userRole.RoleID)
    orgID := role.OwnerOrganizationID
    
    // 3. Lấy permissions của role
    rolePermissions := GetRolePermissions(role.ID)
    
    // 4. Tính toán allowed org IDs dựa trên scope
    for _, rp := range rolePermissions {
        if rp.Scope == 0 {
            // Scope 0: Chỉ organization của role
            allowedOrgIDsMap[orgID] = true
        } else if rp.Scope == 1 {
            // Scope 1: Organization + children
            allowedOrgIDsMap[orgID] = true
            childrenIDs := GetChildrenIDs(orgID)
            for _, childID := range childrenIDs {
                allowedOrgIDsMap[childID] = true
            }
        }
    }
}

// Kết quả: allowedOrgIDs = [team_a, team_b, ...]
```

**Ví dụ:**
```
User có Role A thuộc "Team Bán Hàng A" với Scope 0
→ allowedOrgIDs = [team_a]

User có Role B thuộc "Phòng Kinh Doanh" với Scope 1
→ allowedOrgIDs = [sales_dept, team_a, team_b] (sales_dept + children)
```

### **Bước 2: Lấy Shared Organizations**

Sau khi có `allowedOrgIDs`, hệ thống tìm các organizations được share với user:

```go
// service.organization.share.go:GetSharedOrganizationIDs()

// Query: Tìm các share records có toOrgId trong allowedOrgIDs
filter := bson.M{
    "toOrgId": bson.M{"$in": userOrgIDs}, // userOrgIDs = allowedOrgIDs từ bước 1
}

// Nếu có permissionName cụ thể, filter thêm
if permissionName != "" {
    filter["$or"] = []bson.M{
        {"permissionNames": bson.M{"$exists": false}}, // Không có field = share tất cả
        {"permissionNames": bson.M{"$size": 0}},       // Array rỗng = share tất cả
        {"permissionNames": bson.M{"$in": []string{permissionName}}}, // Chứa permissionName
    }
}

// Query shares
shares := Find(filter)

// Lấy OwnerOrganizationID từ shares (organizations share data với user)
sharedOrgIDs = [sales_dept, ...] // Organizations share data với user's organizations
```

**Ví dụ:**
```
allowedOrgIDs = [team_a]

Query: toOrgId IN [team_a]
Kết quả: 
- Share record: OwnerOrganizationID = sales_dept, ToOrgID = team_a
→ sharedOrgIDs = [sales_dept]
```

### **Bước 3: Hợp Nhất Allowed và Shared Organizations**

Hợp nhất `allowedOrgIDs` và `sharedOrgIDs` để có danh sách cuối cùng:

```go
// handler.base.go:applyOrganizationFilter()

// 1. Lấy allowed organization IDs (từ scope)
allowedOrgIDs, err := services.GetUserAllowedOrganizationIDs(c.Context(), userID, permissionName)

// 2. Lấy organizations được share với user's organizations
sharedOrgIDs, err := services.GetSharedOrganizationIDs(c.Context(), allowedOrgIDs, permissionName)

// 3. Hợp nhất
allOrgIDsMap := make(map[primitive.ObjectID]bool)
for _, orgID := range allowedOrgIDs {
    allOrgIDsMap[orgID] = true
}
for _, orgID := range sharedOrgIDs {
    allOrgIDsMap[orgID] = true
}

// 4. Convert thành slice
finalOrgIDs = [team_a, sales_dept, ...]
```

### **Bước 4: Áp Dụng Filter**

Filter được áp dụng vào mọi query:

```go
// handler.base.go:applyOrganizationFilter()

orgFilter := bson.M{
    "ownerOrganizationId": bson.M{"$in": finalOrgIDs}
}

// Kết hợp với baseFilter từ user
finalFilter := bson.M{
    "$and": []bson.M{
        baseFilter,  // Filter từ user
        orgFilter,   // Filter theo organizations được phép
    },
}
```

**Kết quả:** User chỉ thấy documents có `ownerOrganizationId` trong `finalOrgIDs`.

---

## 📝 Ví Dụ Cụ Thể

### **Scenario: Share Department Data với Teams**

**Cấu trúc tổ chức:**
```
Sales Department (Level 2, ID: sales_dept)
├── Team A (Level 3, ID: team_a)
└── Team B (Level 3, ID: team_b)
```

**Yêu cầu:** Share tất cả data của Sales Department với Team A

**Bước 1: Tạo Share Record**

```json
POST /api/v1/organization-share
{
  "ownerOrganizationId": "sales_dept",
  "toOrgId": "team_a",
  "permissionNames": []  // Share tất cả permissions
}
```

**Bước 2: User Team A Query Data**

1. **Tính allowedOrgIDs:**
   - User có Role thuộc Team A với Scope 0
   - `allowedOrgIDs = [team_a]`

2. **Tính sharedOrgIDs:**
   - Query: `toOrgId IN [team_a]`
   - Kết quả: Share record với `OwnerOrganizationID = sales_dept`
   - `sharedOrgIDs = [sales_dept]`

3. **Hợp nhất:**
   - `finalOrgIDs = [team_a, sales_dept]`

4. **Filter:**
   ```json
   {
     "ownerOrganizationId": {"$in": ["team_a", "sales_dept"]}
   }
   ```

**Kết quả:**
- ✅ User Team A thấy documents có `ownerOrganizationId = team_a` (dữ liệu của chính Team A)
- ✅ User Team A thấy documents có `ownerOrganizationId = sales_dept` (dữ liệu được share)
- ❌ User Team A KHÔNG thấy documents có `ownerOrganizationId = team_b` (không được share)

---

## 🔐 Phân Quyền và Bảo Mật

### **1. Quyền Tạo Share**

Chỉ user có quyền truy cập `OwnerOrganizationID` mới được tạo share:

```go
// handler.organization.share.go:CreateShare()

// Validate: user có quyền share data của ownerOrg
allowedOrgIDs, err := services.GetUserAllowedOrganizationIDs(c.Context(), userID, "")
hasAccess := false
for _, orgID := range allowedOrgIDs {
    if orgID == ownerOrgID {
        hasAccess = true
        break
    }
}

if !hasAccess {
    return error("Bạn không có quyền share data của organization này")
}
```

### **2. Quyền Xóa Share**

Chỉ user tạo share hoặc user có quyền với `OwnerOrganizationID` mới được xóa:

```go
// handler.organization.share.go:DeleteShare()

// Kiểm tra user có phải người tạo không
if share.CreatedBy != userID {
    // Kiểm tra user có quyền với ownerOrg không
    allowedOrgIDs, err := services.GetUserAllowedOrganizationIDs(c.Context(), userID, "")
    // ... validate ...
}
```

### **3. Filter Theo Permission**

Nếu có `permissionName` cụ thể, chỉ share records có permission đó mới được áp dụng:

```go
// service.organization.share.go:GetSharedOrganizationIDs()

if permissionName != "" {
    // Chỉ lấy shares có:
    // 1. PermissionNames rỗng/nil (share tất cả)
    // 2. PermissionNames chứa permissionName
    filter["$or"] = []bson.M{
        {"permissionNames": bson.M{"$exists": false}},
        {"permissionNames": bson.M{"$size": 0}},
        {"permissionNames": bson.M{"$in": []string{permissionName}}},
    }
}
```

**Ví dụ:**
```
Share record 1: OwnerOrganizationID = sales_dept, ToOrgID = team_a, PermissionNames = []
Share record 2: OwnerOrganizationID = sales_dept, ToOrgID = team_a, PermissionNames = ["Order.Read"]

Query với permissionName = "Order.Read":
→ Cả 2 share records đều match (record 1 share tất cả, record 2 share Order.Read)

Query với permissionName = "Customer.Read":
→ Chỉ record 1 match (record 2 không share Customer.Read)
```

---

## 🔄 Tích Hợp Với Cơ Chế Phân Quyền Hiện Tại

### **1. OwnerOrganizationID**

Mỗi document có field `OwnerOrganizationID` để xác định dữ liệu thuộc về tổ chức nào:

```go
type Customer struct {
    ID                  primitive.ObjectID
    OwnerOrganizationID primitive.ObjectID  // Dữ liệu thuộc về tổ chức nào
    Name                string
    // ...
}
```

### **2. Scope của Permission**

- **Scope 0:** Chỉ organization của role
- **Scope 1:** Organization + children (tự động share với children)

### **3. Organization-Level Sharing**

- **Explicit sharing:** Tổ chức A config share với tổ chức B
- **Permission-based:** Có thể share với permissions cụ thể hoặc tất cả

**Kết hợp:**
```
User thấy dữ liệu của:
1. Organizations từ scope (allowedOrgIDs)
2. Organizations được share (sharedOrgIDs)
```

---

## 📊 Luồng Hoàn Chỉnh

### **Ví Dụ: User Query Customers**

```
1. User gửi request: GET /api/v1/customer/find

2. Authentication Middleware:
   - Verify token
   - Lấy user ID
   - Lấy active role

3. Handler.Find():
   a. Parse filter từ query
   
   b. Gọi applyOrganizationFilter():
      - Lấy user ID từ context
      - Gọi GetUserAllowedOrganizationIDs(userID, "customer.read")
        → allowedOrgIDs = [team_a]
      
      - Gọi GetSharedOrganizationIDs(allowedOrgIDs, "customer.read")
        → Query: toOrgId IN [team_a]
        → Kết quả: Share record với OwnerOrganizationID = sales_dept
        → sharedOrgIDs = [sales_dept]
      
      - Hợp nhất: finalOrgIDs = [team_a, sales_dept]
      
      - Thêm filter: {"ownerOrganizationId": {"$in": [team_a, sales_dept]}}
   
   c. Query với filter kết hợp
   
   d. Trả về kết quả

4. Kết quả:
   - Chỉ trả về customers có ownerOrganizationId trong [team_a, sales_dept]
   - User không thấy customers của organizations khác
```

---

## 🎯 Các Trường Hợp Sử Dụng

### **Case 1: Share Tất Cả Permissions**

```json
POST /api/v1/organization-share
{
  "ownerOrganizationId": "sales_dept",
  "toOrgId": "team_a",
  "permissionNames": []  // Share tất cả permissions
}
```

**Kết quả:** Team A thấy tất cả dữ liệu của Sales Department với mọi permission.

### **Case 2: Share Với Permissions Cụ Thể**

```json
POST /api/v1/organization-share
{
  "ownerOrganizationId": "sales_dept",
  "toOrgId": "team_a",
  "permissionNames": ["Order.Read", "Order.Create"]  // Chỉ share Order permissions
}
```

**Kết quả:** 
- Team A thấy Orders của Sales Department (có quyền Read và Create)
- Team A KHÔNG thấy Customers của Sales Department (không có permission)

### **Case 3: Share Nhiều Organizations**

```json
// Share Sales Department với Team A
POST /api/v1/organization-share
{
  "ownerOrganizationId": "sales_dept",
  "toOrgId": "team_a",
  "permissionNames": []
}

// Share Sales Department với Team B
POST /api/v1/organization-share
{
  "ownerOrganizationId": "sales_dept",
  "toOrgId": "team_b",
  "permissionNames": []
}
```

**Kết quả:** 
- Team A và Team B đều thấy dữ liệu của Sales Department
- Team A KHÔNG thấy dữ liệu của Team B (không được share)
- Team B KHÔNG thấy dữ liệu của Team A (không được share)

---

## ⚙️ Implementation Details

### **1. GetSharedOrganizationIDs()**

```go
// service.organization.share.go

func GetSharedOrganizationIDs(ctx context.Context, userOrgIDs []primitive.ObjectID, permissionName string) ([]primitive.ObjectID, error) {
    // 1. Query shares có toOrgId trong userOrgIDs
    filter := bson.M{
        "toOrgId": bson.M{"$in": userOrgIDs},
    }
    
    // 2. Filter theo permissionName nếu có
    if permissionName != "" {
        filter["$or"] = []bson.M{
            {"permissionNames": bson.M{"$exists": false}},
            {"permissionNames": bson.M{"$size": 0}},
            {"permissionNames": bson.M{"$in": []string{permissionName}}},
        }
    }
    
    // 3. Query shares
    shares, err := shareService.Find(ctx, filter, nil)
    
    // 4. Lấy OwnerOrganizationID từ shares
    sharedOrgIDsMap := make(map[primitive.ObjectID]bool)
    for _, share := range shares {
        // Validate permission nếu có
        if permissionName != "" && len(share.PermissionNames) > 0 {
            hasPermission := false
            for _, pn := range share.PermissionNames {
                if pn == permissionName {
                    hasPermission = true
                    break
                }
            }
            if !hasPermission {
                continue
            }
        }
        
        sharedOrgIDsMap[share.OwnerOrganizationID] = true
    }
    
    // 5. Convert thành slice
    result := make([]primitive.ObjectID, 0, len(sharedOrgIDsMap))
    for orgID := range sharedOrgIDsMap {
        result = append(result, orgID)
    }
    
    return result, nil
}
```

### **2. applyOrganizationFilter()**

```go
// handler.base.go

func (h *BaseHandler[T, CreateInput, UpdateInput]) applyOrganizationFilter(c fiber.Ctx, baseFilter bson.M) bson.M {
    // 1. Kiểm tra model có OwnerOrganizationID không
    if !h.hasOrganizationIDField() {
        return baseFilter
    }
    
    // 2. Lấy user ID và permission name
    userIDStr, ok := c.Locals("user_id").(string)
    if !ok {
        return baseFilter
    }
    userID, _ := primitive.ObjectIDFromHex(userIDStr)
    permissionName := h.getPermissionNameFromRoute(c)
    
    // 3. Lấy allowed organization IDs (từ scope)
    allowedOrgIDs, err := services.GetUserAllowedOrganizationIDs(c.Context(), userID, permissionName)
    if err != nil || len(allowedOrgIDs) == 0 {
        return baseFilter
    }
    
    // 4. Lấy organizations được share
    sharedOrgIDs, err := services.GetSharedOrganizationIDs(c.Context(), allowedOrgIDs, permissionName)
    if err == nil && len(sharedOrgIDs) > 0 {
        // 5. Hợp nhất allowedOrgIDs và sharedOrgIDs
        allOrgIDsMap := make(map[primitive.ObjectID]bool)
        for _, orgID := range allowedOrgIDs {
            allOrgIDsMap[orgID] = true
        }
        for _, orgID := range sharedOrgIDs {
            allOrgIDsMap[orgID] = true
        }
        
        // Convert back to slice
        allOrgIDs := make([]primitive.ObjectID, 0, len(allOrgIDsMap))
        for orgID := range allOrgIDsMap {
            allOrgIDs = append(allOrgIDs, orgID)
        }
        allowedOrgIDs = allOrgIDs
    }
    
    // 6. Thêm filter
    orgFilter := bson.M{"ownerOrganizationId": bson.M{"$in": allowedOrgIDs}}
    
    // 7. Kết hợp với baseFilter
    if len(baseFilter) == 0 {
        return orgFilter
    }
    
    return bson.M{
        "$and": []bson.M{
            baseFilter,
            orgFilter,
        },
    }
}
```

---

## 🔒 Bảo Mật

### **1. Validate Quyền Share**

- Chỉ user có quyền truy cập `OwnerOrganizationID` mới được tạo share
- Validate trước khi tạo share record

### **2. Filter Tự Động**

- Mọi query đều được filter theo `ownerOrganizationId`
- User không thể bypass filter bằng cách thêm filter thủ công

### **3. Permission-Based Sharing**

- Có thể share với permissions cụ thể
- Chỉ dữ liệu có permission tương ứng mới được share

---

## ⚠️ Lưu Ý Quan Trọng

### **1. Performance**

- Query `GetSharedOrganizationIDs()` có thể chậm nếu có nhiều shares
- Nên cache kết quả với TTL ngắn (1-5 phút)
- Index trên `toOrgId` và `ownerOrganizationId` để query nhanh

### **2. Circular Sharing**

- Hiện tại không có validation ngăn circular sharing (A share với B, B share với A)
- Có thể gây vấn đề performance nếu có nhiều circular shares

### **3. Cascade Sharing**

- Hiện tại không có cascade sharing (A share với B, B share với C → A không tự động share với C)
- Nếu cần, phải tạo share records riêng

---

## 📝 Tóm Tắt

### **Quy Tắc Vàng:**

1. ✅ **Mỗi document có OwnerOrganizationID** để xác định dữ liệu thuộc về tổ chức nào
2. ✅ **User thấy dữ liệu từ 2 nguồn:**
   - Organizations từ scope (allowedOrgIDs)
   - Organizations được share (sharedOrgIDs)
3. ✅ **Filter tự động áp dụng** cho mọi query
4. ✅ **Permission-based sharing** cho phép share với permissions cụ thể
5. ✅ **Validate quyền** trước khi tạo/xóa share

### **Cơ Chế:**

- **Scope 0/1:** Tự động tính toán allowed organizations từ roles và permissions
- **Organization-Level Sharing:** Explicit sharing thông qua `OrganizationShare` records
- **Filter tự động:** Áp dụng `ownerOrganizationId IN [allowedOrgIDs, sharedOrgIDs]`

---

**Tài liệu này mô tả logic share quyền dữ liệu trong hệ thống. Xem thêm [organization-data-authorization.md](./organization-data-authorization.md) để biết về cơ chế phân quyền dữ liệu cơ bản.**
