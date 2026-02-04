# Đề xuất: Organization Config — bớt endpoint “overdrive”, gần chuẩn CRUD

## 1. Hiện trạng vs chuẩn CRUD của backend

**Chuẩn CRUD trong project** (vd: `/user`, `/organization`, `/organization-share`):

- Prefix = tên resource: `/user`, `/organization-config`, ...
- Đọc: `GET /find`, `GET /find-one?filter={...}`, `GET /find-by-id/:id`
- Tạo/cập nhật: `POST /insert-one`, `POST /upsert-one?filter={...}`, `PUT /update-one`, `PUT /update-by-id/:id`
- Xóa: `DELETE /delete-one?filter={...}`, `DELETE /delete-by-id/:id`

**Organization Config hiện tại** — 4 endpoint custom, không dùng CRUD chuẩn:

| Hiện tại | Chuẩn CRUD tương đương |
|----------|-------------------------|
| `GET /organization/:id/config` | `GET /organization-config/find-one?filter={"ownerOrganizationId":"<id>"}` |
| `GET /organization/:id/config/resolved` | Không có (computed view) |
| `PUT /organization/:id/config` | `POST /organization-config/upsert-one?filter={"ownerOrganizationId":"<id>"}` + body |
| `DELETE /organization/:id/config` | `DELETE /organization-config/delete-one?filter={"ownerOrganizationId":"<id>"}` |

---

## 2. Hai hướng đề xuất

### Hướng A: Resource riêng `/organization-config` + CRUD chuẩn (ít overdrive nhất)

- **Resource:** `organization-config` (giống `organization-share`, `user`, ...).
- **Raw config (1 document):**
  - **Đọc:** `GET /api/v1/organization-config/find-one?filter={"ownerOrganizationId":"<orgId>"}`
    - Trả document đầy đủ; **404** khi chưa có config (chuẩn REST).
    - Client coi 404 = chưa có config, không cần response “document rỗng”.
  - **Upsert:** `POST /api/v1/organization-config/upsert-one?filter={"ownerOrganizationId":"<orgId>"}`  
    Body: `{ "config": {...}, "configMeta": {...} }`  
    → Cần **override** handler `Upsert` (body là DTO `OrganizationConfigUpdateInput`, logic gọi `UpsertByOwnerOrganizationID`).
  - **Xóa:** `DELETE /api/v1/organization-config/delete-one?filter={"ownerOrganizationId":"<orgId>"}`  
    → Cần **override** handler `DeleteOne` (trước khi xóa gọi `ValidateBeforeDelete` / `DeleteByOwnerOrganizationID`).
- **Resolved config (computed):**
  - Chỉ còn **1** endpoint “đặc biệt”: lấy config đã merge theo cây.
  - Cách 1 (query param):  
    `GET /api/v1/organization-config/find-one?filter={"ownerOrganizationId":"<orgId>"}&resolved=true`  
    → Trả `{ "config": {...} }` (chỉ map giá trị). Handler FindOne đọc `resolved=true` thì gọi `GetResolvedConfig` thay vì `GetByOwnerOrganizationID`.
  - Cách 2 (sub-path):  
    `GET /api/v1/organization-config/resolved?ownerOrganizationId=<orgId>`  
    → Một route riêng nhưng vẫn nằm dưới resource `organization-config`, dễ hiểu.

**Kết quả:** 1 resource, 3–4 endpoint (find-one, upsert-one, delete-one + 1 cho resolved), **theo đúng pattern CRUD** của project; chỉ “đặc biệt” duy nhất là resolved (computed view).

---

### Hướng B: Giữ sub-resource `/organization/:id/config`, chỉ gộp 2 GET

- Giữ: `GET /organization/:id/config`, `PUT /organization/:id/config`, `DELETE /organization/:id/config`.
- **Gộp 2 GET thành 1:**  
  `GET /api/v1/organization/:id/config?resolved=true|false`
  - `resolved=false` (mặc định): trả raw document (hoặc document rỗng khi chưa có).
  - `resolved=true`: trả `{ "config": {...} }` (đã merge).
- **Kết quả:** 3 endpoint thay vì 4; vẫn “custom” (sub-resource theo org), nhưng ít route hơn và thống nhất một đường “get”.

---

## 3. So sánh nhanh

| Tiêu chí | Hướng A (resource `/organization-config`) | Hướng B (sub-resource `/:id/config`) |
|----------|-------------------------------------------|----------------------------------------|
| Theo chuẩn CRUD của project | Có (find-one, upsert-one, delete-one) | Không (path custom) |
| Số endpoint “đặc biệt” | 1 (resolved) | 3 (cả get/put/delete đều custom path) |
| Đồng nhất với `/user`, `/organization-share` | Cao | Thấp |
| URL “đọc bằng mắt” | `?filter={"ownerOrganizationId":"..."}` | `/organization/:id/config` gọn hơn |
| Thay đổi code | Đổi route sang `/organization-config`, thêm override Upsert/DeleteOne (và FindOne khi resolved=true) | Chỉ gộp 2 GET thành 1 với query `resolved` |

---

## 4. Gợi ý

- **Muốn bớt lăn tăn “quá nhiều overdrive, không theo chuẩn CRUD”:** nên chọn **Hướng A** — chuyển sang resource `/organization-config` và dùng find-one / upsert-one / delete-one; chỉ giữ một cách lấy “resolved” (query param hoặc sub-path).
- **Muốn đổi ít code, vẫn giữ URL dạng “config của org X”:** dùng **Hướng B** — gộp 2 GET thành `GET /organization/:id/config?resolved=...`, giữ PUT/DELETE như cũ.

---

## 5. Lưu ý khi implement Hướng A

1. **Permission:** Giữ nguyên `OrganizationConfig.Read`, `OrganizationConfig.Update`, `OrganizationConfig.Delete`; chỉ đổi path.
2. **Phân quyền org:** `find-one` / `upsert-one` / `delete-one` với `filter.ownerOrganizationId` vẫn cần kiểm tra user có quyền truy cập org đó (middleware hoặc logic trong handler).
3. **404 vs “document rỗng”:** Chuẩn CRUD là 404 khi không tìm thấy. Client (frontend) đổi từ “expect 200 + body rỗng” sang “expect 404 = chưa có config”.
4. **Resolved:** Nếu dùng `find-one?resolved=true`, handler FindOne của OrganizationConfig cần đọc query và gọi `GetResolvedConfig` khi `resolved=true`, trả về shape `{ "config": ... }`.
