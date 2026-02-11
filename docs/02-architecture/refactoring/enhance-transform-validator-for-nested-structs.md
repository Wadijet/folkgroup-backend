# Mở Rộng Transform Tag và Validator - Hỗ Trợ Nested Struct và Foreign Key Validation

## Vấn Đề Hiện Tại

Một số handlers phải override `InsertOne`/`UpdateOne` chỉ để:
1. **Map nested struct** (Provider.Config, Config) - Transform tag hiện tại không hỗ trợ nested struct
2. **Validate foreign key** (RootRefID, StepID, WorkflowID) - Validator hiện tại không có custom validator để check quan hệ

## Đề Xuất Giải Pháp

### 1. Mở Rộng Transform Tag - Hỗ Trợ Nested Struct

#### 1.1. Thêm Transform Type `nested_struct`

**Format mới:**
```go
transform:"nested_struct" // Tự động map nested struct từ DTO sang Model (recursive)
```

**Ví dụ:**
```go
// DTO
type AIPromptTemplateCreateInput struct {
    Provider *AIPromptTemplateProviderInput `json:"provider,omitempty" transform:"nested_struct"`
}

type AIPromptTemplateProviderInput struct {
    ProfileID string        `json:"profileId,omitempty" transform:"str_objectid_ptr,optional"`
    Config    *AIConfigInput `json:"config,omitempty" transform:"nested_struct"`
}
```

**Implementation:**
- Trong `transformCreateInputToModel`, nếu field có type là struct/pointer struct và có `transform:"nested_struct"`, tự động map recursive
- Map các field con với transform tag của chúng

#### 1.2. Cập Nhật `transformCreateInputToModel` để Hỗ Trợ Recursive

```go
// transformCreateInputToModel - mở rộng để hỗ trợ nested struct
func (h *BaseHandler[T, CreateInput, UpdateInput]) transformCreateInputToModel(input *CreateInput) (*T, error) {
    // ... existing code ...
    
    // Nếu field là struct/pointer struct và có transform:"nested_struct"
    if transformTag == "nested_struct" {
        // Recursive transform nested struct
        nestedModel, err := h.transformNestedStruct(inputField, modelFieldVal.Type())
        if err != nil {
            return nil, err
        }
        modelFieldVal.Set(nestedModel)
        continue
    }
    
    // ... existing code ...
}

// transformNestedStruct transform nested struct từ DTO sang Model (recursive)
func (h *BaseHandler[T, CreateInput, UpdateInput]) transformNestedStruct(inputVal reflect.Value, targetType reflect.Type) (reflect.Value, error) {
    // Tạo instance của target type
    targetVal := reflect.New(targetType).Elem()
    
    // Duyệt qua các field của nested struct
    inputType := inputVal.Type()
    for i := 0; i < inputVal.NumField(); i++ {
        inputField := inputVal.Field(i)
        inputFieldType := inputType.Field(i)
        
        // Kiểm tra transform tag
        transformTag := inputFieldType.Tag.Get("transform")
        if transformTag != "" {
            // Apply transform (có thể recursive nếu là nested_struct)
            // ...
        } else {
            // Copy trực tiếp nếu type tương thích
            // ...
        }
    }
    
    return targetVal, nil
}
```

### 2. Thêm Custom Validator - Foreign Key Validation

#### 2.1. Tạo Custom Validator `exists`

**Format:**
```go
validate:"exists=<collection_name>" // Kiểm tra ObjectID tồn tại trong collection
```

**Ví dụ:**
```go
type AIWorkflowRunCreateInput struct {
    WorkflowID  string `json:"workflowId" validate:"required,exists=ai_workflows" transform:"str_objectid"`
    RootRefID   string `json:"rootRefId,omitempty" validate:"omitempty,exists=content_nodes" transform:"str_objectid_ptr,optional"`
}

type AIPromptTemplateCreateInput struct {
    Provider *AIPromptTemplateProviderInput `json:"provider,omitempty" transform:"nested_struct"`
}

type AIPromptTemplateProviderInput struct {
    ProfileID string `json:"profileId,omitempty" validate:"omitempty,exists=ai_provider_profiles" transform:"str_objectid_ptr,optional"`
}
```

**Implementation:**
```go
// api/internal/global/validator.go

// RegisterExistsValidator đăng ký custom validator để check foreign key
func RegisterExistsValidator() {
    _ = Validate.RegisterValidation("exists", validateExists)
}

// validateExists kiểm tra ObjectID tồn tại trong collection
func validateExists(fl validator.FieldLevel) bool {
    value := fl.Field()
    
    // Lấy collection name từ param
    collectionName := fl.Param()
    if collectionName == "" {
        return false
    }
    
    // Convert value sang ObjectID
    var objID primitive.ObjectID
    switch v := value.Interface().(type) {
    case string:
        if v == "" {
            return true // Empty string = optional, skip validation
        }
        var err error
        objID, err = primitive.ObjectIDFromHex(v)
        if err != nil {
            return false
        }
    case primitive.ObjectID:
        objID = v
    case *primitive.ObjectID:
        if v == nil {
            return true // Nil pointer = optional, skip validation
        }
        objID = *v
    default:
        return false
    }
    
    // Query database để check tồn tại
    // Lấy collection từ global registry
    collection, exist := global.RegistryCollections.Get(collectionName)
    if !exist {
        return false
    }
    
    // Query với filter _id = objID
    ctx := context.Background()
    count, err := collection.CountDocuments(ctx, bson.M{"_id": objID})
    if err != nil {
        return false
    }
    
    return count > 0
}
```

