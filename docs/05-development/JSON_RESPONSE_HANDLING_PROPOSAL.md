# Đề Xuất Xử Lý JSON Response và Fix Cứng Prompt

## Vấn Đề

1. **AI có thể trả về JSON lỗi** (malformed JSON, incomplete JSON, etc.)
2. **User có thể sửa prompt** → vỡ cấu trúc JSON requirement → system không parse được

## Giải Pháp

### 1. Tách Prompt Thành 2 Phần

#### Phần User-Editable (Prompt Body)
- User có thể sửa: mô tả nhiệm vụ, yêu cầu, context
- Lưu trong field `Prompt` của `AIPromptTemplate`

#### Phần System-Fixed (JSON Requirement)
- **KHÔNG cho user sửa**: Phần yêu cầu JSON format
- **System tự append** vào cuối prompt khi render
- Không lưu trong database, chỉ append khi build prompt

### 2. Cấu Trúc Prompt Template

```go
type AIPromptTemplate struct {
    // ... existing fields ...
    Prompt string  // User-editable prompt body (KHÔNG chứa JSON requirement)
    
    // System tự append JSON requirement dựa trên step type
    // Không cần field mới, chỉ cần logic trong render function
}
```

### 3. Logic Render Prompt

```go
func RenderPrompt(template AIPromptTemplate, variables map[string]interface{}, stepType string) (string, error) {
    // 1. Render user prompt với variables
    userPrompt := renderTemplate(template.Prompt, variables)
    
    // 2. System tự append JSON requirement (FIX CỨNG - không cho user sửa)
    jsonRequirement := getJSONRequirementForStepType(stepType)
    
    // 3. Combine
    finalPrompt := userPrompt + "\n\n" + jsonRequirement
    
    return finalPrompt, nil
}

func getJSONRequirementForStepType(stepType string) string {
    switch stepType {
    case "GENERATE":
        return `📤 ĐỊNH DẠNG KẾT QUẢ (BẮT BUỘC - JSON):
Bạn PHẢI trả về JSON format chính xác như sau (không được thay đổi cấu trúc):
{
  "text": "Nội dung chính đầy đủ...",
  "name": "Tên ngắn gọn (tùy chọn)",
  "summary": "Tóm tắt ngắn gọn (tùy chọn)"
}

LƯU Ý:
- Chỉ trả về JSON, không có text thêm trước hoặc sau
- Field "text" là BẮT BUỘC
- Field "name" và "summary" là tùy chọn
- Đảm bảo JSON hợp lệ, có thể parse được`

    case "JUDGE":
        return `📤 ĐỊNH DẠNG KẾT QUẢ (BẮT BUỘC - JSON):
Bạn PHẢI trả về JSON format chính xác như sau:
{
  "score": 8.5,
  "feedback": "Nhận xét chi tiết...",
  "criteriaScores": {
    "relevance": 9,
    "clarity": 8,
    "engagement": 8.5
  }
}

LƯU Ý:
- Chỉ trả về JSON, không có text thêm
- Field "score" là BẮT BUỘC (số từ 0-10)
- Đảm bảo JSON hợp lệ`

    default:
        return ""
    }
}
```

### 4. Xử Lý JSON Parsing Errors

#### Strategy 1: Robust Parsing với Fallback
```go
func ParseAIResponse(responseText string, stepType string) (map[string]interface{}, error) {
    // 1. Try parse JSON trực tiếp
    var result map[string]interface{}
    if err := json.Unmarshal([]byte(responseText), &result); err == nil {
        return result, nil
    }
    
    // 2. Try extract JSON từ markdown code block
    if jsonStr := extractJSONFromMarkdown(responseText); jsonStr != "" {
        if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
            return result, nil
        }
    }
    
    // 3. Try extract JSON từ text (tìm { ... } đầu tiên)
    if jsonStr := extractFirstJSONObject(responseText); jsonStr != "" {
        if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
            return result, nil
        }
    }
    
    // 4. Fallback: Nếu không parse được JSON, trả về plain text
    return map[string]interface{}{
        "text": responseText,  // Lưu toàn bộ response làm text
        "parseError": "Không thể parse JSON, sử dụng plain text",
        "rawResponse": responseText,
    }, nil
}
```

