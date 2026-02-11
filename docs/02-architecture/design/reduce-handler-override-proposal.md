# Đề Xuất Giảm Handler Override Bằng Struct Tag

## Mục Tiêu
Giảm thiểu các handler override để tránh phá vỡ logic chung, tăng tính nhất quán và dễ bảo trì.

## Phân Tích Các Override Hiện Tại

### 1. **Validation** (CÓ THỂ DÙNG STRUCT TAG)
**Vấn đề:**
- Validate provider type: "openai", "anthropic", "google", "cohere", "custom"
- Validate status: "active", "inactive", "archived"
- Validate step type: "GENERATE", "JUDGE", "STEP_GENERATION"
- Validate command type: "START_WORKFLOW", "EXECUTE_STEP"

**Giải pháp:** Dùng `validate:"oneof=..."` tag

### 2. **Default Values** (CÓ THỂ DÙNG STRUCT TAG)
**Vấn đề:**
- Set default Status = "active"
- Set default Status = "pending" (cho workflow run, step run)

**Giải pháp:** Dùng `transform:"string,default=active"` tag (đã có)

### 3. **Nested Struct Mapping** (KHÓ DÙNG STRUCT TAG - CẦN MỞ RỘNG)
**Vấn đề:**
- Map `dto.AIConfigInput` → `models.AIConfig`
- Map `dto.AIPromptTemplateProviderInput` → `models.AIPromptTemplateProvider`
- Map nested arrays (workflow steps)

**Giải pháp:** Mở rộng transform tag hoặc tạo post-transform hook

### 4. **Business Logic Validation** (KHÓ DÙNG STRUCT TAG)
**Vấn đề:**
- Validate RootRefID tồn tại trong database
- Validate Step.ParentLevel match với RootRefType
- Validate schema matching

**Giải pháp:** Giữ override nhưng đơn giản hóa, chỉ giữ logic nghiệp vụ phức tạp

## Đề Xuất Giải Pháp

### Phương Án 1: Dùng Validate Tag Cho Enum (ĐƠN GIẢN - KHUYẾN NGHỊ)

#### 1.1. Thêm Custom Validator Cho Enum

```go
// api/internal/global/validator.go

// RegisterEnumValidator đăng ký validator cho enum values
func RegisterEnumValidator(name string, values []string) error {
    return Validate.RegisterValidation(name, func(fl validator.FieldLevel) bool {
        value := fl.Field().String()
        for _, v := range values {
            if value == v {
                return true
            }
        }
        return false
    })
}

// Hoặc dùng oneof tag có sẵn (đơn giản hơn)
// validate:"oneof=openai anthropic google cohere custom"
```

#### 1.2. Cập Nhật DTOs

```go
// dto.ai.provider.profile.go
type AIProviderProfileCreateInput struct {
    Provider string `json:"provider" validate:"required,oneof=openai anthropic google cohere custom"`
    Status   string `json:"status,omitempty" transform:"string,default=active" validate:"omitempty,oneof=active inactive archived"`
    // ...
}

// dto.ai.step.go
type AIStepCreateInput struct {
    Type   string `json:"type" validate:"required,oneof=GENERATE JUDGE STEP_GENERATION"`
    Status string `json:"status,omitempty" transform:"string,default=active" validate:"omitempty,oneof=active archived draft"`
    // ...
}
```

**Lợi ích:**
- ✅ Loại bỏ validation code trong handler
- ✅ Validation tự động trong BaseHandler
- ✅ Dễ bảo trì, thêm enum value chỉ cần sửa DTO

### Phương Án 2: Mở Rộng Transform Tag Cho Nested Struct (PHỨC TẠP)

#### 2.1. Thêm Transform Type Cho Nested Struct

```go
// api/internal/utility/data.transform.go

// Thêm transform type mới:
// transform:"nested_struct,type=AIConfig" - Map nested struct tự động

// Trong TransformFieldValue:
case "nested_struct":
    return transformNestedStruct(value, config, targetFieldType)
```

#### 2.2. Cập Nhật DTOs

```go
// dto.ai.provider.profile.go
type AIProviderProfileCreateInput struct {
    Config *AIConfigInput `json:"config,omitempty" transform:"nested_struct,type=AIConfig"`
    // ...
}
```

**Nhược điểm:**
- ❌ Phức tạp, cần xử lý reflection sâu
- ❌ Khó debug khi có lỗi
- ❌ Không linh hoạt bằng manual mapping

**Khuyến nghị:** KHÔNG nên làm, giữ override cho nested struct mapping

### Phương Án 3: Post-Transform Hook (CÂN BẰNG)

#### 3.1. Thêm Interface Cho Post-Transform

```go
// api/internal/api/handler/handler.base.go

// PostTransformHook interface cho handler muốn transform sau khi BaseHandler transform
type PostTransformHook[T any, CreateInput any, UpdateInput any] interface {
    PostTransformCreate(ctx context.Context, input *CreateInput, model *T) error
    PostTransformUpdate(ctx context.Context, input *UpdateInput, model *T) error
}
```

#### 3.2. Cập Nhật BaseHandler

```go
// Trong InsertOne, sau khi transformCreateInputToModel:
if hook, ok := h.(PostTransformHook[T, CreateInput, UpdateInput]); ok {
    if err := hook.PostTransformCreate(ctx, &input, model); err != nil {
        // Handle error
    }
}
```

