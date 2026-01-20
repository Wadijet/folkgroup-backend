# Refactor Organization Share Handler - Dùng CRUD Chuẩn

## Vấn Đề Hiện Tại

`OrganizationShareHandler` có 3 custom endpoints (`CreateShare`, `DeleteShare`, `ListShares`) với logic có thể được xử lý bằng CRUD chuẩn nếu đưa logic nghiệp vụ vào service layer.

## Phân Tích Logic

### 1. CreateShare

**Logic hiện tại trong handler:**
- ✅ Validation ObjectID → Có thể dùng `transform` tag trong DTO
- ✅ Authorization check → BaseHandler đã có `validateUserHasAccessToOrg` trong `InsertOne` (KHÔNG KHÁC GÌ CRUD KHÁC)
- ⚠️ Duplicate check với set comparison → **Cần đưa vào service.InsertOne override** (ĐÂY LÀ LOGIC DUY NHẤT KHÁC BIỆT)
- ⚠️ Validation "ownerOrgID không được có trong ToOrgIDs" → **Có thể làm custom validator**
- ✅ Set CreatedBy, CreatedAt → BaseHandler đã tự động set

**Kết luận:** Có thể dùng `InsertOne` nếu đưa duplicate check vào service. Authorization KHÔNG KHÁC GÌ CRUD KHÁC.

### 2. DeleteShare

**Logic hiện tại trong handler:**
- ✅ Authorization check → BaseHandler có `validateOrganizationAccess` hoặc service tự động filter qua `applyOrganizationFilter` (KHÔNG KHÁC GÌ CRUD KHÁC)
- ⚠️ Response format khác (message thay vì document) → **Có thể override response format hoặc frontend xử lý**

**Kết luận:** Có thể dùng `DeleteById` trực tiếp. Authorization KHÔNG KHÁC GÌ CRUD KHÁC.

### 3. ListShares

**Logic hiện tại trong handler:**
- ⚠️ Query $or phức tạp → **Có thể build từ query params hoặc helper function trong service**
- ✅ Authorization check → BaseHandler đã có `applyOrganizationFilter` tự động (KHÔNG KHÁC GÌ CRUD KHÁC)

**Kết luận:** Có thể dùng `Find` với query params. Authorization KHÔNG KHÁC GÌ CRUD KHÁC.

## Đề Xuất Refactor

### Bước 1: Thêm Logic Vào Service Layer

#### 1.1. Override `InsertOne` trong `OrganizationShareService`

```go
// InsertOne override để thêm duplicate check với set comparison
func (s *OrganizationShareService) InsertOne(ctx context.Context, data models.OrganizationShare) (models.OrganizationShare, error) {
    // 1. Validate: ownerOrgID không được có trong ToOrgIDs
    for _, toOrgID := range data.ToOrgIDs {
        if toOrgID == data.OwnerOrganizationID {
            return data, common.NewError(
                common.ErrCodeValidationInput,
                "ownerOrganizationId không được có trong toOrgIds",
                common.StatusBadRequest,
                nil,
            )
        }
    }

    // 2. Check duplicate với set comparison
    existingShares, err := s.Find(ctx, bson.M{
        "ownerOrganizationId": data.OwnerOrganizationID,
    }, nil)
    if err != nil && err != common.ErrNotFound {
        return data, err
    }

    // So sánh với shares hiện có (set comparison)
    for _, existingShare := range existingShares {
        if compareShareSets(data, existingShare) {
            return data, common.NewError(
                common.ErrCodeBusinessOperation,
                "Share với các organizations này đã tồn tại với cùng permissions",
                common.StatusConflict,
                nil,
            )
        }
    }

    // 3. Gọi InsertOne của base service
    return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}

// compareShareSets so sánh 2 shares (set comparison cho ToOrgIDs và PermissionNames)
func compareShareSets(share1, share2 models.OrganizationShare) bool {
    // So sánh ToOrgIDs (không quan tâm thứ tự)
    if !compareObjectIDSets(share1.ToOrgIDs, share2.ToOrgIDs) {
        return false
    }
    // So sánh PermissionNames (không quan tâm thứ tự)
    return compareStringSets(share1.PermissionNames, share2.PermissionNames)
}
```

#### 1.2. DeleteById - KHÔNG CẦN OVERRIDE

**BaseHandler.DeleteById đã xử lý authorization tự động:**
- Nếu model có `OwnerOrganizationID`, BaseHandler sẽ tự động validate qua `validateOrganizationAccess` hoặc service tự filter
- **KHÔNG CẦN override** - Authorization giống hệt các CRUD khác

#### 1.3. Find - KHÔNG CẦN OVERRIDE

**BaseHandler.Find đã xử lý authorization tự động:**
- BaseHandler có `applyOrganizationFilter` tự động thêm filter `ownerOrganizationId` dựa trên role context
- **KHÔNG CẦN override** - Authorization giống hệt các CRUD khác

