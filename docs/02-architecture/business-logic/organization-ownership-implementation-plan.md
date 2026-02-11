# Kế Hoạch Triển Khai Organization Ownership

## 📋 Tổng Quan

Tài liệu này mô tả chi tiết các collection cần triển khai và ảnh hưởng đến CRUD base functions.

## 🎯 Các Collection Cần Triển Khai

### Priority 1 - Business Critical (Bắt buộc)

1. **FbCustomer** (`model.fb.customer.go`)
   - Thêm: `OrganizationID primitive.ObjectID` với index `single:1`
   - Ảnh hưởng: Tất cả CRUD operations

2. **PcPosCustomer** (`model.pc.pos.customer.go`)
   - Thêm: `OrganizationID primitive.ObjectID` với index `single:1`
   - Ảnh hưởng: Tất cả CRUD operations

3. **PcPosOrder** (`model.pc.pos.order.go`)
   - Thêm: `OrganizationID primitive.ObjectID` với index `single:1`
   - Ảnh hưởng: Tất cả CRUD operations

4. **PcPosShop** (`model.pc.pos.shop.go`)
   - Thêm: `OrganizationID primitive.ObjectID` với index `single:1`
   - Ảnh hưởng: Tất cả CRUD operations

5. **PcPosProduct** (`model.pc.pos.product.go`)
   - Thêm: `OrganizationID primitive.ObjectID` với index `single:1`
   - Ảnh hưởng: Tất cả CRUD operations

6. **PcPosWarehouse** (`model.pc.pos.warehouse.go`)
   - Thêm: `OrganizationID primitive.ObjectID` với index `single:1`
   - Ảnh hưởng: Tất cả CRUD operations

7. **FbPage** (`model.fb.page.go`)
   - Thêm: `OrganizationID primitive.ObjectID` với index `single:1`
   - Ảnh hưởng: Tất cả CRUD operations

8. **FbPost** (`model.fb.post.go`)
   - Thêm: `OrganizationID primitive.ObjectID` với index `single:1`
   - Ảnh hưởng: Tất cả CRUD operations

9. **FbConversation** (`model.fb.conversation.go`)
   - Thêm: `OrganizationID primitive.ObjectID` với index `single:1`
   - Ảnh hưởng: Tất cả CRUD operations

10. **FbMessage** (`model.fb.message.go`)
    - Thêm: `OrganizationID primitive.ObjectID` với index `single:1`
    - Ảnh hưởng: Tất cả CRUD operations

### Priority 2 - Tối Ưu Query (Nên thêm)

11. **PcPosCategory** (`model.pc.pos.category.go`)
    - Thêm: `OrganizationID primitive.ObjectID` với index `single:1`
    - Ảnh hưởng: Tất cả CRUD operations
    - Lý do: Có thể lấy qua Shop, nhưng nên thêm trực tiếp để tối ưu

12. **PcPosVariation** (`model.pc.pos.variation.go`)
    - Thêm: `OrganizationID primitive.ObjectID` với index `single:1`
    - Ảnh hưởng: Tất cả CRUD operations
    - Lý do: Có thể lấy qua Product, nhưng nên thêm trực tiếp để tối ưu

13. **FbMessageItem** (`model.fb.message.item.go`)
    - Thêm: `OrganizationID primitive.ObjectID` với index `single:1`
    - Ảnh hưởng: Tất cả CRUD operations
    - Lý do: Có thể lấy qua Conversation, nhưng nên thêm trực tiếp để tối ưu

### Priority 3 - Cần Xác Định Business Logic

14. **PcOrder** (`model.pc.order.go`)
    - Cần xác định: Đơn hàng có thuộc organization không?
    - Nếu có: Thêm `OrganizationID primitive.ObjectID` với index `single:1`

### Priority 4 - Optional

15. **Agent** (`model.auth.agent.go`)
    - Thêm: `OrganizationID *primitive.ObjectID` (nullable) với index `single:1`
    - Ảnh hưởng: Tất cả CRUD operations
    - Lý do: Agent có thể global hoặc thuộc organization

### Priority 5 - Cần Phân Quyền

16. **AccessTokens** (`model.pc.access_token.go`)
    - Thêm: `OrganizationID primitive.ObjectID` với index `single:1`
    - Ảnh hưởng: Tất cả CRUD operations
    - Lý do: Cần phân quyền theo organization

17. **Customer** (`model.customer.go`)
    - Thêm: `OrganizationID primitive.ObjectID` với index `single:1`
    - Ảnh hưởng: Tất cả CRUD operations
    - Lý do: Cần phân quyền theo organization (nếu vẫn còn sử dụng)
    - Lưu ý: Có thể deprecated, nhưng nếu vẫn dùng thì cần thêm organizationId

