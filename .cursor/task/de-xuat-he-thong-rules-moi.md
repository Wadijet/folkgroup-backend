# Đề Xuất Hệ Thống AI Context – Rules, Tasks, Skills, Docs, Workflow

**Ngày:** 2025-03-13  
**Mục tiêu:** Hoàn thiện hệ thống AI context cho dự án FolkForm Backend, tận dụng đầy đủ tính năng Cursor (Rules, Tasks/Commands, Skills, Docs, Automations).

### Mục lục

| # | Mục | Nội dung |
|---|-----|----------|
| 0 | Tổng quan | 5 thành phần |
| 1–2 | Hiện trạng | Vấn đề, tính năng mới |
| 3–5 | Rules | Cấu trúc, chi tiết, AGENTS.md |
| 6–7 | Lợi ích, Kế hoạch | Triển khai |
| 8 | Rủi ro | Giảm thiểu |
| 9 | **Skills** | Workflow phức tạp |
| 10 | **Tasks/Commands** | Task ngắn, slash commands |
| 11 | **Docs** | Tài liệu, AGENTS.md |
| 12 | **Workflow** | Cursor Automations |
| 13–16 | Mở rộng, Cấu trúc, Checklist, Tham khảo | |

---

## 0. Tổng Quan 5 Thành Phần

| Thành phần | Vị trí | Mục đích | Kích hoạt |
|------------|--------|----------|-----------|
| **Rules** | `.cursor/rules/*.mdc` | Chuẩn code, convention, pattern | alwaysApply, globs, @mention |
| **Tasks/Commands** | `.cursor/commands/*.md` | Prompt ngắn, task đơn giản | `/command-name` |
| **Skills** | `.cursor/skills/<name>/` | Workflow nhiều bước, có script | `/skill-name`, auto |
| **Docs** | `docs/`, `AGENTS.md` | Tài liệu kiến trúc, hướng dẫn | Reference, nested AGENTS.md |
| **Workflow (Automations)** | Cursor dashboard | Agent chạy nền theo event | PR, Slack, cron, webhook |

### Quan Hệ Giữa Các Thành Phần

```
                    ┌─────────────────┐
                    │  Automations    │  ← Chạy nền (PR, Slack, cron)
                    │  (Dashboard)    │
                    └────────┬────────┘
                             │
┌──────────┐    ┌────────────┴────────────┐    ┌──────────┐
│  Rules   │◄───►│  Skills / Commands      │◄───►│   Docs   │
│ (chuẩn)  │    │  (workflow, task)       │    │ (tài liệu)│
└──────────┘    └─────────────────────────┘    └──────────┘
     │                      │                         │
     └──────────────────────┴─────────────────────────┘
                    Context cho Agent
```

- **Rules** → Chuẩn hóa output (code style, layer, response format)
- **Skills** → Workflow phức tạp (add-api, create-entity, code-review)
- **Commands** → Task nhanh (update-docs, update-rules)
- **Docs** → Nguồn chân lý (kiến trúc, API, business rules)
- **Automations** → Tự động hóa (review PR, tóm tắt tuần, incident response)

---

## 1. Hiện Trạng

| File | Kích thước | alwaysApply | Vấn đề |
|------|------------|-------------|--------|
| `folkgroup-backend.mdc` | ~480 dòng | true | Quá dài, luôn load → tốn token mọi session |
| `read-docs-first.md` | ~10 dòng | true | Đơn giản, có thể gộp vào rule khác |

**Hạn chế:**
- Một file chứa tất cả → AI phải xử lý 480+ dòng mỗi lần
- Không có rule theo file/ngữ cảnh → Handler, Service, Model đều nhận cùng nội dung
- Không tận dụng globs, description, manual @mention
- Trùng lặp với docs (BANG_QUY_TAC, them-api-moi.md)

---

## 2. Tính Năng Mới Cursor Cần Tận Dụng

| Tính năng | Mô tả | Lợi ích |
|-----------|-------|---------|
| **globs** | Rule chỉ áp dụng khi file khớp pattern | Chỉ load rule phù hợp → tiết kiệm token |
| **description** | AI quyết định có áp dụng không (Apply Intelligently) | Rule chỉ load khi liên quan |
| **alwaysApply: false** | Không load mặc định | Giảm context mỗi session |
| **@rule-name** | Gọi thủ công trong chat | Workflow cụ thể khi cần |
| **Subdirectory rules** | `.cursor/rules/` trong thư mục con | Rule theo module (api/, docs/) |
| **Reference @file** | Trỏ tới file thay vì copy nội dung | Tránh trùng lặp, dễ cập nhật |
| **AGENTS.md** | Hướng dẫn đơn giản, nested support | Hướng dẫn theo thư mục |

