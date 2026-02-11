# Cơ Chế Quản Lý OwnerOrganizationId

## 📋 Tổng Quan

`OwnerOrganizationID` là field dùng cho **phân quyền dữ liệu (data authorization)** - xác định dữ liệu thuộc về tổ chức nào. Cơ chế này đảm bảo user chỉ có thể truy cập và thao tác với dữ liệu của các organizations mà họ có quyền.

## 🎯 Mục Đích

1. **Phân quyền dữ liệu**: Đảm bảo user chỉ truy cập được dữ liệu của organizations được phép
2. **Bảo mật**: Ngăn chặn user truy cập dữ liệu của organizations khác
3. **Linh hoạt**: Cho phép client chỉ định `ownerOrganizationId` từ request hoặc tự động lấy từ context

## 🏗️ Kiến Trúc

### 1. Detection (Phát Hiện)

Hệ thống tự động phát hiện model có field `OwnerOrganizationID` hay không bằng reflection:

```28:44:api/internal/api/handler/handler.base.go
// hasOrganizationIDField kiểm tra model có field OwnerOrganizationID không (dùng reflection)
// Field này dùng cho phân quyền dữ liệu (data authorization) - xác định dữ liệu thuộc về tổ chức nào
func (h *BaseHandler[T, CreateInput, UpdateInput]) hasOrganizationIDField() bool {
	var zero T
	val := reflect.ValueOf(zero)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return false
	}

	// Tìm field OwnerOrganizationID (tên mới cho phân quyền dữ liệu)
	field := val.FieldByName("OwnerOrganizationID")
	return field.IsValid()
}
```

### 2. Helper Functions (Các Hàm Tiện Ích)

#### 2.1. Lấy Organization ID từ Context

```46:57:api/internal/api/handler/handler.base.go
// getActiveOrganizationID lấy active organization ID từ context
func (h *BaseHandler[T, CreateInput, UpdateInput]) getActiveOrganizationID(c fiber.Ctx) *primitive.ObjectID {
	orgIDStr, ok := c.Locals("active_organization_id").(string)
	if !ok || orgIDStr == "" {
		return nil
	}
	orgID, err := primitive.ObjectIDFromHex(orgIDStr)
	if err != nil {
		return nil
	}
	return &orgID
}
```

#### 2.2. Gán Organization ID vào Model

```59:106:api/internal/api/handler/handler.base.go
// setOrganizationID tự động gán ownerOrganizationId vào model (dùng reflection)
// CHỈ gán nếu model có field OwnerOrganizationID
// CHỈ gán từ context nếu model chưa có giá trị (zero) - ưu tiên giá trị từ request body
// **LƯU Ý**: CHỈ set OwnerOrganizationID (phân quyền), KHÔNG set OrganizationID (logic business)
// OrganizationID phải được set riêng từ request body hoặc logic business
func (h *BaseHandler[T, CreateInput, UpdateInput]) setOrganizationID(model interface{}, orgID primitive.ObjectID) {
	// Kiểm tra model có field OwnerOrganizationID không
	if !h.hasOrganizationIDField() {
		return // Model không có OwnerOrganizationID, không cần gán
	}

	// Kiểm tra organizationId không phải zero value
	if orgID.IsZero() {
		return // Không gán zero ObjectID
	}

	val := reflect.ValueOf(model)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	field := val.FieldByName("OwnerOrganizationID")
	if !field.IsValid() || !field.CanSet() {
		return
	}

	// Kiểm tra xem model đã có organizationId chưa (không phải zero)
	// Nếu đã có giá trị hợp lệ từ request body thì không override
	if field.Kind() == reflect.Ptr {
		// Field là pointer
		if !field.IsNil() {
			currentOrgIDPtr := field.Interface().(*primitive.ObjectID)
			if currentOrgIDPtr != nil && !currentOrgIDPtr.IsZero() {
				return // Đã có giá trị hợp lệ, không override
			}
		}
		// Chỉ gán nếu chưa có giá trị hoặc là zero
		field.Set(reflect.ValueOf(&orgID))
	} else {
		// Field là value
		currentOrgID := field.Interface().(primitive.ObjectID)
		if !currentOrgID.IsZero() {
			return // Đã có giá trị hợp lệ từ request body, không override
		}
		// Chỉ gán nếu là zero value
		field.Set(reflect.ValueOf(orgID))
	}
}
```

