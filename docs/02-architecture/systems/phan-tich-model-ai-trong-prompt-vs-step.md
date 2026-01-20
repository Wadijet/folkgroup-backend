# Phân Tích: Model AI Trong Prompt Template vs Step

## 📋 Tổng Quan

Hiện tại hệ thống đang đặt **Model AI configuration** (model, temperature, maxTokens, providerProfileID) trong **Prompt Template**. Tài liệu này phân tích 2 trường hợp và đề xuất giải pháp tốt nhất.

---

## 🔍 Trường Hợp 1: Model Trong Prompt Template (HIỆN TẠI)

### Cấu Trúc Hiện Tại

```go
// AIPromptTemplate
type AIPromptTemplate struct {
    // ... basic info
    Prompt      string
    Variables   []AIPromptTemplateVariable
    
    // ===== AI CONFIG =====
    ProviderProfileID *primitive.ObjectID  // ✅ Có
    Model            string                // ✅ Có
    Temperature      *float64              // ✅ Có
    MaxTokens        *int                 // ✅ Có
}

// AIStep
type AIStep struct {
    // ... basic info
    PromptTemplateID *primitive.ObjectID  // ✅ Chỉ reference đến template
    // ❌ KHÔNG có Model, Temperature, MaxTokens
}
```

### Ví Dụ Thực Tế

```go
// Prompt Template: "Generate STP from Layer"
{
    name: "Generate STP from Layer",
    type: "generate",
    prompt: "...",
    model: "gpt-4",              // ✅ Model ở đây
    temperature: 0.7,           // ✅ Temperature ở đây
    maxTokens: 2000,            // ✅ MaxTokens ở đây
}

// Prompt Template: "Judge Content Candidates"
{
    name: "Judge Content Candidates",
    type: "judge",
    prompt: "...",
    model: "gpt-4",              // ✅ Model ở đây
    temperature: 0.3,            // ✅ Temperature thấp hơn cho judging
    maxTokens: 1500,            // ✅ MaxTokens ở đây
}

// Step: "Generate STP from Layer"
{
    name: "Generate STP from Layer",
    type: "GENERATE",
    promptTemplateID: "...",    // ✅ Reference đến template
    // ❌ Không có model config
}
```

### ✅ Ưu Điểm

1. **Tách Biệt Rõ Ràng**: Prompt content và AI config tách biệt
   - Prompt Template = "Cái gì" (what to ask)
   - Model Config = "Cách hỏi" (how to ask)

2. **Tái Sử Dụng Prompt**: Cùng một prompt có thể dùng với nhiều model khác nhau
   - Ví dụ: Prompt "Generate STP" có thể dùng với GPT-4, Claude, Gemini
   - Chỉ cần tạo nhiều template với cùng prompt nhưng khác model

3. **Versioning Prompt Độc Lập**: Version prompt không ảnh hưởng đến model config
   - Prompt v1.0.0 → v2.0.0: Chỉ thay đổi nội dung prompt
   - Model config giữ nguyên

4. **Quản Lý Tập Trung**: Tất cả AI config ở một nơi (Prompt Template)
   - Dễ quản lý và theo dõi
   - Dễ audit và optimize

5. **Phù Hợp Với Use Case**: 
   - **GENERATE steps**: Cần temperature cao (0.7-0.9) để sáng tạo
   - **JUDGE steps**: Cần temperature thấp (0.2-0.3) để chính xác
   - Mỗi loại prompt có model config phù hợp

### ❌ Nhược Điểm

1. **Không Linh Hoạt Theo Step**: 
   - Nếu muốn dùng cùng prompt nhưng khác model cho từng step → phải tạo nhiều template
   - Ví dụ: Step A và Step B cùng dùng prompt "Generate STP" nhưng Step A dùng GPT-4, Step B dùng Claude

2. **Không Override Được**: 
   - Step không thể override model config từ template
   - Phải tạo template mới nếu muốn thay đổi

3. **Coupling Giữa Prompt và Model**: 
   - Prompt và model bị ràng buộc với nhau
   - Khó tách biệt hoàn toàn

4. **Khó Quản Lý Khi Có Nhiều Template**: 
   - Nếu có 10 steps, mỗi step cần 2-3 model config khác nhau → 20-30 templates
   - Khó maintain