---

## 3. Cấu Trúc Đề Xuất (Rules)

```
.cursor/
├── rules/
│   ├── 00-core.mdc                    # alwaysApply: true – tối thiểu, ~80 dòng
│   ├── 01-handler.mdc                 # globs: api/**/handler/**/*.go
│   ├── 02-service.mdc                 # globs: api/**/service/**/*.go
│   ├── 03-model-dto.mdc               # globs: api/**/models/**/*.go, api/**/dto/**/*.go
│   ├── 04-database.mdc                # Apply Intelligently
│   ├── 05-response-format.mdc        # globs: api/**/handler/**/*.go
│   ├── 06-security-org.mdc            # Apply Intelligently
│   ├── 07-add-api-workflow.mdc        # Apply Manually – @add-api
│   ├── 08-read-docs-first.mdc         # alwaysApply: true
│   └── workflows/
│       └── create-entity.mdc          # @create-entity
├── commands/                          # Xem section 10
├── skills/                            # Xem section 9
└── task/                              # Archive sau khi migrate
```

*Cấu trúc đầy đủ xem section 14.*

---

## 4. Chi Tiết Từng Rule

### 4.1. `00-core.mdc` (alwaysApply: true)

**Mục đích:** Quy tắc cốt lõi, luôn cần – giữ ngắn (~80 dòng).

**Nội dung:**
- Ngôn ngữ: Tiếng Việt cho comment, message, log
- Naming: file, function, variable (tóm tắt)
- Error handling cơ bản
- Context usage
- Anti-patterns quan trọng nhất (5–7 điều)
- Link tới docs: `docs/09-ai-context/BANG_QUY_TAC_THIET_KE_HE_THONG.md`

**Không chứa:** Code mẫu dài, chi tiết từng layer, response format chi tiết.

---

### 4.2. `01-handler.mdc` (globs: `api/**/handler/**/*.go`)

**Kích hoạt:** Khi mở/sửa file trong `handler/`.

**Nội dung:**
- Handler pattern: BaseHandler, SafeHandlerWrapper
- Parse → validate → service → response
- Không business logic, không gọi DB trực tiếp
- Response format (tóm tắt, link tới 05)
- OwnerOrganizationID từ context

---

### 4.3. `02-service.mdc` (globs: `api/**/service/**/*.go`)

**Kích hoạt:** Khi mở/sửa file trong `service/`.

**Nội dung:**
- BaseServiceMongoImpl, context làm tham số đầu tiên
- Business logic, validation phức tạp
- ConvertMongoError, ErrNotFound
- Không parse HTTP, không trả response

---

### 4.4. `03-model-dto.mdc` (globs: `api/**/models/**/*.go`, `api/**/dto/**/*.go`)

**Kích hoạt:** Khi làm việc với model hoặc DTO.

**Nội dung:**
- Model: BSON/JSON tags, index, enum
- DTO: validate, transform, parse query
- Trách nhiệm: Model/DTO không gọi DB, không business logic

---

### 4.5. `04-database.mdc` (Apply Intelligently)

**description:** "MongoDB patterns, query, aggregation, N+1, cursor"

**Kích hoạt:** Khi AI nhận thấy câu hỏi/code liên quan DB.

**Nội dung:**
- bson.M/bson.D, primitive.ObjectID
- Query patterns, defer cursor.Close
- Tránh N+1, aggregation

---

### 4.6. `05-response-format.mdc` (globs: `api/**/handler/**/*.go`)

**Nội dung:**
- fiber.Map, code, message, data, status
- Success vs Error format
- Ví dụ ĐÚNG/SAI ngắn gọn

---

### 4.7. `06-security-org.mdc` (Apply Intelligently)

**description:** "OwnerOrganizationID, phân quyền org, filter theo organization"

**Nội dung:**
- OwnerOrganizationID vs OrganizationID
- getActiveOrganizationID, setOrganizationID
- Query luôn filter theo OwnerOrganizationID
- IsSystem protection

