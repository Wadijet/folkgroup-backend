# Đề xuất: Thống nhất Partial Update và Struct Tag (UpdateById / UpdateOne / Upsert)

## 1. Mâu thuẫn hiện tại

| Endpoint    | Body → xử lý                    | Struct tag (validate, transform) | Chỉ set field có trong body |
|------------|----------------------------------|-----------------------------------|-----------------------------|
| **UpdateById** | Body → **map** (raw JSON)        | ❌ Không (chỉ xử lý tay ownerOrganizationId) | ✅ Có (chỉ key trong map)   |
| **UpdateOne**  | Body → **UpdateInput** → transform → model → map **chỉ non-zero** | ✅ Có | ✅ Có (loop bỏ zero)        |
| **Upsert**     | Body → **CreateInput** → transform → **model** (full) | ✅ Có | ❌ Không (full struct → ToUpdateData → ghi đè nếu không omitempty) |

- **UpdateById** dùng map nên mất validate/transform của DTO, không nhất quán với Insert/Upsert/UpdateOne.
- **Upsert** truyền full model xuống base → cần `omitempty` trên model để tránh ghi đè; nếu quên omitempty thì dễ lỗi.

## 2. Cách các công ty / chuẩn API thường làm

- **Stripe, GitHub, Google Cloud**: PATCH = chỉ cập nhật field có trong body (partial update).
- **Go**:
  - **Pointer cho optional**: `*string`, `*bool` → `nil` = không gửi, khác zero = có gửi. Chuẩn cho PATCH.
  - **Pre-load + Decode**: load document từ DB, decode JSON lên struct đó → chỉ field có trong JSON bị đổi (Go chỉ set field có trong JSON).
  - **Map**: body → `map[string]interface{}` → $set đúng key đó. Đơn giản nhưng mất validation/transform theo struct.
- **Kết luận**: vừa partial update (chỉ field có trong body) vừa giữ validation/transform thì nên: **body → DTO (UpdateInput) → validate + transform → build map chỉ từ field “đã set”** (non-zero hoặc pointer non-nil).

## 3. Đề xuất: UpdateById dùng cùng luồng UpdateOne (struct tag + chỉ non-zero vào Set)

**Nguyên tắc:** UpdateById dùng **UpdateInput** (struct tag) và **chỉ đưa field non-zero** vào `$set`, giống UpdateOne.

**Luồng đề xuất cho UpdateById:**

1. Parse body → **UpdateInput** (không dùng map).
2. **validateInput**(UpdateInput) — struct tag validate.
3. **transformUpdateInputToModel**(UpdateInput) → **model** (chỉ field non-zero trong input được copy sang model).
4. Xử lý **ownerOrganizationId** (validation quyền, gán từ context nếu cần) — có thể trên model hoặc trên map sau bước 5.
5. **Model → map cho $set**: marshal model sang map rồi **chỉ thêm vào Set các entry non-zero** (giống UpdateOne: `for k, v := range modelMap { if !reflect.ValueOf(v).IsZero() { updateData.Set[k] = v } }`).
6. Gọi **BaseService.UpdateById(ctx, id, &UpdateData{ Set: map })**.

**Lợi ích:**

- **Struct tag**: validate, transform (string → ObjectID, v.v.) dùng chung Insert/UpdateOne/UpdateById/Upsert.
- **Partial update**: chỉ field có trong body (non-zero sau unmarshal) mới vào $set, không ghi đè field khác.
- **Nhất quán**: UpdateById và UpdateOne cùng một pattern (UpdateInput → model → map chỉ non-zero → UpdateData).

**Lưu ý:**

- UpdateInput nên có field **optional** cho PATCH: field không gửi = zero → đã bỏ qua ở bước 3 (transform chỉ copy non-zero) và bước 5 (chỉ non-zero vào Set). Field bool cần set `false` thì client gửi `false` → không bị coi là zero nếu dùng bool (hoặc dùng `*bool` nếu cần phân biệt “không gửi” vs “gửi false”).
- **Upsert** giữ nguyên: body → CreateInput → transform → model (full) → base.Upsert. Tránh ghi đè nhờ **omitempty** trên model (đã thêm cho bool và field nhạy cảm).

## 4. Tóm tắt thay đổi code

- **UpdateById (handler)**:
  - Bỏ: decode body thành `map[string]interface{}`.
  - Thêm: parse body → UpdateInput, validateInput, transformUpdateInputToModel → model.
  - Giữ: validate organization (theo id document).
  - Thêm: build map từ model **chỉ non-zero** (copy logic từ UpdateOne), xử lý ownerOrganizationId trên model hoặc map.
  - Gọi: BaseService.UpdateById(ctx, id, &UpdateData{ Set: map }).

Sau khi áp dụng: không còn mâu thuẫn giữa “map” và “struct tag”; UpdateById vừa dùng struct tag vừa partial update như UpdateOne.