### Bước 2: Cập Nhật DTO

#### 2.1. Thêm `transform` tag cho ObjectID validation

```go
type OrganizationShareCreateInput struct {
    OwnerOrganizationID string   `json:"ownerOrganizationId" validate:"required" transform:"str_objectid"` // Tự động convert và validate ObjectID
    ToOrgIDs            []string `json:"toOrgIds,omitempty" transform:"str_objectid_array,optional"`        // Tự động convert mảng ObjectID
    PermissionNames     []string `json:"permissionNames,omitempty"`
    Description         string   `json:"description,omitempty"`
}
```

### Bước 3: Xóa Custom Endpoints, Dùng CRUD Chuẩn

#### 3.1. Xóa `CreateShare` → Dùng `InsertOne`

**Route:**
```go
// Cũ: POST /api/v1/organization-shares (CreateShare)
// Mới: POST /api/v1/organization-shares (InsertOne)
```

**Handler:** Không cần override, dùng `BaseHandler.InsertOne` trực tiếp.

#### 3.2. Xóa `DeleteShare` → Dùng `DeleteOne` với custom response

**Route:**
```go
// Cũ: DELETE /api/v1/organization-shares/:id (DeleteShare)
// Mới: DELETE /api/v1/organization-shares/:id (DeleteOne)
```

**Handler:** Có thể override `DeleteOne` chỉ để thay đổi response format:

```go
// DeleteOne override chỉ để thay đổi response format
func (h *OrganizationShareHandler) DeleteOne(c fiber.Ctx) error {
    err := h.BaseHandler.DeleteOne(c)
    if err == nil {
        // Response đã được set trong BaseHandler, nhưng có thể override nếu cần
        // Hoặc dùng middleware để transform response
    }
    return err
}
```

Hoặc đơn giản hơn, dùng `DeleteOne` trực tiếp và frontend xử lý response.

#### 3.3. Xóa `ListShares` → Dùng `Find` với query params

**Route:**
```go
// Cũ: GET /api/v1/organization-shares?ownerOrganizationId=xxx hoặc ?toOrgId=xxx (ListShares)
// Mới: GET /api/v1/organization-shares?filter={...} (Find)
```

**Query params:**
```json
// Filter cho ownerOrganizationId
GET /api/v1/organization-shares?filter={"ownerOrganizationId":"507f1f77bcf86cd799439011"}

// Filter cho toOrgId (cần build $or query)
GET /api/v1/organization-shares?filter={"$or":[{"toOrgIds":"507f1f77bcf86cd799439011"},{"toOrgIds":{"$size":0}}]}
```

**Handler:** Không cần override, dùng `BaseHandler.Find` trực tiếp.

**Lưu ý:** Query $or phức tạp có thể được build từ query params hoặc tạo helper function trong service.

## Kết Quả

Sau khi refactor:
- ✅ **Giảm code trong handler**: Từ ~450 dòng xuống ~50 dòng (chỉ cần struct definition)
- ✅ **Logic nghiệp vụ tập trung**: Tất cả logic trong service layer
- ✅ **Dùng CRUD chuẩn**: InsertOne, DeleteOne, Find
- ✅ **Dễ maintain**: Logic nghiệp vụ ở một chỗ, dễ test

## Lưu Ý

1. **Response format cho DeleteOne**: Nếu cần response format khác (message thay vì document), có thể:
   - Override `DeleteOne` trong handler (chỉ để thay đổi response)
   - Hoặc dùng middleware để transform response
   - Hoặc frontend xử lý response

2. **Query $or phức tạp cho ListShares**: Có thể:
   - Build từ query params (phức tạp hơn cho frontend)
   - Tạo helper function trong service để build query
   - Hoặc giữ custom endpoint `ListShares` nhưng đơn giản hóa (chỉ build query, không có logic phức tạp)

3. **Backward compatibility**: Cần đảm bảo API contract không thay đổi (hoặc version API nếu thay đổi).

## Kết Luận

**User đúng** - Logic của Organization Share có thể được xử lý bằng CRUD chuẩn:

1. **Authorization KHÔNG KHÁC GÌ CRUD KHÁC:**
   - `InsertOne`: BaseHandler đã có `validateUserHasAccessToOrg`
   - `DeleteById`: BaseHandler có `validateOrganizationAccess` hoặc service tự filter
   - `Find`: BaseHandler có `applyOrganizationFilter` tự động

2. **Logic thực sự khác biệt:**
   - **Duplicate check với set comparison** → Đưa vào `service.InsertOne` override
   - **Validation "ownerOrgID không được có trong ToOrgIDs"** → Custom validator hoặc trong service

3. **Lợi ích:**
   - Giảm code duplication (từ ~450 dòng xuống ~50 dòng)
   - Tập trung logic nghiệp vụ vào service layer
   - Dễ test và maintain
   - Tuân thủ nguyên tắc separation of concerns
