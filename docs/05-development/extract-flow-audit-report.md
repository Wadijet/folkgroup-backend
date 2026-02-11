# Báo cáo kiểm tra hệ thống Extract Data qua Struct Tags

## Tổng quan

Hệ thống **extract** dùng struct tag `extract:"SourcePath\\.field..."` để tự động trích xuất dữ liệu từ các trường map (PanCakeData, PosData, …) vào các trường typed trước khi marshal ra MongoDB.

---

## 1. Điểm Hook

### 1.1 Hook chính: `utility.ToMap()`

**File:** `api/internal/utility/data.bson.go`

```go
func ToMap(s interface{}) (map[string]interface{}, error) {
    // Nếu là value struct → tạo pointer để extract
    if val.Kind() != reflect.Ptr && val.Kind() == reflect.Struct {
        ptrVal = reflect.New(val.Type())
        ptrVal.Elem().Set(val)
        toMarshal = ptrVal.Interface()
    }
    // ← HOOK: Extract chạy trước khi marshal
    if err := ExtractDataIfExists(toMarshal); err != nil {
        return nil, fmt.Errorf("extract data failed: %w", err)
    }
    // Marshal struct → map
    itr, err := bson.Marshal(toMarshal)
    // ...
}
```

### 1.2 Logic Extract: `utility.ExtractDataIfExists()`

**File:** `api/internal/utility/data.extract.go`

- Duyệt các field của struct, đọc tag `extract`
- Lấy giá trị từ source (ví dụ `PosData`, `PanCakeData`) theo đường dẫn
- Convert theo `converter` (int64, time, string, number, …)
- Gán vào field đích và modify struct qua pointer

**Lưu ý:** Extract chỉ chạy khi input là **pointer tới struct**. Nếu input là value hoặc map, hàm return `nil` và không làm gì.

---

## 2. Các điểm gọi ToMap (nơi extract có thể chạy)

| Vị trí | File | Khi nào extract chạy |
|--------|------|----------------------|
| **InsertOne** | `service.base.mongo.go:196` | `utility.ToMap(data)` – data là **struct** Model |
| **InsertMany** | `service.base.mongo.go:247` | `utility.ToMap(item)` – item là **struct** Model |
| **ToUpdateData** | `service.base.mongo.go:64` | `utility.ToMap(data)` – khi data **không** phải `*UpdateData`, `UpdateData`, `[]byte` |
| **UpdateOne** | `service.base.mongo.go:380` | Qua `ToUpdateData(update)` |
| **UpdateMany** | `service.base.mongo.go:443` | Qua `ToUpdateData(update)` |
| **FindOneAndUpdate** | `service.base.mongo.go:571` | Qua `ToUpdateData(update)` |
| **Upsert** | `service.base.mongo.go:950` | Qua `ToUpdateData(data)` |
| **UpsertMany** | `service.base.mongo.go:1277` | `utility.ToMap(item)` – item là **struct** Model |
| **SyncFlattenedFromPosData** | `service.pc.pos.order.go:75` | `utility.ToMap(&order)` – gọi trực tiếp |
| **BsonWrapper.Set / Push / Unset / AddToSet** | `data.bson.go:91–116` | `ToMap(BsonWrapper{Set: data})` – extract chạy trên **BsonWrapper**, không phải `data` bên trong |
| **MyMapDiff** | `data.mapdiff.go:29,33` | `ToMap(oldVal)`, `ToMap(newVal)` – dùng cho map con, extract bỏ qua nếu là map |

---

## 3. Điều kiện để extract thực sự chạy

1. Data phải là **struct** (có tag `extract`) – không phải map / `*UpdateData` / `[]byte`.
2. Data phải đi qua `utility.ToMap()` – tức là qua InsertOne, InsertMany, ToUpdateData, Upsert, UpsertMany.

Khi gửi **bson.M** (hoặc map) vào ToUpdateData:

- `ToUpdateData(bson.M{"$set": {...}})` → gọi `ToMap(bson.M)`
- `ExtractDataIfExists(map)` → return `nil` ngay (không phải struct) → extract không chạy.

---

## 4. Phân tích theo domain

### 4.1 PcPosOrder / PcPosCustomer (PosData)

**Models có extract tags:** `model.pc.pos.order.go`, `model.pc.pos.customer.go`

