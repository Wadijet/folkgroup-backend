---
description: Agent phải đọc tài liệu trước khi sửa code
alwaysApply: true
---

Trước khi sửa code:

1. Đọc **docs/README.md** — entry point, thứ tự đọc
2. Đọc **docs-shared/architecture/vision/ai-commerce-os-platform-l1.md** — vision, chúng ta đang làm gì
3. Đọc **docs/architecture/overview.md** — kiến trúc layers
4. Đọc **docs/module-map/backend-module-map.md** — biết module thuộc đâu, sửa ở đâu
5. Đọc tài liệu liên quan trong docs/ (02-architecture/core, 05-development, api/). **Task chạm** `EmitDataChanged`, `decision_events_queue`, hook AI Decision, ingest CRM sau CRUD → **docs/05-development/NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md**
5b. **Task chạm** định danh công khai, payload event/message, Action–Execution, Rule I/O, Decision packet, Outcome / learning, contract liên module → **docs-shared/architecture/data-contract/unified-data-contract.md** (và rule `.cursor/rules/data-contract.md`)
6. Task chạm repo khác → đọc docs-shared/system-map/system-map.md
7. Tuân theo kiến trúc trong docs
8. Cập nhật docs nếu thay đổi kiến trúc