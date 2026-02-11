# Organization Config: Cấu trúc constraints và validation

## Mục đích

Field `constraints` dùng để **mô tả ràng buộc** cho giá trị config (enum, min/max, pattern, độ dài...) và **validate được** khi đọc/ghi (backend hoặc frontend kiểm tra value theo constraints).

---

## Cấu trúc đề xuất: JSON object (lưu dạng string)

Lưu `constraints` dưới dạng **chuỗi JSON**. Khi validate, parse JSON rồi áp dụng theo từng loại ràng buộc.

### Các loại ràng buộc theo `dataType`

| dataType | Ràng buộc hỗ trợ | Ví dụ JSON |
|----------|------------------|------------|
| **string** | `enum`, `pattern`, `minLength`, `maxLength` | `{"enum":["Asia/HCM","UTC"]}`, `{"pattern":"^[a-z]+$","maxLength":50}` |
| **number** | `enum`, `minimum`, `maximum`, `multipleOf` | `{"minimum":0,"maximum":100}`, `{"enum":[1,2,3]}` |
| **integer** | Giống number (coi là number nguyên) | `{"minimum":0,"maximum":10}` |
| **boolean** | `enum` (vd chỉ cho phép true/false) | Thường không cần |
| **array** | `minItems`, `maxItems`, `items` (schema phần tử) | `{"minItems":1,"maxItems":10}` |
| **object** | `minProperties`, `maxProperties` | `{"minProperties":1}` |

### Ví dụ đầy đủ

```json
{
  "enum": ["Asia/Ho_Chi_Minh", "UTC", "Europe/London"]
}
```

```json
{
  "minimum": 0,
  "maximum": 100
}
```

```json
{
  "pattern": "^[a-zA-Z0-9_-]+$",
  "minLength": 1,
  "maxLength": 64
}
```

```json
{
  "minLength": 1,
  "maxLength": 500
}
```

```json
{
  "enum": [1, 5, 10, 20, 50]
}
```

**Kết hợp nhiều ràng buộc (cùng lúc):**

```json
{
  "minimum": 0,
  "maximum": 100,
  "multipleOf": 5
}
```

---

## Quy ước tên field (để validate được)

| Field trong JSON | Ý nghĩa | Áp dụng cho |
|------------------|----------|-------------|
| `enum` | Mảng giá trị được phép | string, number, boolean |
| `minimum` | Giá trị số tối thiểu (>=) | number |
| `maximum` | Giá trị số tối đa (<=) | number |
| `multipleOf` | Số phải chia hết cho (vd 5 → 0,5,10...) | number |
| `minLength` | Độ dài chuỗi tối thiểu | string |
| `maxLength` | Độ dài chuỗi tối đa | string |
| `pattern` | Regex (ECMA 262), chuỗi phải khớp | string |
| `minItems` | Số phần tử tối thiểu | array |
| `maxItems` | Số phần tử tối đa | array |

- Nếu **không có** field tương ứng trong JSON thì **bỏ qua** ràng buộc đó (không bắt buộc phải có đủ tất cả).
- `pattern`: backend dùng `regexp.MustCompile` (hoặc Compile) với chuỗi đã cho; cần **escape** đúng khi lưu JSON (vd `"pattern": "^[a-z]+$"`).

---

## Validate ở đâu

1. **Backend (khi upsert config):**  
   Trước khi ghi DB, đọc `dataType` + `constraints` (parse JSON), so sánh `value` với từng ràng buộc (enum, min/max, pattern, length...) → trả lỗi 400 nếu không thỏa.

2. **Frontend (form/UI):**  
   Parse cùng cấu trúc JSON, validate khi user nhập (real-time hoặc lúc submit) để giảm request lỗi.

---

## Lưu trong DB

- **1 doc per org (hiện tại):** `configMeta[key].Constraints` là **string** (JSON). Ví dụ: `"{\"enum\":[\"Asia/HCM\",\"UTC\"]}"`.
- **1 doc per key (mô hình mới):** field `constraints` trong document là **string** (JSON). Khi trả API có thể parse và trả luôn object (để frontend dùng) hoặc giữ string.

**Lưu ý:** Nếu muốn query theo constraint (vd “tìm tất cả key có enum”) thì có thể lưu thêm field structured (object); còn chỉ để validate khi ghi/đọc thì string JSON là đủ.

---

## Dùng module validation (go-playground/validator)

Backend **dùng luôn** `global.Validate` (go-playground/validator) qua custom validator **`config_value`**.

### Đăng ký

- Trong `global.InitValidator()` đã đăng ký: `Validate.RegisterValidation("config_value", validateConfigValue)`.
- Struct **ConfigConstraints** và hàm **validateConfigValue** nằm trong `api/internal/global/validator.go`.

### Cách dùng

Struct cần **đúng 3 field** (tên cố định) để validator đọc được:

- **Value** — field cần validate, tag `validate:"config_value"`.
- **DataType** — string: `string`, `number`, `integer`, `boolean`, `array`, `object`.
- **Constraints** — chuỗi JSON (có thể rỗng = bỏ qua validate).

Ví dụ khi validate **một config item** (vd trước khi upsert):

```go
type ConfigValueForValidation struct {
	Value       interface{} `validate:"config_value"`
	DataType    string
	Constraints string
}

// Trước khi ghi DB:
item := ConfigValueForValidation{
	Value:       value,
	DataType:    dataType,
	Constraints: constraintsJSON, // từ meta/constraints
}
if err := global.Validate.Struct(item); err != nil {
	// trả 400, message từ err
}
```

- **Constraints rỗng** → validator bỏ qua (return true).
- **Constraints không phải JSON hợp lệ** → validator trả false (Struct() lỗi).
- Ràng buộc áp dụng: enum, minimum, maximum, multipleOf, minLength, maxLength, pattern, minItems, maxItems (theo dataType).

---

## Tóm tắt

| Câu hỏi | Trả lời |
|--------|--------|
| Constraints dùng cấu trúc gì? | **JSON object** lưu dạng **string**; các field: `enum`, `minimum`, `maximum`, `minLength`, `maxLength`, `pattern`, `minItems`, `maxItems`, `multipleOf`... theo quy ước trên. |
| Validate được không? | **Có.** Backend parse JSON → áp dụng ràng buộc lên `value` theo `dataType` khi ghi (và có thể dùng lại khi đọc); frontend parse cùng cấu trúc để validate form. |
