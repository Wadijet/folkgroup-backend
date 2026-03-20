---
description: Khi nào đọc docs-shared, khi nào cập nhật docs local
alwaysApply: false
---

# Docs Protocol — Backend

## Khi Nào Đọc docs-shared

- **Vision, concept** → `docs-shared/architecture/vision/ai-commerce-os-platform-l1.md` (đọc đầu để hiểu chúng ta đang làm gì)
- API contract, endpoint spec → `docs-shared/ai-context/folkform/api-context.md`
- Module design cross-repo → `docs-shared/ai-context/folkform/design/`
- System map, repo boundary → `docs-shared/system-map/system-map.md`
- Task chạm repo khác → đọc vision/ai-commerce-os-platform-l1, system-map và module-map trước

## Khi Nào Cập Nhật Docs

- API thay đổi → `docs/api/api-overview.md`, `docs-shared/ai-context` nếu contract thay đổi
- Module mới/xóa → `docs/module-map/backend-module-map.md`
- Kiến trúc thay đổi → `docs/architecture/overview.md`, `docs/02-architecture/`