**Đặc điểm quan trọng:**
- ✅ Chỉ gán nếu model có field `OwnerOrganizationID`
- ✅ Ưu tiên giá trị từ request body (không override nếu đã có)
- ✅ Chỉ gán từ context nếu model chưa có giá trị (zero value)

#### 2.3. Lấy Organization ID từ Model

```148:184:api/internal/api/handler/handler.base.go
// getOwnerOrganizationIDFromModel lấy ownerOrganizationId từ model (dùng reflection)
// Tương tự getOrganizationIDFromModel nhưng tên rõ ràng hơn
func (h *BaseHandler[T, CreateInput, UpdateInput]) getOwnerOrganizationIDFromModel(model interface{}) *primitive.ObjectID {
	// Sử dụng lại logic của getOrganizationIDFromModel
	// Vì getOrganizationIDFromModel đã lấy từ OwnerOrganizationID field
	if !h.hasOrganizationIDField() {
		return nil
	}

	val := reflect.ValueOf(model)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	field := val.FieldByName("OwnerOrganizationID")
	if !field.IsValid() {
		return nil
	}

	// Xử lý cả primitive.ObjectID và *primitive.ObjectID
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return nil
		}
		orgID := field.Interface().(*primitive.ObjectID)
		if orgID != nil && !orgID.IsZero() {
			return orgID
		}
	} else {
		orgID := field.Interface().(primitive.ObjectID)
		if !orgID.IsZero() {
			return &orgID
		}
	}

	return nil
}
```

### 3. Validation (Kiểm Tra Quyền)

#### 3.1. Validate User Có Quyền Với Organization

```186:222:api/internal/api/handler/handler.base.go
// validateUserHasAccessToOrg validate user có quyền với organization không
// Dùng để validate khi create/update với ownerOrganizationId từ request
func (h *BaseHandler[T, CreateInput, UpdateInput]) validateUserHasAccessToOrg(c fiber.Ctx, orgID primitive.ObjectID) error {
	// Lấy active role ID từ context (đã được middleware set)
	activeRoleIDStr, ok := c.Locals("active_role_id").(string)
	if !ok || activeRoleIDStr == "" {
		return common.NewError(common.ErrCodeAuthRole, "Không có role context", common.StatusUnauthorized, nil)
	}
	activeRoleID, err := primitive.ObjectIDFromHex(activeRoleIDStr)
	if err != nil {
		return common.NewError(common.ErrCodeAuthRole, "Role ID không hợp lệ", common.StatusUnauthorized, err)
	}

	// Lấy permission name từ context (đã được middleware set)
	permissionName := h.getPermissionNameFromRoute(c)

	// Lấy allowed organization IDs từ active role (đơn giản hơn, chỉ từ role context)
	allowedOrgIDs, err := services.GetAllowedOrganizationIDsFromRole(c.Context(), activeRoleID, permissionName)
	if err != nil {
		return err
	}

	// Kiểm tra organization có trong allowed list không
	for _, allowedOrgID := range allowedOrgIDs {
		if allowedOrgID == orgID {
			return nil // ✅ Có quyền
		}
	}

	// ❌ Không có quyền
	return common.NewError(
		common.ErrCodeAuthRole,
		"Không có quyền với organization này",
		common.StatusForbidden,
		nil,
	)
}
```

**Cơ chế hoạt động:**
1. Lấy `active_role_id` từ context (đã được middleware set)
2. Lấy `permission_name` từ context (đã được middleware set)
3. Gọi `GetAllowedOrganizationIDsFromRole()` để lấy danh sách organizations được phép
4. Kiểm tra `orgID` có trong danh sách được phép không
5. Trả về error nếu không có quyền

#### 3.2. Validate Quyền Truy Cập Document

