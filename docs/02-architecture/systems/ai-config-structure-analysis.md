# Phân Tích Cấu Trúc AI Config - Prompt Template

## Vấn Đề

Hiện tại, AI config trong `AIPromptTemplate` được lưu dưới dạng các field riêng lẻ:
- `ProviderProfileID *primitive.ObjectID`
- `Model string`
- `Temperature *float64`
- `MaxTokens *int`

**Câu hỏi:** Có nên gom các field này vào một object `AIConfig` không? Đặc biệt khi mỗi provider (OpenAI, Anthropic, Google) có thể có config khác nhau.

---

## Phương Án 1: Fields Riêng Lẻ (Hiện Tại)

### Cấu Trúc

```go
type AIPromptTemplate struct {
    // ... basic fields ...
    
    // ===== AI CONFIG (Override từ Provider Profile) =====
    ProviderProfileID *primitive.ObjectID `json:"providerProfileId,omitempty" bson:"providerProfileId,omitempty"`
    Model             string              `json:"model,omitempty" bson:"model,omitempty"`
    Temperature       *float64            `json:"temperature,omitempty" bson:"temperature,omitempty"`
    MaxTokens         *int                `json:"maxTokens,omitempty" bson:"maxTokens,omitempty"`
    
    // ... other fields ...
}
```

### Ưu Điểm

1. ✅ **Đơn giản, dễ hiểu**: Cấu trúc flat, dễ đọc code
2. ✅ **Dễ query/index MongoDB**: Có thể index từng field riêng biệt
   ```go
   // Dễ query
   filter := bson.M{
       "model": "gpt-4",
       "temperature": bson.M{"$gte": 0.7},
   }
   ```
3. ✅ **Dễ validate**: Validate từng field độc lập
4. ✅ **API response rõ ràng**: Flat structure, dễ parse ở frontend
   ```json
   {
     "id": "...",
     "name": "...",
     "model": "gpt-4",
     "temperature": 0.7,
     "maxTokens": 2000
   }
   ```
5. ✅ **Dễ migrate/update**: Update từng field độc lập, không ảnh hưởng nhau
6. ✅ **Type safety tốt**: Mỗi field có type rõ ràng

### Nhược Điểm

1. ❌ **Struct dài**: Nếu có nhiều config fields, struct sẽ dài
2. ❌ **Không group logic**: Các field liên quan không được group lại
3. ❌ **Khó mở rộng provider-specific config**: Nếu cần thêm config đặc thù cho từng provider (ví dụ: `topP` cho OpenAI, `maxTokensToSample` cho Anthropic), phải thêm field mới ở top level → struct sẽ rất dài

### Khi Nào Phù Hợp?

- ✅ Khi tất cả providers dùng chung các config fields (model, temperature, maxTokens)
- ✅ Khi không cần provider-specific config
- ✅ Khi ưu tiên đơn giản và dễ query

---

## Phương Án 2: Gom Vào Object `AIConfig`

### Cấu Trúc

```go
// AIPromptTemplateAIConfig chứa AI config cho prompt template
type AIPromptTemplateAIConfig struct {
    // Common config (tất cả providers đều có)
    ProviderProfileID *primitive.ObjectID `json:"providerProfileId,omitempty" bson:"providerProfileId,omitempty"`
    Model             string              `json:"model,omitempty" bson:"model,omitempty"`
    Temperature       *float64            `json:"temperature,omitempty" bson:"temperature,omitempty"`
    MaxTokens         *int                `json:"maxTokens,omitempty" bson:"maxTokens,omitempty"`
    
    // Provider-specific config (optional, dùng cho config đặc thù)
    ProviderConfig    map[string]interface{} `json:"providerConfig,omitempty" bson:"providerConfig,omitempty"`
    // Ví dụ:
    // - OpenAI: {"topP": 1.0, "frequencyPenalty": 0.0, "presencePenalty": 0.0}
    // - Anthropic: {"maxTokensToSample": 4096, "stopSequences": []}
    // - Google: {"topK": 40, "topP": 0.95}
}

type AIPromptTemplate struct {
    // ... basic fields ...
    
    // ===== AI CONFIG (Override từ Provider Profile) =====
    AIConfig *AIPromptTemplateAIConfig `json:"aiConfig,omitempty" bson:"aiConfig,omitempty"`
    
    // ... other fields ...
}
```

