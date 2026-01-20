# Tag `extract` - Phương Án Cuối Cùng

## Tổng Quan

Tag `extract` dùng để tự động trích xuất dữ liệu từ các field `map[string]interface{}` (như `PanCakeData`, `FacebookData`, `ShopifyData`) vào các field typed của struct.

**Hook Point**: `utility.ToMap()` - được gọi trước khi convert struct sang map để lưu MongoDB.

---

## Format

```
extract:"[<source_field>][\.<nested_path>][,converter=<name>][,format=<value>][,default=<value>][,optional|required]"
```

**Lưu ý**: 
- **Dùng `\.` (backslash + dot) để phân tách các level** (bắt buộc)
- **Dấu chấm (`.`) mặc định là literal trong field name** (không phải phân tách)
- Traverse đến level cuối cùng và lấy giá trị từ đó

---

## Các Thành Phần

### 1. Source Path (Bắt Buộc) - Đường Dẫn Đến Giá Trị

**Format**: `[<source_field>][\.<nested_path>]`

**Mặc định**: Nếu không có field name, mặc định là `PanCakeData`

**Mục đích**: Chỉ định đường dẫn từ field trong struct đến giá trị cần extract. Traverse đến level cuối cùng và lấy giá trị.

**Quy tắc**:
- **Dùng `\.` (backslash + dot) để phân tách các level** (bắt buộc cho tất cả phân cách)
- **Dấu chấm (`.`) mặc định là literal trong field name** (không phải phân tách)
- Traverse từ field → nested maps → level cuối cùng (lấy giá trị)
- Nếu không có field name (bắt đầu bằng `\.` hoặc không có), mặc định là `PanCakeData`
- Escape: `\\.` → literal backslash + dot (nếu cần), `\\` → literal backslash

**Ví dụ**:
```go
// Simple: key trong PanCakeData (mặc định)
PageId string `extract:"id"`                              // PanCakeData["id"]

// Nested trong PanCakeData (dùng \. để phân tách level)
UserName string `extract:"user\.name"`                     // PanCakeData["user"]["name"]

// Field name có dấu chấm (dấu chấm là literal, không escape)
EscapedField string `extract:"user.name"`                 // PanCakeData["user.name"]

// Với source field khác
FacebookPageId string `extract:"FacebookData\.id"`         // FacebookData["id"]

// Source nested: Data\.pancake\.api → traverse đến level cuối cùng
PancakePageId string `extract:"Data\.pancake\.api"`        
// Data (field) → Data["pancake"] → Data["pancake"]["api"]

// Field name có dấu chấm trong nested path
ComplexField string `extract:"Data\.pancake.api\.user.name"`  
// Data["pancake.api"]["user.name"] 
//   - "pancake.api" → field name có dấu chấm (literal)
//   - "user.name" → field name có dấu chấm (literal)

// Deep nested
ComplexPath string `extract:"Data\.pancake\.api\.user\.name"`  
// Data["pancake"]["api"]["user"]["name"]
```

**Công thức**:
- `extract:"id"` → `PanCakeData["id"]`
- `extract:"user\.name"` → `PanCakeData["user"]["name"]` (dùng `\.` để phân tách)
- `extract:"user.name"` → `PanCakeData["user.name"]` (dấu chấm là literal trong field name)
- `extract:"Data\.pancake\.api"` → `Data["pancake"]["api"]`

---

### 3. Converter (Optional)

**Default**: `string`

**Có sẵn**:
- `converter=time` - Parse time string → int64 timestamp
- `converter=number` - Convert json.Number/string → string/int64
- `converter=string` - Convert bất kỳ → string (default)
- `converter=int64` - Convert → int64
- `converter=bool` - Convert → bool

**Ví dụ**:
```go
UpdatedAt int64 `extract:"updated_at,converter=time"`
CreatedAt int64 `extract:"created_at,converter=time,format=2006-01-02"`
OrderId string `extract:"id,converter=number"`
Count int64 `extract:"count,converter=int64"`
IsActive bool `extract:"is_active,converter=bool"`
```

---

### 4. Options

#### `format=<time_format>`
Format cho time converter (default: `2006-01-02T15:04:05`)

#### `default=<value>`
Giá trị mặc định nếu path không tồn tại

**Ví dụ**:
```go
Status string `extract:"status,default=active"`
Count int64 `extract:"count,default=0"`
IsActive bool `extract:"is_active,default=true"`
```

---

### 5. Flags

#### `optional`
Skip field nếu không tồn tại (không error, giữ zero value)

#### `required`
Bắt buộc phải có giá trị (error nếu không tồn tại)

**Mặc định**: Không required (skip nếu không tồn tại)

