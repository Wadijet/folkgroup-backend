# Rà Soát Toàn Bộ CRUD Override - Handler và Service

## Tổng Quan

Tài liệu này rà soát tất cả các override của CRUD methods trong handlers và services, phân tích lý do tại sao phải override và có thể dùng CRUD base không.

**Mục tiêu**: Đảm bảo mỗi override đều có lý do rõ ràng và không thể thay thế bằng CRUD base.

---

## 1. Handler Overrides

### 1.1. InsertOne Overrides

#### 1.1.1. AIProviderProfileHandler.InsertOne

**File**: `api/core/api/handler/handler.ai.provider.profile.go`

**Lý do override**:
- **Map nested struct Config**: Transform tag không hỗ trợ nested struct với nhiều level (Config.Model, Config.Temperature, etc.)
- **Logic đặc biệt**: Cần map manual từ `dto.AIConfigInput` sang `models.AIConfig`

**Có thể dùng CRUD base không?**
- ❌ **KHÔNG** - Transform tag `nested_struct` đã được thêm nhưng handler này vẫn override để map manual
- **Đề xuất**: Có thể xóa override và dùng `transform:"nested_struct"` trong DTO (giống như AIPromptTemplateHandler đã làm)

**Kết luận**: ⚠️ **CÓ THỂ XÓA** - Nên dùng `transform:"nested_struct"` trong DTO

---

#### 1.1.2. AIWorkflowCommandHandler.InsertOne

**File**: `api/core/api/handler/handler.ai.workflow.command.go`

**Lý do override**:
1. **Conditional validation phức tạp**:
   - Validate WorkflowID bắt buộc khi CommandType = START_WORKFLOW
   - Validate StepID bắt buộc khi CommandType = EXECUTE_STEP
   - Không thể dùng struct tag (conditional validation)

2. **Cross-field validation**:
   - Validate StepID và ParentLevel matching (nếu CommandType = EXECUTE_STEP)
   - Kiểm tra StepID tồn tại trong database
   - Kiểm tra Step.ParentLevel có match với RootRefType không (level matching)

3. **Cross-collection validation**:
   - Validate RootRefID tồn tại trong production hoặc draft
   - Kiểm tra RootRefType đúng với type của RootRefID
   - Kiểm tra RootRefID đã được commit (production) hoặc là draft đã được approve

**Có thể dùng CRUD base không?**
- ❌ **KHÔNG** - Logic validation phức tạp với conditional, cross-field, và cross-collection validation
- **Kết luận**: ✅ **CẦN GIỮ** - Logic nghiệp vụ phức tạp, không thể thay thế bằng CRUD base

---

#### 1.1.3. DraftApprovalHandler.InsertOne

**File**: `api/core/api/handler/handler.content.draft.approval.go`

**Lý do override**:
1. **Cross-field validation**:
   - Validate "ít nhất một target" (workflowRunID, draftNodeID, draftVideoID, hoặc draftPublicationID)
   - Không thể dùng struct tag (cross-field validation)

2. **Logic nghiệp vụ đặc biệt**:
   - Set RequestedBy từ context (user_id)
   - Set RequestedAt = timestamp hiện tại
   - Convert nhiều optional ObjectID fields

**Có thể dùng CRUD base không?**
- ❌ **KHÔNG** - Cross-field validation "ít nhất một target" không thể dùng struct tag
- **Kết luận**: ✅ **CẦN GIỮ** - Cross-field validation phức tạp

---

#### 1.1.4. AIStepHandler.InsertOne

**File**: `api/core/api/handler/handler.ai.step.go`

**Lý do override**:
1. **Business logic validation**:
   - Validate input/output schema phải match với standard schema cho từng step type
   - Đảm bảo mapping chính xác giữa output của step này và input của step tiếp theo
   - Cho phép mở rộng thêm fields nhưng không được thiếu required fields

**Có thể dùng CRUD base không?**
- ❌ **KHÔNG** - Schema validation phức tạp, cần gọi `models.ValidateStepSchema()`
- **Kết luận**: ✅ **CẦN GIỮ** - Business logic validation phức tạp

---

#### 1.1.5. AIWorkflowRunHandler.InsertOne

