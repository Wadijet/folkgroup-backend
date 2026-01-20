# Đề Xuất Thiết Kế: Share Cho Nhiều Tổ Chức và Share Cho Tất Cả

**Mục đích:** Đề xuất các phương án thiết kế để hỗ trợ:
1. Share cho nhiều tổ chức cùng lúc (bulk share)
2. Share cho tất cả tổ chức (public share)

---

## 📋 Yêu Cầu

1. ✅ **Share cho nhiều tổ chức:** Organization A share data với [Team B, Team C, Team D] cùng lúc
2. ✅ **Share cho tất cả:** Organization A share data với tất cả organizations trong hệ thống
3. ✅ **Backward compatible:** Vẫn hỗ trợ share cho 1 tổ chức như hiện tại
4. ✅ **Performance:** Query nhanh, không ảnh hưởng performance

---

## 💡 Các Phương Án

### **Phương Án 1: ToOrgIDs là Mảng (Array) - Không Khuyến Nghị**

#### **Thiết Kế:**
```go
type OrganizationShare struct {
    ID                  primitive.ObjectID
    OwnerOrganizationID primitive.ObjectID
    ToOrgIDs            []primitive.ObjectID  // Mảng organizations nhận data
    IsPublicShare       bool                  // true = share với tất cả
    PermissionNames     []string
    CreatedAt           int64
    CreatedBy           primitive.ObjectID
}
```

#### **Ưu Điểm:**
- ✅ Một record share nhiều orgs → ít records hơn
- ✅ Dễ quản lý: một share record cho nhiều orgs

#### **Nhược Điểm:**
- ❌ **Khó query:** Phải dùng `$elemMatch` hoặc `$in` phức tạp
- ❌ **Khó index:** Không thể index array hiệu quả
- ❌ **Khó quản lý:** Không thể xóa share cho 1 org cụ thể (phải update cả mảng)
- ❌ **Khó audit:** Không biết khi nào share cho org nào
- ❌ **Performance:** Query chậm hơn với array

#### **Ví Dụ Query:**
```go
// Query phức tạp để tìm shares
filter := bson.M{
    "$or": []bson.M{
        {"toOrgIDs": bson.M{"$in": userOrgIDs}},
        {"isPublicShare": true},
    },
}
```

**Kết luận:** ❌ Không khuyến nghị vì phức tạp và performance kém.

---

### **Phương Án 2: Giữ Nguyên Model, Bulk Create - Khuyến Nghị**

#### **Thiết Kế:**
```go
type OrganizationShare struct {
    ID                  primitive.ObjectID
    OwnerOrganizationID primitive.ObjectID
    ToOrgID             *primitive.ObjectID  // null nếu IsPublicShare = true
    IsPublicShare       bool                  // true = share với tất cả
    PermissionNames     []string
    CreatedAt           int64
    CreatedBy           primitive.ObjectID
}
```

#### **Cách Hoạt Động:**
- **Share cho 1 org:** Tạo 1 record với `ToOrgID = orgID`, `IsPublicShare = false`
- **Share cho nhiều orgs:** Tạo nhiều records (mỗi record cho 1 org)
- **Share cho tất cả:** Tạo 1 record với `ToOrgID = null`, `IsPublicShare = true`

#### **Ưu Điểm:**
- ✅ **Đơn giản:** Model đơn giản, dễ hiểu
- ✅ **Query nhanh:** Index trên `ToOrgID` và `IsPublicShare` hiệu quả
- ✅ **Dễ quản lý:** Có thể xóa share cho 1 org cụ thể
- ✅ **Dễ audit:** Mỗi share record rõ ràng
- ✅ **Performance tốt:** Query đơn giản với index

#### **Nhược Điểm:**
- ⚠️ Nhiều records hơn khi share cho nhiều orgs (nhưng không phải vấn đề lớn)

#### **Ví Dụ Query:**
```go
// Query đơn giản và nhanh
filter := bson.M{
    "$or": []bson.M{
        {"toOrgId": bson.M{"$in": userOrgIDs}},
        {"isPublicShare": true},
    },
}
```

#### **API Design:**
```json
// Share cho 1 org
POST /api/v1/organization-share
{
  "ownerOrganizationId": "sales_dept",
  "toOrgId": "team_a",
  "permissionNames": []
}

// Share cho nhiều orgs (bulk)
POST /api/v1/organization-share/bulk
{
  "ownerOrganizationId": "sales_dept",
  "toOrgIds": ["team_a", "team_b", "team_c"],
  "permissionNames": []
}

// Share cho tất cả
POST /api/v1/organization-share
{
  "ownerOrganizationId": "sales_dept",
  "shareToAll": true,
  "permissionNames": []
}
```

**Kết luận:** ✅ **Khuyến nghị** - Đơn giản, hiệu quả, dễ maintain.

---

### **Phương Án 3: Hai Model Riêng Biệt**

