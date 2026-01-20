# Giải Pháp Sharing Dữ Liệu: Organization-Level Sharing

**Mục đích:** Đề xuất giải pháp sharing dữ liệu từ cấp cao xuống cấp thấp - đơn giản, nhanh, gọn, phù hợp với yêu cầu.

---

## 📋 Yêu Cầu

1. ✅ **Cấp trên tự động thấy cấp dưới** nếu có scope (đã có sẵn)
2. ❌ **Cấp dưới KHÔNG tự động thấy cấp trên** - cần cơ chế sharing

---

## 💡 Giải Pháp: Organization-Level Sharing

### **Nguyên Tắc:**

- Share ở **organization level**: Organization A config "share all data with Organization B"
- Tất cả documents của Organization A tự động visible cho Organization B
- Không cần thêm field vào từng document

### **Ưu Điểm:**

1. ✅ **Đơn giản:** Chỉ cần 1 collection `organization_shares`
2. ✅ **Nhanh:** Query đơn giản, performance tốt
3. ✅ **Gọn:** Không cần thêm field vào mọi document
4. ✅ **Dễ maintain:** Quản lý tập trung ở organization level
5. ✅ **Linh hoạt:** Có thể share với permissions cụ thể hoặc tất cả
6. ✅ **Audit trail:** Có thể track ai share, khi nào

---

## 🏗️ Kiến Trúc

### **1. Model OrganizationShare**

```go
// models/mongodb/model.organization.share.go

type OrganizationShare struct {
    ID              primitive.ObjectID   `json:"id,omitempty" bson:"_id,omitempty"`
    FromOrgID       primitive.ObjectID   `json:"fromOrgId" bson:"fromOrgId" index:"single:1"`      // Organization share data
    ToOrgID         primitive.ObjectID   `json:"toOrgId" bson:"toOrgId" index:"single:1"`          // Organization nhận data
    PermissionNames []string            `json:"permissionNames,omitempty" bson:"permissionNames,omitempty"` // [] hoặc nil = tất cả permissions
    CreatedAt       int64               `json:"createdAt" bson:"createdAt"`
    CreatedBy       primitive.ObjectID  `json:"createdBy" bson:"createdBy"`
}
```

**Quy tắc:**
- `PermissionNames = []` hoặc `nil` → Share với **tất cả permissions**
- `PermissionNames = ["Order.Read", "Order.Create"]` → Chỉ share với permissions cụ thể
- Index trên `fromOrgID` và `toOrgID` để query nhanh

### **2. Service OrganizationShare**

```go
// services/service.organization.share.go

type OrganizationShareService struct {
    *BaseServiceMongoImpl[models.OrganizationShare]
}

func NewOrganizationShareService() (*OrganizationShareService, error) {
    // ... init service ...
}

// GetSharedOrganizationIDs lấy organizations được share với user's organizations
func GetSharedOrganizationIDs(ctx context.Context, userOrgIDs []primitive.ObjectID, permissionName string) ([]primitive.ObjectID, error) {
    shareService, err := NewOrganizationShareService()
    if err != nil {
        return nil, err
    }
    
    // Query: toOrgID trong userOrgIDs
    filter := bson.M{
        "toOrgID": bson.M{"$in": userOrgIDs},
    }
    
    // Nếu có permissionName, filter thêm
    if permissionName != "" {
        // Share nếu:
        // 1. PermissionNames rỗng/nil (share tất cả permissions)
        // 2. PermissionNames chứa permissionName cụ thể
        filter["$or"] = []bson.M{
            {"permissionNames": bson.M{"$exists": false}}, // Không có field
            {"permissionNames": bson.M{"$size": 0}},       // Array rỗng
            {"permissionNames": permissionName},           // Chứa permissionName
        }
    }
    
    shares, err := shareService.Find(ctx, filter, nil)
    if err != nil {
        return nil, err
    }
    
    // Lấy fromOrgIDs (organizations share data với user)
    sharedOrgIDsMap := make(map[primitive.ObjectID]bool)
    for _, share := range shares {
        // Nếu có permissionName, kiểm tra kỹ hơn
        if permissionName != "" {
            // Nếu PermissionNames không rỗng và không chứa permissionName → skip
            if len(share.PermissionNames) > 0 {
                hasPermission := false
                for _, pn := range share.PermissionNames {
                    if pn == permissionName {
                        hasPermission = true
                        break
                    }
                }
                if !hasPermission {
                    continue // Skip share này
                }
            }
        }
        
        sharedOrgIDsMap[share.FromOrgID] = true
    }
    
    // Convert to slice
    result := make([]primitive.ObjectID, 0, len(sharedOrgIDsMap))
    for orgID := range sharedOrgIDsMap {
        result = append(result, orgID)
    }
    
    return result, nil
}
```

### **3. Cập Nhật Filter**