#### Strategy 2: Retry với Clarification
```go
func ParseAIResponseWithRetry(responseText string, stepType string, aiService AIService) (map[string]interface{}, error) {
    result, err := ParseAIResponse(responseText, stepType)
    if err == nil && result["text"] != nil {
        return result, nil
    }
    
    // Nếu parse lỗi, gửi lại prompt yêu cầu JSON
    clarificationPrompt := `Response của bạn không phải JSON hợp lệ. 
Vui lòng trả về JSON format chính xác như yêu cầu:
` + getJSONRequirementForStepType(stepType)
    
    retryResponse, err := aiService.Call(clarificationPrompt)
    if err != nil {
        return nil, err
    }
    
    return ParseAIResponse(retryResponse, stepType)
}
```

#### Strategy 3: Validation và Error Reporting
```go
func ValidateParsedOutput(parsed map[string]interface{}, stepType string) error {
    switch stepType {
    case "GENERATE":
        if text, ok := parsed["text"].(string); !ok || text == "" {
            return fmt.Errorf("Field 'text' là bắt buộc nhưng thiếu hoặc rỗng")
        }
        // name và summary là optional, không cần validate
    case "JUDGE":
        if score, ok := parsed["score"].(float64); !ok {
            return fmt.Errorf("Field 'score' là bắt buộc nhưng thiếu hoặc không phải số")
        }
        if score < 0 || score > 10 {
            return fmt.Errorf("Score phải từ 0-10, nhận được: %f", score)
        }
    }
    return nil
}
```

### 5. Cấu Trúc Code

```
api/core/api/services/
  service.ai.prompt.template.go
    - RenderPrompt() - Render user prompt + append JSON requirement
    - getJSONRequirementForStepType() - Get system-fixed JSON requirement
    
  service.ai.response.parser.go (NEW)
    - ParseAIResponse() - Parse JSON với fallback strategies
    - extractJSONFromMarkdown() - Extract JSON từ ```json ... ```
    - extractFirstJSONObject() - Extract JSON object đầu tiên
    - ValidateParsedOutput() - Validate parsed output theo schema
    
  service.ai.run.go
    - CreateAIRun() - Tạo AI run
    - ExecuteAIRun() - Execute và parse response
      - Call AI API
      - Parse response với ParseAIResponse()
      - Validate với ValidateParsedOutput()
      - Retry nếu cần
```

### 6. Error Handling Flow

```
1. AI trả về response
   ↓
2. ParseAIResponse() - Try parse JSON
   ↓
3. Nếu parse thành công → ValidateParsedOutput()
   ↓
4. Nếu validate OK → Return parsed output
   ↓
5. Nếu parse/validate lỗi:
   - Log error với raw response
   - Try fallback strategies (extract from markdown, extract first JSON)
   - Nếu vẫn lỗi → Return error với raw response
   - Option: Retry với clarification prompt
```

### 7. Lưu Ý

1. **JSON Requirement là System-Fixed**: User không thể sửa, system tự append
2. **Robust Parsing**: Nhiều fallback strategies để handle edge cases
3. **Error Logging**: Log đầy đủ raw response khi parse lỗi để debug
4. **Validation**: Validate parsed output theo schema trước khi sử dụng
5. **Retry Strategy**: Có thể retry với clarification prompt nếu cần (optional)

### 8. Ví Dụ Implementation