**File**: `api/core/api/handler/handler.ai.workflow.run.go`

**Lý do override**:
1. **Set default values cho business logic**:
   - CurrentStepIndex = 0 (business logic - bắt đầu từ step đầu tiên)
   - StepRunIDs = [] (business logic - chưa có step run nào)

2. **Cross-collection validation**:
   - Validate RootRefID tồn tại trong production hoặc draft
   - Kiểm tra RootRefType đúng với type của RootRefID
   - Kiểm tra RootRefID đã được commit (production) hoặc là draft đã được approve

**Có thể dùng CRUD base không?**
- ⚠️ **CÓ THỂ MỘT PHẦN**:
  - Set default values có thể dùng transform tag `transform:"int,default=0"` và `transform:"str_objectid_array,default=[]"`
  - Cross-collection validation vẫn cần override
- **Kết luận**: ⚠️ **CÓ THỂ ĐƠN GIẢN HÓA** - Chỉ giữ phần cross-collection validation, set default values có thể dùng transform tag

---

#### 1.1.6. AIWorkflowHandler.InsertOne

**File**: `api/core/api/handler/handler.ai.workflow.go`

**Lý do override**:
1. **Convert nested structures phức tạp**:
   - Steps: Convert từ `[]dto.AIWorkflowStepReferenceInput` sang `[]models.AIWorkflowStepReference`
   - Mỗi Step có Policy nested: Convert từ `dto.AIWorkflowStepPolicyInput` sang `models.AIWorkflowStepPolicy`
   - DefaultPolicy: Convert từ `dto.AIWorkflowStepPolicyInput` sang `*models.AIWorkflowStepPolicy`
   - Transform tag không hỗ trợ nested struct arrays và nested pointer structs

**Có thể dùng CRUD base không?**
- ❌ **KHÔNG** - Transform tag không hỗ trợ nested struct arrays
- **Kết luận**: ✅ **CẦN GIỮ** - Nested struct arrays không thể dùng transform tag

---

#### 1.1.7. OrganizationHandler.InsertOne

**File**: `api/core/api/handler/handler.auth.organization.go`

**Lý do override**:
1. **Logic nghiệp vụ phức tạp**:
   - Tính toán Path dựa trên parent.Path + "/" + code
   - Tính toán Level dựa trên Type và parent.Level
   - Query database để lấy parent organization
   - Validate Type và parent relationship

2. **Business rules**:
   - System: Level = -1, Path = "/" + code
   - Group: Level = 0, Path = "/" + code
   - Company: Level = parentLevel + 1
   - Team: Level = parentLevel + 1 (có thể là 4+)

**Có thể dùng CRUD base không?**
- ❌ **KHÔNG** - Logic tính toán Path/Level phức tạp, cần query parent từ database
- **Kết luận**: ✅ **CẦN GIỮ** - Logic nghiệp vụ phức tạp với tính toán dựa trên parent

---

#### 1.1.8. NotificationRoutingHandler.InsertOne

**File**: `api/core/api/handler/handler.notification.routing.go`

**Lý do override**:
1. **Validation uniqueness phức tạp**:
   - Kiểm tra đã có rule cho eventType và ownerOrganizationId chưa
   - Kiểm tra đã có rule cho domain và ownerOrganizationId chưa (nếu có domain)
   - Query database để check duplicate

2. **Logic nghiệp vụ đặc biệt**:
   - Chỉ check rules active (isActive = true)
   - Validate eventType bắt buộc

**Có thể dùng CRUD base không?**
- ❌ **KHÔNG** - Validation uniqueness cần query database, không thể dùng struct tag
- **Kết luận**: ✅ **CẦN GIỮ** - Validation uniqueness phức tạp với query database

---

#### 1.1.9. NotificationChannelHandler.InsertOne

**File**: `api/core/api/handler/handler.notification.channel.go`

**Lý do override**:
1. **Validation uniqueness rất phức tạp**:
   - Kiểm tra đã có channel với cùng name, channelType và ownerOrganizationId chưa
   - Kiểm tra duplicate recipients cho email (MongoDB $in operator)
   - Kiểm tra duplicate webhookUrl cho webhook
   - Kiểm tra duplicate botToken cho telegram
   - Query database nhiều lần cho từng loại channel