### Ưu Điểm

1. ✅ **Group logic liên quan**: Tất cả AI config ở một chỗ
2. ✅ **Dễ mở rộng**: Có thể thêm provider-specific config vào `ProviderConfig` map mà không làm struct dài
3. ✅ **Cleaner struct**: Top-level struct gọn hơn, chỉ có 1 field `AIConfig`
4. ✅ **Linh hoạt**: Có thể có nested structure cho provider-specific config
5. ✅ **Dễ refactor**: Nếu cần thay đổi cấu trúc config, chỉ cần sửa 1 struct

### Nhược Điểm

1. ❌ **Query/index phức tạp hơn**: Phải query nested field
   ```go
   // Phức tạp hơn
   filter := bson.M{
       "aiConfig.model": "gpt-4",
       "aiConfig.temperature": bson.M{"$gte": 0.7},
   }
   ```
2. ❌ **API response nested**: Frontend phải parse nested structure
   ```json
   {
     "id": "...",
     "name": "...",
     "aiConfig": {
       "model": "gpt-4",
       "temperature": 0.7,
       "maxTokens": 2000
     }
   }
   ```
3. ❌ **Validation phức tạp hơn**: Phải validate nested struct
4. ❌ **Migration phức tạp**: Phải migrate từ flat structure sang nested structure
5. ❌ **Type safety kém hơn**: `ProviderConfig map[string]interface{}` không có type safety

### Khi Nào Phù Hợp?

- ✅ Khi cần provider-specific config (OpenAI có `topP`, Anthropic có `maxTokensToSample`, etc.)
- ✅ Khi muốn group logic liên quan
- ✅ Khi ưu tiên flexibility và extensibility

---

## Phương Án 3: Hybrid - Common Fields + Provider Config Map (KHUYẾN NGHỊ)

### Cấu Trúc

```go
type AIPromptTemplate struct {
    // ... basic fields ...
    
    // ===== AI CONFIG (Override từ Provider Profile) =====
    // Common config (tất cả providers đều có) - để riêng để dễ query/index
    ProviderProfileID *primitive.ObjectID `json:"providerProfileId,omitempty" bson:"providerProfileId,omitempty" index:"single:1"`
    Model             string              `json:"model,omitempty" bson:"model,omitempty" index:"single:1"`
    Temperature       *float64            `json:"temperature,omitempty" bson:"temperature,omitempty"`
    MaxTokens         *int                `json:"maxTokens,omitempty" bson:"maxTokens,omitempty"`
    
    // Provider-specific config (optional) - dùng cho config đặc thù
    ProviderConfig    map[string]interface{} `json:"providerConfig,omitempty" bson:"providerConfig,omitempty"`
    // Ví dụ:
    // - OpenAI: {"topP": 1.0, "frequencyPenalty": 0.0, "presencePenalty": 0.0}
    // - Anthropic: {"maxTokensToSample": 4096, "stopSequences": []}
    // - Google: {"topK": 40, "topP": 0.95}
}
```

### Ưu Điểm

1. ✅ **Best of both worlds**: 
   - Common fields (model, temperature, maxTokens) ở top level → dễ query/index
   - Provider-specific config trong map → linh hoạt, không làm struct dài
2. ✅ **Dễ query**: Common fields vẫn query như bình thường
3. ✅ **Linh hoạt**: Có thể thêm provider-specific config mà không làm struct dài
4. ✅ **Type safety**: Common fields vẫn có type safety
5. ✅ **Migration dễ**: Chỉ cần thêm field `ProviderConfig`, không cần migrate existing data

