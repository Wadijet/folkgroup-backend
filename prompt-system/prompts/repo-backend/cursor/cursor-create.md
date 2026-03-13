Bạn là một Principal Backend Architect, AI Development Workflow Architect và Cursor System Designer.

Nhiệm vụ của bạn KHÔNG phải viết code nghiệp vụ mới.
Nhiệm vụ của bạn là thiết kế và tạo hệ thống `.cursor` cho repository backend.

==================================================
I. BỐI CẢNH REPOSITORY
==================================================

Repository hiện tại là:
folkgroup-backend/

Backend sở hữu:
- business logic
- domain services
- data access
- validation rules
- API implementation
- integration endpoints

Backend không sở hữu:
- UI logic
- frontend presentation
- automation logic của agent
- canonical system architecture

==================================================
II. KIẾN TRÚC TÀI LIỆU
==================================================

Repo backend có hai tầng tài liệu:
1. docs/ → local backend docs
2. docs-shared/ → symlink tới ../../docs

==================================================
III. ĐỌC 3 LỚP TRI THỨC
==================================================

Trước khi thiết kế `.cursor`, bạn phải đọc:
1. docs-shared/
   - docs-shared/README.md
   - docs-shared/system-map/system-map.md
   - docs-shared/modules/module-map.md
2. docs/
   - docs/README.md
   - docs/architecture/
   - docs/module-map/
   - docs/domain/
   - docs/api/
   - docs/conventions/
3. codebase backend

Sau khi đọc, hãy đối chiếu:
- shared docs
- local docs
- implementation thực tế

Xác định mismatch và dùng hiểu biết đó để thiết kế `.cursor`.

==================================================
IV. MỤC TIÊU CỦA .CURSOR
==================================================

`.cursor` phải giúp Cursor:
- hiểu backend architecture
- hiểu domain logic
- hiểu module boundaries
- hiểu API structure
- tránh sửa repo ngoài scope backend
- biết khi nào cần đọc docs-shared

==================================================
V. CẤU TRÚC .CURSOR
==================================================

Tạo thư mục:
.cursor/

Cấu trúc tối thiểu:
.cursor/
  rules/
  prompts/
  skills/
  tasks/
  workflows/

==================================================
VI. RULES
==================================================

Tạo rules tối thiểu:
- rules/backend-architecture.md
- rules/repo-boundaries.md
- rules/domain-logic.md
- rules/api-structure.md
- rules/docs-protocol.md
- rules/safety-guardrails.md

Rules phải mô tả:
- backend ownership
- module boundaries
- domain layer rules
- API conventions
- khi nào đọc docs-shared
- khi nào bắt buộc update docs local hoặc shared docs

==================================================
VII. PROMPTS
==================================================

Tạo prompt cho backend development:
- prompts/backend-architect.md
- prompts/backend-feature-design.md
- prompts/backend-code-review.md
- prompts/backend-refactor.md

==================================================
VIII. SKILLS
==================================================

Tạo skills:
- analyze-backend-module.md
- design-api-endpoint.md
- review-domain-logic.md
- refactor-service-layer.md
- validate-data-flow.md

==================================================
IX. TASKS
==================================================

Tạo tasks:
- add-api-endpoint.md
- add-domain-service.md
- refactor-module.md
- update-data-model.md
- fix-backend-bug.md

==================================================
X. WORKFLOWS
==================================================

Tạo workflows:
- add-feature.md
- refactor-module.md
- debug-backend-issue.md
- review-backend-change.md

==================================================
XI. SAFETY GUARDRAILS
==================================================

Cursor phải tránh:
- sửa repo frontend
- sửa repo agent
- thay đổi shared docs nếu không có task rõ ràng
- tạo coupling giữa repo không cần thiết

==================================================
XII. AI READING ORDER
==================================================

Định nghĩa thứ tự Cursor nên đọc khi mở repo backend:
1. docs/README.md
2. docs/architecture/overview.md
3. docs/module-map/backend-module-map.md
4. docs/domain/domain-overview.md
5. docs/api/api-overview.md
6. docs-shared khi cần context hệ thống hoặc canonical contract

==================================================
XIII. THỰC THI
==================================================

Hãy tạo thật các file trong `.cursor/` với nội dung usable ngay.
Ưu tiên rules + prompts + workflows.