---

## 🔍 Trường Hợp 2: Model Trong Step (ĐỀ XUẤT)

### Cấu Trúc Đề Xuất

```go
// AIPromptTemplate
type AIPromptTemplate struct {
    // ... basic info
    Prompt      string
    Variables   []AIPromptTemplateVariable
    // ❌ KHÔNG có Model, Temperature, MaxTokens
}

// AIStep
type AIStep struct {
    // ... basic info
    PromptTemplateID *primitive.ObjectID
    
    // ===== AI CONFIG =====
    ProviderProfileID *primitive.ObjectID  // ✅ Thêm vào đây
    Model            string                // ✅ Thêm vào đây
    Temperature      *float64              // ✅ Thêm vào đây
    MaxTokens        *int                 // ✅ Thêm vào đây
}
```

### Ví Dụ Thực Tế

```go
// Prompt Template: "Generate STP from Layer"
{
    name: "Generate STP from Layer",
    type: "generate",
    prompt: "...",
    // ❌ Không có model config
}

// Step: "Generate STP from Layer"
{
    name: "Generate STP from Layer",
    type: "GENERATE",
    promptTemplateID: "...",
    model: "gpt-4",              // ✅ Model ở đây
    temperature: 0.7,            // ✅ Temperature ở đây
    maxTokens: 2000,             // ✅ MaxTokens ở đây
}

// Step: "Judge STP Candidates"
{
    name: "Judge STP Candidates",
    type: "JUDGE",
    promptTemplateID: "...",      // Cùng prompt template "Judge Content Candidates"
    model: "gpt-4",               // ✅ Model ở đây
    temperature: 0.3,             // ✅ Temperature thấp hơn
    maxTokens: 1500,             // ✅ MaxTokens ở đây
}
```

### ✅ Ưu Điểm

1. **Linh Hoạt Theo Step**: 
   - Mỗi step có thể dùng model config riêng
   - Cùng một prompt template nhưng khác model cho từng step

2. **Override Dễ Dàng**: 
   - Step có thể override model config từ template (nếu có)
   - Hoặc set riêng hoàn toàn

3. **Tách Biệt Hoàn Toàn**: 
   - Prompt Template = Pure content (chỉ có prompt text)
   - Step = Execution config (có model, temperature, etc.)

4. **Dễ Quản Lý Workflow**: 
   - Tất cả config của step ở một nơi
   - Dễ thấy step nào dùng model gì

5. **Phù Hợp Với Workflow Logic**: 
   - Workflow = tập hợp các steps
   - Mỗi step có thể có model config riêng phù hợp với nhiệm vụ

### ❌ Nhược Điểm

1. **Trùng Lặp Config**: 
   - Nếu nhiều steps dùng cùng model config → phải set lại nhiều lần
   - Ví dụ: 10 GENERATE steps đều dùng GPT-4, temperature 0.7 → phải set 10 lần

2. **Khó Quản Lý Tập Trung**: 
   - Model config rải rác ở nhiều steps
   - Khó audit và optimize toàn bộ

3. **Không Tái Sử Dụng Model Config**: 
   - Không thể tạo "model profile" để reuse
   - Phải set lại cho mỗi step

4. **Phức Tạp Hơn**: 
   - Step phải quản lý cả prompt template ID và model config
   - Nhiều fields hơn

---

## 🎯 So Sánh Trực Tiếp

| Tiêu Chí | Model Trong Prompt Template | Model Trong Step |
|----------|------------------------------|------------------|
| **Tái sử dụng prompt** | ✅ Dễ (cùng prompt, khác model = nhiều template) | ✅ Dễ (cùng prompt template ID) |
| **Tái sử dụng model config** | ✅ Dễ (cùng model config trong template) | ❌ Khó (phải set lại cho mỗi step) |
| **Linh hoạt theo step** | ❌ Khó (phải tạo template mới) | ✅ Dễ (set riêng cho mỗi step) |
| **Quản lý tập trung** | ✅ Tốt (tất cả ở template) | ❌ Kém (rải rác ở steps) |
| **Override** | ❌ Không thể | ✅ Có thể |
| **Phù hợp với use case** | ✅ Tốt (GENERATE vs JUDGE có config khác) | ✅ Tốt (mỗi step có config riêng) |
| **Độ phức tạp** | ⭐⭐ (đơn giản) | ⭐⭐⭐ (phức tạp hơn) |
| **Maintainability** | ✅ Tốt | ⚠️ Trung bình |

