---
description: Khi nào đọc docs-shared, khi nào cập nhật docs local
alwaysApply: false
---

# Docs Protocol — Backend

## Nguyên tắc một nguồn (cross-repo)

- Tài liệu **dùng chung backend + frontend + agent** (API contract, data contract, vision, system-map, `opsTier` / live envelope…) chỉ sửa tại **`docs-shared`** trên workspace (thư mục cạnh repo, hoặc junction `folkgroup-backend/docs-shared` → đúng cây đó).
- **Không** nhân bản nội dung đó vào `docs/` hay vào thư mục `docs-shared` “copy” trong backend — tránh hai bản lệch nhau.
- Backend chỉ giữ **`docs/`** cho nội dung **chỉ backend** (handler pattern, module map nội bộ, THIET_KE triển khai, v.v.) và **link** sang `docs-shared` khi cần.

## Khi Nào Đọc docs-shared

- **Vision, concept** → `docs-shared/architecture/vision/ai-commerce-os-platform-l1.md` (đọc đầu để hiểu chúng ta đang làm gì)
- **Hợp đồng dữ liệu** (ID, event, action, decision, outcome, queue) → `docs-shared/architecture/data-contract/unified-data-contract.md` — đồng bộ với rule `.cursor/rules/data-contract.md`
- API contract, endpoint spec → `docs-shared/ai-context/folkform/api-context.md`
- Bổ sung **AI Decision live / `opsTier`** (hợp đồng JSON feed + ingest) → `docs-shared/architecture/vision/16-ai-decision-ops-tier.md`, `docs-shared/ai-context/folkform/api-context-ai-decision-ops-tier.md`
- Module design cross-repo → `docs-shared/ai-context/folkform/design/`
- System map, repo boundary → `docs-shared/system-map/system-map.md`
- Task chạm repo khác → đọc vision/ai-commerce-os-platform-l1, system-map và module-map trước

## Khi Nào Cập Nhật Docs

- API thay đổi → `docs/api/api-overview.md`, `docs-shared/ai-context` nếu contract thay đổi
- Module mới/xóa → `docs/module-map/backend-module-map.md`
- Kiến trúc thay đổi → `docs/architecture/overview.md`, `docs/02-architecture/`