```go
// service.ai.prompt.template.go
func (s *AIPromptTemplateService) RenderPrompt(
    ctx context.Context,
    template models.AIPromptTemplate,
    variables map[string]interface{},
    stepType string,
) (string, error) {
    // 1. Render user prompt
    userPrompt, err := s.renderTemplate(template.Prompt, variables)
    if err != nil {
        return "", err
    }
    
    // 2. System append JSON requirement (FIX CỨNG)
    jsonRequirement := s.getJSONRequirementForStepType(stepType)
    
    // 3. Combine
    finalPrompt := userPrompt
    if jsonRequirement != "" {
        finalPrompt += "\n\n" + jsonRequirement
    }
    
    return finalPrompt, nil
}

func (s *AIPromptTemplateService) getJSONRequirementForStepType(stepType string) string {
    // System-fixed JSON requirements - không cho user sửa
    requirements := map[string]string{
        models.AIStepTypeGenerate: `📤 ĐỊNH DẠNG KẾT QUẢ (BẮT BUỘC - JSON):
Bạn PHẢI trả về JSON format chính xác như sau:
{
  "text": "Nội dung chính đầy đủ...",
  "name": "Tên ngắn gọn (tùy chọn)",
  "summary": "Tóm tắt ngắn gọn (tùy chọn)"
}

LƯU Ý:
- Chỉ trả về JSON, không có text thêm trước hoặc sau
- Field "text" là BẮT BUỘC
- Field "name" và "summary" là tùy chọn
- Đảm bảo JSON hợp lệ, có thể parse được`,
        
        models.AIStepTypeJudge: `📤 ĐỊNH DẠNG KẾT QUẢ (BẮT BUỘC - JSON):
Bạn PHẢI trả về JSON format chính xác như sau:
{
  "score": 8.5,
  "feedback": "Nhận xét chi tiết...",
  "criteriaScores": {
    "relevance": 9,
    "clarity": 8,
    "engagement": 8.5
  }
}

LƯU Ý:
- Chỉ trả về JSON, không có text thêm
- Field "score" là BẮT BUỘC (số từ 0-10)
- Đảm bảo JSON hợp lệ`,
    }
    
    return requirements[stepType]
}
```

```go
// service.ai.response.parser.go (NEW)
func ParseAIResponse(responseText string, stepType string) (map[string]interface{}, error) {
    // Strategy 1: Parse JSON trực tiếp
    var result map[string]interface{}
    if err := json.Unmarshal([]byte(responseText), &result); err == nil {
        if validateParsedOutput(result, stepType) == nil {
            return result, nil
        }
    }
    
    // Strategy 2: Extract từ markdown code block
    if jsonStr := extractJSONFromMarkdown(responseText); jsonStr != "" {
        if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
            if validateParsedOutput(result, stepType) == nil {
                return result, nil
            }
        }
    }
    
    // Strategy 3: Extract JSON object đầu tiên
    if jsonStr := extractFirstJSONObject(responseText); jsonStr != "" {
        if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
            if validateParsedOutput(result, stepType) == nil {
                return result, nil
            }
        }
    }
    
    // Strategy 4: Fallback - return plain text
    return map[string]interface{}{
        "text": responseText,
        "parseError": "Không thể parse JSON, sử dụng plain text",
        "rawResponse": responseText,
    }, nil
}

func extractJSONFromMarkdown(text string) string {
    // Tìm ```json ... ``` hoặc ``` ... ```
    re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n(.*?)\\n```")
    matches := re.FindStringSubmatch(text)
    if len(matches) > 1 {
        return strings.TrimSpace(matches[1])
    }
    return ""
}

func extractFirstJSONObject(text string) string {
    // Tìm { ... } đầu tiên
    start := strings.Index(text, "{")
    if start == -1 {
        return ""
    }
    
    depth := 0
    for i := start; i < len(text); i++ {
        if text[i] == '{' {
            depth++
        } else if text[i] == '}' {
            depth--
            if depth == 0 {
                return text[start : i+1]
            }
        }
    }
    return ""
}
```

## Kết Luận

1. **Tách prompt**: User-editable body + System-fixed JSON requirement
2. **Robust parsing**: Nhiều fallback strategies
3. **Validation**: Validate parsed output trước khi sử dụng
4. **Error handling**: Log đầy đủ, fallback về plain text nếu cần
5. **Không cho user sửa JSON requirement**: System tự append, đảm bảo cấu trúc nhất quán