---

## 💡 Đề Xuất: Hybrid Approach (KẾT HỢP)

### Giải Pháp: Model Config Có Thể Ở Cả 2 Nơi

```go
// AIPromptTemplate - Có default model config
type AIPromptTemplate struct {
    // ... basic info
    Prompt      string
    Variables   []AIPromptTemplateVariable
    
    // ===== AI CONFIG (Default) =====
    ProviderProfileID *primitive.ObjectID  // ✅ Default
    Model            string                // ✅ Default
    Temperature      *float64              // ✅ Default
    MaxTokens        *int                 // ✅ Default
}

// AIStep - Có thể override model config
type AIStep struct {
    // ... basic info
    PromptTemplateID *primitive.ObjectID
    
    // ===== AI CONFIG (Override - Optional) =====
    ProviderProfileID *primitive.ObjectID  // ✅ Optional override
    Model            string                // ✅ Optional override
    Temperature      *float64              // ✅ Optional override
    MaxTokens        *int                 // ✅ Optional override
}
```

### Logic Resolve Model Config

```go
func ResolveModelConfig(step *AIStep, template *AIPromptTemplate) ModelConfig {
    // Ưu tiên: Step config > Template config
    return ModelConfig{
        ProviderProfileID: step.ProviderProfileID ?? template.ProviderProfileID,
        Model:            step.Model ?? template.Model,
        Temperature:     step.Temperature ?? template.Temperature,
        MaxTokens:       step.MaxTokens ?? template.MaxTokens,
    }
}
```

### ✅ Ưu Điểm Hybrid

1. **Best of Both Worlds**: 
   - Template có default config (tái sử dụng)
   - Step có thể override (linh hoạt)

2. **Backward Compatible**: 
   - Giữ nguyên cấu trúc hiện tại
   - Chỉ thêm optional fields vào Step

3. **Linh Hoạt Tối Đa**: 
   - Use case đơn giản: Chỉ set trong template
   - Use case phức tạp: Override trong step

4. **Dễ Migrate**: 
   - Code hiện tại vẫn hoạt động
   - Từ từ migrate sang override khi cần

---

## 🎯 Kết Luận & Khuyến Nghị

### Trường Hợp Nào Phù Hợp?

#### **Model Trong Prompt Template** phù hợp khi:
- ✅ Prompt và model config có mối quan hệ chặt chẽ
- ✅ Cùng một prompt thường dùng cùng một model config
- ✅ Use case đơn giản, không cần override
- ✅ Muốn quản lý tập trung

#### **Model Trong Step** phù hợp khi:
- ✅ Mỗi step có yêu cầu model config khác nhau
- ✅ Cần linh hoạt override model config
- ✅ Workflow phức tạp với nhiều steps khác nhau
- ✅ Muốn tách biệt hoàn toàn prompt và execution config

#### **Hybrid Approach** phù hợp khi:
- ✅ Cần cả 2: default config và override
- ✅ Muốn backward compatible
- ✅ Hệ thống lớn với nhiều use cases khác nhau
- ✅ **⭐ KHUYẾN NGHỊ CHO HỆ THỐNG HIỆN TẠI**

---

## 📝 Khuyến Nghị Cho FolkForm

### Đề Xuất: **Hybrid Approach**

**Lý do:**
1. ✅ Giữ nguyên cấu trúc hiện tại (Model trong Template)
2. ✅ Thêm optional override trong Step (linh hoạt)
3. ✅ Backward compatible (không breaking changes)
4. ✅ Phù hợp với use case hiện tại và tương lai

**Implementation:**
1. Giữ nguyên `AIPromptTemplate` có model config (default)
2. Thêm optional model config vào `AIStep` (override)
3. Logic resolve: Step config > Template config
4. Migration: Từ từ, không cần migrate ngay