```go
// handler.base.go:applyOrganizationFilter()

func (h *BaseHandler[T, CreateInput, UpdateInput]) applyOrganizationFilter(c fiber.Ctx, baseFilter bson.M) bson.M {
    // ... kiểm tra model có OrganizationID ...
    
    // Lấy allowed organization IDs (chỉ từ scope, KHÔNG có parents)
    allowedOrgIDs, err := services.GetUserAllowedOrganizationIDs(c.Context(), userID, permissionName)
    if err != nil || len(allowedOrgIDs) == 0 {
        return baseFilter
    }
    
    // Lấy organizations được share với user's organizations
    sharedOrgIDs, err := services.GetSharedOrganizationIDs(c.Context(), allowedOrgIDs, permissionName)
    if err == nil && len(sharedOrgIDs) > 0 {
        // Hợp nhất allowedOrgIDs và sharedOrgIDs
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
    
    // Filter
    orgFilter := bson.M{"organizationId": bson.M{"$in": allowedOrgIDs}}
    
    // Kết hợp với baseFilter
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

### **4. Loại Bỏ Logic Tự Động Thêm Parents**

```go
// service.organization.helper.go

func GetUserAllowedOrganizationIDs(ctx context.Context, userID primitive.ObjectID, permissionName string) ([]primitive.ObjectID, error) {
    // ... logic hiện tại (bước 1-5) để tính allowedOrgIDs từ scope ...
    
    // ❌ XÓA bước 7: Tự động thêm parent organizations
    
    // 6. Convert map thành slice (KHÔNG thêm parents)
    result := make([]primitive.ObjectID, 0, len(allowedOrgIDsMap))
    for orgID := range allowedOrgIDsMap {
        result = append(result, orgID)
    }
    
    return result, nil
}
```

---

## 🔧 Implementation Plan

### **Bước 1: Loại Bỏ Logic Tự Động Thêm Parents**

Xóa phần tự động thêm parent organizations trong `GetUserAllowedOrganizationIDs()`.

### **Bước 2: Tạo Model OrganizationShare**

Tạo model `OrganizationShare` với các fields: `FromOrgID`, `ToOrgID`, `PermissionNames`, `CreatedAt`, `CreatedBy`.

### **Bước 3: Tạo Service OrganizationShare**

Tạo service với method `GetSharedOrganizationIDs()` để query organizations được share.

### **Bước 4: Cập Nhật Filter**

Cập nhật `applyOrganizationFilter()` để include shared organizations vào filter.

### **Bước 5: Tạo API Quản Lý Sharing**

```go
// handler.organization.share.go

// POST /api/v1/organization-shares
// Body: { 
//   "fromOrgId": "org1", 
//   "toOrgId": "org2", 
//   "permissionNames": ["Order.Read", "Order.Create"] // Optional: [] hoặc null = tất cả permissions
// }

// DELETE /api/v1/organization-shares/:id

// GET /api/v1/organization-shares?fromOrgId=xxx
```

---

## 📝 Ví Dụ Sử Dụng

### **Scenario: Share Department Data với Teams**

```
Cấu trúc:
Sales Department (Level 2, ID: sales_dept)
├── Team A (Level 3, ID: team_a)
└── Team B (Level 3, ID: team_b)

Yêu cầu: Share tất cả data của Sales Department với Team A và Team B

Solution:
1. Admin tạo 2 sharing records:
   - fromOrgId: sales_dept, toOrgId: team_a, permissionNames: [] (tất cả permissions)
   - fromOrgId: sales_dept, toOrgId: team_b, permissionNames: [] (tất cả permissions)
   
   Hoặc nếu chỉ share với permissions cụ thể:
   - fromOrgId: sales_dept, toOrgId: team_a, permissionNames: ["Order.Read", "Order.Create"]

2. Khi user Team A query:
   - allowedOrgIDs từ scope: [team_a]
   - sharedOrgIDs: [sales_dept] (từ organization_shares)
   - finalOrgIDs: [team_a, sales_dept]
   
3. Kết quả:
   ✅ Thấy documents có organizationId = team_a
   ✅ Thấy documents có organizationId = sales_dept (được share)
   ❌ KHÔNG thấy documents có organizationId = team_b (không được share)
```

---

## ⚠️ Lưu Ý

1. **Validate sharing:**
   - Chỉ cho phép share với organizations trong cùng cây (optional)
   - Validate user có quyền share data của fromOrg

2. **Performance:**
   - Cache `GetSharedOrganizationIDs()` với TTL ngắn
   - Index trên `toOrgID` và `fromOrgID`

3. **Security:**
   - Validate user có quyền share trước khi tạo share record
   - Không cho phép share với organizations ngoài cây (optional)

---

## 📝 Tóm Tắt

**Giải pháp:** Organization-Level Sharing

**Implementation:**
1. Loại bỏ logic tự động thêm parents
2. Tạo collection `organization_shares`
3. Cập nhật filter để include shared organizations
4. Tạo API quản lý sharing

**Kết quả:**
- ✅ Cấp trên tự động thấy cấp dưới (scope)
- ✅ Cấp dưới thấy cấp trên khi được share (explicit)
- ✅ Đơn giản, nhanh, gọn
- ✅ Phù hợp với yêu cầu

---

**Tài liệu này đề xuất giải pháp cụ thể, nhanh, gọn, đơn giản, phù hợp với yêu cầu bài toán.**
