# Tóm Tắt Logic Đã Sửa Đổi - Mở Rộng Transform Tag và Validator

## Tổng Quan

Mở rộng hệ thống transform tag và validator để hỗ trợ:
1. **Nested struct mapping** - Tự động map nested struct từ DTO sang Model
2. **Foreign key validation** - Tự động validate ObjectID tồn tại trong collection

**Mục tiêu**: Giảm code duplication, loại bỏ các override không cần thiết trong handlers.

---

## 1. Mở Rộng Transform Tag - Hỗ Trợ Nested Struct

### 1.1. Thêm Transform Type `nested_struct`

**File**: `api/core/utility/data.transform.go`

**Thay đổi**:
- Thêm case `"nested_struct"` trong `applyTransform()` để nhận diện nested struct transform
- Transform type này sẽ được xử lý ở level cao hơn (trong `transformCreateInputToModel`)

```go
case "nested_struct":
    // Nested struct transform - sẽ được xử lý ở level cao hơn
    return value, nil
```

### 1.2. Tạo Hàm `transformNestedStruct` (Recursive)

**File**: `api/core/api/handler/handler.base.go`

**Chức năng**: Transform nested struct từ DTO sang Model một cách recursive

**Logic**:
1. Xử lý pointer (nil pointer → return zero value)
2. Kiểm tra input phải là struct
3. Tạo instance của target type (pointer hoặc value)
4. Duyệt qua các field của input struct:
   - Nếu có `transform:"nested_struct"` → gọi recursive
   - Nếu có transform tag khác → apply transform
   - Nếu không có transform tag → copy trực tiếp nếu type tương thích
5. Return target value (pointer nếu targetType là pointer)

**Ví dụ sử dụng**:
```go
// DTO
type AIPromptTemplateProviderInput struct {
    ProfileID string        `json:"profileId,omitempty" transform:"str_objectid_ptr,optional"`
    Config    *AIConfigInput `json:"config,omitempty" transform:"nested_struct"`
}

// Model
type AIPromptTemplateProvider struct {
    ProfileID *primitive.ObjectID
    Config    *AIConfig
}
```

### 1.3. Cập Nhật `transformCreateInputToModel` và `transformUpdateInputToModel`

**File**: `api/core/api/handler/handler.base.go`

**Thay đổi**:
- Thêm logic xử lý `transform:"nested_struct"` trước khi xử lý transform thông thường
- Nếu `transformConfig.Type == "nested_struct"`:
  - Gọi `transformNestedStruct()` để transform recursive
  - Set giá trị vào Model field
  - Continue (bỏ qua logic transform thông thường)

**Vị trí trong code**:
```go
// Xử lý nested struct transform
if transformConfig.Type == "nested_struct" {
    nestedModel, err := h.transformNestedStruct(inputField, modelField.Type)
    // ... set vào Model field
    continue
}
```

---

## 2. Thêm Custom Validator - Foreign Key Validation

### 2.1. Tạo Custom Validator `exists`

**File**: `api/core/global/validator.go`

**Chức năng**: Kiểm tra ObjectID tồn tại trong collection (foreign key validation)

**Format**: `validate:"exists=<collection_name>"`

**Logic**:
1. Lấy collection name từ param
2. Convert value sang ObjectID (hỗ trợ string, ObjectID, *ObjectID)
3. Nếu value là empty/nil → return true (optional field, skip validation)
4. Lấy collection từ `RegistryCollections`
5. Query database với filter `{"_id": objID}`
6. Return `count > 0`

**Ví dụ sử dụng**:
```go
type AIPromptTemplateProviderInput struct {
    ProfileID string `json:"profileId,omitempty" validate:"omitempty,exists=ai_provider_profiles" transform:"str_objectid_ptr,optional"`
}
```

**Đăng ký validator**:
```go
_ = Validate.RegisterValidation("exists", validateExists)
```

---

## 3. Cập Nhật BaseHandler.UpdateOne

### 3.1. Thay Đổi Từ Parse Map Sang Dùng DTO

**File**: `api/core/api/handler/handler.base.crud.go`

**Trước**:
- Parse input thành `map[string]interface{}` trực tiếp từ request body
- Không hỗ trợ transform tag và nested struct

**Sau**:
- Parse request body thành DTO (`UpdateInput`)
- Validate input với struct tag
- Transform DTO sang Model sử dụng `transformUpdateInputToModel` (hỗ trợ nested struct)
- Convert Model sang `UpdateData` với `$set` operator

