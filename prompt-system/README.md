# Prompt System for Docs Refactor + Cursor Setup

Gói này gồm bộ prompt hoàn chỉnh để:
- refactor hệ thống tài liệu
- reindex tài liệu
- tạo `.cursor`
- review `.cursor`
- duy trì living docs theo codebase

## Cấu trúc
- `prompts/prompt-quy-trinh-refactor-docs.md`: **prompt quy trình** — chạy lần lượt các prompt cần thiết (chọn scope A/B/C)
- `prompts/pipeline.md`: thứ tự chạy chuẩn (danh sách)
- `prompts/workspace/...`: prompt cấp workspace
- `prompts/repo-backend/...`: prompt cấp repo backend
- `prompts/maintenance/...`: prompt bảo trì định kỳ

## Thứ tự chạy khuyến nghị
1. workspace/docs/docs-recreate.md
2. workspace/docs/docs-recreate-review.md
3. workspace/docs/docs-refactor.md
4. workspace/docs/docs-reindex.md
5. workspace/docs/docs-reindex-review.md
6. workspace/cursor/cursor-create.md
7. workspace/cursor/cursor-create-review.md
8. repo-backend/docs/docs-recreate.md
9. repo-backend/docs/docs-recreate-review.md
10. repo-backend/docs/docs-refactor.md
11. repo-backend/docs/docs-reindex.md
12. repo-backend/docs/docs-reindex-review.md
13. repo-backend/cursor/cursor-create.md
14. repo-backend/cursor/cursor-create-review.md
15. maintenance/docs-sync-with-code.md (chạy định kỳ)
16. maintenance/docs-build-module-map.md (khi module đổi)
17. maintenance/docs-build-api-overview.md (khi API đổi)
18. maintenance/docs-audit.md / cursor-audit.md (audit định kỳ)

## Quy trình refactor docs đầy đủ
- **Tài liệu thiết kế:** `docs/05-development/QUY_TRINH_REFACTOR_DOCS.md`
- **Prompt chạy toàn bộ:** `prompts/repo-backend/docs/docs-refactor-full-process.md`
- **Quy trình refactor .cursor:** `docs/05-development/QUY_TRINH_REFACTOR_CURSOR.md`

## Ghi chú
- Prompt đã được viết theo phong cách ép Cursor phải thực thi thật, không chỉ audit.
- Prompt backend giả định repo có `docs/` và `docs-shared/`.