#### **Thiết Kế:**
```go
// Share cho org cụ thể
type OrganizationShare struct {
    ID                  primitive.ObjectID
    OwnerOrganizationID primitive.ObjectID
    ToOrgID             primitive.ObjectID
    PermissionNames     []string
    CreatedAt           int64
    CreatedBy           primitive.ObjectID
}

// Share cho tất cả
type OrganizationPublicShare struct {
    ID                  primitive.ObjectID
    OwnerOrganizationID primitive.ObjectID
    PermissionNames     []string
    CreatedAt           int64
    CreatedBy           primitive.ObjectID
}
```

#### **Ưu Điểm:**
- ✅ Rõ ràng, tách biệt logic
- ✅ Query đơn giản cho từng loại

#### **Nhược Điểm:**
- ❌ **Code duplicate:** Logic tương tự ở 2 nơi
- ❌ **Phức tạp:** Phải query 2 collections
- ❌ **Khó maintain:** Phải update 2 nơi khi có thay đổi

**Kết luận:** ❌ Không khuyến nghị vì duplicate code.

---

## 🎯 Phương Án Được Chọn: Phương Án 2

### **Lý Do:**
1. ✅ **Đơn giản:** Model đơn giản, dễ hiểu
2. ✅ **Performance:** Query nhanh với index
3. ✅ **Linh hoạt:** Hỗ trợ cả 3 trường hợp (1 org, nhiều orgs, tất cả)
4. ✅ **Dễ maintain:** Code tập trung, không duplicate
5. ✅ **Dễ audit:** Mỗi share record rõ ràng

---

## 🏗️ Implementation Details

### **1. Model Design**

```go
type OrganizationShare struct {
    ID                  primitive.ObjectID  `json:"id,omitempty" bson:"_id,omitempty"`
    OwnerOrganizationID primitive.ObjectID  `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
    ToOrgID             *primitive.ObjectID `json:"toOrgId,omitempty" bson:"toOrgId,omitempty" index:"single:1"` // null nếu IsPublicShare = true
    IsPublicShare       bool                `json:"isPublicShare" bson:"isPublicShare" index:"single:1"`          // true = share với tất cả
    PermissionNames     []string            `json:"permissionNames,omitempty" bson:"permissionNames,omitempty"`
    CreatedAt           int64               `json:"createdAt" bson:"createdAt"`
    CreatedBy           primitive.ObjectID  `json:"createdBy" bson:"createdBy"`
}
```

**Indexes:**
- `ownerOrganizationId` (single)
- `toOrgId` (single, sparse - để hỗ trợ null)
- `isPublicShare` (single)
- Compound index: `(ownerOrganizationId, toOrgId, isPublicShare)`

### **2. DTO Design**

```go
type OrganizationShareCreateInput struct {
    OwnerOrganizationID string   `json:"ownerOrganizationId" validate:"required"`
    
    // Option 1: Share cho 1 org
    ToOrgID            string   `json:"toOrgId,omitempty" validate:"required_without=ToOrgIDs,required_without=ShareToAll"`
    
    // Option 2: Share cho nhiều orgs
    ToOrgIDs           []string `json:"toOrgIds,omitempty" validate:"required_without=ToOrgID,required_without=ShareToAll"`
    
    // Option 3: Share cho tất cả
    ShareToAll         bool     `json:"shareToAll,omitempty" validate:"required_without=ToOrgID,required_without=ToOrgIDs"`
    
    PermissionNames    []string `json:"permissionNames,omitempty"`
}
```

**Validation Rules:**
- Phải có 1 trong 3: `ToOrgID`, `ToOrgIDs`, hoặc `ShareToAll = true`
- Không được có nhiều hơn 1 option cùng lúc

### **3. Handler Design**

```go
// CreateShare - Share cho 1 org hoặc tất cả
func (h *OrganizationShareHandler) CreateShare(c fiber.Ctx) error {
    // Parse input
    // Validate: có ToOrgID hoặc ShareToAll
    // Tạo 1 record
}

