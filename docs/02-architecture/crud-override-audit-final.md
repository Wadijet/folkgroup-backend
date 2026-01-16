# Rà Soát CRUD Override - Handler và Service

## Tổng Quan

Tài liệu này phân tích tất cả các CRUD override trong handlers và services để xác định những cái nào có thể thay thế bằng BaseHandler/BaseService.

---

## 1. Handler Overrides - InsertOne

### 1.1. ✅ **CẦN GIỮ** - Logic nghiệp vụ phức tạp

#### AIStepHandler.InsertOne
- **Lý do**: Validate schema phức tạp với `models.ValidateStepSchema()` - không thể dùng struct tag
- **Logic đặc biệt**: Validate input/output schema theo standard schema cho từng step type
- **Kết luận**: ✅ **CẦN GIỮ** - Business logic validation phức tạp

#### AIWorkflowHandler.InsertOne
- **Lý do**: Convert nested struct arrays (`Steps []AIWorkflowStepReference`) - transform tag không hỗ trợ arrays
- **Logic đặc biệt**: 
  - Convert `[]dto.AIWorkflowStepReferenceInput` → `[]models.AIWorkflowStepReference`
  - Convert nested Policy trong mỗi Step: `dto.AIWorkflowStepPolicyInput` → `models.AIWorkflowStepPolicy`
  - Convert DefaultPolicy: `dto.AIWorkflowStepPolicyInput` → `*models.AIWorkflowStepPolicy`
- **Kết luận**: ✅ **CẦN GIỮ** - Transform tag chỉ hỗ trợ nested struct, không hỗ trợ nested struct arrays

#### AIWorkflowRunHandler.InsertOne
- **Lý do**: 
  1. Set default values cho business logic (CurrentStepIndex = 0, StepRunIDs = [])
  2. Validate RootRefID phức tạp (tìm trong production hoặc draft, validate type, validate approval status)
- **Logic đặc biệt**: 
  - Cross-collection validation (production + draft)
  - Validate sequential level constraint
- **Kết luận**: ✅ **CẦN GIỮ** - Business logic validation và default values phức tạp

#### AIWorkflowCommandHandler.InsertOne
- **Lý do**: 
  1. Conditional validation (WorkflowID bắt buộc khi CommandType = START_WORKFLOW, StepID bắt buộc khi CommandType = EXECUTE_STEP)
  2. Validate StepID và ParentLevel matching
  3. Validate RootRefID phức tạp (tương tự AIWorkflowRunHandler)
- **Logic đặc biệt**: 
  - Conditional field validation
  - Cross-collection validation
  - Level matching validation
- **Kết luận**: ✅ **CẦN GIỮ** - Conditional validation và business logic phức tạp

#### DraftApprovalHandler.InsertOne
- **Lý do**: 
  1. Cross-field validation: Phải có ít nhất một target (workflowRunID, draftNodeID, draftVideoID, hoặc draftPublicationID)
  2. Set RequestedBy từ context (không cho phép client chỉ định)
  3. Set RequestedAt và Status = "pending" tự động
- **Logic đặc biệt**: 
  - Cross-field validation (không thể dùng struct tag đơn giản)
  - Set fields từ context
- **Kết luận**: ✅ **CẦN GIỮ** - Cross-field validation và logic nghiệp vụ đặc biệt

#### OrganizationHandler.InsertOne
- **Lý do**: 
  1. Tính toán Path và Level dựa trên parent (query database để lấy parent)
  2. Validate Type và parent relationship
  3. Logic tính toán Level phức tạp (System = -1, Group = 0, Company = 1, etc.)
- **Logic đặc biệt**: 
  - Query database để lấy parent
  - Tính toán Path và Level
- **Kết luận**: ✅ **CẦN GIỮ** - Logic nghiệp vụ phức tạp (tính toán Path/Level)

#### NotificationChannelHandler.InsertOne
- **Lý do**: 
  1. Validation uniqueness rất phức tạp với nhiều điều kiện:
     - Name + ChannelType + OwnerOrganizationID (unique)
     - Email: Recipients phải unique trong organization
     - Telegram: ChatIDs phải unique trong organization
     - Webhook: WebhookURL phải unique trong organization
  2. Query database nhiều lần để check duplicate
