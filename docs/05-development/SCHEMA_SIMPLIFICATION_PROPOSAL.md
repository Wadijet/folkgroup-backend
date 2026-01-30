# Đề Xuất Đơn Giản Hóa Schema: Text + Metadata

## Nguyên Tắc: 2 Lớp Dữ Liệu

### Lớp AI (Inner Layer)
- **AI chỉ biết**: Input = TEXT, Output = TEXT
- AI không biết gì về structure, schema, metadata, parentId, targetLevel, etc.
- AI chỉ nhận prompt (text) và trả về response (text)

### Lớp Logic (Outer Layer)
- **System tự xử lý**:
  - Tự lấy parent node từ database (không cần đưa parentId cho AI)
  - Tự build prompt từ parent node data (không cần đưa parentType, targetLevel cho AI)
  - Tự parse AI response text → structured data (không cần AI trả về JSON)
  - Tự tạo draft node từ structured data (không cần AI biết về node structure)

## Phân Tích Hiện Tại

### ContentNode/DraftContentNode
- **Text** (string, required): Nội dung chính của node
- **Name** (string, optional): Tên node
- **Metadata** (object, optional): Metadata bổ sung

### Step Output Hiện Tại
- `content` (string) → map vào `Text` của node
- `title` (string) → có thể map vào `Name` của node
- `summary` (string) → có thể lưu trong `Metadata`
- `metadata` (object) → merge vào `Metadata` của node
- `generatedAt`, `model`, `tokens` → system metadata (lưu trong step run, không cần trong output)

## Đề Xuất Đơn Giản Hóa

### 1. GENERATE Step Output
**Hiện tại:**
```json
{
  "content": "...",
  "title": "...",
  "summary": "...",
  "metadata": {...},
  "generatedAt": "...",
  "model": "...",
  "tokens": {...}
}
```

**Đề xuất (yêu cầu AI trả về JSON):**
```json
{
  "text": "...",           // Nội dung chính (REQUIRED) - map vào node.Text
  "name": "...",          // Tên node (optional) - map vào node.Name
  "summary": "..."        // Tóm tắt (optional) - lưu vào node.Metadata.summary
}
```

**Lý do:**
- **Yêu cầu AI trả về JSON format** để dễ parse và extract `text`, `name`, `summary`
- Node có field `Name` (optional) → cần `name` từ AI
- Node có field `Text` (required) → cần `text` từ AI
- Node có field `Metadata` (optional) → `summary` lưu vào `node.Metadata.summary`
- System metadata (`generatedAt`, `model`, `tokens`) lưu trong step run, không cần trong output
- Prompt template sẽ yêu cầu AI: "Trả về JSON format: {text: '...', name: '...', summary: '...'}"

### 2. GENERATE Step Input
**Hiện tại:**
```json
{
  "pillarId": "...",
  "pillarName": "...",
  "pillarDescription": "...",
  "targetAudience": "B2B",
  "context": {
    "industry": "...",
    "productType": "...",
    "tone": "..."
  }
}
```

**Đề xuất (đơn giản - chỉ text + metadata):**
```json
{
  "parentText": "...",         // Text của parent node (REQUIRED nếu có parent)
  "metadata": {                // Metadata tùy chọn (optional)
    "targetAudience": "B2B",
    "tone": "..."
  }
}
```

**Lý do:**
- **System tự lấy parent node** từ database (dựa trên step config: ParentLevel, TargetLevel)
- **System tự build prompt** từ parent node data (parentNode.Text, parentNode.Type, etc.)
- **Không cần đưa parentId, parentType, targetLevel cho AI** - system tự biết
- Chỉ cần `parentText` để system build prompt
- `metadata` optional - nếu cần thêm context cho prompt (targetAudience, tone, etc.)

### 3. JUDGE Step Input
**Hiện tại:**
```json
{
  "content": "...",
  "title": "...",
  "summary": "...",
  "criteria": {...},
  "context": {...}
}
```

**Đề xuất:**
```json
{
  "text": "...",              // Text cần đánh giá (map từ GENERATE output.text)
  "criteria": {...},          // Tiêu chí đánh giá (system tự lấy từ step config)
  "metadata": {                // Metadata tùy chọn (optional)
    "title": "...",
    "summary": "..."
  }
}
```

**Lý do:**
- Chỉ cần `text` để đánh giá - AI chỉ cần text
- `criteria` system tự lấy từ step config (không cần AI biết)
- `metadata` optional - nếu cần thêm context cho prompt

### 4. JUDGE Step Output
**Hiện tại:**
```json
{
  "score": 8.5,
  "criteriaScores": {...},
  "feedback": "...",
  "judgedAt": "..."
}
```

**Đề xuất:**
```json
{
  "score": 8.5,               // Điểm tổng thể
  "metadata": {                // Metadata tùy chọn
    "criteriaScores": {...},
    "feedback": "..."
  }
}
```

**Lý do:**
- `score` là kết quả chính
- `criteriaScores`, `feedback` có thể lưu trong `metadata`
- `judgedAt` lưu trong step run (system metadata)

## So Sánh

### Trước (phức tạp):
- GENERATE output: `content`, `title`, `summary`, `metadata`, `generatedAt`, `model`, `tokens`
- JUDGE input: `content`, `title`, `summary`, `criteria`, `context`
- JUDGE output: `score`, `criteriaScores`, `feedback`, `judgedAt`

### Sau (đơn giản):
- GENERATE input: `parentText` + `metadata` (optional)
- GENERATE output: `text` (required) + `name` (optional) + `summary` (optional) - **AI trả về JSON**
- JUDGE input: `text` + `criteria` + `metadata` (optional)
- JUDGE output: `score` (required) + `metadata` (optional) - **AI trả về JSON**

## Lợi Ích

1. **Đơn giản hóa**: Chỉ cần `text` thay vì `content`, `title`, `summary`
2. **Linh hoạt**: Metadata có thể chứa bất kỳ field nào nếu cần
3. **Nhất quán**: Tất cả đều dùng `text` (giống node.Text)
4. **Dễ mapping**: `text` → `node.Text` trực tiếp

## Lưu Ý

### System Tự Xử Lý (Không Cần Trong Schema)
- **Lấy parent node**: System tự query từ database dựa trên step config (ParentLevel, TargetLevel)
- **Build prompt**: System tự build từ parent node data + prompt template
- **Parse AI response**: System tự parse text response → structured data (text, metadata)
- **Tạo draft node**: System tự tạo từ structured data
- **System metadata**: `generatedAt`, `model`, `tokens`, `judgedAt` lưu trong step run, không cần trong output

### Schema Chỉ Cho AI Layer
- **GENERATE Input**: `parentText` (system tự lấy từ DB) + `metadata` (optional)
- **GENERATE Output**: `text` (required) + `name` (optional) + `summary` (optional) - **AI trả về JSON format**
- **JUDGE Input**: `text` (system lấy từ GENERATE output) + `criteria` (system tự lấy) + `metadata` (optional)
- **JUDGE Output**: `score` (required) + `metadata` (optional) - **AI trả về JSON format**

### AI Response Format
- **Yêu cầu AI trả về JSON**: Prompt template sẽ yêu cầu AI trả về JSON format
- **System parse JSON**: System tự parse JSON response → extract `text`, `name`, `summary`
- **Map vào node**: 
  - `text` → `node.Text`
  - `name` → `node.Name`
  - `summary` → `node.Metadata.summary`

### Metadata
- Metadata là optional, có thể để trống nếu không cần
- Prompt template có thể dùng các field trong metadata để render prompt
- Metadata có thể chứa bất kỳ field nào: title, summary, targetAudience, tone, etc.
