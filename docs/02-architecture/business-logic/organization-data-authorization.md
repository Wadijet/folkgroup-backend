# Quản Lý và Phân Quyền Dữ Liệu Theo Tổ Chức Dạng Cây

**Mục đích:** Mô tả chi tiết cách hệ thống quản lý và phân quyền từng dòng dữ liệu thuộc tổ chức nào trong cấu trúc cây.

---

## 📋 Tổng Quan

Hệ thống sử dụng **cấu trúc tổ chức dạng cây (tree structure)** để quản lý dữ liệu và phân quyền. Mỗi dòng dữ liệu thuộc về một tổ chức cụ thể, và quyền truy cập được tính toán dựa trên:
- **Vị trí của user trong cây tổ chức**
- **Scope của permission** (0 hoặc 1)
- **Quan hệ cha-con trong cây tổ chức**

---

## 🏗️ Cấu Trúc Tổ Chức Dạng Cây

### 1. **Mô Hình Dữ Liệu**

Mỗi tổ chức được lưu với các trường:

```go
type Organization struct {
    ID       primitive.ObjectID  // ID duy nhất
    Name     string              // Tên tổ chức
    Code     string              // Mã tổ chức (unique)
    Type     string              // Loại: system, group, company, department, division, team
    ParentID *primitive.ObjectID // ID tổ chức cha (null nếu là root)
    Path     string              // Đường dẫn cây: "/system/group1/company1/dept1"
    Level    int                 // Cấp độ: -1 (system), 0 (group), 1 (company), ...
    IsActive bool                // Trạng thái hoạt động
}
```

### 2. **Ví Dụ Cấu Trúc Cây**

```
System (Level -1, Path: "/system")
└── Tập Đoàn ABC (Level 0, Path: "/system/abc_group")
    ├── Công Ty Miền Bắc (Level 1, Path: "/system/abc_group/north_company")
    │   ├── Phòng Kinh Doanh (Level 2, Path: "/system/abc_group/north_company/sales_dept")
    │   │   ├── Team Bán Hàng A (Level 3, Path: "/system/abc_group/north_company/sales_dept/team_a")
    │   │   └── Team Bán Hàng B (Level 3, Path: "/system/abc_group/north_company/sales_dept/team_b")
    │   └── Phòng Marketing (Level 2, Path: "/system/abc_group/north_company/marketing_dept")
    └── Công Ty Miền Nam (Level 1, Path: "/system/abc_group/south_company")
        └── Phòng Kỹ Thuật (Level 2, Path: "/system/abc_group/south_company/tech_dept")
```

### 3. **Quan Hệ Cha-Con**

- **ParentID**: Trỏ trực tiếp đến tổ chức cha
- **Path**: Đường dẫn đầy đủ từ root đến tổ chức hiện tại
  - Dùng để query nhanh tất cả children: `path LIKE "/system/abc_group/north_company/%"`
  - Đảm bảo tính nhất quán và hiệu năng

---

## 📦 Quản Lý Dữ Liệu Thuộc Tổ Chức

### 1. **Gán Dữ Liệu Vào Tổ Chức**

Mỗi document trong database có field `organizationId`:

```go
type Order struct {
    ID             primitive.ObjectID
    OrganizationID primitive.ObjectID  // ✅ Dòng dữ liệu thuộc tổ chức nào
    CustomerName   string
    TotalAmount   float64
    // ... các trường khác
}
```

**Quy tắc:**
- ✅ Mỗi dòng dữ liệu **PHẢI** có `organizationId`
- ✅ `organizationId` được tự động gán khi tạo mới (từ active organization context)
- ✅ Không cho phép update `organizationId` trực tiếp (bảo mật)

### 2. **Tự Động Gán OrganizationId**

Khi tạo mới document:

```go
// handler.base.crud.go:InsertOne()
activeOrgID := h.getActiveOrganizationID(c)  // Lấy từ context/header
if activeOrgID != nil {
    h.setOrganizationID(input, *activeOrgID)  // Tự động gán
}
```

**Nguồn `activeOrgID`:**
- Từ header `X-Active-Organization-ID` (nếu có)
- Hoặc từ context được set bởi middleware

---

## 🔐 Phân Quyền Truy Cập Dữ Liệu

### 1. **Nguyên Tắc Cơ Bản**

User chỉ có thể truy cập dữ liệu của:
- ✅ Tổ chức mà role của user thuộc về
- ✅ Tổ chức con (children) nếu có Scope = 1
- ✅ Tổ chức cha (parents) - tự động thêm

### 2. **Scope của Permission**

Mỗi permission trong role có **scope**:

#### **Scope 0: Chỉ Tổ Chức Của Role**
```
User có Role A thuộc "Team Bán Hàng A"
→ Chỉ thấy dữ liệu có organizationId = "Team Bán Hàng A"
```

