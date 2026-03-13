# Prompt: Refactor Docs Toàn Bộ — Quy Trình Đầy Đủ

**Mục đích:** Chạy toàn bộ quy trình refactor docs theo tài liệu thiết kế. Dùng khi cần refactor từ đầu hoặc refactor lớn.

**Tham chiếu:** `docs/05-development/QUY_TRINH_REFACTOR_DOCS.md`

---

## Nhiệm Vụ

Bạn là Principal Documentation Architect. Thực hiện **toàn bộ** quy trình refactor docs backend theo tài liệu `docs/05-development/QUY_TRINH_REFACTOR_DOCS.md`.

---

## Thứ Tự Thực Hiện

### 1. Phase 0 — Chuẩn Bị
- Đọc docs-shared/ (nếu có)
- Đọc docs/ hiện tại
- Quét codebase: api/internal/api/, routers, services
- So sánh docs vs code, xác định mismatch

### 2. Phase 1 — Phân Tích
- Quét toàn bộ docs, lập danh sách: keep / merge / move / delete / rewrite
- Phân loại theo vai trò (architecture, module-map, api, domain, dev guide, archive)

### 3. Phase 2 — Refactor Cấu Trúc
- Tạo cấu trúc thư mục mới nếu cần
- Di chuyển, đổi tên, gộp, xóa/archive file theo danh sách

### 4. Phase 3 — Nội Dung
- Viết lại / cập nhật tài liệu thiếu hoặc sai
- Áp dụng format chuẩn (Purpose, Scope, Changelog, Related Code)
- Cập nhật cross-links, docs/README.md

### 5. Phase 4 — Đồng Bộ Codebase
- Chạy logic tương đương docs-build-module-map
- Chạy logic tương đương docs-build-api-overview
- Kiểm tra mọi đường dẫn khớp code

### 6. Phase 5 — Audit
- Chạy logic tương đương docs-audit
- Chạy logic tương đương docs-recreate-review
- Chạy logic tương đương docs-reindex-review
- Tạo/cập nhật docs/CHANGELOG.md

---

## Quy Tắc

- **Thực hiện thay đổi trực tiếp** trong repository, không chỉ đề xuất
- Tuân thủ cấu trúc và format trong QUY_TRINH_REFACTOR_DOCS.md
- Phân biệt rõ local docs vs docs-shared
- Mỗi thay đổi quan trọng ghi vào CHANGELOG

---

## Sau Khi Xong

Thông báo:
1. Tóm tắt thay đổi (số file moved/merged/deleted/created)
2. Cấu trúc docs mới
3. Các vấn đề còn tồn (nếu có)
4. Gợi ý chạy quy trình refactor .cursor: `docs/05-development/QUY_TRINH_REFACTOR_CURSOR.md`