#### 2.2. Tạo Custom Validator `exists_oneof` (Cho Multiple Collections)

**Format:**
```go
validate:"exists_oneof=<collection1>,<collection2>,..." // Kiểm tra ObjectID tồn tại trong một trong các collections
```

**Ví dụ:**
```go
type AIWorkflowRunCreateInput struct {
    RootRefID   string `json:"rootRefId,omitempty" validate:"omitempty,exists_oneof=content_nodes,draft_content_nodes" transform:"str_objectid_ptr,optional"`
}
```

### 3. Tạo Custom Validator `exists_with_type` (Cho Validation Phức Tạp)

**Format:**
```go
validate:"exists_with_type=<collection>:<field>=<value>" // Kiểm tra ObjectID tồn tại và có field = value
```

**Ví dụ:**
```go
type AIWorkflowRunCreateInput struct {
    RootRefID   string `json:"rootRefId,omitempty" validate:"omitempty,exists_with_type=content_nodes:type=layer" transform:"str_objectid_ptr,optional"`
    RootRefType string `json:"rootRefType,omitempty"` // "layer", "stp", etc.
}
```

**Lưu ý:** Validator này phức tạp hơn, có thể cần validate trong handler nếu logic quá phức tạp.

---

## Kết Quả Sau Khi Mở Rộng

### Handlers Có Thể Đơn Giản Hóa

#### 1. AIPromptTemplateHandler

**Trước:**
```go
func (h *AIPromptTemplateHandler) InsertOne(c fiber.Ctx) error {
    // ... parse input ...
    
    // Map nested struct Provider từ DTO sang Model (manual)
    if input.Provider != nil {
        provider := &models.AIPromptTemplateProvider{}
        if input.Provider.ProfileID != "" {
            profileID, err := primitive.ObjectIDFromHex(input.Provider.ProfileID)
            // ... manual conversion ...
        }
        if input.Provider.Config != nil {
            provider.Config = &models.AIConfig{
                Model: input.Provider.Config.Model,
                // ... manual mapping ...
            }
        }
        model.Provider = provider
    }
    
    // ... rest of logic ...
}
```

**Sau:**
```go
// KHÔNG CẦN OVERRIDE - Dùng BaseHandler.InsertOne trực tiếp
// DTO đã có transform:"nested_struct" và validate:"exists=..."
```

**DTO:**
```go
type AIPromptTemplateCreateInput struct {
    Provider *AIPromptTemplateProviderInput `json:"provider,omitempty" transform:"nested_struct"`
}

type AIPromptTemplateProviderInput struct {
    ProfileID string        `json:"profileId,omitempty" validate:"omitempty,exists=ai_provider_profiles" transform:"str_objectid_ptr,optional"`
    Config    *AIConfigInput `json:"config,omitempty" transform:"nested_struct"`
}
```

#### 2. AIProviderProfileHandler

**Trước:**
```go
func (h *AIProviderProfileHandler) InsertOne(c fiber.Ctx) error {
    // ... parse input ...
    
    // Map nested struct Config từ DTO sang Model (manual)
    if input.Config != nil {
        model.Config = &models.AIConfig{
            Model: input.Config.Model,
            // ... manual mapping ...
        }
    }
    
    // ... rest of logic ...
}
```

**Sau:**
```go
// KHÔNG CẦN OVERRIDE - Dùng BaseHandler.InsertOne trực tiếp
```

**DTO:**
```go
type AIProviderProfileCreateInput struct {
    Config *AIConfigInput `json:"config,omitempty" transform:"nested_struct"`
}
```

#### 3. AIWorkflowRunHandler

**Trước:**
```go
func (h *AIWorkflowRunHandler) InsertOne(c fiber.Ctx) error {
    // ... parse input ...
    
    // Validate RootRefID phức tạp (query database)
    if model.RootRefID != nil && model.RootRefType != "" {
        // Query content node service và draft content node service
        // Validate tồn tại, đúng type, đã commit/approve
    }
    
    // ... rest of logic ...
}
```

**Sau:**
```go
// Có thể đơn giản hóa một phần với validator
// Nhưng logic phức tạp (check đã commit/approve) vẫn cần trong handler
```

