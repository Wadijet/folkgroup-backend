# Organization Config: 1 document per org vs 1 document per key

## Hai mô hình

| | **A: 1 doc per org** (hiện tại) | **B: 1 doc per key** |
|--|--------------------------------|----------------------|
| **Lưu trữ** | 1 document/org, 2 field map: `config`, `configMeta` | N document/org (N = số key), mỗi doc = 1 cặp (org, key) |
| **Ví dụ** | 100 org, mỗi org 20 key → **100 document** | 100 org × 20 key → **2000 document** |
| **Đọc 1 key** | Đọc cả document (20 key), lấy 1 key | `findOne({ ownerOrganizationId, key })` — **1 document** |
| **Ghi 1 key** | Đọc doc → merge → ghi cả document (hoặc merge ở backend) | `upsertOne({ ownerOrganizationId, key }, { value, name, ... })` — **1 document** (value + metadata gộp chung) |
| **Resolved (merge cây)** | Với mỗi org trong chain: đọc 1 doc → merge trong memory | Query `find({ ownerOrganizationId: { $in: [root,...,org] } })` → group by key → merge theo thứ tự chain + lock |

---

## So sánh chi tiết

### 1. Đọc/ghi từng key

- **A:** Muốn đọc/ghi 1 key vẫn phải đụng cả document (đọc hết key, ghi phải merge hoặc gửi full).
- **B:** Đọc 1 key = 1 `findOne(orgId, key)`. Ghi 1 key = 1 upsert. **Mỗi tổ chức có config khác nhau** (key khác nhau) không ảnh hưởng nhau, không cần load toàn bộ.

### 2. Resolved config (merge theo cây)

- **A:** Số lần đọc DB = số org trong chain (vd 5 org → 5 doc). Mỗi doc có thể lớn (nhiều key).
- **B:** 1 query `ownerOrganizationId ∈ [root, ..., org]` → nhiều doc (số org × số key trung bình). Trong memory: group by key, sort theo chain, áp dụng lock. Logic phức tạp hơn một chút nhưng rõ ràng.

### 3. Concurrent update

- **A:** Hai user sửa hai key khác nhau cùng org → cùng đọc 1 doc, cùng ghi → dễ last-write-wins (cần merge hoặc lock).
- **B:** Hai key = hai document → cập nhật không đè lẫn nhau.

### 4. Số document & index

- **A:** Ít document, index đơn giản (vd `ownerOrganizationId` unique).
- **B:** Nhiều document hơn, cần **unique index (ownerOrganizationId, key)**. Query theo org hoặc theo (org, key) đều tối ưu được.

### 5. “Mỗi tổ chức có config khác nhau”

- **A:** Vẫn đúng: mỗi org 1 doc, bên trong map key→value có thể khác hẳn nhau. Chỉ là “khác nhau” nằm trong cùng 1 document.
- **B:** “Khác nhau” thể hiện bằng **số lượng và tập key khác nhau**; thêm/xóa key = thêm/xóa document, không đụng key khác. Phù hợp khi nhiều org, mỗi org ít key riêng và không muốn mỗi lần đọc/ghi phải load/send hết.

---

## Khi nào nên “mỗi key 1 document”

**Nên dùng B (1 doc per key) khi:**

- Cần **đọc/ghi từng key** thường xuyên (API theo key, UI chỉ load 1 vài key).
- **Số key mỗi org không cố định**, hoặc org này 5 key, org kia 50 key — tránh document quá lớn hoặc sparse map.
- **Concurrent update** nhiều (nhiều user/sync sửa key khác nhau cùng lúc).
- Muốn **thêm/xóa key** không ảnh hưởng key khác (không read-modify-write cả document).

**Có thể giữ A (1 doc per org) khi:**

- Số key mỗi org **ít và ổn định** (vd vài chục key), và thường dùng **resolved full** hoặc load cả config một lần.
- Ưu tiên **đơn giản**: ít document, logic merge resolved đã có sẵn, không muốn đổi schema.

---

## Gợi ý

- **Thực tế mỗi tổ chức có config khác nhau** (key khác, số lượng khác) và muốn **mỗi lần đọc/ghi không phải làm việc với tất cả key** → **nên chuyển sang 1 document per key** (model B).
- Cần thêm:
  - Model mới (vd `OrganizationConfigItem`: `ownerOrganizationId`, `key`, `value`, và các field metadata cùng document — không nested `meta`).
  - Unique index `(ownerOrganizationId, key)`.
  - Service/handler: get/set/delete theo (orgId, key); GetResolvedConfig query theo chain rồi group-by-key + merge (giữ logic lock như hiện tại).

---

## Schema gợi ý (model B): value + metadata gộp một document

**Ý tưởng:** Mỗi key = 1 document; **value và metadata nằm chung trong document đó** (không tách nested `meta`).

```text
Collection: auth_organization_config_items

OrganizationConfigItem:
  _id                    ObjectId
  ownerOrganizationId    ObjectId   (index: compound unique với key)
  key                    string    (vd "timezone", "maxUsers")
  value                  interface (giá trị — string, number, boolean, object, array)
  name                   string    (tên hiển thị, vd "Múi giờ")
  description            string    (mô tả)
  dataType               string    (string, number, boolean, object, array)
  constraints            string    (optional, ràng buộc: enum, min, max, pattern...)
  allowOverride          bool      (true = cấp dưới được ghi đè; false = khóa)
  isSystem               bool
  createdAt              int64
  updatedAt              int64

Unique index: (ownerOrganizationId, key)
Index: ownerOrganizationId (query tất cả key của một org hoặc $in chain)
```

- Một document = một key, đủ **cả value lẫn metadata** (name, description, dataType, constraints, allowOverride).
- Resolved / lock: dùng field `allowOverride` ngay trong document đó.

API có thể giữ resource `/organization-config` và thêm:

- `GET /organization-config/find-one?filter={"ownerOrganizationId":"...","key":"timezone"}` → 1 item (value + metadata trong cùng response).
- `GET /organization-config/find?filter={"ownerOrganizationId":"..."}` → tất cả key của org (raw).
- `POST /organization-config/upsert-one` body `{ ownerOrganizationId, key, value, name, description, dataType, constraints, allowOverride }` → upsert 1 document (value + metadata gộp chung).
- `DELETE /organization-config/delete-one?filter={"ownerOrganizationId":"...","key":"..."}`.
- `GET /organization-config/resolved?ownerOrganizationId=...` → merge theo cây (query items theo chain, group by key, apply lock), trả về map `{ key: value }` (chỉ value; metadata không cần trong resolved nếu client chỉ cần giá trị).

Như vậy: mỗi tổ chức có config khác nhau, mỗi lần đọc/ghi không cần làm việc với tất cả key, và **mỗi document 1 key với value + metadata gộp một chỗ**.
