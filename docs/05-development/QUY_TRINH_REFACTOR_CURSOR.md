# Quy Trình Refactor Hệ Thống .cursor (Sau Khi Docs Đã Refactor)

**Mục đích:** Sau khi docs đã được refactor, refactor lại hệ thống file trong `.cursor` để đồng bộ với cấu trúc docs mới và tối ưu cho AI.

**Điều kiện tiên quyết:** Docs đã refactor xong (theo [QUY_TRINH_REFACTOR_DOCS.md](./QUY_TRINH_REFACTOR_DOCS.md)).

---

## I. Thực Trạng .cursor Hiện Tại

```
.cursor/
├── rules/
│   ├── folkgroup-backend.mdc    # Rules chính
│   └── read-docs-first.md       # Hướng dẫn đọc docs
└── task/
    ├── generate-rules.md
    ├── update-rules.md
    ├── organize-docs.md
    ├── update-docs.md
    ├── de-xuat-he-thong-rules-moi.md
    └── ...
```

**Vấn đề:**
- Chưa có `prompts/`, `skills/`, `workflows/` như cursor-create đề xuất
- Tasks có thể trùng vai trò hoặc outdated
- Rules cần cập nhật đường dẫn docs sau refactor
- Chưa có quy trình audit .cursor

---

## II. Mục Tiêu Sau Refactor .cursor

| Yêu cầu | Mô tả |
|---------|-------|
| **Đồng bộ docs** | Rules, prompts tham chiếu đúng cấu trúc docs mới |
| **Tinh gọn** | Gộp/xóa file trùng vai trò, tránh dư thừa |
| **Phù hợp AI** | AI reading order, guardrails rõ ràng |
| **Dễ bảo trì** | Có audit định kỳ |

---

## III. Cấu Trúc .cursor Đề Xuất (Sau Refactor)

```
.cursor/
├── rules/
│   ├── folkgroup-backend.mdc    # Rules chính (đã có)
│   └── read-docs-first.md       # Thứ tự đọc docs (cập nhật đường dẫn)
├── prompts/                     # (Tùy chọn) Prompts cho dev
│   ├── backend-architect.md
│   ├── backend-feature-design.md
│   └── ...
├── task/                        # Tasks cụ thể
│   ├── add-api-endpoint.md
│   ├── refactor-module.md
│   └── ...
└── workflows/                   # (Tùy chọn) Workflows phức tạp
    └── ...
```

**Nguyên tắc:**
- Ưu tiên **rules** — Cursor luôn áp dụng
- **Prompts / skills / workflows** chỉ tạo khi thực sự cần
- **Tasks** gọn, không trùng nhau

---

## IV. Quy Trình Refactor .cursor (Chi Tiết)

### Phase 0: Chuẩn Bị

1. **Đọc docs mới**
   - `docs/README.md` — entry point, AI reading order
   - `docs/architecture/`, `docs/module-map/`, `docs/api/`
   - `docs/05-development/` — conventions, thêm API

2. **Đọc codebase**
   - Cấu trúc module, router, service
   - Patterns hiện tại

3. **Đọc cursor-create**
   - `prompt-system/prompts/repo-backend/cursor/cursor-create.md`

---

### Phase 1: Cập Nhật Rules

4. **Cập nhật `read-docs-first.md`**
   - Thứ tự đọc theo docs mới:
     1. `docs/README.md`
     2. `docs/architecture/overview.md` (hoặc `docs/02-architecture/core/tong-quan.md`)
     3. `docs/module-map/backend-module-map.md` (hoặc `docs/backend-module-map.md`)
     4. `docs/domain/`, `docs/api/`
     5. `docs-shared/` khi cần

5. **Cập nhật `folkgroup-backend.mdc`**
   - Sửa mọi đường dẫn docs cũ → docs mới
   - Tham chiếu: `docs/05-development/them-api-moi.md`, `docs/02-architecture/...`
   - Bảng tài liệu tham khảo cuối file

---

### Phase 2: Refactor Tasks

6. **Phân tích tasks hiện có**
   - `keep` — vẫn cần, cập nhật nội dung
   - `merge` — gộp với task khác
   - `delete` — không còn cần

7. **Cập nhật / tạo tasks**
   - `organize-docs.md` → có thể gộp vào quy trình refactor docs
   - `update-docs.md` → tham chiếu docs-sync-with-code
   - `generate-rules.md`, `update-rules.md` → giữ nếu dùng
   - `de-xuat-he-thong-rules-moi.md` → archive nếu đã xử lý

8. **Tạo tasks mới (nếu cần)**
   - `add-api-endpoint.md` — tham chiếu them-api-moi.md
   - `refactor-module.md`
   - `update-data-model.md`

---

### Phase 3: Prompts / Workflows (Tùy Chọn)

9. **Đánh giá nhu cầu**
   - Nếu team dùng Cursor Composer với custom prompts → tạo
   - Nếu không → bỏ qua, rules đủ

10. **Tạo prompts (nếu cần)**
    - Tham chiếu `cursor-create.md` section VII
    - Đặt trong `.cursor/prompts/`

11. **Tạo workflows (nếu cần)**
    - add-feature, refactor-module, debug-backend-issue
    - Tham chiếu `cursor-create.md` section X

---

### Phase 4: Audit

12. **Chạy cursor-audit**
    - Prompt: `prompt-system/prompts/maintenance/cursor-audit.md`
    - Phát hiện: rules trùng, prompts trùng vai trò, tasks trùng workflow
    - Tinh gọn .cursor xuống mức nhỏ nhưng mạnh

13. **Kiểm tra cross-reference**
    - Rules → docs
    - Docs → rules (phần "Tài liệu tham khảo")
    - Đảm bảo link không broken

---

## V. Cập Nhật read-docs-first.md (Mẫu)

```markdown
# Thứ Tự Đọc Docs Cho Cursor AI

Khi mở repo backend, đọc theo thứ tự:

1. docs/README.md — Entry point, mục lục
2. docs/module-map/backend-module-map.md — Map module → code
3. docs/02-architecture/core/tong-quan.md — Kiến trúc layers
4. docs theo task — 05-development/, 03-api/, 02-architecture/...
5. docs-shared/ — Khi cần context hệ thống, API contract

Khi task chạm repo khác: Đọc docs-shared/system-map/system-map.md trước.
```

*(Điều chỉnh đường dẫn theo cấu trúc docs sau refactor.)*

---

## VI. Checklist Hoàn Thành Refactor .cursor

- [ ] read-docs-first.md đã cập nhật đường dẫn docs mới
- [ ] folkgroup-backend.mdc đã cập nhật đường dẫn
- [ ] Tasks đã phân tích: keep/merge/delete
- [ ] Không còn task trùng vai trò
- [ ] Đã chạy cursor-audit
- [ ] Cross-reference docs ↔ .cursor chính xác
- [ ] Prompts/workflows chỉ tạo khi cần

---

## VII. Tài Liệu Tham Khảo

- [QUY_TRINH_REFACTOR_DOCS.md](./QUY_TRINH_REFACTOR_DOCS.md) — Quy trình refactor docs
- `prompt-system/prompts/repo-backend/cursor/cursor-create.md` — Thiết kế .cursor
- `prompt-system/prompts/repo-backend/cursor/cursor-create-review.md` — Review
- `prompt-system/prompts/maintenance/cursor-audit.md` — Audit .cursor