**Lợi ích**:
- Hỗ trợ nested struct trong update
- Hỗ trợ transform tag (ObjectID conversion, default values)
- Hỗ trợ validation với struct tag

**Thêm import**:
```go
import (
    "reflect"
    "go.mongodb.org/mongo-driver/bson"
)
```

---

## 4. Cập Nhật DTOs

### 4.1. AIPromptTemplateProviderInput

**File**: `api/core/api/dto/dto.ai.prompt.template.go`

**Thay đổi**:
```go
type AIPromptTemplateProviderInput struct {
    ProfileID string        `json:"profileId,omitempty" validate:"omitempty,exists=ai_provider_profiles" transform:"str_objectid_ptr,optional"`
    Config    *AIConfigInput `json:"config,omitempty" transform:"nested_struct"`
}
```

### 4.2. AIPromptTemplateCreateInput và UpdateInput

**File**: `api/core/api/dto/dto.ai.prompt.template.go`

**Thay đổi**:
```go
type AIPromptTemplateCreateInput struct {
    // ...
    Provider *AIPromptTemplateProviderInput `json:"provider,omitempty" transform:"nested_struct"`
    // ...
}

type AIPromptTemplateUpdateInput struct {
    // ...
    Provider *AIPromptTemplateProviderInput `json:"provider,omitempty" transform:"nested_struct"`
    // ...
}
```

### 4.3. AIProviderProfileCreateInput và UpdateInput

**File**: `api/core/api/dto/dto.ai.provider.profile.go`

**Thay đổi**:
```go
type AIProviderProfileCreateInput struct {
    // ...
    Config *AIConfigInput `json:"config,omitempty" transform:"nested_struct"`
    // ...
}

type AIProviderProfileUpdateInput struct {
    // ...
    Config *AIConfigInput `json:"config,omitempty" transform:"nested_struct"`
    // ...
}
```

---

## 5. Xóa Override Không Cần Thiết

### 5.1. AIRunHandler.InsertOne

**File**: `api/core/api/handler/handler.ai.run.go`

**Trước**: Override method với ~60 dòng code, chỉ copy logic BaseHandler

**Sau**: Xóa hoàn toàn, dùng `BaseHandler.InsertOne` trực tiếp

**Xóa import không sử dụng**:
- `meta_commerce/core/common`
- `github.com/gofiber/fiber/v3`
- `go.mongodb.org/mongo-driver/bson/primitive`

### 5.2. AIStepRunHandler.InsertOne

**File**: `api/core/api/handler/handler.ai.step.run.go`

**Trước**: Override method với ~60 dòng code, chỉ copy logic BaseHandler

**Sau**: Xóa hoàn toàn, dùng `BaseHandler.InsertOne` trực tiếp

**Xóa import không sử dụng**:
- `meta_commerce/core/common`
- `github.com/gofiber/fiber/v3`
- `go.mongodb.org/mongo-driver/bson/primitive`

### 5.3. AIPromptTemplateHandler.InsertOne và UpdateOne

**File**: `api/core/api/handler/handler.ai.prompt.template.go`

**Trước**: 
- `InsertOne`: ~110 dòng code, map nested struct Provider manual
- `UpdateOne`: ~100 dòng code, map nested struct Provider manual

**Sau**: Xóa hoàn toàn, dùng `BaseHandler.InsertOne` và `BaseHandler.UpdateOne` trực tiếp

**Lý do**: Nested struct Provider đã được xử lý tự động bởi `transform:"nested_struct"`

### 5.4. AIProviderProfileHandler.InsertOne và UpdateOne

**File**: `api/core/api/handler/handler.ai.provider.profile.go`

**Trước**: 
- `InsertOne`: ~90 dòng code, map nested struct Config manual
- `UpdateOne`: ~90 dòng code, map nested struct Config manual

**Sau**: 
- `InsertOne`: Vẫn giữ lại (user đã revert) - có thể xóa sau khi test
- `UpdateOne`: Xóa hoàn toàn, dùng `BaseHandler.UpdateOne` trực tiếp

**Lý do**: Nested struct Config đã được xử lý tự động bởi `transform:"nested_struct"`

---

## 6. Flow Hoạt Động Mới

### 6.1. InsertOne Flow

```
1. Request → ParseRequestBody → DTO (CreateInput)
2. Validate DTO với struct tag (validate, exists, etc.)
3. Transform DTO → Model:
   - Duyệt qua các field trong DTO
   - Nếu có transform:"nested_struct":
     → Gọi transformNestedStruct() (recursive)
     → Map nested struct từ DTO sang Model
   - Nếu có transform tag khác:
     → Apply transform (ObjectID conversion, default values, etc.)
   - Nếu không có transform tag:
     → Copy trực tiếp nếu type tương thích
4. Xử lý ownerOrganizationId (BaseHandler logic)
5. Insert vào database (BaseService.InsertOne)
```

