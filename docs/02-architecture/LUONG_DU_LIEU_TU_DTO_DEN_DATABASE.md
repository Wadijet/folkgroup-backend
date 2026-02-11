# Luồng dữ liệu từ DTO đến Database

Tài liệu mô tả toàn bộ luồng từ khi request body (DTO) được nhận cho đến khi dữ liệu được ghi vào MongoDB, và **những gì can thiệp vào data** trên từng bước.

---

## 1. Tổng quan luồng (InsertOne làm ví dụ)

```
Request Body (JSON)
    → ParseRequestBody (JSON decode + validate)
    → DTO (CreateInput)
    → transformCreateInputToModel (DTO → Model, áp dụng transform tag)
    → Model (T)
    → Xử lý OwnerOrganizationID (handler)
    → BaseService.InsertOne(ctx, model)
    → validateSystemDataInsert, ToMap, xóa empty string, thêm createdAt/updatedAt
    → collection.InsertOne(ctx, dataMap)
    → MongoDB
```

---

## 2. Chi tiết từng bước và điểm can thiệp vào data

### 2.1. Request Body → DTO (CreateInput / UpdateInput)

**Vị trí:** `handler.base.go` — `ParseRequestBody(c, &input)`

**Thao tác:**

1. **Đọc body:** `body := c.Body()` — raw bytes từ HTTP request.
2. **JSON decode:** `json.NewDecoder(reader).Decode(input)`  
   - Map key JSON → field struct theo tag `json:"..."`.  
   - Kiểu dữ liệu trong DTO giữ nguyên (ví dụ: `ownerOrganizationId` là `string` nếu DTO khai báo `string`).
3. **Validate:** `validateInput(input)`  
   - Dùng `github.com/go-playground/validator`: struct tag `validate:"required"`, `oneof=...`, v.v.  
   - Nếu lỗi → trả lỗi, không chuyển sang bước sau.

**Can thiệp vào data tại bước này:**

- Chỉ **parse + validate**. Không đổi kiểu (string → ObjectID) trong `ParseRequestBody` cho **body**.  
- Các field có tag `transform` trên DTO **chưa** được áp dụng ở bước parse body; chúng được áp dụng ở bước **transform DTO → Model** (xem 2.3).

**Lưu ý:** Với **query/params**, handler có áp dụng transform tag ngay sau khi bind (ParseRequestQuery, ParseQueryParams, ParseRequestParams).

---

### 2.2. Validate input (lần 2 trong InsertOne)

**Vị trí:** `handler.base.crud.go` — `InsertOne()` gọi `validateInput(&input)` sau `ParseRequestBody`.

**Thao tác:** Giống 2.1 — `global.Validate.Struct(input)`.

**Can thiệp:** Chỉ kiểm tra, không sửa giá trị (trừ khi validator tùy biến có logic set value).

---

### 2.3. DTO → Model (transform)

**Vị trí:** `handler.base.go` — `transformCreateInputToModel(&input)` / `transformUpdateInputToModel(&input)`.

**Thao tác:**

1. Tạo instance Model (T) mới.
2. Duyệt từng field của DTO (CreateInput/UpdateInput):
   - **Có struct tag `transform`:**
     - Parse tag: `utility.ParseTransformTag(transformTag)` → config (type, optional, map, default, format).
     - Transform giá trị: `utility.TransformFieldValue(value, config, targetFieldType)`.
   - **Transform type thường dùng** (trong `utility/data.transform.go`):
     - `str_objectid`: string → `primitive.ObjectID`
     - `str_objectid_ptr`: string → `*primitive.ObjectID`
     - `str_time`, `str_int64`, `str_bool`, `str_number`, v.v.
   - **Map field:** tag `transform:"...,map=FieldName"` → gán vào field Model tên `FieldName` (khác tên DTO).
   - **Nested struct:** `nested_struct` → transform đệ quy.
3. Field không có `transform` nhưng trùng tên và type tương thích → copy trực tiếp.

**Can thiệp vào data:**

- **Đổi kiểu:** string → ObjectID, string → int64, v.v.  
- **Đổi tên field:** DTO field A → Model field B qua `map=B`.  
- **Default / optional:** giá trị mặc định hoặc bỏ qua field optional.

Kết quả: có **Model (T)** đúng kiểu và tên field BSON/JSON của collection.

---

### 2.4. OwnerOrganizationID (phân quyền dữ liệu)

**Vị trí:** `handler.base.crud.go` (InsertOne, UpdateOne, UpdateById, Upsert, BulkUpsert, …) và `handler.base.go` (setOrganizationID, getActiveOrganizationID, validateUserHasAccessToOrg).

**Thao tác:**

1. **Lấy giá trị từ model:** `getOwnerOrganizationIDFromModel(model)` (reflection, field `OwnerOrganizationID`).
2. **Nếu đã có trong request (khác zero):**
   - `validateUserHasAccessToOrg(c, *ownerOrgIDFromRequest)` — kiểm tra user/role có quyền với org đó không (allowedOrgIDs từ role + shared orgs).
   - Nếu có quyền → **giữ nguyên** `OwnerOrganizationID` từ request.
3. **Nếu không có hoặc zero:**
   - `activeOrgID := getActiveOrganizationID(c)` — lấy từ `c.Locals("active_organization_id")` (middleware/auth đã set).
   - `setOrganizationID(model, *activeOrgID)` — gán vào model bằng reflection (chỉ khi model có field `OwnerOrganizationID` và hiện tại đang zero).

**Can thiệp vào data:**

- **Gán `OwnerOrganizationID`** từ context khi request không gửi hoặc gửi zero.  
- **Không ghi đè** nếu request đã gửi org hợp lệ và user có quyền.

---