**Ví dụ**:
```go
OptionalField string `extract:"optional_field,optional"`
PageId string `extract:"id,required"`
```

---

## Ví Dụ Phân Tích Source Path

### Ví Dụ 1: Simple

```go
type Model struct {
    PanCakeData map[string]interface{} `json:"panCakeData" bson:"panCakeData"`
    PageId      string                  `extract:"id"`
}
```

**Giải thích**:
- Source path: `"id"` (mặc định từ `PanCakeData`)
- Traverse: `PanCakeData["id"]`
- **Kết quả**: `PanCakeData["id"]` → `PageId`

---

### Ví Dụ 2: Nested

```go
type Model struct {
    PanCakeData map[string]interface{} `json:"panCakeData" bson:"panCakeData"`
    UserName    string                 `extract:"user\.name"`
}
```

**Giải thích**:
- Source path: `user\.name` (mặc định từ `PanCakeData`, dùng `\.` để phân tách)
- Traverse: `PanCakeData["user"]` → `PanCakeData["user"]["name"]`
- **Kết quả**: `PanCakeData["user"]["name"]` → `UserName`

---

### Ví Dụ 3: Với Source Field Khác

```go
type Model struct {
    PanCakeData  map[string]interface{} `json:"panCakeData" bson:"panCakeData"`
    FacebookData map[string]interface{} `json:"facebookData" bson:"facebookData"`
    
    PancakePageId  string `extract:"id"`                    // PanCakeData["id"]
    FacebookPageId string `extract:"FacebookData\.id"`       // FacebookData["id"]
}
```

**Giải thích**:
- `extract:"id"`:
  - Source path: `id` → mặc định `PanCakeData.id`
  - Traverse: `PanCakeData["id"]`
  - **Kết quả**: `PanCakeData["id"]` → `PancakePageId`
  
- `extract:"FacebookData\.id"`:
  - Source path: `FacebookData\.id`
  - Traverse: `FacebookData` (field) → `FacebookData["id"]`
  - **Kết quả**: `FacebookData["id"]` → `FacebookPageId`

---

### Ví Dụ 4: Source Nested

```go
type Model struct {
    Data map[string]interface{} `json:"data" bson:"data"`
    
    PageId string `extract:"Data\.pancake\.api"`
}
```

**Giải thích**:
- Source path: `Data\.pancake\.api` (dùng `\.` để phân tách)
- Traverse: `Data` (field) → `Data["pancake"]` → `Data["pancake"]["api"]`
- **Kết quả**: `Data["pancake"]["api"]` → `PageId`

---

### Ví Dụ 5: Deep Nested

```go
type Model struct {
    Data map[string]interface{} `json:"data" bson:"data"`
    
    UserName string `extract:"Data\.pancake\.api\.user\.name"`
}
```

**Giải thích**:
- Source path: `Data\.pancake\.api\.user\.name` (dùng `\.` để phân tách)
- Traverse: `Data` → `Data["pancake"]` → `Data["pancake"]["api"]` → `Data["pancake"]["api"]["user"]` → `Data["pancake"]["api"]["user"]["name"]`
- **Kết quả**: `Data["pancake"]["api"]["user"]["name"]` → `UserName`

---

## Ví Dụ Đầy Đủ

```go
type FbPage struct {
    ID              primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
    
    // Simple extraction
    PageId          string                 `extract:"id"`
    PageName        string                 `extract:"name"`
    PageUsername    string                 `extract:"username"`
    
    // With converter
    PanCakeUpdatedAt int64                `extract:"updated_at,converter=time"`
    
    // With default
    Status          string                 `extract:"status,default=active"`
    
    // Source field
    PanCakeData     map[string]interface{} `json:"panCakeData" bson:"panCakeData"`
    CreatedAt       int64                 `json:"createdAt" bson:"createdAt"`
    UpdatedAt       int64                 `json:"updatedAt" bson:"updatedAt"`
}
```

```go
type UnifiedOrder struct {
    // PanCakeData (default)
    PancakeOrderId string `extract:"id"`
    PancakeStatus  string `extract:"status"`
    
    // ShopifyData
    ShopifyOrderId string  `extract:"ShopifyData\.id"`
    ShopifyTotal   float64 `extract:"ShopifyData\.total_price,converter=number"`
    
    // FacebookData
    FacebookPageId    string `extract:"FacebookData\.page_id"`
    FacebookUpdatedAt int64  `extract:"FacebookData\.updated_at,converter=time"`
    
    // Source fields
    PanCakeData  map[string]interface{} `json:"panCakeData" bson:"panCakeData"`
    ShopifyData  map[string]interface{} `json:"shopifyData" bson:"shopifyData"`
    FacebookData map[string]interface{} `json:"facebookData" bson:"facebookData"`
}
```

---