**DTO:**
```go
type AIWorkflowRunCreateInput struct {
    WorkflowID  string `json:"workflowId" validate:"required,exists=ai_workflows" transform:"str_objectid"`
    RootRefID   string `json:"rootRefId,omitempty" validate:"omitempty,exists_oneof=content_nodes,draft_content_nodes" transform:"str_objectid_ptr,optional"`
    RootRefType string `json:"rootRefType,omitempty" validate:"omitempty,oneof=layer stp"`
}
```

**Lưu ý:** Logic check "đã commit/approve" vẫn cần trong handler vì quá phức tạp.

#### 4. AIWorkflowCommandHandler

**Trước:**
```go
func (h *AIWorkflowCommandHandler) InsertOne(c fiber.Ctx) error {
    // Validate CommandType và StepID/WorkflowID dựa trên CommandType
    if input.CommandType == models.AIWorkflowCommandTypeStartWorkflow {
        if input.WorkflowID == "" {
            // Error
        }
    } else if input.CommandType == models.AIWorkflowCommandTypeExecuteStep {
        if input.StepID == "" {
            // Error
        }
    }
    
    // Validate StepID và ParentLevel matching
    // Validate RootRefID phức tạp
}
```

**Sau:**
```go
// Có thể đơn giản hóa một phần với validator
// Nhưng cross-field validation (CommandType → StepID/WorkflowID) vẫn cần trong handler
```

**DTO:**
```go
type AIWorkflowCommandCreateInput struct {
    CommandType string `json:"commandType" validate:"required,oneof=START_WORKFLOW EXECUTE_STEP"`
    WorkflowID  string `json:"workflowId,omitempty" validate:"omitempty,exists=ai_workflows" transform:"str_objectid,optional"`
    StepID      string `json:"stepId,omitempty" validate:"omitempty,exists=ai_steps" transform:"str_objectid,optional"`
    RootRefID   string `json:"rootRefId,omitempty" validate:"omitempty,exists_oneof=content_nodes,draft_content_nodes" transform:"str_objectid_ptr,optional"`
}
```

**Lưu ý:** Cross-field validation (CommandType → StepID/WorkflowID) vẫn cần trong handler, nhưng có thể đơn giản hóa.

---

## Kết Luận

### Override Có Thể Loại Bỏ Hoàn Toàn

1. **AIPromptTemplateHandler.InsertOne/UpdateOne** ✅
   - Sau khi có `transform:"nested_struct"` và `validate:"exists=ai_provider_profiles"`
   - **Giảm**: ~100 dòng code

2. **AIProviderProfileHandler.InsertOne/UpdateOne** ✅
   - Sau khi có `transform:"nested_struct"`
   - **Giảm**: ~80 dòng code

### Override Có Thể Đơn Giản Hóa (Nhưng Vẫn Cần)

1. **AIWorkflowRunHandler.InsertOne** ⚠️
   - Có thể dùng `validate:"exists=..."` cho RootRefID
   - Nhưng logic check "đã commit/approve" vẫn cần trong handler
   - **Giảm**: ~30 dòng code (chỉ validate tồn tại)

2. **AIWorkflowCommandHandler.InsertOne** ⚠️
   - Có thể dùng `validate:"exists=..."` cho StepID, WorkflowID, RootRefID
   - Nhưng cross-field validation vẫn cần trong handler
   - **Giảm**: ~20 dòng code (chỉ validate tồn tại)

### Override Vẫn Cần Giữ

1. **AIStepHandler.InsertOne** ✅ - Validate schema phức tạp
2. **AIWorkflowHandler.InsertOne** ✅ - Convert nested structures (Steps, DefaultPolicy)
3. **DraftApprovalHandler.InsertOne** ✅ - Validate cross-field
4. **OrganizationHandler.InsertOne** ✅ - Tính Path và Level
5. **NotificationChannelHandler.InsertOne** ✅ - Validate uniqueness phức tạp
6. **NotificationRoutingHandler.InsertOne** ✅ - Validate uniqueness phức tạp

---

## Lợi Ích

1. **Giảm code duplication**: ~180 dòng code (AIPromptTemplateHandler + AIProviderProfileHandler)
2. **Tự động hóa**: Nested struct mapping và foreign key validation tự động
3. **Dễ maintain**: Logic tập trung trong transform tag và validator
4. **Consistency**: Tất cả handlers dùng cùng cơ chế

---

## Implementation Plan

### Phase 1: Mở Rộng Transform Tag
1. Thêm `transform:"nested_struct"` support
2. Cập nhật `transformCreateInputToModel` để recursive
3. Test với AIPromptTemplateHandler và AIProviderProfileHandler

### Phase 2: Thêm Custom Validator
1. Implement `validate:"exists=<collection>"`
2. Implement `validate:"exists_oneof=<collection1>,<collection2>"`
3. Test với AIWorkflowRunHandler và AIWorkflowCommandHandler

### Phase 3: Refactor Handlers
1. Xóa override trong AIPromptTemplateHandler
2. Xóa override trong AIProviderProfileHandler
3. Đơn giản hóa override trong AIWorkflowRunHandler và AIWorkflowCommandHandler