---

### 4.8. `07-add-api-workflow.mdc` (Apply Manually)

**description:** "Quy trình thêm API/entity mới – gọi bằng @add-api"

**Cách dùng:** Gõ `@add-api` hoặc `@07-add-api-workflow` khi cần thêm API.

**Nội dung:**
- 6 bước: Model → DTO → Service → Handler → Router → Collection
- Link: `docs/05-development/them-api-moi.md`
- Checklist nhanh

---

### 4.9. `08-read-docs-first.mdc` (alwaysApply: true)

**Nội dung:** (Gộp từ read-docs-first.md hiện tại)
- Trước khi sửa code: đọc docs liên quan
- Tuân kiến trúc trong docs
- Cập nhật docs nếu kiến trúc thay đổi
- Link bảng "Khi cần gì → đọc tài liệu nào" trong BANG_QUY_TAC

---

### 4.10. `workflows/create-entity.mdc` (Apply Manually)

**description:** "Template tạo entity mới – @create-entity"

**Nội dung:** Template code cho Model, DTO, Service, Handler (reference từ docs).

---

## 5. AGENTS.md (Nested Instructions)

*Chi tiết cấu trúc docs xem section 11. Đây là phần AGENTS.md cụ thể.*

### 5.1. Root `AGENTS.md`

Nội dung ngắn (~20 dòng):
- Dự án: FolkForm Backend (Go, Fiber, MongoDB)
- Luôn đọc docs trước khi sửa
- Rules trong `.cursor/rules/`, Skills trong `.cursor/skills/`, Commands trong `.cursor/commands/`
- Link BANG_QUY_TAC: `docs/09-ai-context/BANG_QUY_TAC_THIET_KE_HE_THONG.md`

### 5.2. `docs/AGENTS.md`

- Khi sửa docs: tuân cấu trúc thư mục, format markdown
- Cập nhật BANG_QUY_TAC khi thêm quy tắc mới
- Reference docs thay vì copy

### 5.3. `api/AGENTS.md` (tùy chọn)

- Khi sửa API: đọc `docs/05-development/them-api-moi.md`, `docs/09-ai-context/handler-pattern-crud-vs-custom.md`

---

## 6. Lợi Ích Dự Kiến

| Trước | Sau |
|-------|-----|
| ~490 dòng load mỗi session | ~80–150 dòng (core + read-docs) |
| Một rule cho mọi ngữ cảnh | Rule phù hợp theo file đang mở |
| Trùng lặp với docs | Reference docs, không copy |
| Khó tìm workflow thêm API | @add-api gọi đúng lúc |
| Không có rule theo thư mục | AGENTS.md trong docs/, api/ |

---

## 7. Kế Hoạch Triển Khai

### Phase 1: Rules (Ưu tiên cao)

| Bước | Nội dung | Ước lượng |
|------|----------|-----------|
| 1 | Tạo `00-core.mdc` (rút gọn từ folkgroup-backend) | 1h |
| 2 | Tạo `08-read-docs-first.mdc` (gộp read-docs-first) | 15 phút |
| 3 | Tách `01-handler`, `02-service`, `03-model-dto` từ rule hiện tại | 1.5h |
| 4 | Tạo `04-database`, `05-response-format`, `06-security-org` | 1h |
| 5 | Tạo `07-add-api-workflow`, `workflows/create-entity` | 45 phút |
| 6 | Xóa/archive `folkgroup-backend.mdc`, `read-docs-first.md` | 15 phút |
| 7 | Cập nhật BANG_QUY_TAC trỏ tới cấu trúc mới | 30 phút |
| 8 | Test: mở file handler, service, model – kiểm tra rule áp dụng | 30 phút |

### Phase 2: Docs và AGENTS.md

| Bước | Nội dung | Ước lượng |
|------|----------|-----------|
| 9 | Tạo `AGENTS.md` (root, docs/, api/) | 30 phút |
| 10 | Rà soát docs, đảm bảo link BANG_QUY_TAC đúng | 30 phút |

### Phase 3: Commands (Tasks)

| Bước | Nội dung | Ước lượng |
|------|----------|-----------|
| 11 | Tạo `.cursor/commands/` và `update-rules.md`, `update-docs.md` | 30 phút |
| 12 | Migrate nội dung từ `.cursor/task/` sang commands | 30 phút |
| 13 | Tạo `task/README.md` giải thích đã migrate | 15 phút |