## Quy Tắc Phân Tách Level

**Quy tắc**:
- **`\.` (backslash + dot) → Phân tách level** (bắt buộc cho tất cả các phân cách)
- **Dấu chấm (`.`) → Literal trong field name** (mặc định là literal, không phải phân tách)
- Escape: `\\.` → literal backslash + dot (nếu cần), `\\` → literal backslash

**Ví dụ**:
```go
// Nested map (dùng \. để phân tách level)
UserName string `extract:"user\.name"`                     
// → PanCakeData["user"]["name"] (2 levels, phân tách bằng \.)

// Field name có dấu chấm (dấu chấm là literal, không escape)
EscapedField string `extract:"user.name"`                
// → PanCakeData["user.name"] (1 field name có dấu chấm)

// Field name có nhiều dấu chấm (tất cả đều là literal)
ComplexField string `extract:"user.name.field"`         
// → PanCakeData["user.name.field"] (1 field name có nhiều dấu chấm)

// Mixed: nested (dùng \.) + field name có dấu chấm
MixedPath string `extract:"metadata\.user.name\.created.date"`  
// → PanCakeData["metadata"]["user.name"]["created.date"]
//   - "metadata" → nested map (phân tách bằng \.)
//   - "user.name" → field name có dấu chấm (literal)
//   - "created.date" → field name có dấu chấm (literal)

// Deep nested với field name có dấu chấm
DeepNested string `extract:"Data\.pancake.api\.user.name\.field"`  
// → Data["pancake.api"]["user.name"]["field"]
//   - "pancake.api" → field name có dấu chấm (literal)
//   - "user.name" → field name có dấu chấm (literal)
```

**Áp dụng cho**:
- ✅ Source field: `extract:"Data\.pancake.api"` → `Data["pancake.api"]` (dấu chấm là literal)
- ✅ Nested path: `extract:"user\.name"` → `PanCakeData["user"]["name"]` (dùng `\.` để phân tách)
- ✅ Field name có dấu chấm: `extract:"user.name"` → `PanCakeData["user.name"]` (dấu chấm là literal)
- ✅ Deep nested: `extract:"Data\.pancake.api\.user.name"` → `Data["pancake.api"]["user.name"]`

**Lưu ý**: 
- **Tất cả các phân cách level phải dùng `\.` (backslash + dot)**
- Dấu chấm (`.`) mặc định là literal trong field name, không cần escape

---

## Implementation

### Hook Point

Trong `utility.ToMap()`:
```go
func ToMap(s interface{}) (map[string]interface{}, error) {
    // Extract data từ source fields vào typed fields
    if err := extractDataIfExists(s); err != nil {
        return nil, err
    }
    
    // Convert sang map như bình thường
    var stringInterfaceMap map[string]interface{}
    itr, err := bson.Marshal(s)
    if err != nil {
        return nil, err
    }
    err = bson.Unmarshal(itr, &stringInterfaceMap)
    return stringInterfaceMap, err
}
```

### Parse Flow

1. **Parse Source Path**
   - Parse toàn bộ path với escape handling
   - Split bằng `\.` (backslash + dot) để phân tách level
   - Xác định field name (nếu có) hoặc dùng `PanCakeData` (mặc định)
   - Traverse đến level cuối cùng và lấy giá trị

2. **Parse Options**
   - Flags: `optional`, `required`
   - Options: `converter=`, `format=`, `default=`

3. **Extract & Convert**
   - Lấy value từ level cuối cùng
   - Apply converter nếu có
   - Set vào field

---

## Tóm Tắt

**Format**:
```
extract:"[<source_field>][\.<nested_path>][,converter=<name>][,format=<value>][,default=<value>][,optional|required]"
```

**Quy tắc phân tách**:
- `\.` (backslash + dot) → Phân tách level (bắt buộc cho tất cả các phân cách)
- Dấu chấm (`.`) → Literal trong field name (mặc định)

**Đặc điểm**:
- ✅ Hỗ trợ multiple sources (`PanCakeData`, `FacebookData`, `ShopifyData`, ...)
- ✅ Hỗ trợ nested path (`Data\.pancake\.api` - dùng `\.` để phân tách)
- ✅ Hỗ trợ field name có dấu chấm (`user.name` - dấu chấm là literal)
- ✅ Hỗ trợ converters (time, number, int64, bool, string)
- ✅ Hỗ trợ default values
- ✅ Hỗ trợ optional/required flags
- ✅ Hook tại `utility.ToMap()` - tự động cho tất cả CRUD operations
- ✅ **Đơn giản**: Dùng `\.` (backslash + dot) để phân tách level, dấu chấm (`.`) là literal trong field name

**Đủ mạnh để config mọi trường hợp sử dụng!**