### 6.2. UpdateOne Flow

```
1. Request → ParseRequestBody → DTO (UpdateInput)
2. Validate DTO với struct tag
3. Transform DTO → Model (tương tự InsertOne, hỗ trợ nested struct)
4. Convert Model → UpdateData với $set operator
5. Xử lý ownerOrganizationId (BaseHandler logic)
6. Update database (BaseService.UpdateOne)
```

### 6.3. Nested Struct Transform Flow

```
1. Kiểm tra field có transform:"nested_struct"
2. Xử lý pointer (nil → zero value)
3. Tạo instance của target type
4. Duyệt qua các field của nested struct:
   - Nếu có transform:"nested_struct" → recursive
   - Nếu có transform tag khác → apply transform
   - Nếu không có → copy trực tiếp
5. Return transformed nested struct
```

---

## 7. Kết Quả

### 7.1. Code Reduction

- **AIRunHandler**: Giảm ~60 dòng code
- **AIStepRunHandler**: Giảm ~60 dòng code
- **AIPromptTemplateHandler**: Giảm ~210 dòng code (InsertOne + UpdateOne)
- **AIProviderProfileHandler**: Giảm ~90 dòng code (UpdateOne)

**Tổng giảm**: ~420 dòng code

### 7.2. Tính Năng Mới

1. **Nested struct mapping tự động**: Không cần map manual
2. **Foreign key validation tự động**: Validate ObjectID tồn tại trong collection
3. **Transform tag hỗ trợ recursive**: Có thể nested nhiều level
4. **UpdateOne hỗ trợ nested struct**: Có thể update nested struct

### 7.3. Lợi Ích

1. **Giảm code duplication**: Logic tập trung trong BaseHandler
2. **Dễ maintain**: Chỉ cần sửa ở một nơi
3. **Consistency**: Tất cả handlers dùng cùng cơ chế
4. **Type safety**: Transform tag đảm bảo type conversion đúng
5. **Validation tự động**: Foreign key validation tự động

---

## 8. Ví Dụ Sử Dụng

### 8.1. DTO với Nested Struct

```go
type AIPromptTemplateCreateInput struct {
    Name     string                          `json:"name" validate:"required"`
    Provider *AIPromptTemplateProviderInput  `json:"provider,omitempty" transform:"nested_struct"`
}

type AIPromptTemplateProviderInput struct {
    ProfileID string        `json:"profileId,omitempty" validate:"omitempty,exists=ai_provider_profiles" transform:"str_objectid_ptr,optional"`
    Config    *AIConfigInput `json:"config,omitempty" transform:"nested_struct"`
}

type AIConfigInput struct {
    Model          string                 `json:"model,omitempty"`
    Temperature    *float64               `json:"temperature,omitempty"`
    MaxTokens      *int                   `json:"maxTokens,omitempty"`
    ProviderConfig map[string]interface{} `json:"providerConfig,omitempty"`
    PricingConfig  map[string]interface{} `json:"pricingConfig,omitempty"`
}
```

### 8.2. Handler Không Cần Override

```go
// Trước: Cần override InsertOne để map nested struct
func (h *AIPromptTemplateHandler) InsertOne(c fiber.Ctx) error {
    // ... 110 dòng code map manual
}

// Sau: Không cần override, dùng BaseHandler trực tiếp
// BaseHandler.InsertOne tự động xử lý nested struct với transform:"nested_struct"
```

---

## 9. Lưu Ý

1. **Recursive depth**: Transform nested struct hỗ trợ recursive, nhưng cần đảm bảo không có circular reference
2. **Performance**: Foreign key validation query database, có thể ảnh hưởng performance nếu validate nhiều field
3. **Error handling**: Transform nested struct có thể fail nếu type không tương thích
4. **Optional fields**: Nested struct có thể là nil/optional, cần xử lý đúng

---

## 10. Tương Lai

Có thể mở rộng thêm:
1. **exists_oneof**: Validate ObjectID tồn tại trong một trong các collections
2. **exists_with_type**: Validate ObjectID tồn tại và có field = value
3. **Transform tag cho array/slice**: Hỗ trợ transform array của nested struct
4. **Custom transform function**: Cho phép register custom transform function