#### **Scope 1: Tổ Chức + Children**
```
User có Role A thuộc "Phòng Kinh Doanh" với Scope 1
→ Thấy dữ liệu của:
  - Phòng Kinh Doanh
  - Team Bán Hàng A (con)
  - Team Bán Hàng B (con)
```

### 3. **Tự Động Thêm Parent Organizations**

**Logic đặc biệt:** User tự động thấy dữ liệu của **TẤT CẢ** parent organizations.

```go
// service.organization.helper.go:GetUserAllowedOrganizationIDs()

// 1. Tính toán allowed orgs từ scope
allowedOrgIDs = [team_a]  // Scope 0

// 2. Tự động thêm parents
parentIDs = GetParentIDs(team_a)
// → ["sales_dept", "north_company", "abc_group", "system"]

// 3. Kết quả cuối cùng
finalOrgIDs = [team_a, sales_dept, north_company, abc_group, system]
```

**Ví dụ:**
```
User ở "Team Bán Hàng A" (Level 3)
→ Tự động thấy dữ liệu của:
  ✅ Team Bán Hàng A (chính nó)
  ✅ Phòng Kinh Doanh (parent)
  ✅ Công Ty Miền Bắc (parent)
  ✅ Tập Đoàn ABC (parent)
  ✅ System (root)
```

**Lý do:** Dữ liệu ở cấp cao (Department/Company) thường là dữ liệu chung, tất cả teams con cần thấy.

---

## 🔍 Cơ Chế Filter Tự Động

### 1. **Tự Động Thêm Filter OrganizationId**

Mọi query đều được tự động thêm filter:

```go
// handler.base.go:applyOrganizationFilter()

// 1. Lấy allowed organization IDs
allowedOrgIDs := GetUserAllowedOrganizationIDs(userID, permissionName)
// → [team_a, sales_dept, north_company, abc_group, system]

// 2. Thêm filter vào query
filter := bson.M{
    "$and": []bson.M{
        baseFilter,  // Filter từ user
        {
            "organizationId": bson.M{
                "$in": allowedOrgIDs  // ✅ Chỉ lấy dữ liệu của các orgs được phép
            }
        }
    }
}
```

### 2. **Áp Dụng Cho Tất Cả Operations**

Filter được áp dụng tự động cho:
- ✅ `Find()` - Tìm nhiều documents
- ✅ `FindOne()` - Tìm một document
- ✅ `FindWithPagination()` - Tìm với phân trang
- ✅ `UpdateOne()` - Cập nhật một document
- ✅ `UpdateMany()` - Cập nhật nhiều documents
- ✅ `DeleteMany()` - Xóa nhiều documents
- ✅ `Upsert()` - Insert hoặc update

**Lưu ý:** Một số operations **THIẾU** filter (xem báo cáo đánh giá).

### 3. **Validate Access Trước Khi Thao Tác**

Với operations theo ID, validate trước:

```go
// handler.base.crud.go:FindOneById()

// 1. Validate user có quyền truy cập document này không
if err := h.validateOrganizationAccess(c, id); err != nil {
    return err  // 403 Forbidden
}

// 2. Mới query document
doc := h.BaseService.FindOneById(id)
```

**Logic validate:**
```go
// handler.base.go:validateOrganizationAccess()

// 1. Lấy document
doc := FindOneById(id)
docOrgID := doc.OrganizationID

// 2. Lấy allowed org IDs của user
allowedOrgIDs := GetUserAllowedOrganizationIDs(userID, permissionName)

// 3. Kiểm tra document có thuộc allowed orgs không
for _, allowedOrgID := range allowedOrgIDs {
    if allowedOrgID == docOrgID {
        return nil  // ✅ Có quyền
    }
}

return error  // ❌ Không có quyền
```

---

## 📊 Luồng Xử Lý Hoàn Chỉnh

### **Ví Dụ: User Query Orders**

```
1. User gửi request: GET /api/v1/orders?filter={...}

2. Authentication Middleware:
   - Verify token
   - Lấy user ID
   - Kiểm tra X-Active-Role-ID header
   - Lấy permissions từ active role

3. Handler.Find():
   - Parse filter từ query: {"status": "pending"}
   - Gọi applyOrganizationFilter():
     a. Lấy user ID từ context
     b. Gọi GetUserAllowedOrganizationIDs(userID, "order.read")
       - Lấy tất cả roles của user
       - Với mỗi role có permission "order.read":
         * Scope 0: [role.organizationId]
         * Scope 1: [role.organizationId, ...children]
       - Tự động thêm parents
       - Kết quả: [team_a, sales_dept, north_company, ...]
     c. Thêm filter: {"organizationId": {"$in": allowedOrgIDs}}
   - Query: {"$and": [{"status": "pending"}, {"organizationId": {"$in": [...]}}]}
   - Trả về kết quả

4. Kết quả:
   - Chỉ trả về orders có organizationId trong danh sách allowed
   - User không thấy orders của organizations khác
```

