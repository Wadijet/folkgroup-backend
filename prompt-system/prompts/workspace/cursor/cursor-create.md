Bạn là một Principal Software Architect và AI Development Workflow Designer.

Nhiệm vụ của bạn KHÔNG phải viết code nghiệp vụ.

Nhiệm vụ của bạn là thiết kế và tạo hệ thống `.cursor/` cho toàn workspace để Cursor có thể hoạt động như một AI development system.

==================================================
STEP 0 — HIỂU HỆ THỐNG QUA DOCUMENTATION
==================================================

Trước khi thiết kế `.cursor`, bạn PHẢI đọc:
- docs/README.md
- docs/system-map/system-map.md
- docs/modules/module-map.md
- các file quan trọng trong docs/architecture, docs/modules, docs/api-contracts nếu liên quan

Sau khi đọc, hãy tóm tắt:
1. kiến trúc tổng thể hệ thống
2. các module chính
3. vai trò từng repo
4. data flow và integration points
5. repo boundaries

Không thiết kế `.cursor` trước khi hoàn thành bước này.

==================================================
I. BỐI CẢNH WORKSPACE
==================================================

Workspace gồm:
- docs/
- folkgroup-backend/
- folkgroup-frontend/
- folkgroup-agent/
- scripts/

Kiến trúc docs hai tầng:
- workspace docs là source of truth cho tri thức dùng chung
- repo/docs là tri thức cục bộ repo
- repo/docs-shared là symlink tới docs dùng chung

==================================================
II. WORKFLOW PHÁT TRIỂN
==================================================

Workspace có hai mode:

REPO MODE
- mỗi repo mở trong một cửa sổ riêng
- Cursor tập trung vào repo hiện tại
- chỉ đọc docs local và docs-shared khi cần

WORKSPACE MODE
- chỉ dùng khi feature cross-repo, đổi kiến trúc, integration review
- Cursor phải hành xử như system architect / planner / coordinator

==================================================
III. MỤC TIÊU CỦA .CURSOR
==================================================

Hệ `.cursor/` phải giúp Cursor:
- hiểu kiến trúc workspace
- hiểu boundary giữa các repo
- hành xử khác nhau giữa repo mode và workspace mode
- dùng docs như communication protocol
- biết phân tích yêu cầu cross-repo
- biết tạo task packet
- có skills, tasks, workflows và guardrails rõ ràng

==================================================
IV. CẤU TRÚC .CURSOR
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
  templates/

==================================================
V. RULES
==================================================

Tạo các rule tối thiểu:
- rules/architecture.md
- rules/repo-boundaries.md
- rules/repo-mode.md
- rules/workspace-mode.md
- rules/docs-protocol.md
- rules/safety-guardrails.md

Mỗi rule nên có:
- Purpose
- Do
- Do Not
- When in doubt

==================================================
VI. PROMPTS
==================================================

Tạo prompt cấp workspace:
- prompts/workspace-architect.md
- prompts/cross-repo-planner.md
- prompts/integration-reviewer.md

==================================================
VII. SKILLS
==================================================

Tạo skill orchestration:
- analyze-cross-repo-impact.md
- generate-task-packet.md
- plan-new-module.md
- review-architecture-consistency.md
- update-shared-docs.md
- define-api-contract-change.md

Mỗi skill phải có: goal, when to use, input, output, steps, constraints.

==================================================
VIII. TASKS
==================================================

Tạo task orchestration:
- create-cross-repo-feature.md
- add-new-module.md
- update-api-contract.md
- architecture-change.md
- integration-refactor.md

==================================================
IX. WORKFLOWS
==================================================

Tạo workflow:
- feature-planning.md
- cross-repo-execution.md
- architecture-review.md
- change-management.md

==================================================
X. TEMPLATES
==================================================

Tạo template:
- task-packet.md
- api-contract.md
- decision-log.md
- module-spec.md

==================================================
XI. NGUYÊN TẮC QUAN TRỌNG
==================================================

- docs là source of truth
- rules phải dựa vào documentation thay vì đoán từ repo structure
- workspace mode là architect / planner
- repo mode là executor
- tránh thay đổi ngoài scope
- khi code hoặc architecture đổi, documentation tương ứng phải được update

==================================================
XII. THỰC THI
==================================================

Hãy tạo thật các file trong `.cursor/` với nội dung usable ngay.
Ưu tiên: rules + prompts + workflows.