2. **Logic nghiệp vụ đặc biệt**:
   - Check tất cả channels (cả active và inactive) để tránh duplicate
   - Sử dụng MongoDB $in operator để check duplicate trong arrays

**Có thể dùng CRUD base không?**
- ❌ **KHÔNG** - Validation uniqueness rất phức tạp với nhiều điều kiện khác nhau tùy theo channelType
- **Kết luận**: ✅ **CẦN GIỮ** - Validation uniqueness rất phức tạp với query database nhiều lần

---

### 1.2. UpdateOne Overrides

**Không có handler nào override UpdateOne** - Tất cả đều dùng `BaseHandler.UpdateOne` với transform tag `nested_struct`

---

### 1.3. DeleteOne Overrides

**Không có handler nào override DeleteOne** - Tất cả đều dùng `BaseHandler.DeleteOne`

---

### 1.4. Find/FindOne Overrides

**Không có handler nào override Find/FindOne** - Tất cả đều dùng `BaseHandler.Find` và `BaseHandler.FindOne`

---

## 2. Service Overrides

### 2.1. InsertOne Overrides

#### 2.1.1. OrganizationShareService.InsertOne

**File**: `api/core/api/services/service.organization.share.go`

**Lý do override**:
1. **Validation nghiệp vụ**:
   - Validate ownerOrgID không được có trong ToOrgIDs

2. **Duplicate check với set comparison**:
   - So sánh ToOrgIDs không quan tâm thứ tự (set comparison)
   - So sánh PermissionNames không quan tâm thứ tự (set comparison)
   - Query tất cả shares của ownerOrg và so sánh thủ công

**Có thể dùng CRUD base không?**
- ❌ **KHÔNG** - Duplicate check với set comparison không thể dùng struct tag hoặc unique index
- **Kết luận**: ✅ **CẦN GIỮ** - Logic duplicate check phức tạp với set comparison

---

#### 2.1.2. DraftContentNodeService.InsertOne

**File**: `api/core/api/services/service.draft.content.node.go`

**Lý do override**:
1. **Cross-collection validation**:
   - Kiểm tra parent phải tồn tại và đã được commit (production) hoặc là draft đã được approve
   - Query parent trong production trước, nếu không có thì query trong draft

2. **Sequential level constraint validation**:
   - Validate Type và parent.Type theo sequential level constraint
   - Đảm bảo level hierarchy đúng (ví dụ: layer → stp → insight)

**Có thể dùng CRUD base không?**
- ❌ **KHÔNG** - Cross-collection validation và sequential level constraint không thể dùng struct tag
- **Kết luận**: ✅ **CẦN GIỮ** - Cross-collection validation và business logic phức tạp

---

### 2.2. UpdateOne Overrides

**Không có service nào override UpdateOne** - Tất cả đều dùng `BaseServiceMongoImpl.UpdateOne`

---

### 2.3. DeleteOne Overrides

#### 2.3.1. OrganizationService.DeleteOne và DeleteById

**File**: `api/core/api/services/service.auth.organization.go`

**Lý do override**:
1. **Validation trước khi xóa**:
   - Kiểm tra organization có children không (cascade delete protection)
   - Kiểm tra organization có users không
   - Kiểm tra organization có data liên quan không
   - Không cho phép xóa organization có dependencies

**Có thể dùng CRUD base không?**
- ❌ **KHÔNG** - Validation cascade delete protection cần query database để check dependencies
- **Kết luận**: ✅ **CẦN GIỮ** - Cascade delete protection logic

---

#### 2.3.2. RoleService.DeleteOne và DeleteById

**File**: `api/core/api/services/service.auth.role.go`

**Lý do override**:
1. **Validation trước khi xóa**:
   - Kiểm tra role có users không
   - Kiểm tra role có permissions không
   - Không cho phép xóa role có dependencies

**Có thể dùng CRUD base không?**
- ❌ **KHÔNG** - Validation cascade delete protection cần query database để check dependencies
- **Kết luận**: ✅ **CẦN GIỮ** - Cascade delete protection logic

---

#### 2.3.3. UserRoleService.DeleteOne và DeleteById

**File**: `api/core/api/services/service.auth.user_role.go`