### Phase 4: Skills

| Bước | Nội dung | Ước lượng |
|------|----------|-----------|
| 14 | Tạo skills: add-api, create-entity, code-review, run-tests | 1.5h |
| 15 | Tạo skills: generate-rules, organize-docs (từ task) | 1h |
| 16 | Chạy `/migrate-to-skills` nếu cần | 30 phút |

### Phase 5: Workflow (Automations) – Tùy chọn

| Bước | Nội dung | Ước lượng |
|------|----------|-----------|
| 17 | Cấu hình Automation Code Review (GitHub PR trigger) | 1h |
| 18 | Cấu hình Automation Weekly Summary (nếu dùng Slack) | 30 phút |

**Tổng Phase 1–2 (Rules + Docs):** ~6 giờ  
**Tổng Phase 1–4 (+ Commands + Skills):** ~10 giờ  
**Tổng đầy đủ (+ Automations):** ~11.5 giờ

---

## 8. Rủi Ro và Giảm Thiểu

| Rủi ro | Giảm thiểu |
|--------|------------|
| Rule không load khi cần | Mô tả description rõ, test với @mention |
| Thiếu quy tắc quan trọng | Giữ 00-core + 08 đủ nội dung cốt lõi |
| Team chưa quen @mention | Document trong README/onboarding |
| Globs không khớp | Kiểm tra path thực tế (api/internal/api/...) |

---

## 9. Skills và Workflows (Bổ Sung)

**Skills** khác với Rules: dùng cho workflow nhiều bước, có script, template. Cursor load từ `.cursor/skills/` hoặc `.agents/skills/`.

### 9.1. Rules vs Skills

| | Rules | Skills |
|---|-------|--------|
| **Mục đích** | Chuẩn code, convention, pattern | Workflow, task nhiều bước |
| **Kích hoạt** | alwaysApply, globs, @mention | Auto (khi relevant) hoặc `/skill-name` |
| **Nội dung** | Hướng dẫn tĩnh | Hướng dẫn + scripts, references, assets |
| **Ví dụ** | Handler pattern, response format | Thêm API, tạo entity, deploy |

### 9.2. Cấu Trúc Skills Đề Xuất

```
.cursor/skills/   (hoặc .agents/skills/)
├── add-api/
│   ├── SKILL.md           # Quy trình 6 bước thêm API
│   └── references/
│       └── TEMPLATE.md    # Template Model, DTO, Service, Handler
├── create-entity/
│   ├── SKILL.md           # Workflow tạo entity mới
│   └── references/
│       └── examples.md    # Ví dụ từ codebase
├── code-review/
│   └── SKILL.md           # Checklist review theo chuẩn dự án
└── run-tests/
    ├── SKILL.md           # Chạy test, coverage
    └── scripts/
        └── run-tests.sh   # Script chạy test (tùy chọn)
```

### 9.3. Chi Tiết Từng Skill

| Skill | Mô tả | Kích hoạt | disable-model-invocation |
|-------|-------|-----------|---------------------------|
| **add-api** | Quy trình thêm API: Model → DTO → Service → Handler → Router → Collection | `/add-api` hoặc khi user nói "thêm API" | false (AI tự áp dụng khi relevant) |
| **create-entity** | Template tạo entity mới với đầy đủ file | `/create-entity` | true (chỉ khi gọi thủ công) |
| **code-review** | Review code theo chuẩn FolkForm (layer, response, org) | Khi user yêu cầu review | false |
| **run-tests** | Chạy test, kiểm tra coverage | Khi user nói "chạy test", "test" | false |

### 9.4. Lợi Ích Skills

- **Scripts**: Có thể đính kèm script (ví dụ `run-tests.sh`) thay vì để AI tự viết
- **Progressive loading**: `references/` chỉ load khi cần → tiết kiệm token
- **Portable**: Chuẩn Agent Skills, dùng được trên nhiều tool (Cursor, Claude Code, Codex...)
- **Migrate**: Dùng `/migrate-to-skills` để chuyển rules "Apply Intelligently" và slash commands sang skills

### 9.5. Phân Chia Rules vs Skills