### 2.5. Context bổ sung (user id)

**Vị trí:** `handler.base.crud.go` — trước khi gọi service.

**Thao tác:** Nếu `c.Locals("user_id")` có giá trị → `ctx = services.SetUserIDToContext(ctx, userID)`.

**Can thiệp:** Chỉ thêm thông tin vào context cho service (không đổi nội dung model).

---

### 2.6. Service layer — InsertOne

**Vị trí:** `api/internal/api/services/service..base.mongo.go` — `BaseServiceMongoImpl.InsertOne(ctx, data T)`.

**Thao tác lần lượt:**

1. **validateSystemDataInsert(ctx, data)**  
   - Lấy `IsSystem` từ model (reflection).  
   - Nếu model có `IsSystem == true` và context **không** phải init (system data) → trả lỗi "Không thể tạo dữ liệu với IsSystem = true".  
   - Nếu có field `IsSystem` → **set `IsSystem = false`** (reflection) để đảm bảo user không tạo dữ liệu system.

2. **Chuyển Model → map:** `utility.ToMap(data)`  
   - **ExtractDataIfExists(toMarshal):** Nếu model có struct tag `extract:"SourcePath"` (ví dụ `extract:"PanCakeData\\.name"`), giá trị được lấy từ source (nested map/field) và gán vào field đích → **data bị sửa trước khi marshal**.  
   - **bson.Marshal(toMarshal)** → **bson.Unmarshal** vào `map[string]interface{}`.  
   - Tên key trong map theo tag **bson** của model (ví dụ: `ownerOrganizationId`, `_id`, `createdAt`).

3. **Xóa empty string khỏi map:**  
   - Duyệt `dataMap`, nếu value là `string` và `== ""` → `delete(dataMap, key)`.  
   - Mục đích: tránh vi phạm sparse unique index (MongoDB không coi empty string là “không có field”).

4. **Thêm timestamp:**  
   - `dataMap["createdAt"] = time.Now().UnixMilli()`  
   - `dataMap["updatedAt"] = time.Now().UnixMilli()`  
   - Ghi đè nếu key đã tồn tại.

5. **Ghi DB:** `s.collection.InsertOne(ctx, dataMap)`.  
   - Nếu map không có `_id` hoặc `_id` zero, MongoDB driver sẽ sinh `_id` mới.

6. **Đọc lại document:** `FindOne(ctx, {"_id": result.InsertedID})` → decode vào `T` và trả về.

**Can thiệp vào data tại service:**

- **IsSystem** bị set về `false` nếu có field này.  
- **Extract:** Các field có tag `extract` được điền từ source (can thiệp vào nội dung model trước khi ToMap).  
- **Empty string** bị loại khỏi document.  
- **createdAt / updatedAt** luôn do service set (theo thời điểm insert).

---

## 3. Bảng tóm tắt: Ai can thiệp gì vào data

| Bước | Vị trí | Can thiệp vào data |
|------|--------|---------------------|
| 1. Parse body | Handler — ParseRequestBody | Chỉ parse JSON + validate; không transform kiểu cho body. |
| 2. Validate | Handler — validateInput | Kiểm tra theo tag `validate`, thường không sửa giá trị. |
| 3. DTO → Model | Handler — transformCreateInputToModel / transformUpdateInputToModel | Đổi kiểu (str→ObjectID, …), map field, default, nested. |
| 4. OwnerOrganizationID | Handler — setOrganizationID / getActiveOrganizationID | Gán org từ context nếu request không gửi/zero; validate quyền nếu gửi. |
| 5. Context | Handler | Chỉ set user id vào context, không sửa model. |
| 6. InsertOne (service) | Service — validateSystemDataInsert | Set IsSystem = false (reflection) khi có field. |
| 7. ToMap | Utility — ToMap → ExtractDataIfExists | Điền field từ source theo tag `extract`; BSON marshal theo tag `bson`. |
| 8. Chuẩn hóa map | Service — vòng for dataMap | Xóa key có value là empty string. |
| 9. Timestamp | Service | Thêm/ghi đè createdAt, updatedAt. |
| 10. InsertOne (driver) | MongoDB driver | Sinh _id nếu thiếu/zero. |

---

## 4. Luồng Update (UpdateOne / UpdateById) — điểm khác biệt

- **Parse:** Body → `UpdateInput` (ParseRequestBody + validateInput).  
- **Transform:** `transformUpdateInputToModel(&input)` → model (T) với đầy đủ transform tag.  
- **OwnerOrganizationID:** Cùng logic: nếu có trong request thì validate quyền; nếu zero thì set từ context.  
- **Service:**  
  - UpdateOne/UpdateById: `ToUpdateData(update)` → `$set` (và có thể `$unset`, …).  
  - **validateSystemDataUpdate:** Không cho phép sửa dữ liệu có `IsSystem == true` (trừ context đặc biệt).  
  - Luôn thêm `updatedAt` vào `$set`.  
  - Có thể xử lý empty string (unset field) để tương thích sparse index.

---

## 5. Các struct tag liên quan đến data

- **DTO:**  
  - `json:"..."` — map từ request JSON.  
  - `validate:"..."` — rule validator.  
  - `transform:"..."` — kiểu convert và map field khi DTO → Model (str_objectid, map=..., optional, default).  
- **Model:**  
  - `bson:"..."` — tên field trong MongoDB.  
  - `json:"..."` — response API.  
  - `extract:"SourcePath"` — giá trị lấy từ field/object khác trước khi marshal (dùng trong ToMap).  
  - `index:"..."` — chỉ cho index, không đổi giá trị.

Toàn bộ luồng từ DTO đến database và mọi điểm can thiệp vào data đều nằm trong các bước và tag trên.
