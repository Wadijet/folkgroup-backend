# Quy Trình Refactor Tài Liệu Dự Án (AI-Driven)

**Mục đích:** Thiết kế quy trình để AI refactor toàn bộ docs của dự án, đảm bảo tài liệu khoa học, phù hợp codebase, dễ bảo trì và có audit rõ ràng.

**Phạm vi:** `docs/` (local backend) + `.cursor/` (rules, tasks, prompts).

---

## I. Thực Trạng Hiện Tại

- Tài liệu nhiều (~175+ files), sắp xếp lung tung
- Cấu trúc `01-getting-started`, `02-architecture`, ... trộn lẫn với `backend-module-map.md` ở root
- `02-architecture/` có nhiều thư mục con (core, systems, analysis, design, business-logic, solutions, other) — khó điều hướng
- Nhiều file DE_XUAT_*, BAO_CAO_*, CTA_* — vai trò không rõ
- Thiếu format chuẩn (Purpose, Scope, Changelog, Ownership)
- Khi codebase thay đổi, docs chưa có quy trình sync rõ ràng

---

## II. Mục Tiêu Sau Refactor

| Yêu cầu | Mô tả |
|---------|-------|
| **Phản ánh codebase** | Tài liệu thể hiện đúng cấu trúc, module, API, domain hiện tại |
| **Sync với code** | Khi code thay đổi → docs được cập nhật theo quy trình |
| **Format chuẩn** | Mỗi doc quan trọng có: Purpose, Scope, Related Code, Changelog |
| **Cấu trúc khoa học** | Theo codebase, dễ AI đọc, có AI reading order |
| **Audit rõ ràng** | Có bước audit kết quả trước khi kết thúc |

---

## III. Cấu Trúc Docs Đề Xuất (Sau Refactor)

```
docs/
├── README.md                    # Entry point, AI reading order, mục lục
├── CHANGELOG.md                 # Lịch sử thay đổi docs
├── architecture/
│   ├── README.md
│   ├── overview.md             # Kiến trúc tổng quan
│   ├── core/                   # Auth, RBAC, Organization, Database
│   ├── systems/                # Logging, Notification, Worker, Content
│   └── decisions/              # ADR, design decisions
├── module-map/
│   ├── README.md
│   └── backend-module-map.md   # Map module → code, docs
├── api/
│   ├── README.md
│   └── api-overview.md         # Tổng quan endpoints
├── domain/
│   ├── README.md
│   └── domain-overview.md      # Business logic, rules
├── data-model/                 # Collections, schemas (nếu cần)
├── flows/                      # Luồng xử lý chính
├── conventions/                # Coding standards, patterns
├── development/                # Thêm API, thêm service, cấu trúc code
├── deployment/                 # Production, MongoDB, Firebase, systemd
├── testing/
├── troubleshooting/
└── archive/                    # Tài liệu cũ, báo cáo, đề xuất đã xử lý
```

**Nguyên tắc:**
- Mỗi thư mục có README nếu đủ lớn
- File trùng vai trò → gộp
- File outdated / không phản ánh code → archive hoặc xóa
- Phân biệt rõ **local docs** vs **docs-shared** (symlink workspace)

---

## IV. Format Chuẩn Cho Mỗi Tài Liệu Quan Trọng

```markdown
# [Tiêu đề]

## Purpose
Mục đích của tài liệu này.

## Scope
Phạm vi: module/domain nào, điều kiện áp dụng.

## Nội dung chính
...

## Related Code
- `api/internal/api/<module>/...`
- File, function cụ thể nếu cần

## Related Docs
- [Link tài liệu liên quan]

## Ownership
- Module/team chịu trách nhiệm cập nhật

## Changelog
| Ngày | Thay đổi |
|------|----------|
| YYYY-MM-DD | Mô tả thay đổi |
```

---

## V. Quy Trình Refactor Docs (Chi Tiết)

### Phase 0: Chuẩn Bị

1. **Đọc 3 lớp tri thức**
   - `docs-shared/` (nếu có) — system map, module map workspace
   - `docs/` — toàn bộ tài liệu hiện có
   - Codebase — `api/internal/api/`, routers, services, models

2. **So sánh**
   - docs-shared vs docs local
   - docs local vs codebase
   - Xác định mismatch

---

### Phase 1: Phân Tích

3. **Quét docs hiện có, lập danh sách**
   - `keep` — giữ nguyên, có thể chỉnh format
   - `merge` — gộp với file khác
   - `move` — di chuyển sang thư mục mới
   - `delete` — xóa (hoặc archive)
   - `rewrite` — viết lại

4. **Phân loại theo vai trò**
   - Architecture / Design
   - Module map / API overview
   - Business logic / Domain
   - Development guide
   - Deployment / Testing / Troubleshooting
   - Archive (DE_XUAT, BAO_CAO, CTA cũ...)

---

### Phase 2: Refactor Cấu Trúc

5. **Tạo cấu trúc thư mục mới** (nếu chưa có)
6. **Di chuyển / đổi tên file** theo danh sách đã phân tích
7. **Gộp file trùng vai trò**
8. **Xóa / archive** file không cần thiết

---

### Phase 3: Nội Dung

9. **Viết lại / cập nhật** tài liệu thiếu hoặc sai
10. **Áp dụng format chuẩn** (Purpose, Scope, Changelog, Related Code)
11. **Cập nhật cross-links** giữa các file
12. **Cập nhật docs/README.md** — entry point, AI reading order, mục lục