| Nội dung | Nên đặt ở |
|----------|-----------|
| Handler pattern, response format, naming | **Rules** (globs) |
| Quy trình thêm API (6 bước + template) | **Skill** (add-api) |
| Template tạo entity | **Skill** (create-entity) |
| Checklist review | **Skill** (code-review) |
| Chạy test, deploy | **Skill** (run-tests, deploy) |

### 9.6. Migrate Rules sang Skills

Một số rule "Apply Manually" có thể chuyển sang skill:

- `07-add-api-workflow.mdc` → skill `add-api`
- `workflows/create-entity.mdc` → skill `create-entity`

**Cách migrate:** Gõ `/migrate-to-skills` trong Agent chat. Cursor sẽ chuyển rules/commands phù hợp sang `.cursor/skills/`.

---

## 10. Tasks và Commands

**Commands** là prompt ngắn gọi bằng `/command-name`. Lưu trong `.cursor/commands/`. Cursor tự discover khi gõ `/`.

### 10.1. Tasks Hiện Có (`.cursor/task/`) → Chuyển Sang Commands/Skills

| Task hiện tại | Chuyển thành | Lý do |
|---------------|--------------|-------|
| `update-rules.md` | Command `/update-rules` | Đơn giản, 4 bước |
| `update-docs.md` | Command `/update-docs` | Đơn giản, 4 bước |
| `generate-rules.md` | Skill `generate-rules` | Phức tạp, nhiều bước |
| `organize-docs.md` | Skill `organize-docs` | Phức tạp, cấu trúc docs |

### 10.2. Cấu Trúc Commands Đề Xuất

```
.cursor/
├── commands/                    # Slash commands (prompt ngắn)
│   ├── update-rules.md          # Cập nhật rules theo thay đổi project
│   ├── update-docs.md           # Cập nhật docs khi có module/API mới
│   ├── fix-errors.md            # Sửa lỗi compile/lint
│   └── run-tests.md             # Chạy test (hoặc dùng skill run-tests)
└── task/                        # (Tùy chọn) Giữ làm reference hoặc archive
    └── README.md                # Giải thích task đã migrate sang đâu
```

### 10.3. Nội Dung Từng Command

**`update-rules.md`:**
```markdown
# Cập nhật Rules

Bạn là Rule Maintenance Agent. Nhiệm vụ: giữ rules đồng bộ với codebase.

1. Phân tích thay đổi project (git diff, file mới)
2. So sánh với rules hiện tại
3. Phát hiện rules lỗi thời
4. Cập nhật file trong .cursor/rules/ nếu cần

Không xóa rules trừ khi kiến trúc đã thay đổi. Output: danh sách file đã cập nhật.
```

**`update-docs.md`:**
```markdown
# Cập nhật Docs

Bạn là Documentation Maintenance Agent.

1. Phát hiện module mới, API mới, thay đổi schema
2. Cập nhật docs tương ứng trong docs/
3. Cập nhật BANG_QUY_TAC nếu có quy tắc mới

Không xóa docs trừ khi feature đã bị gỡ. Tham chiếu: docs/09-ai-context/BANG_QUY_TAC_THIET_KE_HE_THONG.md
```

### 10.4. Commands vs Skills – Khi Nào Dùng Gì

| Tiêu chí | Command | Skill |
|----------|---------|-------|
| Độ phức tạp | 1–5 bước, prompt ngắn | Nhiều bước, cần script/reference |
| Độ dài | < 50 dòng | Có thể > 100 dòng |
| Scripts | Không | Có (scripts/, references/) |
| Ví dụ | update-docs, fix-errors | add-api, generate-rules, organize-docs |

---

## 11. Docs (Tài Liệu)

Tài liệu là nguồn chân lý cho kiến trúc, API, business rules. Rules/Skills **reference** docs, không copy nội dung.

### 11.1. Cấu Trúc Docs Hiện Tại (Tham Chiếu)

```
docs/
├── 01-getting-started/
├── 02-architecture/          # Kiến trúc, core, business-logic
├── 03-api/
├── 04-deployment/
├── 05-development/           # them-api-moi.md, coding-standards.md
├── 06-testing/
├── 07-troubleshooting/
├── 08-archive/
└── 09-ai-context/            # BANG_QUY_TAC, handler-pattern
```

### 11.2. Docs Quan Trọng Cho AI Context