### Collections KHÔNG CẦN

- **Customer** (`model.customer.go`) - Deprecated, không cần cập nhật
- **User** (`model.auth.user.go`) - Đã có cơ chế gián tiếp qua UserRole
- **Organizations** (`model.auth.organization.go`) - Chính nó là organization
- **Permissions** (`model.auth.permission.go`) - System-wide
- **UserRoles** (`model.auth.user_role.go`) - Mapping table
- **RolePermissions** (`model.auth.role_permission.go`) - Mapping table

## 🔧 Ảnh Hưởng Đến CRUD Base Functions

### BaseHandler Structure

```go
type BaseHandler[T any, CreateInput any, UpdateInput any] struct {
    BaseService   services.BaseServiceMongo[T]
    filterOptions FilterOptions
}
```

### Các Functions Cần Thay Đổi

#### 1. InsertOne() - Tự động gán organizationId

**File**: `api/internal/api/handler/handler.base.crud.go`

**Thay đổi:**
```go
func (h *BaseHandler[T, CreateInput, UpdateInput]) InsertOne(c fiber.Ctx) error {
    return h.SafeHandler(c, func() error {
        // Parse request body
        input := new(T)
        if err := h.ParseRequestBody(c, input); err != nil {
            // ... error handling
        }

        // ✅ THÊM: Tự động gán organizationId
        activeOrgID := h.getActiveOrganizationID(c)
        if activeOrgID != nil {
            h.setOrganizationID(input, *activeOrgID)
        }

        data, err := h.BaseService.InsertOne(c.Context(), *input)
        h.HandleResponse(c, data, err)
        return nil
    })
}
```

**Helper functions cần thêm:**
```go
// hasOrganizationIDField kiểm tra model có field OrganizationID không (dùng reflection)
func (h *BaseHandler[T, CreateInput, UpdateInput]) hasOrganizationIDField() bool {
    var zero T
    val := reflect.ValueOf(zero)
    if val.Kind() == reflect.Ptr {
        val = val.Elem()
    }
    
    if val.Kind() != reflect.Struct {
        return false
    }
    
    field := val.FieldByName("OrganizationID")
    return field.IsValid()
}

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

// setOrganizationID tự động gán organizationId vào model (dùng reflection)
// CHỈ gán nếu model có field OrganizationID
func (h *BaseHandler[T, CreateInput, UpdateInput]) setOrganizationID(model interface{}, orgID primitive.ObjectID) {
    // Kiểm tra model có field OrganizationID không
    if !h.hasOrganizationIDField() {
        return // Model không có OrganizationID, không cần gán
    }
    
    val := reflect.ValueOf(model)
    if val.Kind() == reflect.Ptr {
        val = val.Elem()
    }
    
    field := val.FieldByName("OrganizationID")
    if field.IsValid() && field.CanSet() {
        // Xử lý cả primitive.ObjectID và *primitive.ObjectID
        if field.Kind() == reflect.Ptr {
            // Field là pointer
            field.Set(reflect.ValueOf(&orgID))
        } else {
            // Field là value
            field.Set(reflect.ValueOf(orgID))
        }
    }
}
```

#### 2. InsertMany() - Tự động gán organizationId cho tất cả items

**File**: `api/internal/api/handler/handler.base.crud.go`

**Thay đổi:**
```go
func (h *BaseHandler[T, CreateInput, UpdateInput]) InsertMany(c fiber.Ctx) error {
    return h.SafeHandler(c, func() error {
        // Parse request body thành array
        var inputs []T
        // ... parse logic

        // ✅ THÊM: Tự động gán organizationId cho tất cả items
        activeOrgID := h.getActiveOrganizationID(c)
        if activeOrgID != nil {
            for i := range inputs {
                h.setOrganizationID(&inputs[i], *activeOrgID)
            }
        }

        data, err := h.BaseService.InsertMany(c.Context(), inputs)
        h.HandleResponse(c, data, err)
        return nil
    })
}
```

#### 3. Find() - Tự động filter theo organizationId

**File**: `api/internal/api/handler/handler.base.crud.go`

**Thay đổi:**
```go
func (h *BaseHandler[T, CreateInput, UpdateInput]) Find(c fiber.Ctx) error {
    return h.SafeHandler(c, func() error {
        filter, err := h.processFilter(c)
        if err != nil {
            h.HandleResponse(c, nil, err)
            return nil
        }

        // ✅ THÊM: Tự động thêm filter organizationId
        filter = h.applyOrganizationFilter(c, filter)

        options, err := h.processMongoOptions(c, false)
        // ... rest of code
    })
}
```