```287:339:api/internal/api/handler/handler.base.go
// validateOrganizationAccess validate user có quyền truy cập document này không
// CHỈ validate nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
func (h *BaseHandler[T, CreateInput, UpdateInput]) validateOrganizationAccess(c fiber.Ctx, documentID string) error {
	// ✅ QUAN TRỌNG: Kiểm tra model có field OwnerOrganizationID không
	if !h.hasOrganizationIDField() {
		return nil // Model không có OwnerOrganizationID, không cần validate
	}

	// Lấy document
	id, err := primitive.ObjectIDFromHex(documentID)
	if err != nil {
		return common.NewError(common.ErrCodeValidationInput, "ID không hợp lệ", common.StatusBadRequest, err)
	}

	doc, err := h.BaseService.FindOneById(c.Context(), id)
	if err != nil {
		return err
	}

	// Lấy organizationId từ document (dùng reflection)
	docOrgID := h.getOrganizationIDFromModel(doc)
	if docOrgID == nil {
		return nil // Không có organizationId, không cần validate
	}

	// Lấy active role ID từ context (đã được middleware set)
	activeRoleIDStr, ok := c.Locals("active_role_id").(string)
	if !ok || activeRoleIDStr == "" {
		return common.NewError(common.ErrCodeAuthRole, "Không có role context", common.StatusUnauthorized, nil)
	}
	activeRoleID, err := primitive.ObjectIDFromHex(activeRoleIDStr)
	if err != nil {
		return common.NewError(common.ErrCodeAuthRole, "Role ID không hợp lệ", common.StatusUnauthorized, err)
	}

	// Lấy permission name từ context (đã được middleware set)
	permissionName := h.getPermissionNameFromRoute(c)

	// Lấy allowed organization IDs từ active role (đơn giản hơn, chỉ từ role context)
	allowedOrgIDs, err := services.GetAllowedOrganizationIDsFromRole(c.Context(), activeRoleID, permissionName)
	if err != nil {
		return err
	}

	// Kiểm tra document có thuộc allowed organizations không
	for _, allowedOrgID := range allowedOrgIDs {
		if allowedOrgID == *docOrgID {
			return nil // Có quyền truy cập
		}
	}

	return common.NewError(common.ErrCodeAuthRole, "Không có quyền truy cập", common.StatusForbidden, nil)
}
```

### 4. Filter (Lọc Dữ Liệu)

#### 4.1. Áp Dụng Organization Filter

```224:285:api/internal/api/handler/handler.base.go
// applyOrganizationFilter tự động thêm filter ownerOrganizationId
// CHỈ áp dụng nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
func (h *BaseHandler[T, CreateInput, UpdateInput]) applyOrganizationFilter(c fiber.Ctx, baseFilter bson.M) bson.M {
	// ✅ QUAN TRỌNG: Kiểm tra model có field OwnerOrganizationID không
	if !h.hasOrganizationIDField() {
		return baseFilter // Model không có OwnerOrganizationID, không cần filter
	}

	// Lấy active role ID từ context (đã được middleware set)
	activeRoleIDStr, ok := c.Locals("active_role_id").(string)
	if !ok || activeRoleIDStr == "" {
		return baseFilter // Không có active role, không filter
	}
	activeRoleID, err := primitive.ObjectIDFromHex(activeRoleIDStr)
	if err != nil {
		return baseFilter
	}

	// Lấy permission name từ context (đã được middleware set)
	permissionName := h.getPermissionNameFromRoute(c)

	// Lấy allowed organization IDs từ active role (đơn giản hơn, chỉ từ role context)
	allowedOrgIDs, err := services.GetAllowedOrganizationIDsFromRole(c.Context(), activeRoleID, permissionName)
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

	// Thêm filter ownerOrganizationId (phân quyền dữ liệu)
	orgFilter := bson.M{"ownerOrganizationId": bson.M{"$in": allowedOrgIDs}}

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

**Đặc điểm:**
- ✅ Tự động thêm filter `ownerOrganizationId` vào tất cả queries
- ✅ Bao gồm cả organizations được share
- ✅ Chỉ áp dụng nếu model có field `OwnerOrganizationID`

## 🔄 Flow Xử Lý Trong CRUD Operations

### 1. CREATE (InsertOne, InsertMany, Upsert)

```56:71:api/internal/api/handler/handler.base.crud.go
		// ✅ Xử lý ownerOrganizationId: Cho phép chỉ định từ request hoặc dùng context
		ownerOrgIDFromRequest := h.getOwnerOrganizationIDFromModel(model)
		if ownerOrgIDFromRequest != nil && !ownerOrgIDFromRequest.IsZero() {
			// Có ownerOrganizationId trong request → Validate quyền
			if err := h.validateUserHasAccessToOrg(c, *ownerOrgIDFromRequest); err != nil {
				h.HandleResponse(c, nil, err)
				return nil
			}
			// ✅ Có quyền → Giữ nguyên ownerOrganizationId từ request
		} else {
			// Không có trong request → Dùng context (backward compatible)
			activeOrgID := h.getActiveOrganizationID(c)
			if activeOrgID != nil && !activeOrgID.IsZero() {
				h.setOrganizationID(model, *activeOrgID)
			}
		}