// CreateBulkShare - Share cho nhiều orgs
func (h *OrganizationShareHandler) CreateBulkShare(c fiber.Ctx) error {
    // Parse input với ToOrgIDs
    // Validate: có ToOrgIDs
    // Tạo nhiều records (mỗi record cho 1 org)
    // Trả về danh sách IDs đã tạo
}
```

### **4. Service Design**

```go
// GetSharedOrganizationIDs - Cập nhật để xử lý IsPublicShare
func GetSharedOrganizationIDs(ctx context.Context, userOrgIDs []primitive.ObjectID, permissionName string) ([]primitive.ObjectID, error) {
    // Query 1: Shares có ToOrgID trong userOrgIDs
    filter1 := bson.M{
        "toOrgId": bson.M{"$in": userOrgIDs},
        "isPublicShare": false,
    }
    
    // Query 2: Public shares (IsPublicShare = true)
    filter2 := bson.M{
        "isPublicShare": true,
    }
    
    // Hợp nhất kết quả
    // Lấy OwnerOrganizationID từ cả 2 queries
}
```

---

## 📝 Ví Dụ Sử Dụng

### **Case 1: Share Cho 1 Org**

```json
POST /api/v1/organization-share
{
  "ownerOrganizationId": "sales_dept",
  "toOrgId": "team_a",
  "permissionNames": []
}
```

**Kết quả:** Tạo 1 record với `ToOrgID = team_a`, `IsPublicShare = false`

---

### **Case 2: Share Cho Nhiều Orgs**

```json
POST /api/v1/organization-share/bulk
{
  "ownerOrganizationId": "sales_dept",
  "toOrgIds": ["team_a", "team_b", "team_c"],
  "permissionNames": []
}
```

**Kết quả:** Tạo 3 records:
- Record 1: `ToOrgID = team_a`, `IsPublicShare = false`
- Record 2: `ToOrgID = team_b`, `IsPublicShare = false`
- Record 3: `ToOrgID = team_c`, `IsPublicShare = false`

---

### **Case 3: Share Cho Tất Cả**

```json
POST /api/v1/organization-share
{
  "ownerOrganizationId": "sales_dept",
  "shareToAll": true,
  "permissionNames": []
}
```

**Kết quả:** Tạo 1 record với `ToOrgID = null`, `IsPublicShare = true`

**Lưu ý:** Khi query, tất cả organizations đều thấy data của `sales_dept`.

---

## 🔍 Query Logic

### **GetSharedOrganizationIDs()**

```go
func GetSharedOrganizationIDs(ctx context.Context, userOrgIDs []primitive.ObjectID, permissionName string) ([]primitive.ObjectID, error) {
    // 1. Query shares có ToOrgID trong userOrgIDs
    filter1 := bson.M{
        "toOrgId": bson.M{"$in": userOrgIDs},
        "isPublicShare": false,
    }
    
    // 2. Query public shares
    filter2 := bson.M{
        "isPublicShare": true,
    }
    
    // 3. Nếu có permissionName, filter thêm
    if permissionName != "" {
        permissionFilter := bson.M{
            "$or": []bson.M{
                {"permissionNames": bson.M{"$exists": false}},
                {"permissionNames": bson.M{"$size": 0}},
                {"permissionNames": bson.M{"$in": []string{permissionName}}},
            },
        }
        filter1 = bson.M{"$and": []bson.M{filter1, permissionFilter}}
        filter2 = bson.M{"$and": []bson.M{filter2, permissionFilter}}
    }
    
    // 4. Query cả 2
    shares1, _ := shareService.Find(ctx, filter1, nil)
    shares2, _ := shareService.Find(ctx, filter2, nil)
    
    // 5. Hợp nhất và lấy OwnerOrganizationID
    sharedOrgIDsMap := make(map[primitive.ObjectID]bool)
    for _, share := range shares1 {
        sharedOrgIDsMap[share.OwnerOrganizationID] = true
    }
    for _, share := range shares2 {
        sharedOrgIDsMap[share.OwnerOrganizationID] = true
    }
    
    // 6. Convert to slice
    result := make([]primitive.ObjectID, 0, len(sharedOrgIDsMap))
    for orgID := range sharedOrgIDsMap {
        result = append(result, orgID)
    }
    
    return result, nil
}
```

---

## ⚠️ Lưu Ý Quan Trọng

### **1. Validation**

- ✅ Validate: Không được có cả `ToOrgID` và `ToOrgIDs` cùng lúc
- ✅ Validate: Không được có cả `ToOrgID`/`ToOrgIDs` và `ShareToAll` cùng lúc
- ✅ Validate: `ShareToAll = true` thì `ToOrgID` phải null
- ✅ Validate: `IsPublicShare = true` thì chỉ có 1 record cho mỗi `OwnerOrganizationID`

### **2. Performance**

- ✅ Index trên `toOrgId` (sparse) để query nhanh
- ✅ Index trên `isPublicShare` để query public shares nhanh
- ✅ Compound index: `(ownerOrganizationId, toOrgId, isPublicShare)`

### **3. Migration**

- ✅ Không cần migration vì chưa có dữ liệu
- ✅ Có thể thêm default: `IsPublicShare = false` cho records cũ (nếu có)

---

## 📊 So Sánh Performance

| Phương Án | Query Time | Index Efficiency | Maintainability |
|-----------|------------|------------------|-----------------|
| Phương Án 1 (Array) | Chậm (phải scan array) | Kém | Khó |
| Phương Án 2 (Bulk Create) | Nhanh (index đơn giản) | Tốt | Dễ |
| Phương Án 3 (2 Models) | Trung bình (2 queries) | Tốt | Khó |

---

## ✅ Kết Luận

**Chọn Phương Án 2: Giữ nguyên model với `ToOrgID` (có thể null) và `IsPublicShare`**

**Lý do:**
1. ✅ Đơn giản, dễ hiểu
2. ✅ Performance tốt với index
3. ✅ Dễ maintain
4. ✅ Hỗ trợ đầy đủ 3 trường hợp
5. ✅ Dễ audit và quản lý

---

**Tài liệu này đề xuất thiết kế cho tính năng share cho nhiều tổ chức và share cho tất cả.**