---

## 🎯 Các Trường Hợp Sử Dụng

### **Case 1: User Ở Cấp Team (Scope 0)**

```
User: Nhân viên Team Bán Hàng A
Role: Sales Staff (Scope 0, Permission: "order.read")
Organization: Team Bán Hàng A

Allowed Organizations:
- Team Bán Hàng A (chính nó)
- Phòng Kinh Doanh (parent)
- Công Ty Miền Bắc (parent)
- Tập Đoàn ABC (parent)
- System (root)

Kết quả:
✅ Thấy orders của Team Bán Hàng A
✅ Thấy orders của Phòng Kinh Doanh (dữ liệu chung)
✅ Thấy orders của Công Ty Miền Bắc (dữ liệu chung)
❌ KHÔNG thấy orders của Team Bán Hàng B (sibling)
❌ KHÔNG thấy orders của Phòng Marketing (sibling)
```

### **Case 2: User Ở Cấp Department (Scope 1)**

```
User: Trưởng Phòng Kinh Doanh
Role: Department Manager (Scope 1, Permission: "order.read")
Organization: Phòng Kinh Doanh

Allowed Organizations:
- Phòng Kinh Doanh (chính nó)
- Team Bán Hàng A (child - Scope 1)
- Team Bán Hàng B (child - Scope 1)
- Công Ty Miền Bắc (parent)
- Tập Đoàn ABC (parent)
- System (root)

Kết quả:
✅ Thấy orders của Phòng Kinh Doanh
✅ Thấy orders của Team Bán Hàng A (child)
✅ Thấy orders của Team Bán Hàng B (child)
✅ Thấy orders của Công Ty Miền Bắc (parent)
❌ KHÔNG thấy orders của Phòng Marketing (sibling)
```

### **Case 3: User Có Nhiều Roles**

```
User: Có 2 roles
- Role A: Team Bán Hàng A (Scope 0, Permission: "order.read")
- Role B: Phòng Marketing (Scope 1, Permission: "order.read")

Allowed Organizations (hợp nhất):
- Team Bán Hàng A (từ Role A)
- Phòng Marketing (từ Role B)
- Team Marketing A (child của Role B - Scope 1)
- Team Marketing B (child của Role B - Scope 1)
- Tất cả parents của cả 2 orgs

Kết quả:
✅ Thấy orders của Team Bán Hàng A
✅ Thấy orders của Phòng Marketing
✅ Thấy orders của các teams con của Phòng Marketing
✅ Thấy orders của các parent organizations
```

---

## ⚙️ Implementation Details

### 1. **GetUserAllowedOrganizationIDs()**

```go
func GetUserAllowedOrganizationIDs(ctx context.Context, userID primitive.ObjectID, permissionName string) ([]primitive.ObjectID, error) {
    // 1. Lấy tất cả roles của user
    userRoles := GetUserRoles(userID)
    
    allowedOrgIDsMap := make(map[primitive.ObjectID]bool)
    
    // 2. Duyệt qua từng role
    for _, userRole := range userRoles {
        role := GetRole(userRole.RoleID)
        orgID := role.OrganizationID
        
        // 3. Lấy permissions của role
        rolePermissions := GetRolePermissions(role.ID)
        
        // 4. Kiểm tra permission cụ thể
        for _, rp := range rolePermissions {
            permission := GetPermission(rp.PermissionID)
            
            // Chỉ xử lý nếu permission name khớp
            if permissionName != "" && permission.Name != permissionName {
                continue
            }
            
            // 5. Tính toán allowed org IDs dựa trên scope
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
    
    // 6. Tự động thêm parent organizations
    allAllowedOrgIDsMap := make(map[primitive.ObjectID]bool)
    for orgID := range allowedOrgIDsMap {
        allAllowedOrgIDsMap[orgID] = true
        parentIDs := GetParentIDs(orgID)
        for _, parentID := range parentIDs {
            allAllowedOrgIDsMap[parentID] = true
        }
    }
    
    // 7. Convert thành slice
    result := make([]primitive.ObjectID, 0, len(allAllowedOrgIDsMap))
    for orgID := range allAllowedOrgIDsMap {
        result = append(result, orgID)
    }
    
    return result, nil
}
```

### 2. **GetChildrenIDs() - Lấy Tất Cả Children**

```go
func (s *OrganizationService) GetChildrenIDs(ctx context.Context, parentID primitive.ObjectID) ([]primitive.ObjectID, error) {
    // Lấy parent organization
    parent := FindOneById(parentID)
    
    // Query tất cả organizations có Path bắt đầu với parent.Path
    filter := bson.M{
        "path": bson.M{"$regex": "^" + parent.Path},
        "isActive": true,
    }
    
    orgs := Find(filter)
    
    // Trả về danh sách IDs
    ids := make([]primitive.ObjectID, 0, len(orgs))
    for _, org := range orgs {
        ids = append(ids, org.ID)
    }
    
    return ids, nil
}
```