```

**Flow:**
1. Kiểm tra xem request có `ownerOrganizationId` không
2. **Nếu có**: Validate quyền với organization đó
   - ✅ Có quyền → Giữ nguyên giá trị từ request
   - ❌ Không có quyền → Trả về lỗi 403 Forbidden
3. **Nếu không có**: Tự động lấy từ context (`active_organization_id`)
   - Gán vào model nếu context có giá trị

### 2. READ (Find, FindOne, FindOneById, FindWithPagination)

**Find, FindOne, FindWithPagination:**
```150:151:api/internal/api/handler/handler.base.crud.go
		// ✅ Tự động thêm filter ownerOrganizationId nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
		filter = h.applyOrganizationFilter(c, filter)
```

**FindOneById:**
```196:200:api/internal/api/handler/handler.base.crud.go
		// ✅ Validate ownerOrganizationId trước khi query nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
		if err := h.validateOrganizationAccess(c, id); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
```

**Flow:**
1. **Find operations**: Tự động thêm filter `ownerOrganizationId` vào query
2. **FindOneById**: Validate quyền truy cập document trước khi query

### 3. UPDATE (UpdateById, UpdateOne, UpdateMany)

**UpdateById:**
```526:569:api/internal/api/handler/handler.base.crud.go
		// ✅ Validate quyền với document hiện tại trước khi update
		if err := h.validateOrganizationAccess(c, id); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Parse input thành map để chỉ update các trường được chỉ định
		var updateData map[string]interface{}
		if err := json.NewDecoder(bytes.NewReader(c.Body())).Decode(&updateData); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu cập nhật phải là một object JSON hợp lệ. Chi tiết lỗi: %v", err),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// ✅ Xử lý ownerOrganizationId: Cho phép update với validation quyền
		if newOwnerOrgIDStr, ok := updateData["ownerOrganizationId"].(string); ok && newOwnerOrgIDStr != "" {
			// Parse ObjectID
			newOwnerOrgID, err := primitive.ObjectIDFromHex(newOwnerOrgIDStr)
			if err != nil {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					"ownerOrganizationId không hợp lệ",
					common.StatusBadRequest,
					err,
				))
				return nil
			}

			// Validate user có quyền với organization mới
			if err := h.validateUserHasAccessToOrg(c, newOwnerOrgID); err != nil {
				h.HandleResponse(c, nil, err)
				return nil
			}

			// ✅ Có quyền cả 2 (document hiện tại + organization mới) → Thay thế string bằng ObjectID trong updateData để MongoDB lưu đúng kiểu
			updateData["ownerOrganizationId"] = newOwnerOrgID
		} else {
			// Không có ownerOrganizationId trong update → Xóa nếu có (giữ nguyên logic cũ)
			delete(updateData, "ownerOrganizationId")
		}
```

**Flow:**
1. **UpdateById**: Validate quyền với document hiện tại trước
2. Kiểm tra xem update data có `ownerOrganizationId` không
3. **Nếu có**: Validate quyền với organization mới
   - ✅ Có quyền → Cho phép update
   - ❌ Không có quyền → Trả về lỗi 403 Forbidden
4. **Nếu không có**: Xóa field khỏi update data (giữ nguyên giá trị cũ)

**UpdateOne, UpdateMany:**
- Tự động thêm filter `ownerOrganizationId` vào filter query
- Validate quyền với organization mới nếu có trong update data

### 4. DELETE (DeleteById, DeleteMany)

**DeleteMany:**
```628:629:api/internal/api/handler/handler.base.crud.go
		// ✅ Tự động thêm filter ownerOrganizationId nếu model có field OwnerOrganizationID (phân quyền dữ liệu)
		filter = h.applyOrganizationFilter(c, filter)