- **Logic đặc biệt**: 
  - Uniqueness validation phức tạp với nhiều điều kiện
  - Query database nhiều lần
- **Kết luận**: ✅ **CẦN GIỮ** - Uniqueness validation phức tạp

#### NotificationRoutingHandler.InsertOne
- **Lý do**: 
  1. Validation uniqueness: EventType + OwnerOrganizationID (unique), Domain + OwnerOrganizationID (unique)
  2. Query database để check duplicate
  3. Validate EventType bắt buộc
- **Logic đặc biệt**: 
  - Uniqueness validation
  - Query database
- **Kết luận**: ✅ **CẦN GIỮ** - Uniqueness validation

### 1.2. ⚠️ **CÓ THỂ XÓA** - Đã có nested_struct support

#### AIProviderProfileHandler.InsertOne
- **Lý do hiện tại**: Map nested struct Config từ DTO sang Model
- **Phân tích**: 
  - DTO đã có `transform:"nested_struct"` cho Config
  - BaseHandler đã hỗ trợ nested struct transform
  - **CÓ THỂ XÓA** và dùng BaseHandler.InsertOne
- **Kết luận**: ⚠️ **CÓ THỂ XÓA** - Nested struct đã được hỗ trợ bởi transform tag

---

## 2. Handler Overrides - UpdateOne

### 2.1. ✅ **ĐÃ XÓA** - Đã có nested_struct support

#### AIPromptTemplateHandler.UpdateOne
- **Trạng thái**: ✅ **ĐÃ XÓA** - Dùng BaseHandler.UpdateOne
- **Lý do**: Nested struct Provider đã được xử lý tự động bởi `transform:"nested_struct"`

#### AIProviderProfileHandler.UpdateOne
- **Trạng thái**: ✅ **ĐÃ XÓA** - Dùng BaseHandler.UpdateOne
- **Lý do**: Nested struct Config đã được xử lý tự động bởi `transform:"nested_struct"`

---

## 3. Service Overrides - InsertOne

### 3.1. ✅ **CẦN GIỮ** - Logic nghiệp vụ phức tạp

#### DraftContentNodeService.InsertOne
- **Lý do**: 
  1. Validate sequential level constraint
  2. Kiểm tra parent phải tồn tại và đã được commit (production) hoặc là draft đã được approve
  3. Cross-collection validation (production + draft)
- **Logic đặc biệt**: 
  - Cross-collection validation
  - Sequential level constraint validation
- **Kết luận**: ✅ **CẦN GIỮ** - Business logic validation phức tạp

---

## 4. Service Overrides - Delete Operations

### 4.1. ✅ **CẦN GIỮ** - Validate trước khi xóa

#### OrganizationService.DeleteOne/DeleteById/DeleteMany/FindOneAndDelete
- **Lý do**: 
  1. Validate trước khi xóa: Kiểm tra organization có children không
  2. Không cho phép xóa organization có children
- **Logic đặc biệt**: 
  - Query database để check children
  - Business rule: Không thể xóa organization có children
- **Kết luận**: ✅ **CẦN GIỮ** - Business rule validation

#### RoleService.DeleteOne/DeleteById/DeleteMany/FindOneAndDelete
- **Lý do**: 
  1. Validate trước khi xóa: Kiểm tra role có user roles không
  2. Không cho phép xóa role đang được sử dụng
- **Logic đặc biệt**: 
  - Query database để check user roles
  - Business rule: Không thể xóa role đang được sử dụng
- **Kết luận**: ✅ **CẦN GIỮ** - Business rule validation

#### UserRoleService.DeleteOne/DeleteById/DeleteMany
- **Lý do**: 
  1. Validate trước khi xóa: Kiểm tra không thể xóa Administrator role
  2. Business rule: Role Administrator phải có ít nhất một user
- **Logic đặc biệt**: 
  - Query database để check Administrator role
  - Business rule: Không thể xóa Administrator role
- **Kết luận**: ✅ **CẦN GIỮ** - Business rule validation

---

## 5. Tổng Kết

### 5.1. Handler Overrides