**Ví dụ:**
```
Parent: Phòng Kinh Doanh (Path: "/system/abc_group/north_company/sales_dept")
Query: path LIKE "/system/abc_group/north_company/sales_dept%"
Kết quả:
- Team Bán Hàng A (Path: "/system/abc_group/north_company/sales_dept/team_a")
- Team Bán Hàng B (Path: "/system/abc_group/north_company/sales_dept/team_b")
```

### 3. **GetParentIDs() - Lấy Tất Cả Parents**

```go
func (s *OrganizationService) GetParentIDs(ctx context.Context, childID primitive.ObjectID) ([]primitive.ObjectID, error) {
    // Lấy child organization
    child := FindOneById(childID)
    
    if child.ParentID == nil {
        return []primitive.ObjectID{}, nil  // Root, không có parent
    }
    
    parentIDs := make([]primitive.ObjectID, 0)
    currentID := *child.ParentID
    
    // Đi ngược lên cây để lấy tất cả parents
    for {
        parent := FindOneById(currentID)
        if err != nil {
            break
        }
        
        parentIDs = append(parentIDs, parent.ID)
        
        if parent.ParentID == nil {
            break  // Đã đến root
        }
        
        currentID = *parent.ParentID
    }
    
    return parentIDs, nil
}
```

**Ví dụ:**
```
Child: Team Bán Hàng A
Parents:
1. Phòng Kinh Doanh (parent trực tiếp)
2. Công Ty Miền Bắc (parent của Phòng Kinh Doanh)
3. Tập Đoàn ABC (parent của Công Ty Miền Bắc)
4. System (root)
```

---

## 🔒 Bảo Mật

### 1. **Không Cho Phép Update OrganizationId**

```go
// handler.base.crud.go:UpdateOne()
delete(updateData, "organizationId")  // ✅ Xóa field này khỏi update data
```

**Lý do:** Ngăn user chuyển dữ liệu sang organization khác.

### 2. **Validate Trước Khi Thao Tác**

- ✅ `FindOneById()` - Validate trước khi query
- ✅ `UpdateById()` - Validate trước khi update
- ❌ `DeleteById()` - **THIẾU** validate (cần sửa)

### 3. **Filter Tự Động**

- ✅ Tất cả queries đều được filter theo allowed organizations
- ✅ User không thể bypass filter bằng cách thêm filter thủ công

---

## ⚠️ Lưu Ý Quan Trọng

### 1. **Logic Tự Động Thêm Parents**

**Hiện tại:** User tự động thấy dữ liệu của tất cả parent organizations.

**Có thể gây vấn đề:**
- User ở cấp thấp có thể thấy quá nhiều dữ liệu
- Không có cách để disable tính năng này

**Cân nhắc:**
- Có thể cần thêm flag `includeParents` trong permission config
- Hoặc chỉ thêm parents nếu có permission đặc biệt

### 2. **Active Role Context**

**Hiện tại:** `GetUserAllowedOrganizationIDs()` lấy permissions từ **TẤT CẢ** roles của user.

**Có thể gây vấn đề:**
- User có nhiều roles → thấy dữ liệu của nhiều organizations
- Không tôn trọng active role context

**Cần sửa:**
- Chỉ lấy permissions từ active role khi tính toán allowed org IDs

### 3. **Performance**

**Vấn đề:**
- Nhiều database queries khi tính toán allowed org IDs
- Không có cache

**Giải pháp:**
- Thêm cache với TTL ngắn (1-5 phút)
- Invalidate cache khi user roles/permissions thay đổi

---

## 📝 Tóm Tắt

### **Quy Tắc Vàng:**

1. ✅ **Mỗi dòng dữ liệu thuộc về một tổ chức** (`organizationId`)
2. ✅ **User chỉ thấy dữ liệu của organizations được phép** (tính từ scope + parents)
3. ✅ **Filter tự động áp dụng cho mọi query** (trừ một số operations còn thiếu)
4. ✅ **Validate access trước khi thao tác** (với operations theo ID)
5. ✅ **Không cho phép update organizationId** (bảo mật)

### **Cấu Trúc Cây:**

```
System (root)
└── Group
    └── Company
        └── Department
            └── Division
                └── Team
```

### **Scope:**

- **Scope 0:** Chỉ tổ chức của role
- **Scope 1:** Tổ chức + tất cả children
- **Tự động:** Thêm tất cả parent organizations

---

**Tài liệu này mô tả logic hiện tại. Xem thêm [data-authorization-review.md](./data-authorization-review.md) để biết các vấn đề cần khắc phục.**
