# Review Backend Docs — 2025-03-13

**Theo:** `prompts/create docs/docs-recreate-review.md`

---

## 1. Đã phản ánh codebase backend thật chưa?

| Vấn đề | Trạng thái | Hành động |
|--------|------------|------------|
| `tong-quan.md` ghi `api/internal/api/services/` | ❌ Sai | Services nằm trong từng module: `api/internal/api/<module>/service/`. Base: `api/internal/api/base/service/` |
| `cau-truc-code.md` | ✅ Đã sửa | Đã cập nhật api/internal/api |
| `backend-module-map.md` | ✅ Đúng | Map 15 router modules, cấu trúc code khớp thực tế |
| Module base, events, initsvc | ⚠️ Không list | Là internal, không phải domain router — giữ nguyên |

---

## 2. Local docs và shared docs đã phân tầng rõ chưa?

| Tiêu chí | Trạng thái |
|----------|------------|
| Bảng Local vs Shared trong README | ✅ Có |
| Link doc-ownership | ✅ Có |
| Phần AI Context & API Contract | ✅ Dùng docs-shared |
| **09-ai-context** vs **docs-shared/ai-context** | ⚠️ Dễ nhầm | 09-ai-context = quy tắc backend cho AI; docs-shared/ai-context = API contract. Cần ghi rõ trong README |

---

## 3. Có tài liệu nào còn trùng vai trò không?

| Cặp | Phân tích |
|-----|-----------|
| logging-system-usage vs log-filter-system | Khác vai trò: usage vs filter — giữ |
| 03-api/* vs docs-shared/api-context | 03-api = mô tả backend; api-context = contract đầy đủ. Bổ sung cho nhau — giữ |
| 02-architecture/design vs docs-shared/ai-context/design | design backend = proposal; design shared = module spec cross-repo — khác nhau |

**Kết luận:** Không trùng vai trò nghiêm trọng.

---

## 4. Có tài liệu nào AI sẽ khó hiểu hoặc khó tìm không?

| Vấn đề | Rủi ro | Hành động |
|--------|--------|-----------|
| Link `../../docs/ai-context/` (Frontend Developer) | Sai khi mở riêng repo | Đổi thành `../docs-shared/ai-context/` |
| 09-ai-context tên dễ nhầm với ai-context shared | AI có thể đọc nhầm | Thêm mục "09-ai-context" trong README, ghi rõ: quy tắc backend |
| backend-module-map nằm ở root docs/ | Dễ tìm | OK |
| Thứ tự đọc ở đầu README | Rõ ràng | OK |

---

## 5. Entry point cho backend repo đã đủ rõ chưa?

| Thành phần | Trạng thái |
|------------|------------|
| Cursor AI — Thứ tự đọc | ✅ Có, ở đầu README |
| Local vs Shared | ✅ Có |
| backend-module-map link | ✅ Có |
| read-docs-first rule | ✅ Đã cập nhật |
| 09-ai-context chưa có trong Mục lục | ⚠️ Thiếu | Thêm vào README |

---

## Tổng kết

- **Điểm tốt:** Entry point rõ, phân tầng local/shared ổn, backend-module-map hữu ích.
- **Cần sửa:** tong-quan.md (đường dẫn services), link Frontend Developer, thêm 09-ai-context vào mục lục và ghi rõ vai trò.