**Lợi ích:**
- ✅ Tách biệt logic transform khỏi CRUD logic
- ✅ Vẫn đảm bảo logic cơ bản (timestamps, ownerOrganizationId) được BaseHandler xử lý
- ✅ Handler chỉ cần implement hook, không override toàn bộ method

**Nhược điểm:**
- ❌ Vẫn cần code trong handler (nhưng ít hơn)
- ❌ Phức tạp hơn struct tag

## Kế Hoạch Triển Khai (Khuyến Nghị)

### Bước 1: Validation với Struct Tag (ƯU TIÊN CAO)
1. ✅ Dùng `validate:"oneof=..."` cho tất cả enum validation
2. ✅ Loại bỏ validation code trong handlers
3. ✅ Test kỹ để đảm bảo error message rõ ràng

**Kết quả:** Giảm ~50% code validation trong handlers

### Bước 2: Default Values với Transform Tag (ĐÃ CÓ)
1. ✅ Đã có `transform:"string,default=active"`
2. ✅ Đảm bảo tất cả default values dùng transform tag

**Kết quả:** Đã giảm code set default values

### Bước 3: Nested Struct Mapping (GIỮ OVERRIDE - ĐƠN GIẢN HÓA)
1. ✅ Giữ override nhưng đơn giản hóa
2. ✅ Chỉ map nested struct, không làm gì khác
3. ✅ Đảm bảo logic cơ bản (timestamps, ownerOrganizationId) vẫn do BaseHandler xử lý

**Kết quả:** Code override ngắn gọn, dễ đọc

### Bước 4: Business Logic Validation (GIỮ OVERRIDE)
1. ✅ Giữ override cho validation nghiệp vụ phức tạp (RootRefID, ParentLevel matching)
2. ✅ Đảm bảo comment rõ ràng về lý do override

**Kết quả:** Logic nghiệp vụ phức tạp được xử lý đúng

## Ví Dụ Cụ Thể

### Trước (Override nhiều):

```go
func (h *AIProviderProfileHandler) InsertOne(c fiber.Ctx) error {
    // ... parse input ...
    
    // Validate provider type
    validProviders := []string{"openai", "anthropic", ...}
    providerValid := false
    for _, validProvider := range validProviders {
        if input.Provider == validProvider {
            providerValid = true
            break
        }
    }
    if !providerValid {
        // Error...
    }
    
    // Validate status
    validStatuses := []string{"active", "inactive", "archived"}
    // ... validation code ...
    
    // Set default status
    if input.Status == "" {
        input.Status = "active"
    }
    
    // Transform DTO sang Model
    model, err := h.transformCreateInputToModel(&input)
    
    // Map nested struct Config
    if input.Config != nil {
        model.Config = &models.AIConfig{...}
    }
    
    // Set timestamps
    now := time.Now().UnixMilli()
    model.CreatedAt = now
    model.UpdatedAt = now
    
    // Set ownerOrganizationId
    // ... code ...
    
    // Insert
    data, err := h.BaseService.InsertOne(ctx, *model)
    // ...
}
```

### Sau (Giảm override):

```go
// DTO
type AIProviderProfileCreateInput struct {
    Provider string `json:"provider" validate:"required,oneof=openai anthropic google cohere custom"`
    Status   string `json:"status,omitempty" transform:"string,default=active" validate:"omitempty,oneof=active inactive archived"`
    Config   *AIConfigInput `json:"config,omitempty" transform:"nested_struct,type=AIConfig"` // Nếu implement nested struct transform
}

// Handler - chỉ map nested struct
func (h *AIProviderProfileHandler) InsertOne(c fiber.Ctx) error {
    return h.SafeHandler(c, func() error {
        var input dto.AIProviderProfileCreateInput
        if err := h.ParseRequestBody(c, &input); err != nil {
            // Error handling
        }
        
        // Transform DTO sang Model (validation và default values tự động)
        model, err := h.transformCreateInputToModel(&input)
        if err != nil {
            // Error handling
        }
        
        // Chỉ map nested struct (nếu không dùng transform tag)
        if input.Config != nil {
            model.Config = &models.AIConfig{
                Model:          input.Config.Model,
                Temperature:    input.Config.Temperature,
                MaxTokens:      input.Config.MaxTokens,
                ProviderConfig: input.Config.ProviderConfig,
                PricingConfig:  input.Config.PricingConfig,
            }
        }
        
        // Gọi BaseHandler.InsertOne để xử lý timestamps, ownerOrganizationId
        return h.BaseHandler.InsertOne(c)
    })
}
```

## Kết Luận

**Khuyến nghị:**
1. ✅ **Dùng validate tag cho enum** - Đơn giản, hiệu quả, giảm nhiều code
2. ✅ **Dùng transform tag cho default values** - Đã có, tiếp tục dùng
3. ⚠️ **Giữ override cho nested struct mapping** - Nhưng đơn giản hóa, chỉ map struct
4. ⚠️ **Giữ override cho business logic validation** - Nhưng comment rõ ràng

**Kết quả mong đợi:**
- Giảm ~60-70% code trong handlers override
- Tăng tính nhất quán
- Dễ bảo trì hơn
- Vẫn đảm bảo logic cơ bản được BaseHandler xử lý