| Khi cần | Đọc |
|---------|-----|
| Tổng quan kiến trúc | `docs/02-architecture/core/tong-quan.md`, `README.md` |
| Auth, RBAC, Firebase | `02-architecture/core/authentication.md`, `rbac.md` |
| Thêm/sửa API | `docs/05-development/them-api-moi.md`, `endpoint-workflow-general.md` |
| Handler: CRUD vs custom | `docs/09-ai-context/handler-pattern-crud-vs-custom.md` |
| Phân quyền org | `co-che-quan-ly-ownerorganizationid.md`, `organization-data-authorization.md` |
| Tách layer | `02-architecture/refactoring/layer-separation-principles.md` |
| Bảng quy tắc đầy đủ | `docs/09-ai-context/BANG_QUY_TAC_THIET_KE_HE_THONG.md` |

### 11.3. AGENTS.md (Nested)

| Vị trí | Nội dung |
|--------|----------|
| **Root `AGENTS.md`** | Dự án FolkForm Backend; đọc docs trước khi sửa; rules trong .cursor/rules/ |
| **`docs/AGENTS.md`** | Khi sửa docs: tuân cấu trúc thư mục; cập nhật BANG_QUY_TAC khi thêm quy tắc |
| **`api/AGENTS.md`** | Khi sửa API: đọc them-api-moi.md, handler-pattern-crud-vs-custom.md |

### 11.4. Quy Tắc Docs Cho AI

1. **Reference, không copy**: Rules/Skills trỏ tới `docs/...` thay vì paste nội dung
2. **Một nguồn chân lý**: BANG_QUY_TAC là mục lục; chi tiết trong từng doc
3. **Cập nhật đồng bộ**: Khi thay đổi kiến trúc → cập nhật docs + rules
4. **Link tương đối**: Dùng path `docs/05-development/them-api-moi.md` trong instructions

---

## 12. Workflow (Cursor Automations)