**Lý do override**:
1. **Business rule validation**:
   - Kiểm tra không thể xóa user khỏi role Administrator nếu đó là user cuối cùng
   - Role Administrator phải có ít nhất một user
   - Đảm bảo business rule về Administrator role

**Có thể dùng CRUD base không?**
- ❌ **KHÔNG** - Business rule validation cần query database để check số lượng users trong role
- **Kết luận**: ✅ **CẦN GIỮ** - Business rule validation phức tạp

---

### 2.4. FindOne Overrides

#### 2.4.1. PcOrderService.FindOne

**File**: `api/core/api/services/service.pc.order.go`

**Lý do override**:
- **Signature khác**: `FindOne(ctx, id ObjectID)` thay vì `FindOne(ctx, filter, opts)`
- **Đơn giản hóa**: Wrapper method để tìm theo ID trực tiếp

**Có thể dùng CRUD base không?**
- ⚠️ **CÓ THỂ** - Chỉ là wrapper method, có thể dùng `BaseServiceMongoImpl.FindOneById()` trực tiếp
- **Kết luận**: ⚠️ **CÓ THỂ XÓA** - Chỉ là convenience method, không có logic đặc biệt

---

## 3. Tổng Kết

### 3.1. Handler Overrides - InsertOne

| Handler | Lý do Override | Có thể dùng CRUD base? | Kết luận |
|---------|----------------|------------------------|----------|
| AIProviderProfileHandler | Map nested struct Config | ⚠️ CÓ THỂ - Dùng `transform:"nested_struct"` | ⚠️ **CÓ THỂ XÓA** |
| AIWorkflowCommandHandler | Conditional validation, cross-field, cross-collection | ❌ KHÔNG | ✅ **CẦN GIỮ** |
| DraftApprovalHandler | Cross-field validation "ít nhất một target" | ❌ KHÔNG | ✅ **CẦN GIỮ** |
| AIStepHandler | Schema validation phức tạp | ❌ KHÔNG | ✅ **CẦN GIỮ** |
| AIWorkflowRunHandler | Set default values + cross-collection validation | ⚠️ MỘT PHẦN - Default values có thể dùng transform tag | ⚠️ **CÓ THỂ ĐƠN GIẢN HÓA** |
| AIWorkflowHandler | Nested struct arrays | ❌ KHÔNG | ✅ **CẦN GIỮ** |
| OrganizationHandler | Tính toán Path/Level dựa trên parent | ❌ KHÔNG | ✅ **CẦN GIỮ** |
| NotificationRoutingHandler | Validation uniqueness phức tạp | ❌ KHÔNG | ✅ **CẦN GIỮ** |
| NotificationChannelHandler | Validation uniqueness rất phức tạp | ❌ KHÔNG | ✅ **CẦN GIỮ** |

**Tổng cộng**: 9 handlers override InsertOne
- ✅ **CẦN GIỮ**: 7 handlers
- ⚠️ **CÓ THỂ XÓA/ĐƠN GIẢN HÓA**: 2 handlers (AIProviderProfileHandler, AIWorkflowRunHandler)

---

### 3.2. Service Overrides

#### 3.2.1. InsertOne Overrides

| Service | Lý do Override | Có thể dùng CRUD base? | Kết luận |
|---------|----------------|------------------------|----------|
| OrganizationShareService | Duplicate check với set comparison | ❌ KHÔNG | ✅ **CẦN GIỮ** |
| DraftContentNodeService | Cross-collection validation, sequential level constraint | ❌ KHÔNG | ✅ **CẦN GIỮ** |

**Tổng cộng**: 2 services override InsertOne
- ✅ **CẦN GIỮ**: 2 services

---

#### 3.2.2. DeleteOne/DeleteById Overrides

| Service | Lý do Override | Có thể dùng CRUD base? | Kết luận |
|---------|----------------|------------------------|----------|
| OrganizationService | Cascade delete protection | ❌ KHÔNG | ✅ **CẦN GIỮ** |
| RoleService | Cascade delete protection | ❌ KHÔNG | ✅ **CẦN GIỮ** |
| UserRoleService | Business rule: Administrator role phải có ít nhất 1 user | ❌ KHÔNG | ✅ **CẦN GIỮ** |