**Ví dụ:**
```go
// Template: Default config
{
    name: "Generate STP from Layer",
    model: "gpt-4",           // Default
    temperature: 0.7,         // Default
}

// Step 1: Dùng default từ template
{
    name: "Generate STP from Layer",
    promptTemplateID: "...",
    // Không có model config → dùng từ template
}

// Step 2: Override model
{
    name: "Generate STP from Layer (Claude)",
    promptTemplateID: "...",  // Cùng template
    model: "claude-3-opus",   // Override
    temperature: 0.8,         // Override
}
```

---

## 🔄 Migration Plan (Nếu Chọn Hybrid)

1. **Phase 1**: Thêm optional fields vào `AIStep` (không breaking)
2. **Phase 2**: Update logic resolve model config
3. **Phase 3**: Update UI/API để support override
4. **Phase 4**: Từ từ migrate các steps cần override

---

## 📊 Phân Tích Cho Learning Data & Analytics

### Mục Tiêu Learning Data

Hệ thống cần thu thập và phân tích:
1. **Model Performance**: So sánh hiệu suất giữa các models (GPT-4 vs Claude vs Gemini)
2. **Prompt Versioning**: So sánh hiệu suất giữa các version của prompt
3. **Cost Optimization**: Tối ưu chi phí bằng cách chọn model phù hợp
4. **Quality Metrics**: Đo lường chất lượng output theo model/config
5. **A/B Testing**: Test cùng prompt với khác model, hoặc cùng model với khác prompt

### Cấu Trúc Data Hiện Tại

```go
// AIRun - Lưu tất cả AI calls
type AIRun struct {
    PromptTemplateID *primitive.ObjectID  // ✅ Link về template
    StepRunID        *primitive.ObjectID  // ✅ Link về step run
    Provider         string               // ✅ Provider name
    Model            string               // ✅ Model name
    Cost             *float64             // ✅ Cost
    Latency          *int64               // ✅ Latency
    QualityScore     *float64             // ✅ Quality score
    // ...
}
```

### 🔍 Trường Hợp 1: Model Trong Prompt Template (HIỆN TẠI)

#### ✅ Ưu Điểm Cho Learning Data

1. **Query Theo Template Dễ Dàng**:
   ```javascript
   // So sánh performance của prompt "Generate STP" với các models khác nhau
   db.ai_runs.aggregate([
     { $match: { promptTemplateId: ObjectId("...") } },
     { $group: {
         _id: "$model",
         avgCost: { $avg: "$cost" },
         avgLatency: { $avg: "$latency" },
         avgQuality: { $avg: "$qualityScore" },
         count: { $sum: 1 }
       }
     }
   ])
   ```
   - ✅ Dễ query: Tất cả runs của cùng prompt template
   - ✅ Dễ so sánh: Cùng prompt, khác model (nếu có nhiều templates)

2. **Prompt Versioning Rõ Ràng**:
   ```javascript
   // So sánh prompt v1.0.0 vs v2.0.0 với cùng model
   db.ai_runs.aggregate([
     { $lookup: { from: "ai_prompt_templates", ... } },
     { $match: { "template.version": { $in: ["1.0.0", "2.0.0"] } } },
     { $group: { _id: "$template.version", ... } }
   ])
   ```
   - ✅ Dễ track: Version prompt trong template
   - ✅ Dễ so sánh: Cùng model, khác version prompt

3. **Quản Lý Tập Trung**:
   - ✅ Tất cả config ở template → dễ audit
   - ✅ Dễ optimize: Thay đổi model config ở template → ảnh hưởng tất cả steps dùng template đó

#### ❌ Nhược Điểm Cho Learning Data

1. **Khó So Sánh Cùng Prompt Với Khác Model**:
   ```javascript
   // ❌ Khó: Muốn so sánh cùng prompt "Generate STP" với GPT-4 vs Claude
   // Phải tạo 2 templates: "Generate STP (GPT-4)" và "Generate STP (Claude)"
   // → Khó biết đó là cùng prompt, chỉ khác model
   ```

2. **Khó A/B Testing**:
   - ❌ Muốn test cùng prompt với 2 models → phải tạo 2 templates
   - ❌ Khó track: Template nào là "variant" của template nào?

3. **Khó Aggregate Theo Step**:
   ```javascript
   // ❌ Khó: Muốn biết step "Generate STP" dùng model gì
   // Phải join: Step → Template → Model
   // → Query phức tạp hơn
   ```