**Automations** = Cloud agent chạy nền, kích hoạt theo event. Cấu hình tại [cursor.com/automations](https://cursor.com/automations), không lưu trong repo.

### 12.1. Triggers Có Sẵn

| Trigger | Ví dụ |
|---------|-------|
| **Schedule** | Mỗi sáng, mỗi tuần (cron) |
| **GitHub** | PR mở, PR merge, push branch |
| **Slack** | Tin nhắn mới trong channel |
| **Linear** | Issue tạo, status đổi |
| **PagerDuty** | Incident mới |
| **Webhook** | Gọi HTTP từ CI/CD, monitoring |

### 12.2. Automations Đề Xuất Cho Dự Án

| Automation | Trigger | Mục đích |
|-------------|---------|----------|
| **Security review** | Push to main | Audit diff tìm lỗ hổng bảo mật |
| **Code review** | PR opened/pushed | Review theo chuẩn FolkForm, gợi ý sửa |
| **Weekly summary** | Schedule (mỗi thứ 6) | Tóm tắt thay đổi tuần → Slack |
| **Test coverage** | Schedule (mỗi sáng) | Phát hiện code thiếu test, mở PR thêm test |
| **Incident response** | PagerDuty | Điều tra log, đề xuất fix |

### 12.3. Cấu Hình Automation (Ví Dụ)

**Prompt mẫu cho Code Review:**
```
Review PR theo chuẩn FolkForm Backend:
1. Kiểm tra Handler mỏng (parse → validate → service → response)
2. Kiểm tra business logic nằm trong Service
3. Kiểm tra response format (fiber.Map, code, message, data, status)
4. Kiểm tra filter theo OwnerOrganizationID khi query
5. Comment inline cho các vấn đề tìm thấy
6. Chỉ approve nếu không có vấn đề critical
```

**Tools cần bật:** Comment on pull request, Open pull request (nếu cần sửa)

### 12.4. Lưu Ý

- Automations dùng **Cloud Agent** → tính phí theo usage
- Cần kết nối GitHub, Slack (nếu dùng)
- Có thể config trong Cursor Settings → Rules (hoặc dashboard riêng)

---

## 13. Tùy Chọn Mở Rộng

1. **Team Rules** (nếu dùng Cursor Team/Enterprise): Đưa 00-core lên Team Rules để áp dụng mọi project.
2. **Remote Rules**: Import rule từ repo chung (ví dụ folkform-workspace) nếu nhiều repo dùng chung convention.
3. **Subdirectory `.cursor/rules/`**: Thêm `api/internal/api/.cursor/rules/` nếu cần rule riêng cho từng module (ads, meta, notification).
4. **Skills từ GitHub**: Import skill từ repo (Cursor Settings → Rules → Add Rule → Remote Rule (Github)).
5. **Automations template**: Dùng template từ [Cursor Marketplace](https://cursor.com/marketplace#automations).

---

## 14. Cấu Trúc Tổng Thể (Sau Khi Hoàn Thiện)

```
folkgroup-backend/
├── .cursor/
│   ├── rules/                   # Rules (chuẩn code)
│   │   ├── 00-core.mdc
│   │   ├── 01-handler.mdc
│   │   ├── 02-service.mdc
│   │   ├── 03-model-dto.mdc
│   │   ├── 04-database.mdc
│   │   ├── 05-response-format.mdc
│   │   ├── 06-security-org.mdc
│   │   ├── 07-add-api-workflow.mdc
│   │   ├── 08-read-docs-first.mdc
│   │   └── workflows/
│   │       └── create-entity.mdc
│   ├── commands/                # Commands (task ngắn)
│   │   ├── update-rules.md
│   │   ├── update-docs.md
│   │   └── fix-errors.md
│   ├── skills/                  # Skills (workflow phức tạp)
│   │   ├── add-api/
│   │   ├── create-entity/
│   │   ├── code-review/
│   │   ├── generate-rules/
│   │   ├── organize-docs/
│   │   └── run-tests/
│   └── task/                    # (Archive) Reference hoặc README
│       └── README.md
├── docs/                        # Tài liệu
│   ├── AGENTS.md
│   ├── 02-architecture/
│   ├── 05-development/
│   └── 09-ai-context/
├── api/
│   └── AGENTS.md                # (Tùy chọn)
└── AGENTS.md                    # Root instructions
```

---

## 15. Checklist Trước Khi Áp Dụng

### Rules
- [ ] Đã review cấu trúc thư mục `api/` thực tế (để globs chính xác)
- [ ] Đã backup `folkgroup-backend.mdc` trước khi thay đổi
- [ ] Đã test rule áp dụng khi mở file handler, service, model

### Tasks/Commands
- [ ] Đã tạo `.cursor/commands/` với update-rules, update-docs
- [ ] Đã test `/update-rules`, `/update-docs` trong chat
- [ ] Đã xử lý file cũ trong `.cursor/task/` (archive hoặc README)

### Skills
- [ ] Đã tạo skills: add-api, create-entity, code-review
- [ ] Đã test `/add-api`, `/create-entity`
- [ ] Đã migrate generate-rules, organize-docs từ task

### Docs
- [ ] Đã tạo AGENTS.md (root, docs/, api/)
- [ ] Đã xác nhận path docs và link BANG_QUY_TAC
- [ ] Đã rà soát bảng "Khi cần gì → đọc tài liệu nào"

### Workflow (Automations)
- [ ] (Tùy chọn) Đã kết nối GitHub/Slack
- [ ] (Tùy chọn) Đã cấu hình ít nhất 1 automation

### Chung
- [ ] Đã thống nhất với team về cấu trúc mới
- [ ] Đã document trong README hoặc onboarding

---

## 16. Tài Liệu Tham Khảo

| Nguồn | Link / Path |
|-------|-------------|
| Cursor Rules | [cursor.com/docs/context/rules](https://cursor.com/docs/context/rules) |
| Cursor Skills | [cursor.com/docs/context/skills](https://cursor.com/docs/context/skills) |
| Cursor Automations | [cursor.com/docs/cloud-agent/automations](https://cursor.com/docs/cloud-agent/automations) |
| Agent Skills Standard | [agentskills.io](https://agentskills.io) |
| create-rule SKILL | `~/.cursor/skills-cursor/create-rule/SKILL.md` |
| create-skill SKILL | `~/.cursor/skills-cursor/create-skill/SKILL.md` |
| Bảng quy tắc dự án | `docs/09-ai-context/BANG_QUY_TAC_THIET_KE_HE_THONG.md` |

---

**Tác giả đề xuất:** AI Assistant  
**Cập nhật:** 2025-03-13