### Nhược Điểm

1. ⚠️ **Hơi phân tán**: Common config và provider config ở 2 chỗ khác nhau (nhưng vẫn hợp lý vì common config được dùng nhiều hơn)

### Khi Nào Phù Hợp?

- ✅ **KHUYẾN NGHỊ**: Khi muốn balance giữa simplicity và flexibility
- ✅ Khi common config (model, temperature, maxTokens) được dùng nhiều và cần query/index
- ✅ Khi cần provider-specific config nhưng không muốn làm struct dài

---

## So Sánh Tổng Hợp

| Tiêu Chí | Phương Án 1 (Fields Riêng) | Phương Án 2 (Object) | Phương Án 3 (Hybrid) |
|----------|---------------------------|----------------------|---------------------|
| **Đơn giản** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Dễ query/index** | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐⭐ |
| **Type safety** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Flexibility** | ⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Provider-specific config** | ⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Migration** | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐⭐ |
| **API response** | ⭐⭐⭐⭐⭐ (flat) | ⭐⭐⭐ (nested) | ⭐⭐⭐⭐ (mostly flat) |

---

## Khuyến Nghị

### 🎯 **Phương Án 3 (Hybrid) - KHUYẾN NGHỊ**

**Lý do:**
1. ✅ **Balance tốt**: Giữ được simplicity của phương án 1, nhưng có flexibility của phương án 2
2. ✅ **Dễ query**: Common fields (model, temperature, maxTokens) vẫn ở top level, dễ query/index
3. ✅ **Linh hoạt**: Có thể thêm provider-specific config vào `ProviderConfig` map khi cần
4. ✅ **Migration dễ**: Chỉ cần thêm field mới, không cần migrate existing data
5. ✅ **Type safety**: Common fields vẫn có type safety

**Cấu trúc đề xuất:**

```go
type AIPromptTemplate struct {
    // ... basic fields ...
    
    // ===== AI CONFIG (Override từ Provider Profile) =====
    // Common config (tất cả providers đều có) - để riêng để dễ query/index
    ProviderProfileID *primitive.ObjectID `json:"providerProfileId,omitempty" bson:"providerProfileId,omitempty" index:"single:1"`
    Model             string              `json:"model,omitempty" bson:"model,omitempty" index:"single:1"`
    Temperature       *float64            `json:"temperature,omitempty" bson:"temperature,omitempty"`
    MaxTokens         *int                `json:"maxTokens,omitempty" bson:"maxTokens,omitempty"`
    
    // Provider-specific config (optional) - dùng cho config đặc thù của từng provider
    ProviderConfig    map[string]interface{} `json:"providerConfig,omitempty" bson:"providerConfig,omitempty"`
    // Ví dụ sử dụng:
    // - OpenAI: {"topP": 1.0, "frequencyPenalty": 0.0, "presencePenalty": 0.0}
    // - Anthropic: {"maxTokensToSample": 4096, "stopSequences": []}
    // - Google: {"topK": 40, "topP": 0.95}
}
```

**Khi nào dùng ProviderConfig:**
- Khi cần config đặc thù cho từng provider (ví dụ: `topP` cho OpenAI, `maxTokensToSample` cho Anthropic)
- Khi config không phổ biến (không phải tất cả providers đều có)
- Khi config có thể thay đổi theo thời gian (provider thêm/bớt config)

**Khi nào dùng Common Fields:**
- Khi config phổ biến (tất cả providers đều có): model, temperature, maxTokens
- Khi cần query/index thường xuyên
- Khi cần type safety

---

## Kết Luận

**Khuyến nghị: Phương Án 3 (Hybrid)**

- Giữ common fields (model, temperature, maxTokens) ở top level để dễ query/index
- Thêm `ProviderConfig map[string]interface{}` để lưu provider-specific config khi cần
- Balance tốt giữa simplicity và flexibility
- Migration dễ, không cần thay đổi existing data