---

### 🔍 Trường Hợp 2: Model Trong Step

#### ✅ Ưu Điểm Cho Learning Data

1. **Query Theo Step Dễ Dàng**:
   ```javascript
   // So sánh performance của step "Generate STP" với các models
   db.ai_runs.aggregate([
     { $lookup: { from: "ai_step_runs", ... } },
     { $match: { "stepRun.stepId": ObjectId("...") } },
     { $group: {
         _id: "$model",
         avgCost: { $avg: "$cost" },
         avgQuality: { $avg: "$qualityScore" }
       }
     }
   ])
   ```
   - ✅ Dễ query: Tất cả runs của cùng step
   - ✅ Dễ so sánh: Cùng step, khác model

2. **A/B Testing Dễ Dàng**:
   ```javascript
   // Test cùng step với 2 models khác nhau
   // Step A: promptTemplateId = X, model = "gpt-4"
   // Step B: promptTemplateId = X, model = "claude-3-opus"
   // → Dễ so sánh: Cùng prompt, khác model
   ```

3. **Workflow-Level Analytics**:
   ```javascript
   // Phân tích toàn bộ workflow: Step nào dùng model gì?
   db.ai_steps.find({ workflowId: ... })
   // → Thấy ngay: Step 1 dùng GPT-4, Step 2 dùng Claude
   ```

4. **Granular Control**:
   - ✅ Mỗi step có thể track riêng model performance
   - ✅ Dễ optimize: Thay đổi model cho từng step riêng

#### ❌ Nhược Điểm Cho Learning Data

1. **Khó Query Theo Template**:
   ```javascript
   // ❌ Khó: Muốn biết prompt "Generate STP" performance với tất cả models
   // Phải join: Template → Steps → Runs → Aggregate
   // → Query phức tạp hơn
   ```

2. **Trùng Lặp Data**:
   - ❌ Nhiều steps dùng cùng model config → data trùng lặp
   - ❌ Khó aggregate: Phải group theo nhiều fields

3. **Khó Versioning Prompt**:
   - ❌ Prompt version trong template, nhưng model trong step
   - ❌ Khó track: Cùng prompt version, khác model config

---

### 🔍 Hybrid Approach Cho Learning Data

#### ✅ Ưu Điểm Tối Đa

1. **Query Linh Hoạt**:
   ```javascript
   // Query theo Template (dùng default từ template)
   db.ai_runs.aggregate([
     { $match: { promptTemplateId: ObjectId("...") } },
     { $lookup: { from: "ai_prompt_templates", ... } },
     { $addFields: {
         actualModel: { $ifNull: ["$model", "$template.model"] }
       }
     }
   ])
   
   // Query theo Step (dùng override từ step)
   db.ai_runs.aggregate([
     { $lookup: { from: "ai_step_runs", ... } },
     { $lookup: { from: "ai_steps", ... } },
     { $addFields: {
         actualModel: { 
           $ifNull: [
             "$step.model",           // Override từ step
             "$template.model"        // Default từ template
           ]
         }
       }
     }
   ])
   ```

2. **A/B Testing Tối Ưu**:
   ```javascript
   // Test cùng prompt với 2 models:
   // - Template: model = "gpt-4" (default)
   // - Step A: không override → dùng GPT-4
   // - Step B: override model = "claude-3-opus"
   // → Dễ so sánh: Cùng prompt, khác model
   ```

3. **Analytics Đa Chiều**:
   ```javascript
   // Phân tích theo Template
   db.ai_runs.group({
     key: { promptTemplateId: 1, model: 1 },
     reduce: function(curr, result) { ... }
   })
   
   // Phân tích theo Step
   db.ai_runs.group({
     key: { stepId: 1, model: 1 },
     reduce: function(curr, result) { ... }
   })
   
   // Phân tích theo Workflow
   db.ai_runs.group({
     key: { workflowId: 1, model: 1 },
     reduce: function(curr, result) { ... }
   })
   ```