| Handler | Method | Trạng thái | Lý do |
|----------|--------|------------|-------|
| AIStepHandler | InsertOne | ✅ CẦN GIỮ | Validate schema phức tạp |
| AIWorkflowHandler | InsertOne | ✅ CẦN GIỮ | Convert nested struct arrays |
| AIWorkflowRunHandler | InsertOne | ✅ CẦN GIỮ | Validate RootRefID phức tạp |
| AIWorkflowCommandHandler | InsertOne | ✅ CẦN GIỮ | Conditional validation |
| DraftApprovalHandler | InsertOne | ✅ CẦN GIỮ | Cross-field validation |
| OrganizationHandler | InsertOne | ✅ CẦN GIỮ | Tính toán Path/Level |
| NotificationChannelHandler | InsertOne | ✅ CẦN GIỮ | Uniqueness validation phức tạp |
| NotificationRoutingHandler | InsertOne | ✅ CẦN GIỮ | Uniqueness validation |
| AIProviderProfileHandler | InsertOne | ⚠️ CÓ THỂ XÓA | Nested struct đã hỗ trợ |
| AIPromptTemplateHandler | UpdateOne | ✅ ĐÃ XÓA | Nested struct đã hỗ trợ |
| AIProviderProfileHandler | UpdateOne | ✅ ĐÃ XÓA | Nested struct đã hỗ trợ |

### 5.2. Service Overrides

| Service | Method | Trạng thái | Lý do |
|---------|--------|------------|-------|
| DraftContentNodeService | InsertOne | ✅ CẦN GIỮ | Validate sequential level constraint |
| OrganizationService | DeleteOne/DeleteById/DeleteMany/FindOneAndDelete | ✅ CẦN GIỮ | Validate có children |
| RoleService | DeleteOne/DeleteById/DeleteMany/FindOneAndDelete | ✅ CẦN GIỮ | Validate có user roles |
| UserRoleService | DeleteOne/DeleteById/DeleteMany | ✅ CẦN GIỮ | Validate Administrator role |

---

## 6. Khuyến Nghị

### 6.1. Có thể xóa ngay

1. **AIProviderProfileHandler.InsertOne** ⚠️
   - **Lý do**: Nested struct Config đã được hỗ trợ bởi `transform:"nested_struct"`
   - **Hành động**: Xóa override, dùng BaseHandler.InsertOne
   - **Rủi ro**: Thấp - đã test với UpdateOne

### 6.2. Cần giữ (tất cả đều hợp lệ)

Tất cả các override còn lại đều có lý do tồn tại hợp lệ:
- Logic nghiệp vụ phức tạp không thể thay thế bằng struct tag
- Cross-collection validation
- Uniqueness validation phức tạp
- Conditional validation
- Business rule validation

### 6.3. Có thể mở rộng trong tương lai

1. **Hỗ trợ nested struct arrays** trong transform tag
   - Nếu implement được, có thể xóa `AIWorkflowHandler.InsertOne`
   - Cần hỗ trợ: `transform:"nested_struct_array"` cho `[]AIWorkflowStepReference`

2. **Custom validator cho cross-field validation**
   - Nếu implement được, có thể đơn giản hóa `DraftApprovalHandler.InsertOne`
   - Cần hỗ trợ: `validate:"at_least_one=workflowRunId,draftNodeId,draftVideoId,draftPublicationId"`

3. **Custom validator cho uniqueness**
   - Nếu implement được, có thể đơn giản hóa `NotificationChannelHandler.InsertOne` và `NotificationRoutingHandler.InsertOne`
   - Cần hỗ trợ: `validate:"unique=name,channelType,ownerOrganizationId"`

---

## 7. Kết Luận

### 7.1. Tổng số override

- **Handler InsertOne**: 9 overrides
  - ✅ Cần giữ: 8
  - ⚠️ Có thể xóa: 1 (AIProviderProfileHandler)
- **Handler UpdateOne**: 0 overrides (đã xóa hết)
- **Service InsertOne**: 1 override (cần giữ)
- **Service Delete**: 12 overrides (cần giữ)

### 7.2. Đánh giá

**Tất cả các override đều có lý do tồn tại hợp lệ**, trừ:
- **AIProviderProfileHandler.InsertOne**: Có thể xóa vì nested struct đã được hỗ trợ

**Tỷ lệ override hợp lệ**: 95% (20/21)

### 7.3. Hành động đề xuất

1. **Ngay lập tức**: Xóa `AIProviderProfileHandler.InsertOne` và test
2. **Tương lai**: Mở rộng transform tag để hỗ trợ nested struct arrays (nếu cần)