```

**DeleteById:**
- Không có validation đặc biệt (có thể cần thêm trong tương lai)

## 🔐 Cơ Chế Authorization

### 1. GetAllowedOrganizationIDsFromRole

Hàm này lấy danh sách organizations mà role được phép truy cập dựa trên:
- **Role's OwnerOrganizationID**: Organization mà role thuộc về
- **Permission**: Permission name từ route
- **Scope**: 
  - `Scope = 0`: Chỉ organization của role
  - `Scope = 1`: Organization + children organizations

### 2. GetSharedOrganizationIDs

Lấy danh sách organizations được share với user's organizations thông qua cơ chế sharing.

### 3. Kết Hợp

Danh sách organizations cuối cùng = **Allowed Organizations** + **Shared Organizations**

## 📊 Ví Dụ Sử Dụng

### Ví Dụ 1: Tạo Mới Với OwnerOrganizationId Từ Request

```json
POST /api/customers
{
  "name": "Customer A",
  "ownerOrganizationId": "507f1f77bcf86cd799439011"
}
```

**Flow:**
1. Parse request body
2. Transform DTO sang Model
3. Phát hiện có `ownerOrganizationId` trong request
4. Validate quyền với organization `507f1f77bcf86cd799439011`
5. ✅ Có quyền → Lưu với `ownerOrganizationId = 507f1f77bcf86cd799439011`
6. ❌ Không có quyền → Trả về lỗi 403

### Ví Dụ 2: Tạo Mới Không Có OwnerOrganizationId

```json
POST /api/customers
{
  "name": "Customer B"
}
```

**Flow:**
1. Parse request body
2. Transform DTO sang Model
3. Không có `ownerOrganizationId` trong request
4. Lấy từ context (`active_organization_id`)
5. Tự động gán vào model
6. Lưu với `ownerOrganizationId` từ context

### Ví Dụ 3: Tìm Kiếm

```
GET /api/customers?filter={"name":"Customer A"}
```

**Flow:**
1. Parse filter từ query string
2. Tự động thêm filter `ownerOrganizationId` vào query
3. Query chỉ trả về customers thuộc organizations mà user có quyền

**Filter thực tế:**
```json
{
  "$and": [
    {"name": "Customer A"},
    {"ownerOrganizationId": {"$in": ["507f1f77bcf86cd799439011", "507f1f77bcf86cd799439012"]}}
  ]
}
```

### Ví Dụ 4: Update OwnerOrganizationId

```json
PUT /api/customers/507f1f77bcf86cd799439013
{
  "ownerOrganizationId": "507f1f77bcf86cd799439014"
}
```

**Flow:**
1. Validate quyền với document hiện tại (customer `507f1f77bcf86cd799439013`)
2. Parse update data
3. Phát hiện có `ownerOrganizationId` mới trong update
4. Validate quyền với organization mới (`507f1f77bcf86cd799439014`)
5. ✅ Có quyền cả 2 → Cho phép update
6. ❌ Không có quyền → Trả về lỗi 403

## ⚠️ Lưu Ý Quan Trọng

### 1. Ưu Tiên Giá Trị Từ Request

- Nếu client gửi `ownerOrganizationId` trong request → **Ưu tiên giá trị từ request** (sau khi validate quyền)
- Nếu không có trong request → Tự động lấy từ context

### 2. Validation Bắt Buộc

- **Luôn validate quyền** khi client chỉ định `ownerOrganizationId`
- Không cho phép set `ownerOrganizationId` cho organization mà user không có quyền

### 3. Tự Động Filter

- Tất cả **READ operations** tự động filter theo `ownerOrganizationId`
- User chỉ thấy dữ liệu của organizations mà họ có quyền

### 4. Model Phải Có Field OwnerOrganizationID

- Cơ chế chỉ hoạt động nếu model có field `OwnerOrganizationID`
- Nếu model không có field này → Không áp dụng validation và filter

### 5. Backward Compatible

- Nếu không có `ownerOrganizationId` trong request → Tự động lấy từ context
- Đảm bảo tương thích với code cũ

## 🔍 Debugging

### Kiểm Tra Model Có Field OwnerOrganizationID

```go
hasField := handler.hasOrganizationIDField()
```

### Kiểm Tra Organization ID Từ Context

```go
activeOrgID := handler.getActiveOrganizationID(c)
```

### Kiểm Tra Organization ID Từ Model

```go
orgID := handler.getOwnerOrganizationIDFromModel(model)
```

### Validate Quyền

```go
err := handler.validateUserHasAccessToOrg(c, orgID)
if err != nil {
    // Không có quyền
}
```

## 📝 Tóm Tắt

Cơ chế quản lý `ownerOrganizationId` đảm bảo:

1. ✅ **Bảo mật**: User chỉ truy cập được dữ liệu của organizations được phép
2. ✅ **Linh hoạt**: Cho phép client chỉ định hoặc tự động lấy từ context
3. ✅ **Tự động**: Tự động filter và validate trong tất cả CRUD operations
4. ✅ **Validation**: Luôn validate quyền trước khi cho phép set `ownerOrganizationId`
5. ✅ **Backward Compatible**: Tương thích với code cũ không có `ownerOrganizationId` trong request