4. **Cost Optimization**:
   ```javascript
   // Tìm model tốt nhất cho từng prompt
   db.ai_runs.aggregate([
     { $match: { promptTemplateId: ObjectId("...") } },
     { $group: {
         _id: "$model",
         avgCost: { $avg: "$cost" },
         avgQuality: { $avg: "$qualityScore" },
         efficiency: { $divide: ["$avgQuality", "$avgCost"] }
       }
     },
     { $sort: { efficiency: -1 } }
   ])
   ```

---

## 🎯 Kết Luận Cho Learning Data

### ⭐ Model Trong Step LỢI THẾ HƠN Cho Learning Data

**Lý do:**

1. **✅ Granular Tracking**: 
   - Mỗi step có thể track riêng model performance
   - Dễ optimize từng step riêng biệt

2. **✅ A/B Testing Dễ Dàng**: 
   - Cùng prompt, khác model → chỉ cần tạo 2 steps
   - Dễ so sánh và track results

3. **✅ Workflow-Level Analytics**: 
   - Dễ thấy toàn bộ workflow dùng models gì
   - Dễ optimize cost/quality cho từng step

4. **✅ Flexible Queries**: 
   - Query theo step, workflow, hoặc template đều được
   - Linh hoạt hơn cho analytics

### ⚠️ Nhưng Hybrid Vẫn Tốt Nhất

**Lý do:**

1. **✅ Best of Both Worlds**: 
   - Default từ template (dễ quản lý)
   - Override từ step (linh hoạt)

2. **✅ Backward Compatible**: 
   - Giữ nguyên cấu trúc hiện tại
   - Từ từ migrate

3. **✅ Analytics Tối Ưu**: 
   - Query được cả 2 cách
   - Phù hợp với mọi use case

---

## 📊 Recommendation Cho Learning Data

### Đề Xuất: **Hybrid Approach** (Ưu Tiên Step Override)

**Implementation:**
1. Giữ model config trong Template (default)
2. Thêm model config vào Step (override - **ƯU TIÊN**)
3. Logic resolve: **Step config > Template config**
4. **AIRun lưu actual model được dùng** (từ step hoặc template)

**Ví dụ AIRun:**
```go
AIRun {
    PromptTemplateID: "...",      // Link về template
    StepRunID: "...",             // Link về step run
    StepID: "...",                // Link về step definition
    
    // Actual config được dùng (resolve từ step hoặc template)
    Provider: "openai",           // ✅ Actual
    Model: "gpt-4",               // ✅ Actual (từ step hoặc template)
    Temperature: 0.7,             // ✅ Actual
    
    // Metadata để tracking
    TemplateModel: "gpt-4",       // ✅ Model từ template (để so sánh)
    StepModel: "gpt-4",          // ✅ Model từ step (nếu có override)
    UsedFrom: "step",            // ✅ "step" hoặc "template"
}
```

**Analytics Queries:**
```javascript
// 1. So sánh model performance theo step
db.ai_runs.aggregate([
  { $lookup: { from: "ai_step_runs", ... } },
  { $group: {
      _id: { stepId: "$stepRun.stepId", model: "$model" },
      avgCost: { $avg: "$cost" },
      avgQuality: { $avg: "$qualityScore" }
    }
  }
])

// 2. So sánh cùng prompt với khác model (A/B testing)
db.ai_runs.aggregate([
  { $match: { promptTemplateId: ObjectId("...") } },
  { $group: {
      _id: "$model",
      avgCost: { $avg: "$cost" },
      avgQuality: { $avg: "$qualityScore" },
      count: { $sum: 1 }
    }
  }
])

// 3. Tìm model tốt nhất cho từng step
db.ai_runs.aggregate([
  { $lookup: { from: "ai_step_runs", ... } },
  { $group: {
      _id: { stepId: "$stepRun.stepId", model: "$model" },
      efficiency: { $avg: { $divide: ["$qualityScore", "$cost"] } }
    }
  },
  { $sort: { efficiency: -1 } },
  { $group: {
      _id: "$_id.stepId",
      bestModel: { $first: "$_id.model" },
      bestEfficiency: { $first: "$efficiency" }
    }
  }
])
```

---

**Kết luận: Hybrid Approach với ưu tiên Step override là tốt nhất cho learning data và analytics.**

---

**Tài liệu này giúp quyết định architecture phù hợp cho hệ thống AI workflow của FolkForm.**