**Tổng cộng**: 3 services override DeleteOne/DeleteById
- ✅ **CẦN GIỮ**: 3 services

---

#### 3.2.3. FindOne Overrides

| Service | Lý do Override | Có thể dùng CRUD base? | Kết luận |
|---------|----------------|------------------------|----------|
| PcOrderService | Wrapper method cho convenience | ⚠️ CÓ THỂ - Dùng `FindOneById()` | ⚠️ **CÓ THỂ XÓA** |

**Tổng cộng**: 1 service override FindOne
- ⚠️ **CÓ THỂ XÓA**: 1 service

---

## 4. Đề Xuất Cải Thiện

### 4.1. Có Thể Xóa Override

1. **AIProviderProfileHandler.InsertOne**:
   - **Lý do**: Có thể dùng `transform:"nested_struct"` trong DTO
   - **Hành động**: Xóa override, thêm `transform:"nested_struct"` vào DTO

2. **PcOrderService.FindOne**:
   - **Lý do**: Chỉ là wrapper method, không có logic đặc biệt
   - **Hành động**: Xóa override, dùng `BaseServiceMongoImpl.FindOneById()` trực tiếp

### 4.2. Có Thể Đơn Giản Hóa

1. **AIWorkflowRunHandler.InsertOne**:
   - **Lý do**: Set default values có thể dùng transform tag
   - **Hành động**: Dùng `transform:"int,default=0"` và `transform:"str_objectid_array,default=[]"` trong DTO
   - **Giữ lại**: Phần cross-collection validation

---

## 5. Kết Luận

### 5.1. Tổng Số Override

- **Handler InsertOne**: 9 handlers
  - ✅ **CẦN GIỮ**: 7 handlers
  - ⚠️ **CÓ THỂ XÓA/ĐƠN GIẢN HÓA**: 2 handlers

- **Service InsertOne**: 2 services
  - ✅ **CẦN GIỮ**: 2 services

- **Service DeleteOne/DeleteById**: 3 services
  - ✅ **CẦN GIỮ**: 3 services

- **Service FindOne**: 1 service
  - ⚠️ **CÓ THỂ XÓA**: 1 service

### 5.2. Lý Do Chính Của Override

1. **Cross-field validation**: Conditional validation dựa trên giá trị của field khác
2. **Cross-collection validation**: Validate ObjectID tồn tại trong collection khác
3. **Business logic phức tạp**: Tính toán Path/Level, schema validation, sequential level constraint
4. **Validation uniqueness phức tạp**: Query database để check duplicate với nhiều điều kiện
5. **Nested struct arrays**: Transform tag không hỗ trợ nested struct arrays
6. **Cascade delete protection**: Kiểm tra dependencies trước khi xóa
7. **Business rules**: Rules đặc biệt như Administrator role phải có ít nhất 1 user

### 5.3. Đánh Giá

**Tỷ lệ override hợp lệ**: ~90% (11/12 handlers/services cần giữ override)

**Các override đều có lý do rõ ràng**:
- Logic nghiệp vụ phức tạp
- Cross-field/cross-collection validation
- Business rules đặc biệt
- Cascade delete protection

**Có thể cải thiện**:
- 2 handlers có thể xóa/đơn giản hóa override
- 1 service có thể xóa override (wrapper method)

---

## 6. Hành Động Tiếp Theo

### 6.1. Ngay Lập Tức

1. ✅ **Xóa AIProviderProfileHandler.InsertOne** và dùng `transform:"nested_struct"` trong DTO
2. ✅ **Xóa PcOrderService.FindOne** và dùng `BaseServiceMongoImpl.FindOneById()` trực tiếp

### 6.2. Có Thể Làm Sau

1. ⚠️ **Đơn giản hóa AIWorkflowRunHandler.InsertOne**: Dùng transform tag cho default values, chỉ giữ cross-collection validation

---

## 7. Lưu Ý

- Tất cả các override đều có comment rõ ràng về lý do override
- Các override đều đảm bảo logic cơ bản của BaseHandler/BaseService (validation, timestamps, ownerOrganizationId)
- Không có override nào chỉ copy logic BaseHandler/BaseService mà không có logic đặc biệt