---

### Phase 4: Đồng Bộ Với Codebase

13. **Build module map** từ code thực tế
   - Prompt: `prompt-system/prompts/repo-backend/docs/docs-build-module-map.md`
14. **Build API overview** từ code
   - Prompt: `prompt-system/prompts/repo-backend/docs/docs-build-api-overview.md`
15. **Kiểm tra** mọi đường dẫn, tên module, tên service khớp code

---

### Phase 5: Audit

16. **Chạy docs-audit**
   - Prompt: `prompt-system/prompts/maintenance/docs-audit.md`
   - Output: danh sách keep/merge/move/delete/rewrite + sửa ngay vấn đề rõ ràng

17. **Chạy docs-recreate-review**
   - Prompt: `prompt-system/prompts/repo-backend/docs/docs-recreate-review.md`
   - Kiểm tra: entry point, cấu trúc phản ánh codebase, local vs shared

18. **Chạy docs-reindex-review**
   - Prompt: `prompt-system/prompts/repo-backend/docs/docs-reindex-review.md`
   - Kiểm tra: navigation, AI reading order, cross-links

19. **Tạo CHANGELOG.md** — ghi lại toàn bộ thay đổi refactor

---

## VI. Quy Trình Duy Trì (Sau Refactor)

### Khi Codebase Thay Đổi

| Thay đổi | Hành động |
|----------|-----------|
| API mới / API đổi | Cập nhật `docs/api/api-overview.md` |
| Module mới / module xóa | Cập nhật `docs/module-map/backend-module-map.md` |
| Domain logic đổi | Cập nhật `docs/domain/` |
| Architecture đổi | Cập nhật `docs/architecture/` |

**Prompt sync:** `prompt-system/prompts/repo-backend/docs/docs-sync-with-code.md`

### Định Kỳ

- **Hàng tuần / sau sprint:** Chạy `docs-sync-with-code` để phát hiện drift
- **Hàng tháng:** Chạy `docs-audit` để kiểm tra tổng thể

---

## VII. Pipeline Prompts (Tham Chiếu)

Thứ tự chạy theo `prompt-system/prompts/pipeline.md`:

| Phase | Prompt | Mục đích |
|-------|--------|----------|
| 1 | `workspace/docs/docs-recreate.md` | Tạo khung docs workspace |
| 2 | `workspace/docs/docs-recreate-review.md` | Review khung |
| 3 | `workspace/docs/docs-refactor.md` | Refactor docs workspace |
| 4 | `workspace/docs/docs-reindex.md` | Reindex navigation |
| 5 | `workspace/docs/docs-reindex-review.md` | Review navigation |
| 6–7 | `workspace/cursor/*` | Cursor workspace |
| 8 | `repo-backend/docs/docs-recreate.md` | Tạo khung docs backend |
| 9 | `repo-backend/docs/docs-recreate-review.md` | Review |
| 10 | **`repo-backend/docs/docs-refactor.md`** | **Refactor docs backend** |
| 11 | `repo-backend/docs/docs-reindex.md` | Reindex |
| 12 | `repo-backend/docs/docs-reindex-review.md` | Review |
| 13–14 | `repo-backend/cursor/*` | Cursor backend |
| 15 | `maintenance/docs-sync-with-code.md` | Sync docs với code |
| 16 | `maintenance/docs-build-module-map.md` | Build module map |
| 17 | `maintenance/docs-build-api-overview.md` | Build API overview |
| 18 | `maintenance/docs-audit.md` | Audit docs |
| 19 | `maintenance/cursor-audit.md` | Audit .cursor |

---

## VIII. Quy Trình Refactor .cursor (Sau Khi Docs Xong)

Xem chi tiết: [QUY_TRINH_REFACTOR_CURSOR.md](./QUY_TRINH_REFACTOR_CURSOR.md)

**Prompt chạy refactor toàn bộ:** `prompt-system/prompts/repo-backend/docs/docs-refactor-full-process.md`

Tóm tắt:
1. Đọc docs mới + codebase
2. Thiết kế `.cursor/` theo `cursor-create.md`
3. Refactor rules, prompts, tasks
4. Chạy `cursor-audit.md`
5. Cập nhật cross-reference giữa docs và .cursor

---

## IX. Checklist Hoàn Thành Refactor

- [ ] Cấu trúc docs theo cấu trúc đề xuất
- [ ] Mỗi doc quan trọng có format chuẩn (Purpose, Scope, Changelog)
- [ ] docs/README.md có AI reading order rõ ràng
- [ ] backend-module-map.md phản ánh code thực tế
- [ ] api-overview.md phản ánh endpoints thực tế
- [ ] Local vs shared docs phân tầng rõ
- [ ] Không còn file trùng vai trò
- [ ] File outdated đã archive hoặc xóa
- [ ] Cross-links chính xác
- [ ] CHANGELOG.md đã cập nhật
- [ ] Đã chạy docs-audit, docs-recreate-review, docs-reindex-review
- [ ] .cursor đã refactor theo quy trình

---

## X. Tài Liệu Tham Khảo

- `prompt-system/prompts/pipeline.md` — Pipeline tổng thể
- `prompt-system/prompts/repo-backend/docs/docs-refactor.md` — Prompt refactor chi tiết
- `docs/08-archive/REVIEW_BACKEND_DOCS_2025-03.md` — Ví dụ review
- `.cursor/rules/folkgroup-backend.mdc` — Rules backend (được docs tham chiếu)
