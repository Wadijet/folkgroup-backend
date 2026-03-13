# Prompt Quy Trình — Refactor Docs + .cursor

**Mục đích:** File này hướng dẫn AI thực hiện **lần lượt** các prompt cần thiết để refactor docs và .cursor. Mỗi bước: đọc nội dung prompt tương ứng rồi thực thi.

**Cách dùng:** Mở file này, chọn scope phù hợp, rồi thực hiện từng bước theo thứ tự. Sau mỗi bước xong mới chuyển sang bước tiếp theo.

---

## Chọn Scope

| Scope | Mô tả | Dùng khi |
|-------|-------|----------|
| **A. Backend Docs** | Chỉ refactor docs backend | Đã có docs workspace, chỉ cần backend |
| **B. Backend Docs + Cursor** | Docs + refactor .cursor | Refactor toàn bộ repo backend |
| **C. Full** | Workspace + Backend + Cursor | Refactor từ đầu toàn workspace |

---

## Scope A — Backend Docs (8 bước)

Thực hiện lần lượt, mỗi bước: **đọc nội dung file prompt** rồi **thực thi** theo đúng nội dung đó.

| # | Prompt | Đường dẫn file | Mục đích |
|---|--------|----------------|----------|
| 1 | Tạo khung docs | `prompts/repo-backend/docs/docs-recreate.md` | Tạo cấu trúc docs chuẩn |
| 2 | Review khung | `prompts/repo-backend/docs/docs-recreate-review.md` | Kiểm tra entry point, cấu trúc |
| 3 | Refactor docs | `prompts/repo-backend/docs/docs-refactor.md` | Refactor toàn bộ, gộp/xóa/sắp xếp |
| 4 | Reindex | `prompts/repo-backend/docs/docs-reindex.md` | Cập nhật navigation, cross-links |
| 5 | Review reindex | `prompts/repo-backend/docs/docs-reindex-review.md` | Kiểm tra navigation |
| 6 | Build module map | `prompts/repo-backend/docs/docs-build-module-map.md` | Đồng bộ module map với code |
| 7 | Build API overview | `prompts/repo-backend/docs/docs-build-api-overview.md` | Đồng bộ API overview với code |
| 8 | Audit docs | `prompts/maintenance/docs-audit.md` | Audit cuối, sửa vấn đề còn lại |

---

## Scope B — Backend Docs + Cursor (11 bước)

Chạy Scope A (bước 1–8), sau đó:

| # | Prompt | Đường dẫn file | Mục đích |
|---|--------|----------------|----------|
| 9 | Tạo .cursor | `prompts/repo-backend/cursor/cursor-create.md` | Thiết kế rules, prompts, tasks |
| 10 | Review .cursor | `prompts/repo-backend/cursor/cursor-create-review.md` | Review và tinh gọn .cursor |
| 11 | Audit .cursor | `prompts/maintenance/cursor-audit.md` | Audit .cursor cuối cùng |

---

## Scope C — Full (19 bước)

Chạy theo thứ tự trong `prompts/pipeline.md`:

### Phase 1 — Workspace Docs (1–5)
| # | Prompt | Đường dẫn |
|---|--------|-----------|
| 1 | docs-recreate | `prompts/workspace/docs/docs-recreate.md` |
| 2 | docs-recreate-review | `prompts/workspace/docs/docs-recreate-review.md` |
| 3 | docs-refactor | `prompts/workspace/docs/docs-refactor.md` |
| 4 | docs-reindex | `prompts/workspace/docs/docs-reindex.md` |
| 5 | docs-reindex-review | `prompts/workspace/docs/docs-reindex-review.md` |

### Phase 2 — Workspace Cursor (6–7)
| # | Prompt | Đường dẫn |
|---|--------|-----------|
| 6 | cursor-create | `prompts/workspace/cursor/cursor-create.md` |
| 7 | cursor-create-review | `prompts/workspace/cursor/cursor-create-review.md` |

### Phase 3 — Backend Docs (8–12)
| # | Prompt | Đường dẫn |
|---|--------|-----------|
| 8 | docs-recreate | `prompts/repo-backend/docs/docs-recreate.md` |
| 9 | docs-recreate-review | `prompts/repo-backend/docs/docs-recreate-review.md` |
| 10 | docs-refactor | `prompts/repo-backend/docs/docs-refactor.md` |
| 11 | docs-reindex | `prompts/repo-backend/docs/docs-reindex.md` |
| 12 | docs-reindex-review | `prompts/repo-backend/docs/docs-reindex-review.md` |

### Phase 4 — Backend Cursor (13–14)
| # | Prompt | Đường dẫn |
|---|--------|-----------|
| 13 | cursor-create | `prompts/repo-backend/cursor/cursor-create.md` |
| 14 | cursor-create-review | `prompts/repo-backend/cursor/cursor-create-review.md` |

### Phase 5 — Maintenance (15–19)
| # | Prompt | Đường dẫn |
|---|--------|-----------|
| 15 | docs-sync-with-code | `prompts/repo-backend/docs/docs-sync-with-code.md` |
| 16 | docs-build-module-map | `prompts/repo-backend/docs/docs-build-module-map.md` |
| 17 | docs-build-api-overview | `prompts/repo-backend/docs/docs-build-api-overview.md` |
| 18 | docs-audit | `prompts/maintenance/docs-audit.md` |
| 19 | cursor-audit | `prompts/maintenance/cursor-audit.md` |

---

## Quy Tắc Thực Hiện

1. **Mỗi bước:** Đọc toàn bộ nội dung file prompt → thực thi theo đúng yêu cầu
2. **Không bỏ qua:** Phải hoàn thành bước N trước khi sang bước N+1
3. **Thực thi thật:** Các prompt yêu cầu sửa thật, không chỉ đề xuất — phải thực hiện thay đổi
4. **Báo cáo ngắn:** Sau mỗi bước, tóm tắt 1–2 dòng đã làm gì (trừ khi prompt yêu cầu báo cáo chi tiết)

---

## Lưu Ý Đường Dẫn

- Tất cả đường dẫn tính từ thư mục `prompt-system/`
- Ví dụ: `prompts/repo-backend/docs/docs-refactor.md` = `prompt-system/prompts/repo-backend/docs/docs-refactor.md`

---

## Tài Liệu Tham Khảo

- `docs/05-development/QUY_TRINH_REFACTOR_DOCS.md` — Quy trình thiết kế chi tiết
- `docs/05-development/QUY_TRINH_REFACTOR_CURSOR.md` — Quy trình refactor .cursor
- `prompts/pipeline.md` — Danh sách pipeline gốc