**Helper function:**
```go
// applyOrganizationFilter tự động thêm filter organizationId
// CHỈ áp dụng nếu model có field OrganizationID
func (h *BaseHandler[T, CreateInput, UpdateInput]) applyOrganizationFilter(c fiber.Ctx, baseFilter bson.M) bson.M {
    // ✅ QUAN TRỌNG: Kiểm tra model có field OrganizationID không
    if !h.hasOrganizationIDField() {
        return baseFilter // Model không có OrganizationID, không cần filter
    }
    
    // Lấy permission name từ route (nếu có)
    permissionName := h.getPermissionNameFromRoute(c)
    
    // Lấy user ID
    userIDStr, ok := c.Locals("user_id").(string)
    if !ok {
        return baseFilter // Không có user ID, không filter
    }
    userID, err := primitive.ObjectIDFromHex(userIDStr)
    if err != nil {
        return baseFilter
    }

    // Lấy allowed organization IDs (bao gồm cả parent)
    allowedOrgIDs, err := h.BaseService.GetUserAllowedOrganizationIDs(c.Context(), userID, permissionName)
    if err != nil || len(allowedOrgIDs) == 0 {
        return baseFilter
    }

    // Thêm filter organizationId
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

#### 4. FindOne() - Tự động filter theo organizationId

**File**: `api/internal/api/handler/handler.base.crud.go`

**Thay đổi:**
```go
func (h *BaseHandler[T, CreateInput, UpdateInput]) FindOne(c fiber.Ctx) error {
    return h.SafeHandler(c, func() error {
        filter, err := h.processFilter(c)
        if err != nil {
            h.HandleResponse(c, nil, err)
            return nil
        }

        // ✅ THÊM: Tự động thêm filter organizationId
        filter = h.applyOrganizationFilter(c, filter)

        // ... rest of code
    })
}
```

#### 5. FindOneById() - Validate organizationId

**File**: `api/internal/api/handler/handler.base.crud.go`

**Thay đổi:**
```go
func (h *BaseHandler[T, CreateInput, UpdateInput]) FindOneById(c fiber.Ctx) error {
    return h.SafeHandler(c, func() error {
        id := c.Params("id")
        // ... validate ID

        // ✅ THÊM: Validate organizationId trước khi query
        if err := h.validateOrganizationAccess(c, id); err != nil {
            h.HandleResponse(c, nil, err)
            return nil
        }

        // ... rest of code
    })
}
```

**Helper functions:**
```go
// getOrganizationIDFromModel lấy organizationId từ model (dùng reflection)
func (h *BaseHandler[T, CreateInput, UpdateInput]) getOrganizationIDFromModel(model T) *primitive.ObjectID {
    // Kiểm tra model có field OrganizationID không
    if !h.hasOrganizationIDField() {
        return nil // Model không có OrganizationID
    }
    
    val := reflect.ValueOf(model)
    if val.Kind() == reflect.Ptr {
        val = val.Elem()
    }
    
    field := val.FieldByName("OrganizationID")
    if !field.IsValid() {
        return nil
    }
    
    // Xử lý cả primitive.ObjectID và *primitive.ObjectID
    if field.Kind() == reflect.Ptr {
        if field.IsNil() {
            return nil
        }
        orgID := field.Interface().(*primitive.ObjectID)
        return orgID
    } else {
        orgID := field.Interface().(primitive.ObjectID)
        return &orgID
    }
}