| Luồng | Extract chạy? | Ghi chú |
|-------|----------------|---------|
| API InsertOne (tạo order/customer) | Có | Data là struct → ToMap → extract chạy |
| API Upsert (upsert order/customer) | Có | Data là struct → ToUpdateData → ToMap → extract chạy |
| API UpdateById với struct | Có | Update là struct → ToUpdateData → ToMap → extract chạy |
| SyncFlattenedFromPosData | Có | Gọi trực tiếp `ExtractDataIfExists(&order)` rồi `ToMap` |
| Webhook Pancake POS | Không | Handler chỉ lưu webhook_log, không ghi vào pc_pos_orders/customers |

Kết luận: Extract hoạt động đúng với luồng API CRUD của PcPosOrder và PcPosCustomer.

---

### 4.2 PcOrder / FbCustomer / FbPage / FbConversation (PanCakeData)

**Models có extract tags:** `model.pc.order.go`, `model.fb.customer.go`, `model.fb.page.go`, `model.fb.conversation.go`

| Luồng | Extract chạy? | Ghi chú |
|-------|----------------|---------|
| Webhook Pancake (order_created/updated) | Không | `handleOrderEvent` dùng `bson.M{"$set": {...}}` trực tiếp |
| Webhook Pancake (customer_updated) | Không | `handleCustomerEvent` dùng `bson.M{"$set": {...}}` trực tiếp |
| Webhook Pancake (conversation_updated) | Không | Dùng `bson.M{"$set": {...}}` trực tiếp |
| API InsertOne/Upsert với struct | Có | Data là struct → ToMap → extract chạy |

Webhook hiện tại:

- Gọi `FindOneAndUpdate` với `update = bson.M{"$set": bson.M{"panCakeData": data, "updatedAt": now}, "$setOnInsert": {...}}`
- Không tạo struct (PcOrder, FbCustomer, …) có `PanCakeData` rồi gọi ToMap
- Document chỉ nhận `panCakeData` (raw map), các field flatten (customerId, name, pageId, …) không được extract

Kết luận: Extract không chạy trong luồng webhook Pancake; muốn flatten thì phải đổi webhook sang dùng struct + Upsert (tương tự PcPosOrder).

---

## 5. Trường hợp đặc biệt

### 5.1 BsonWrapper / CustomBson

`CustomBson.Set(data)`, `Push(data)`, … gọi `ToMap(BsonWrapper{Set: data})`.

- Extract chạy trên struct `BsonWrapper`, không phải trên `data` bên trong.
- `BsonWrapper` không có tag `extract` → extract không làm gì hữu ích.
- Dù `data` có extract tags, nó nằm trong field `Set` nên không bao giờ được extract.

Kết luận: Khi dùng `BsonWrapper` với struct có extract tags, extract không chạy trên struct đó.

### 5.2 UpdateData / []byte

`ToUpdateData` trả về sớm nếu data là `*UpdateData`, `UpdateData` hoặc `[]byte`:

- Không gọi `ToMap` → không có extract.
- Đây là hành vi mong muốn: UpdateData là cấu trúc update đã chuẩn bị, không cần extract.

---

## 6. Tóm tắt

| Luồng | Extract chạy? |
|-------|----------------|
| InsertOne / InsertMany | Có (data là struct) |
| Upsert / UpsertMany | Có (data là struct) |
| UpdateOne / UpdateById / UpdateMany / Upsert (CRUD handler) | Có (đã sửa: dùng utility.ToMap thay bson.Marshal) |
| UpdateOne / … với bson.M hoặc UpdateData (từ code khác) | Không |
| Webhook Pancake (order, customer, conversation) | Không (dùng bson.M) |
| Webhook Pancake POS | N/A (chỉ lưu webhook_log) |
| PcPosOrder / PcPosCustomer qua API | Có |
| SyncFlattenedFromPosData | Có |

---

## 7. Khuyến nghị

1. Nếu cần flatten từ PanCakeData trong webhook Pancake: đổi `handleOrderEvent`, `handleCustomerEvent`, `handleConversationEvent` sang tạo struct (PcOrder, FbCustomer, FbConversation) với `PanCakeData` rồi gọi `Upsert` thay vì `FindOneAndUpdate` với bson.M.
2. `extract-flow-verification.md` mô tả luồng PcPosOrder.Upsert là chuẩn; cần ghi rõ webhook Pancake (không phải Pancake POS) hiện dùng bson.M nên extract không chạy.
3. Cập nhật docs để phân biệt rõ:
   - Pancake webhook: bson.M → không extract
   - Pancake POS webhook: chỉ lưu log
   - API CRUD / Upsert với struct: extract chạy đầy đủ