// validateOrganizationAccess validate user có quyền truy cập document này không
// CHỈ validate nếu model có field OrganizationID
func (h *BaseHandler[T, CreateInput, UpdateInput]) validateOrganizationAccess(c fiber.Ctx, documentID string) error {
    // ✅ QUAN TRỌNG: Kiểm tra model có field OrganizationID không
    if !h.hasOrganizationIDField() {
        return nil // Model không có OrganizationID, không cần validate
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

    // Lấy allowed organization IDs
    userIDStr, _ := c.Locals("user_id").(string)
    userID, _ := primitive.ObjectIDFromHex(userIDStr)
    permissionName := h.getPermissionNameFromRoute(c)
    
    allowedOrgIDs, err := h.BaseService.GetUserAllowedOrganizationIDs(c.Context(), userID, permissionName)
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

#### 6. UpdateOne() - Tự động filter và validate organizationId

**File**: `api/internal/api/handler/handler.base.crud.go`

**Thay đổi:**
```go
func (h *BaseHandler[T, CreateInput, UpdateInput]) UpdateOne(c fiber.Ctx) error {
    return h.SafeHandler(c, func() error {
        filter, err := h.processFilter(c)
        if err != nil {
            h.HandleResponse(c, nil, err)
            return nil
        }

        // ✅ THÊM: Tự động thêm filter organizationId
        filter = h.applyOrganizationFilter(c, filter)

        // ✅ THÊM: Không cho phép update organizationId (bảo mật)
        var updateData map[string]interface{}
        if err := json.NewDecoder(bytes.NewReader(c.Body())).Decode(&updateData); err != nil {
            // ... error handling
        }
        delete(updateData, "organizationId") // Xóa organizationId khỏi update data

        // ... rest of code
    })
}
```

#### 7. UpdateMany() - Tự động filter theo organizationId

**File**: `api/internal/api/handler/handler.base.crud.go`

**Thay đổi:**
```go
func (h *BaseHandler[T, CreateInput, UpdateInput]) UpdateMany(c fiber.Ctx) error {
    return h.SafeHandler(c, func() error {
        filter, err := h.processFilter(c)
        if err != nil {
            h.HandleResponse(c, nil, err)
            return nil
        }

        // ✅ THÊM: Tự động thêm filter organizationId
        filter = h.applyOrganizationFilter(c, filter)

        // ✅ THÊM: Không cho phép update organizationId
        var updateData map[string]interface{}
        // ... parse và xóa organizationId

        // ... rest of code
    })
}
```

#### 8. DeleteOne() - Validate organizationId

**File**: `api/internal/api/handler/handler.base.crud.go`

**Thay đổi:**
```go
func (h *BaseHandler[T, CreateInput, UpdateInput]) DeleteOne(c fiber.Ctx) error {
    return h.SafeHandler(c, func() error {
        filter, err := h.processFilter(c)
        if err != nil {
            h.HandleResponse(c, nil, err)
            return nil
        }

        // ✅ THÊM: Tự động thêm filter organizationId
        filter = h.applyOrganizationFilter(c, filter)

        // ... rest of code
    })
}
```

#### 9. DeleteMany() - Tự động filter theo organizationId

**File**: `api/internal/api/handler/handler.base.crud.go`

**Thay đổi:**
```go
func (h *BaseHandler[T, CreateInput, UpdateInput]) DeleteMany(c fiber.Ctx) error {
    return h.SafeHandler(c, func() error {
        filter, err := h.processFilter(c)
        if err != nil {
            h.HandleResponse(c, nil, err)
            return nil
        }

        // ✅ THÊM: Tự động thêm filter organizationId
        filter = h.applyOrganizationFilter(c, filter)

        // ... rest of code
    })
}
```

#### 10. Upsert() - Tự động gán organizationId

**File**: `api/internal/api/handler/handler.base.crud.go`

**Thay đổi:**
```go
func (h *BaseHandler[T, CreateInput, UpdateInput]) Upsert(c fiber.Ctx) error {
    return h.SafeHandler(c, func() error {
        filter, err := h.processFilter(c)
        if err != nil {
            h.HandleResponse(c, nil, err)
            return nil
        }

        // ✅ THÊM: Tự động thêm filter organizationId vào filter
        filter = h.applyOrganizationFilter(c, filter)

        input := new(T)
        if err := h.ParseRequestBody(c, input); err != nil {
            // ... error handling
        }

        // ✅ THÊM: Tự động gán organizationId
        activeOrgID := h.getActiveOrganizationID(c)
        if activeOrgID != nil {
            h.setOrganizationID(input, *activeOrgID)
        }

        // ... rest of code
    })
}
```

#### 11. UpsertMany() - Tự động gán organizationId cho tất cả items

**File**: `api/internal/api/handler/handler.base.crud.go`

**Thay đổi:**
```go
func (h *BaseHandler[T, CreateInput, UpdateInput]) UpsertMany(c fiber.Ctx) error {
    return h.SafeHandler(c, func() error {
        filter, err := h.processFilter(c)
        if err != nil {
            h.HandleResponse(c, nil, err)
            return nil
        }

        // ✅ THÊM: Tự động thêm filter organizationId
        filter = h.applyOrganizationFilter(c, filter)

        var inputs []T
        // ... parse logic

        // ✅ THÊM: Tự động gán organizationId cho tất cả items
        activeOrgID := h.getActiveOrganizationID(c)
        if activeOrgID != nil {
            for i := range inputs {
                h.setOrganizationID(&inputs[i], *activeOrgID)
            }
        }

        // ... rest of code
    })
}
```

## 📝 Tóm Tắt Ảnh Hưởng

### Functions Cần Thay Đổi

1. ✅ **InsertOne** - Tự động gán `organizationId`
2. ✅ **InsertMany** - Tự động gán `organizationId` cho tất cả items
3. ✅ **Find** - Tự động filter theo `organizationId` (bao gồm parent)
4. ✅ **FindOne** - Tự động filter theo `organizationId`
5. ✅ **FindOneById** - Validate `organizationId` trước khi query
6. ✅ **UpdateOne** - Tự động filter và không cho phép update `organizationId`
7. ✅ **UpdateMany** - Tự động filter và không cho phép update `organizationId`
8. ✅ **DeleteOne** - Tự động filter theo `organizationId`
9. ✅ **DeleteMany** - Tự động filter theo `organizationId`
10. ✅ **Upsert** - Tự động gán `organizationId` và filter
11. ✅ **UpsertMany** - Tự động gán `organizationId` cho tất cả items và filter

### Helper Functions Cần Thêm

1. ✅ `hasOrganizationIDField()` - **QUAN TRỌNG**: Kiểm tra model có field OrganizationID không (reflection)
2. ✅ `getActiveOrganizationID(c)` - Lấy active organization ID từ context
3. ✅ `setOrganizationID(model, orgID)` - Tự động gán organizationId vào model (chỉ nếu có field)
4. ✅ `applyOrganizationFilter(c, baseFilter)` - Tự động thêm filter organizationId (chỉ nếu có field)
5. ✅ `validateOrganizationAccess(c, documentID)` - Validate user có quyền truy cập (chỉ nếu có field)
6. ✅ `getOrganizationIDFromModel(model)` - Lấy organizationId từ model (reflection)
7. ✅ `getPermissionNameFromRoute(c)` - Lấy permission name từ route (nếu có)

### Service Functions Cần Thêm

1. ✅ `GetUserAllowedOrganizationIDs(ctx, userID, permissionName)` - Tính toán allowed org IDs (bao gồm parent)
2. ✅ `GetParentIDs(ctx, childID)` - Lấy tất cả parent IDs (inverse lookup)

## ⚠️ Lưu Ý

1. **Reflection**: Sử dụng reflection để tự động detect và set `OrganizationID` field
2. **Backward Compatibility**: 
   - ✅ **QUAN TRỌNG**: Các model không có `OrganizationID` sẽ **KHÔNG bị ảnh hưởng**
   - ✅ Logic luôn check `hasOrganizationIDField()` trước khi áp dụng filter/gán giá trị
   - ✅ Nếu model không có `OrganizationID`, CRUD hoạt động bình thường như trước
3. **Performance**: 
   - Filter organizationId chỉ được thêm vào query nếu model có field
   - Cần đảm bảo có index cho các model có `OrganizationID`
4. **Security**: 
   - Không cho phép user update `organizationId` trực tiếp
   - Validate user có quyền truy cập document trước khi update/delete
5. **Validation**: 
   - Validate user có quyền truy cập document trước khi update/delete
   - Chỉ validate nếu model có field `OrganizationID`

## ✅ Đảm Bảo Backward Compatibility

### Các Collection KHÔNG CÓ OrganizationID

**Ví dụ: User, Permission, Organization, UserRole, RolePermission**

**Behavior:**
- ✅ `InsertOne()` - Không gán `organizationId` (vì không có field)
- ✅ `Find()` - Không thêm filter `organizationId` (vì không có field)
- ✅ `UpdateOne()` - Không validate `organizationId` (vì không có field)
- ✅ `DeleteOne()` - Không filter `organizationId` (vì không có field)
- ✅ **Tất cả CRUD operations hoạt động bình thường như trước**

### Các Collection CÓ OrganizationID

**Ví dụ: FbCustomer, PcPosCustomer, FbPage, etc.**

**Behavior:**
- ✅ `InsertOne()` - Tự động gán `organizationId` từ context
- ✅ `Find()` - Tự động filter theo `organizationId` (bao gồm parent)
- ✅ `UpdateOne()` - Validate và filter theo `organizationId`
- ✅ `DeleteOne()` - Filter theo `organizationId`
- ✅ **Tất cả CRUD operations có organization filtering**

### Logic Check

```go
// Mọi function đều check trước khi áp dụng
if !h.hasOrganizationIDField() {
    return // Không có field, không làm gì cả
}

// Chỉ khi có field mới áp dụng logic
// ...
```

