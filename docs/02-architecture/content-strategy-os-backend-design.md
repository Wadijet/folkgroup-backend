# Content Strategy Operating System - Backend Design

## 📋 Tổng Quan

**Content Strategy Operating System** là hệ thống quản lý và tạo nội dung tự động với 8 cấp độ nội dung (L1-L8), sử dụng AI để generate và judge content, hỗ trợ A/B testing và learning từ metrics.

**Ngày tạo:** 2025-01-XX  
**Phiên bản:** v1.0  
**Kiến trúc:** 3 Modules độc lập

---

## 🎯 Mục Tiêu Hệ Thống

1. **Tự động hóa quy trình tạo nội dung** từ ý tưởng đến xuất bản
2. **Đo lường và so sánh** hiệu quả của content (human vs AI, prompt versions, etc.)
3. **Học hỏi và tối ưu** từ dữ liệu thực tế (metrics, A/B testing)
4. **Hỗ trợ human-in-the-loop** cho approval và chỉnh sửa
5. **Traceability đầy đủ** từ content đến AI runs, prompts, và metrics

---

## 📊 8 Cấp Độ Nội Dung (Content Levels)

Hệ thống quản lý 8 cấp độ nội dung theo cấu trúc phân cấp:

```
L1: Layer (Lớp)
  └─ L2: STP (Segmentation, Targeting, Positioning)
      └─ L3: Insight (Thông tin chi tiết)
          └─ L4: Content Line (Dòng nội dung)
              └─ L5: Gene (Gen nội dung)
                  └─ L6: Script (Kịch bản)
                      └─ L7: Video (Video)
                          └─ L8: Publication (Xuất bản)
```

### Mô Tả Chi Tiết

| Level | Tên | Mô Tả | Ví Dụ |
|-------|-----|-------|-------|
| **L1** | Layer | Lớp nội dung tổng quát | "Giải trí", "Giáo dục", "Kinh doanh" |
| **L2** | STP | Phân khúc, đối tượng, định vị | "Gen Z, 18-25, thích TikTok" |
| **L3** | Insight | Thông tin chi tiết, góc nhìn | "Gen Z thích nội dung ngắn, visual" |
| **L4** | Content Line | Dòng nội dung cụ thể | "Tips học tập hiệu quả" |
| **L5** | Gene | Gen nội dung (tone, style) | "Vui vẻ, năng động, emoji" |
| **L6** | Script | Kịch bản chi tiết | "Hook: 3 giây đầu... Body: ..." |
| **L7** | Video | Video đã render | File video.mp4 |
| **L8** | Publication | Xuất bản trên platform | Facebook post, TikTok video |

---

## 🏗️ Kiến Trúc 3 Modules

Hệ thống được chia thành 3 modules độc lập:

```
┌─────────────────────────────────────────────────────────────┐
│              Module 1: Content Storage                      │
│         (Pure Storage - Lưu trữ nội dung)                   │
│                                                              │
│  Collections:                                               │
│  - content_nodes (L1-L6)                                     │
│  - videos (L7)                                               │
│  - publications (L8)                                         │
│  - draft_content_nodes, draft_videos, draft_publications    │
│  - draft_approvals                                           │
│                                                              │
│  Chức năng: CRUD operations, approval workflow             │
└─────────────────────────────────────────────────────────────┘
                                ↓
                        ┌──────────────────┐
                        │  HTTP API (REST)  │
                        └──────────────────┘
                                ↓
┌─────────────────────────────────────────────────────────────┐
│              Module 2: AI Service                           │
│    (AI Orchestration - Gọi AI APIs)                         │
│                                                              │
│  Collections:                                                │
│  - workflows, steps, prompt_templates                       │
│  - workflow_runs, step_runs                                 │
│  - generation_batches, candidates                           │
│  - ai_runs                                                   │
│  - workflow_commands (queue)                                │
│                                                              │
│  Chức năng: AI generation, judging, workflow execution      │
└─────────────────────────────────────────────────────────────┘
                                ↓
                        ┌──────────────────┐
                        │  HTTP API (REST) │
                        └──────────────────┘
                                ↓
┌─────────────────────────────────────────────────────────────┐
│              Module 3: Analytics/Learning                   │
│    (Ghép data & Tính toán)                                  │
│                                                              │
│  Collections:                                                │
│  - content_performance                                       │
│  - ai_performance                                           │
│  - content_experiments, content_variants                    │
│  - experiment_results                                       │
│  - learning_insights, recommendations                       │
│  - rollup_scores                                            │
│                                                              │
│  Chức năng: Metrics aggregation, A/B testing, learning     │
└─────────────────────────────────────────────────────────────┘
```

---

## 📦 Module 1: Content Storage

### Mục Đích

**Module 1 là hệ thống lưu trữ nội dung thuần túy:**
- Lưu trữ content nodes (L1-L6)
- Lưu trữ videos (L7)
- Lưu trữ publications (L8)
- Lưu trữ drafts (bản nháp)
- **KHÔNG** có business logic phức tạp
- **KHÔNG** tính toán metrics
- **KHÔNG** gọi AI
- Chỉ CRUD operations

### Chức Năng Cụ Thể

**1. Content Nodes Management (L1-L6):**
- Create: Tạo content node (thủ công hoặc từ Module 2)
- Read: Đọc content node theo ID, type, parent
- Update: Cập nhật content node
- Delete: Xóa content node (soft delete)
- Tree operations: Lấy children, ancestors

**2. Videos Management (L7):**
- Create: Tạo video record
- Read: Đọc video theo ID, script ID
- Update: Cập nhật video (status, asset URL, metadata)
- Link: Link video với script

**3. Publications Management (L8):**
- Create: Tạo publication record
- Read: Đọc publication theo ID, video ID, platform
- Update: Cập nhật publication (status, metrics)
- **MetricsRaw: Lưu raw metrics từ platform (views, likes, shares, comments)**
  - MetricsRaw là thuộc tính của Publication
  - Lưu trực tiếp trong `publications` collection
  - Module 3 đọc MetricsRaw để tính toán performance

**4. Drafts Management:**
- Create: Tạo draft node/video/publication
- Read: Đọc draft theo ID, workflow run ID
- Update: Cập nhật draft (edit trước khi approve)
- Commit: Commit draft → production (sau khi approve)
- Approval: Quản lý approval requests

### Data Models

**Collections:**

| Collection | Nhiệm Vụ | Mô Tả Chi Tiết |
|------------|----------|----------------|
| `content_nodes` | Lưu trữ production nodes | Content nodes đã được duyệt và commit (L1-L6: Layer, STP, Insight, Content Line, Gene, Script) - Có creator type, creation method |
| `videos` | Lưu trữ production videos | Videos đã được duyệt và commit (L7) - Link với script, có asset URL, metadata |
| `publications` | Lưu trữ production publications | Publications đã được duyệt và commit (L8) - Link với video, platform, **có MetricsRaw (views, likes, shares, comments)** |
| `draft_content_nodes` | Lưu trữ draft nodes | Bản nháp content nodes (L1-L6) - Chưa được duyệt, có approval status, link về workflow run ID |
| `draft_videos` | Lưu trữ draft videos | Bản nháp videos (L7) - Chưa được duyệt, link về draft script |
| `draft_publications` | Lưu trữ draft publications | Bản nháp publications (L8) - Chưa được duyệt, link về draft video |
| `draft_approvals` | Quản lý approvals | Approval requests và decisions - Track approval workflow, có status (pending, approved, rejected) |

**Lưu ý về MetricsRaw:**
- **MetricsRaw lưu trong `publications` collection (Module 1)**
- Format: `{ "views": 1000, "likes": 50, "shares": 10, "comments": 5, "platform_specific": {...} }`
- Update qua API: `PUT /api/v1/content/publications/:id/metrics` (hoặc dùng CRUD: `PUT /api/v1/content/publications/update-by-id/:id`)
- Module 3 đọc MetricsRaw từ Module 1 để tính toán performance

**Fields chính:**
- Content Node: ID, Type, ParentID, Name, Text, Status, CreatorType, CreationMethod, CreatedBy, CreatedAt
- Video: ID, ScriptID, Status, AssetURL, Meta, CreatedAt
- Publication: ID, VideoID, Platform, Status, MetricsRaw, PublishedAt

### API Design

**Nguyên tắc:**
- ✅ **Ưu tiên CRUD mặc định từ BaseHandler:** Tất cả collections sử dụng CRUD operations có sẵn (InsertOne, Find, FindOneById, UpdateById, DeleteById, etc.)
- ✅ **Hạn chế custom endpoints:** Chỉ tạo custom endpoint khi có business logic phức tạp không thể thực hiện bằng CRUD + filter query
- ✅ **Sử dụng filter query:** Query data bằng filter query string thay vì tạo endpoint đặc thù
- ✅ **Custom endpoints chỉ cho business logic đặc thù:** GetTree (recursive), CommitDraftNode (workflow), Approval workflows

### Use Cases

**1. Human tạo content thủ công:**
```
Human tạo content node → Module 1 lưu trực tiếp vào content_nodes (không qua draft)
- Type: layer, stp, insight, etc.
- CreatorType: "human"
- CreationMethod: "manual"
```

**2. AI tạo content (từ Module 2):**
```
Module 2 tạo draft node → Module 1 lưu vào draft_content_nodes
- Type: stp, insight, etc.
- WorkflowRunId: link về workflow run
- CreatedByRunId: link về AI run
→ Human review → Approve → Commit → Production
```

**3. Query content với filter:**
```
Query content nodes/videos/publications bằng filter query
- Filter theo type, parentId, status, workflowRunId, etc.
- Hỗ trợ pagination, sorting
```

**4. Update metrics từ platform:**
```
External system update MetricsRaw trong publications
→ Module 1 cập nhật metricsRaw
→ Module 3 đọc MetricsRaw để tính toán performance
```

**5. Approval workflow:**
```
Human request approval → Module 1 tạo approval request
Human approve/reject → Module 1 commit drafts → production (nếu approve)
```

---

## 🤖 Module 2: AI Service

### Mục Đích

**Module 2 là hệ thống điều phối AI:**
- Quản lý workflows và steps
- Quản lý prompt templates
- Thực thi workflows (generate content)
- Judge content (scoring)
- A/B testing prompts và models
- **KHÔNG** lưu trữ content (chỉ tạo draft trong Module 1)
- **KHÔNG** tính toán metrics (Module 3 làm)

### Chức Năng Cụ Thể

**1. Workflow Management:**
- Define workflows (sequence of steps)
- Dynamic step generation (AI tạo steps tiếp theo dựa trên context)
- Step types: GENERATE, JUDGE, STEP_GENERATION

**2. Prompt Template Management:**
- Versioned prompt templates
- Variable substitution
- Strict JSON input/output schemas
- Types: `generate`, `judge`, `step_generation`

**3. Workflow Execution:**
- Execute workflows (tạo workflow runs)
- Generate content candidates
- Judge candidates (scoring)
- Select best candidates
- Create draft nodes trong Module 1

**4. Command Queue:**
- Queue cho bot (folkgroup-agent) xử lý
- Bot query commands và tạo workers
- Process commands async

**5. AI Run Tracking:**
- Log tất cả AI calls (prompt, model, cost, latency, quality score)
- Traceability: link từ content → candidate → AI run

### Data Models

**Collections:**

| Collection | Nhiệm Vụ | Mô Tả Chi Tiết |
|------------|----------|----------------|
| `workflows` | Định nghĩa workflows | Workflow definitions với steps, policies |
| `steps` | Định nghĩa steps | Step definitions với input/output schemas, prompt template IDs |
| `prompt_templates` | Quản lý prompts | Prompt templates với versioning, variables, types |
| `workflow_runs` | Lịch sử workflow runs | Workflow execution history |
| `step_runs` | Lịch sử step runs | Step execution history trong workflow runs |
| `generation_batches` | Batches của candidates | Batches chứa nhiều candidates được generate cùng lúc |
| `candidates` | Content candidates | Candidates được generate, có judge scores, selected flag |
| `ai_runs` | Lịch sử AI calls | Tất cả AI API calls (GENERATE + JUDGE) với cost, latency, quality |
| `workflow_commands` | Command queue | Queue commands cho bot xử lý (START_WORKFLOW, etc.) |

**Lưu ý quan trọng:**
- Module 2 **KHÔNG** có draft collections
- Module 2 chỉ lưu lịch sử runs, không phân biệt draft/production
- Draft chỉ tồn tại trong Module 1 (content approval)

### Workflow Execution Flow

```
1. Bot (folkgroup-agent) query workflow_commands queue
   ↓
2. Bot tạo worker để xử lý command
   ↓
3. Bot gọi Module 2 API: POST /api/v2/workflow-runs
   ↓
4. Module 2 execute workflow:
   a. Lấy workflow definition
   b. Execute từng step:
      - GENERATE: Gọi AI → Tạo candidates
      - JUDGE: Gọi AI → Score candidates
      - STEP_GENERATION: Gọi AI → Tạo steps tiếp theo
   c. Select best candidates
   d. Tạo draft nodes trong Module 1 (POST /api/v1/drafts/nodes)
   ↓
5. Workflow run completed
   ↓
6. Human review drafts trong Module 1
   ↓
7. Human approve → Commit drafts → Production
```

### Two-Step Level Transition (GENERATE/JUDGE)

Mỗi level transition (ví dụ: Layer → STP) phải có 2 bước riêng biệt:

1. **GENERATE Step:**
   - AI generate content candidates
   - Tạo nhiều candidates (batch)
   - Lưu vào `candidates` collection

2. **JUDGE Step:**
   - AI judge/scoring candidates
   - Tính quality score cho mỗi candidate
   - Select candidate tốt nhất
   - Commit candidate → draft node trong Module 1

**Lý do:**
- Tách biệt generation và judging để A/B testing
- Có thể test prompt versions riêng cho GENERATE và JUDGE
- Có thể so sánh judge scores với actual performance

### API Design

**Nguyên tắc:** Ưu tiên CRUD mặc định từ BaseHandler, chỉ tạo custom endpoint khi có business logic phức tạp (workflow execution, orchestration).

### Use Cases

**1. Bot xử lý workflow command:**
```
1. Bot query workflow_commands queue (filter: status=pending)
2. Bot tạo worker cho mỗi command
3. Worker tạo workflow run → Module 2 execute workflow
4. Module 2 execute workflow → Tạo drafts trong Module 1
5. Bot update command status = completed
```

**2. Query workflow runs/AI runs:**
```
Query workflow runs, step runs, AI runs, candidates bằng filter query
- Filter theo workflowId, status, promptTemplateId, provider, etc.
- Module 3 đọc AI runs để tính toán metrics
```

**3. Human prompt individual step:**
```
Human trigger step execution với custom prompt
→ Module 2 execute step với custom prompt
→ Tạo draft trong Module 1
```

---

## 📊 Module 3: Analytics/Learning

### Mục Đích

**Module 3 là hệ thống phân tích và học hỏi:**
- Aggregation metrics từ Module 1 và Module 2
- A/B testing (prompts, models, creation methods)
- Performance analysis (human vs AI)
- Learning insights và recommendations
- **KHÔNG** tạo content
- **KHÔNG** gọi AI
- Chỉ đọc data và tính toán

### Chức Năng Cụ Thể

**1. Metrics Aggregation:**
- Aggregate metrics từ publications (views, likes, shares, comments)
- Roll-up scores từ lower levels lên higher levels
- Calculate performance metrics (engagement rate, conversion rate, etc.)

**2. A/B Testing:**
- Compare prompt versions
- Compare AI models
- Compare creation methods (human vs AI vs hybrid)
- Compare content variants
- Statistical significance testing

**3. Performance Analysis:**
- Human vs AI content performance
- Prompt version performance
- Model performance (cost, latency, quality)
- Creation method performance

**4. Learning & Recommendations:**
- Generate insights từ metrics
- Recommend best prompts/models
- Recommend creation strategies
- Predict content performance

### Data Models

**Collections:**

| Collection | Nhiệm Vụ | Mô Tả Chi Tiết |
|------------|----------|----------------|
| `content_performance` | Performance metrics cho content | Aggregated metrics từ publications, roll-up scores |
| `ai_performance` | Performance metrics cho AI | Cost, latency, quality scores từ AI runs |
| `content_experiments` | A/B testing experiments | Experiment definitions, variants, results |
| `content_variants` | Content variants trong experiments | Variants của content để test |
| `experiment_results` | Kết quả A/B testing | Statistical analysis, winners, significance |
| `learning_insights` | Insights từ data | Insights và patterns được phát hiện |
| `recommendations` | Recommendations | Recommendations cho prompts, models, strategies |
| `rollup_scores` | Roll-up scores | Scores được roll-up từ lower levels |

### Data Flow

**Module 3 đọc từ Module 1:**
- Content nodes (để biết creator type, creation method)
- Publications và MetricsRaw (views, likes, shares, comments)
- Draft nodes (để track draft performance)

**Module 3 đọc từ Module 2:**
- AI runs (prompt, model, cost, latency, quality score)
- Workflow runs (để biết workflow nào tạo content)
- Candidates (để so sánh candidates với final content)
- Prompt templates (để biết prompt version)

**Module 3 tính toán:**
- Performance metrics: So sánh human vs AI content
- A/B testing: So sánh prompt versions, creation methods
- Cost analysis: AI cost vs human time cost
- Quality analysis: AI judge score vs human rating vs actual performance
- Learning insights: Từ metrics → insights → recommendations

### API Design

**Nguyên tắc:** Ưu tiên CRUD mặc định từ BaseHandler. Module 3 chủ yếu đọc data từ Module 1 và Module 2, tính toán và lưu kết quả vào collections của mình. Custom endpoints chỉ cần khi có business logic tính toán phức tạp không thể thực hiện bằng CRUD.

---

## 🔄 Service Communication Flow

### Kiến Trúc Tổng Quan

```
┌─────────────────────────────────────────────────────────────┐
│              Module 1: Content Storage                      │
│         (Pure Storage - Lưu trữ nội dung)                   │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐                        │
│  │   REST API   │  │   Database   │                        │
│  │   Server     │  │   (MongoDB)   │                        │
│  └──────────────┘  └──────────────┘                        │
│         │                                                  │
│         │ HTTP API (REST)                                  │
└─────────┼──────────────────────────────────────────────────┘
          │
          │ HTTP Requests (Create/Read Content)
          │
┌─────────┼──────────────────────────────────────────────────┐
│         │         Module 2: AI Service                       │
│         │    (AI Orchestration - Gọi AI APIs)              │
│         │                                                    │
│  ┌──────▼──────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Content   │  │   Workflow   │  │   AI Client  │      │
│  │  API Client │  │   Executor   │  │   (OpenAI,   │      │
│  │  (Module 1) │  │              │  │   Claude)    │      │
│  └─────────────┘  └──────────────┘  └──────────────┘      │
│                                                              │
│  ┌──────────────┐                                           │
│  │   Database   │  (Riêng của Module 2)                     │
│  │  (MongoDB)   │  - prompts, templates                     │
│  └──────────────┘  - workflow_runs, step_runs              │
│                    - ai_runs, candidates                    │
└─────────────────────────────────────────────────────────────┘
          │                              │
          │ Read Data                     │ Read Data
          │                               │
┌─────────┼───────────────────────────────┼──────────────────┐
│         │         Module 3: Analytics   │                  │
│         │    (Ghép data & Tính toán)    │                  │
│         │                                │                  │
│  ┌──────▼──────┐  ┌──────────────┐  ┌───▼──────┐         │
│  │   Content   │  │   AI Data    │  │ Analytics │         │
│  │  API Client │  │  API Client  │  │  Engine   │         │
│  │  (Module 1) │  │  (Module 2)  │  │           │         │
│  └─────────────┘  └──────────────┘  └───────────┘         │
│                                                              │
│  ┌──────────────┐                                           │
│  │   Database   │  (Riêng của Module 3)                     │
│  │  (MongoDB)   │  - performance_metrics                      │
│  └──────────────┘  - experiments, variants                 │
│                    - rollup_scores, insights                │
└─────────────────────────────────────────────────────────────┘
```

### Communication Pattern

**Module 2 → Module 1 (API Calls):**

1. **Read Operations:**
   ```go
   // Module 2 cần đọc parent node để generate tiếp (có thể là draft hoặc production)
   GET /api/v1/drafts/nodes/:id  // Nếu parent là draft
   GET /api/v1/content/nodes/:id // Nếu parent là production
   ```

2. **Write Operations:**
   ```go
   // Module 2 tạo draft node sau khi generate (luôn tạo draft, không tạo production trực tiếp)
   POST /api/v1/drafts/nodes
   Body: { type, text, parentDraftId, workflowRunId, ... }
   ```

3. **Update Operations:**
   ```go
   // Module 2 update draft node (nếu cần)
   PUT /api/v1/drafts/nodes/:id
   ```

**Lưu ý:**
- Module 2 **luôn tạo draft nodes** trong Module 1 (không tạo production trực tiếp)
- Module 2 không commit draft → production (Module 1 làm việc này sau khi approve)
- Module 2 chỉ mark candidate as "selected" khi commit sang Module 1

**Module 3 → Module 1 (Read Only):**
```go
// Module 3 đọc content để tính toán metrics
GET /api/v1/content/nodes
GET /api/v1/content/nodes/:id
GET /api/v1/publications
GET /api/v1/publications/:id
```

**Module 3 → Module 2 (Read Only):**
```go
// Module 3 đọc AI runs, prompts để tính toán
GET /api/v2/ai-runs
GET /api/v2/workflow-runs/:id
GET /api/v2/prompt-templates/:id
GET /api/v2/candidates
```

**Module 1 không gọi Module 2 hoặc Module 3:**
- Module 1 hoạt động độc lập
- Module 1 không biết về Module 2 và Module 3
- Module 1 chỉ expose APIs, không phụ thuộc vào modules khác

**Module 2 không gọi Module 3:**
- Module 2 chỉ gọi AI APIs và Module 1 API
- Module 2 không tính toán metrics
- Module 3 đọc data từ Module 2 (read-only)

---

## 🔄 Data Flow: Module 3 Ghép Data Từ Module 1 và 2

**Module 3 đọc từ Module 1:**
- Content nodes (để biết creator type, creation method)
- Publications và metrics (views, likes, shares, comments)
- Draft nodes (để track draft performance)

**Module 3 đọc từ Module 2:**
- AI runs (prompt, model, cost, latency, quality score)
- Workflow runs (để biết workflow nào tạo content)
- Candidates (để so sánh candidates với final content)
- Prompt templates (để biết prompt version)

**Module 3 tính toán:**
- Performance metrics: So sánh human vs AI content
- A/B testing: So sánh prompt versions, creation methods
- Cost analysis: AI cost vs human time cost
- Quality analysis: AI judge score vs human rating vs actual performance
- Learning insights: Từ metrics → insights → recommendations

---

## 🤖 Bot Integration (folkgroup-agent)

### Kiến Trúc

```
┌─────────────────────────────────────────────────────────────┐
│              Module 2: AI Service (Backend)                 │
│                                                              │
│  Collections:                                                │
│  - workflow_commands (queue yêu cầu AI)                     │
│  - workflows, steps, prompt_templates                        │
│  - workflow_runs, step_runs, candidates, ai_runs            │
│                                                              │
│  API Endpoints:                                              │
│  - POST /api/v2/workflow-commands (tạo yêu cầu)            │
│  - GET /api/v2/workflow-commands (agent query commands)    │
│  - POST /api/v2/workflow-runs (agent start workflow)       │
└─────────────────────────────────────────────────────────────┘
                                ↓
                        ┌──────────────────┐
                        │  Agent           │
                        │  (folkgroup-agent)│
                        │                  │
                        │  - Check-in job  │
                        │  - Sync jobs     │
                        │  - Workflow job  │ ← Job mới
                        └──────────────────┘
                                ↓
                        Workflow Job:
                        - Query commands
                        - Tạo workers
                        - Xử lý từng yêu cầu
```

### Workflow Command Processing

**1. Tạo Command:**
```go
// External system hoặc Module 2 API
POST /api/v2/workflow-commands
Body: {
    commandType: "START_WORKFLOW",
    workflowId: "...",
    rootRefId: "...",
    rootRefType: "...",
    params: {...}
}
```

**2. Bot Query Commands:**
```go
// Bot (folkgroup-agent) query commands
GET /api/v2/workflow-commands?status=pending&agentId=...
```

**3. Bot Process Command:**
```go
// Bot tạo worker để xử lý
worker := NewWorkflowWorker(command)
go worker.Process()

// Worker gọi Module 2 API để start workflow
POST /api/v2/workflow-runs
Body: {
    workflowId: command.WorkflowID,
    rootRefId: command.RootRefID,
    rootRefType: command.RootRefType,
    params: command.Params
}
```

**4. Module 2 Execute Workflow:**
- Execute workflow steps
- Generate candidates
- Judge candidates
- Create draft nodes trong Module 1

**5. Bot Update Command Status:**
```go
PUT /api/v2/workflow-commands/:id
Body: {
    status: "completed",
    result: {...}
}
```

---

## 🔄 Workflow Execution Logic Chi Tiết

### Workflow Execution Flow (Module 2)

**1. Khởi tạo Workflow Run:**
```
Bot tạo workflow run với:
- workflowId: ID của workflow definition
- rootRefId: ID của content node bắt đầu (ví dụ: Layer L1)
- rootRefType: Type của root content (ví dụ: "layer")
- params: Tham số bổ sung (organizationId, userId, etc.)
```

**2. Module 2 Execute Workflow:**
```
a. Load workflow definition từ workflows collection
b. Lấy root content từ Module 1 (query content node theo rootRefId)
c. Execute từng step trong workflow:
   
   Step 1: GENERATE (Layer → STP)
   - Load prompt template cho GENERATE step
   - Gọi AI với prompt + context (root Layer)
   - Parse response → Tạo candidates
   - Lưu vào generation_batch → candidates collection
   
   Step 2: JUDGE (Score STP candidates)
   - Load prompt template cho JUDGE step
   - Gọi AI với prompt + candidates
   - Parse response → Quality scores
   - Select candidate tốt nhất (highest score)
   
   Step 3: COMMIT (Create draft node)
   - Tạo draft node trong Module 1
   - Type: "stp"
   - Text: selectedCandidate.Text
   - Link về workflowRunId, createdByRunId, createdByCandidateID
   
   Step 4: GENERATE (STP → Insight)
   - Load parent draft node từ Module 1
   - Gọi AI với prompt + context (STP draft)
   - Generate candidates → Judge → Select → Create draft
   
   ... tiếp tục cho các levels tiếp theo
```

**3. Workflow Run Completed:**
```
- Tất cả draft nodes đã được tạo trong Module 1
- Workflow run status = "completed"
- Bot update command status = "completed"
```

**4. Human Review & Approval:**
```
Human query drafts theo workflowRunId → Review drafts
Human request approval → Module 1 tạo approval request
Human approve/reject → Module 1 commit tất cả drafts → production (nếu approve)
```

### Dynamic Step Generation Logic

**Step Type: STEP_GENERATION**

Khi workflow đến step có type `STEP_GENERATION`, AI sẽ tự động tạo các steps tiếp theo:

```
1. AI nhận context:
   - Current level (ví dụ: L3 - Insight)
   - Parent content
   - Workflow goals
   - Available prompt templates

2. AI generate next steps:
   {
     "nextSteps": [
       {
         "stepType": "GENERATE",
         "targetLevel": "L4",
         "promptTemplateId": "template-123",
         "inputSchema": {...},
         "outputSchema": {...}
       },
       {
         "stepType": "JUDGE",
         "targetLevel": "L4",
         "promptTemplateId": "template-456",
         "inputSchema": {...},
         "outputSchema": {...}
       }
     ]
   }

3. Module 2 tạo step definitions và execute
```

### Two-Step Level Transition Logic

**Ví dụ: Layer (L1) → STP (L2)**

**Step 1: GENERATE_STP**
```go
// Prompt template: "generate_stp_from_layer"
Input: {
    "layer": {
        "id": "layer-123",
        "text": "Giải trí"
    },
    "context": {...}
}

Output: {
    "candidates": [
        {"text": "Gen Z, 18-25, thích TikTok", "metadata": {...}},
        {"text": "Millennials, 26-35, thích YouTube", "metadata": {...}},
        {"text": "Gen X, 36-50, thích Facebook", "metadata": {...}}
    ]
}

// Lưu vào generation_batch và candidates
```

**Step 2: JUDGE_STP**
```go
// Prompt template: "judge_stp_candidates"
Input: {
    "candidates": [...],
    "layer": {...},
    "criteria": {
        "targetAudience": "Gen Z",
        "platform": "TikTok"
    }
}

Output: {
    "scores": [
        {"candidateId": "candidate-1", "score": 0.95, "reasoning": "..."},
        {"candidateId": "candidate-2", "score": 0.72, "reasoning": "..."},
        {"candidateId": "candidate-3", "score": 0.58, "reasoning": "..."}
    ]
}

// Select candidate-1 (highest score)
// Create draft node trong Module 1
```

---

## 📊 A/B Testing Logic (Module 3)

### Experiment Setup

**1. Tạo Experiment:**
```go
POST /api/v3/experiments
Body: {
    name: "Test Prompt Version for STP Generation",
    type: "PROMPT_VERSION",
    variants: [
        {
            variantId: "variant-1",
            promptTemplateId: "template-v1",
            description: "Prompt version 1.0"
        },
        {
            variantId: "variant-2",
            promptTemplateId: "template-v2",
            description: "Prompt version 2.0"
        }
    ],
    targetLevel: "L2",  // STP
    metrics: ["engagement_rate", "conversion_rate"]
}
```

**2. Module 2 Execute với Variants:**
```
- Module 2 tạo workflow runs với các prompt variants
- Mỗi variant tạo content riêng
- Content được publish và track metrics
```

**3. Module 3 Analyze:**
```
- Aggregate metrics từ publications
- Compare variants
- Calculate statistical significance
- Determine winner
```

### Performance Comparison Logic

**Human vs AI Content:**

```
1. Module 3 query Module 1:
   - Content nodes với creatorType = "human"
   - Content nodes với creatorType = "ai"
   - Publications của cả 2 loại

2. Aggregate metrics:
   - Human: Avg views, likes, engagement rate
   - AI: Avg views, likes, engagement rate

3. Compare:
   - Performance difference
   - Cost analysis (human time vs AI cost)
   - Quality analysis (AI judge score vs actual performance)

4. Generate insights:
   - "AI content performs 20% better on TikTok"
   - "Human content performs 15% better on Facebook"
   - "Hybrid approach (AI generate + Human edit) performs best"
```

---

## 🎯 Use Cases Chi Tiết

### Use Case 1: Tạo Content Từ Đầu (Full Workflow)

**Scenario:** Tạo content từ Layer (L1) đến Publication (L8)

**Flow:**
```
1. User tạo Layer thủ công → Module 1 lưu vào content_nodes

2. User trigger workflow → Module 2 tạo workflow command

3. Bot process command:
   - Query workflow commands queue
   - Start workflow run
   - Execute steps:
     * GENERATE STP → JUDGE STP → Create draft STP
     * GENERATE Insight → JUDGE Insight → Create draft Insight
     * ... tiếp tục đến Script
   - Tạo draft nodes trong Module 1

4. Human review:
   - Query drafts theo workflowRunId
   - Review tất cả drafts
   - Approve hoặc reject

5. Approve & Commit:
   - Human approve → Module 1 commit tất cả drafts → production

6. External system tạo video:
   - Render video từ script
   - Tạo video record trong Module 1

7. External system publish:
   - Tạo publication record trong Module 1
   - Platform: "tiktok"
   - Status: "published"

8. Platform update metrics:
   - Update MetricsRaw trong publication

9. Module 3 analyze:
   - Aggregate metrics từ publications và AI runs
   - Compare với experiments
   - Generate insights và recommendations
```

### Use Case 2: Human Tạo Content Thủ Công

**Scenario:** Human tạo content không qua AI

**Flow:**
```
1. Human tạo Layer → Module 1 lưu vào content_nodes
   - CreatorType: "human"
   - CreationMethod: "manual"

2. Human tạo STP → Module 1 lưu vào content_nodes
   - ParentId: link về Layer
   - CreatorType: "human"
   - CreationMethod: "manual"

3. ... tiếp tục tạo các levels

4. Module 3 track:
   - Track creatorType = "human"
   - Track creationMethod = "manual"
   - So sánh performance với AI content
```

### Use Case 3: A/B Testing Prompt Versions

**Scenario:** Test 2 prompt versions để generate STP

**Flow:**
```
1. Module 3 tạo experiment → Lưu vào experiments collection
   - Type: "PROMPT_VERSION"
   - Variants: template-v1, template-v2

2. Module 2 execute với variants:
   - Workflow run 1: Dùng template-v1 → Tạo content variant-1
   - Workflow run 2: Dùng template-v2 → Tạo content variant-2

3. Both variants được publish:
   - Variant 1: Publication A
   - Variant 2: Publication B

4. Module 3 collect metrics:
   - Query publications từ Module 1
   - Publication A: views=1000, likes=50
   - Publication B: views=1200, likes=70

5. Module 3 analyze:
   - Tính toán statistical significance
   - Variant 2 (template-v2) performs 20% better
   - Statistical significance: 95%
   - Winner: template-v2

6. Module 3 generate recommendation:
   - Lưu recommendation vào recommendations collection
   - "Recommend using template-v2 for STP generation"
```

### Use Case 4: Human-in-the-Loop (Prompt Individual Step)

**Scenario:** Human muốn prompt một step cụ thể với custom prompt

**Flow:**
```
1. Human xem workflow run → Query workflow run từ Module 2

2. Human prompt step cụ thể:
   - Trigger step execution với custom prompt
   - Params: custom parameters

3. Module 2 execute step:
   - Gọi AI với custom prompt
   - Generate candidates
   - Judge và select
   - Create draft trong Module 1

4. Human review và approve → Module 1 commit draft → production
```

---

## 🔍 Traceability Chain

### Full Traceability từ Content đến AI Run

```
Publication (L8)
  ↓ createdByRunID
Workflow Run (Module 2)
  ↓ stepRuns
Step Run (GENERATE STP)
  ↓ generationBatch
Generation Batch
  ↓ candidates
Candidate (selected = true)
  ↓ createdByAIRunID
AI Run (GENERATE)
  ↓ promptTemplateId
Prompt Template (version, variables)

Candidate (selected = true)
  ↓ judgedByAIRunID
AI Run (JUDGE)
  ↓ promptTemplateId
Prompt Template (JUDGE version)
```

**Query Examples:**
```go
// Tìm tất cả AI runs tạo ra một publication
GET /api/v2/ai-runs?createdPublicationId=publication-id

// Tìm tất cả candidates của một workflow run
GET /api/v2/candidates?workflowRunId=workflow-run-id

// Tìm prompt template version được dùng
GET /api/v2/prompt-templates/:id
```

---

## 📝 Key Design Decisions

### 1. API Design: CRUD-First Approach

- **Ưu tiên CRUD mặc định từ BaseHandler:** Tất cả các collections đều sử dụng CRUD endpoints có sẵn (InsertOne, Find, FindOneById, UpdateById, DeleteById, etc.)
- **Hạn chế custom endpoints:** Chỉ tạo custom endpoint khi có business logic phức tạp không thể thực hiện bằng CRUD + filter query
- **Sử dụng filter query string:** Thay vì tạo endpoint `/api/v1/content/nodes/by-type/:type`, dùng `GET /api/v1/content/nodes?filter[type]=layer`
- **Sử dụng update-by-id:** Thay vì tạo endpoint `/api/v1/content/publications/:id/metrics`, dùng `PUT /api/v1/content/publications/update-by-id/:id` với body `{"metricsRaw": {...}}`
- **Custom endpoints chỉ cho business logic đặc thù:** 
  - GetTree (recursive logic)
  - CommitDraftNode (workflow logic)
  - Approval workflows (RequestApproval, Approve, Reject)
  - Workflow execution (orchestration logic)

**Ví dụ Custom Endpoints hợp lệ:**
- GetTree - Recursive tree traversal (không thể dùng CRUD)
- CommitDraftNode - Business logic: commit draft → production
- Approval workflows - Workflow logic: approve và commit

**Ví dụ KHÔNG cần custom endpoint (dùng CRUD + filter):**
- Query nodes theo type → Dùng Find với filter `type`
- Query nodes theo parentId → Dùng Find với filter `parentId`
- Update metricsRaw → Dùng UpdateById với body chứa `metricsRaw`

### 2. Service Independence

- **Module 1 hoạt động độc lập:** Có thể tạo content thủ công mà không cần Module 2
- **Module 2 là external client:** Gọi Module 1 API như một client bên ngoài
- **Module 3 chỉ đọc:** Không modify data trong Module 1 và Module 2

### 3. Draft System

- **Draft chỉ trong Module 1:** Module 2 không có draft collections
- **Module 2 luôn tạo draft:** Không tạo production trực tiếp
- **Approval workflow:** Human review → Approve → Commit → Production

### 4. Two-Step Level Transition

- **GENERATE step:** AI generate candidates
- **JUDGE step:** AI judge và select best candidate
- **A/B testable:** Có thể test prompt versions riêng cho GENERATE và JUDGE

### 5. Fixed JSON Schemas

- **Input schema:** Strict format cho mỗi step type
- **Output schema:** Strict format cho mỗi step type
- **Schema registry:** Quản lý schemas centrally

### 6. Traceability

- **Content → Candidate → AI Run:** Full traceability chain
- **Reference IDs:** `CreatedByRunID`, `CreatedByStepRunID`, `CreatedByCandidateID`
- **Module 2 chỉ lưu reference IDs:** Không lưu full workflow definitions trong Module 1

### 7. MetricsRaw Storage

- **Lưu trong Module 1:** Publications collection
- **Module 3 đọc:** Tính toán performance từ MetricsRaw
- **Update qua API:** External systems update metrics qua CRUD endpoint `UpdateById`

---

## 🔐 Permissions

### Module 1 Permissions

**Content Nodes:**
- `ContentNode.Insert`, `ContentNode.Read`, `ContentNode.Update`, `ContentNode.Delete`
- `ContentNode.Tree` (GetTree endpoint)
- `ContentNode.SoftDelete` (SoftDelete endpoint)

**Videos:**
- `Video.Insert`, `Video.Read`, `Video.Update`, `Video.Delete`

**Publications:**
- `Publication.Insert`, `Publication.Read`, `Publication.Update`, `Publication.Delete`

**Draft Content Nodes:**
- `DraftContentNode.Insert`, `DraftContentNode.Read`, `DraftContentNode.Update`, `DraftContentNode.Delete`
- `DraftContentNode.Commit` (CommitDraftNode endpoint)

**Draft Videos:**
- `DraftVideo.Insert`, `DraftVideo.Read`, `DraftVideo.Update`, `DraftVideo.Delete`

**Draft Publications:**
- `DraftPublication.Insert`, `DraftPublication.Read`, `DraftPublication.Update`, `DraftPublication.Delete`

**Approval Requests:**
- `ApprovalRequest.Read`
- `ApprovalRequest.Request` (RequestApprovalForWorkflowRun)
- `ApprovalRequest.Approve` (ApproveDraftWorkflowRun)
- `ApprovalRequest.Reject` (RejectDraftWorkflowRun)

---

## 📚 Tài Liệu Liên Quan

- [API Documentation](../03-api/)
- [Testing Guide](../06-testing/)
- [Deployment Guide](../04-deployment/)

---

## 🧮 Logic Tính Toán Chi Tiết

### Module 3: Metrics Aggregation Logic

**1. Roll-up Scores từ Lower Levels:**

```
L8 (Publication) Metrics:
  - views, likes, shares, comments
  ↓ roll-up
L7 (Video) Score:
  - Sum of all publication metrics
  - Avg engagement rate
  ↓ roll-up
L6 (Script) Score:
  - Sum of all video scores
  - Avg performance
  ↓ roll-up
... tiếp tục đến L1 (Layer)
```

**2. Performance Metrics Calculation:**

```go
// Engagement Rate
engagementRate = (likes + shares + comments) / views

// Conversion Rate (nếu có conversion tracking)
conversionRate = conversions / views

// Cost per Engagement
costPerEngagement = aiCost / totalEngagements

// Quality Score (từ AI judge)
qualityScore = avgJudgeScore

// Performance Score (tổng hợp)
performanceScore = (
    engagementRate * 0.4 +
    conversionRate * 0.3 +
    qualityScore * 0.2 +
    (1 / costPerEngagement) * 0.1
)
```

**3. A/B Testing Statistical Analysis:**

```go
// T-test hoặc Chi-square test
significance = calculateStatisticalSignificance(
    variantAMetrics,
    variantBMetrics
)

// Winner determination
if significance > 0.95 && variantA.performance > variantB.performance {
    winner = variantA
} else if significance > 0.95 && variantB.performance > variantA.performance {
    winner = variantB
} else {
    winner = null  // Không có winner rõ ràng
}
```

### Module 3: Learning & Recommendations Logic

**1. Pattern Detection:**

```go
// Phát hiện patterns từ metrics
patterns = detectPatterns(contentNodes, publications, aiRuns)

// Ví dụ patterns:
// - "Prompt version 2.0 performs better for Gen Z content"
// - "Claude model has lower cost but similar quality to GPT-4"
// - "Human-edited AI content performs 15% better than pure AI"
```

**2. Recommendation Generation:**

```go
// Dựa trên patterns và experiments
recommendations = generateRecommendations(patterns, experiments)

// Ví dụ recommendations:
// - "Use prompt template v2.0 for STP generation"
// - "Use Claude for cost-sensitive workflows"
// - "Use hybrid approach (AI generate + Human edit) for Facebook"
```

**3. Performance Prediction:**

```go
// Predict content performance dựa trên historical data
predictedPerformance = predictPerformance(
    contentType,
    creatorType,
    creationMethod,
    promptVersion,
    model,
    platform
)

// Sử dụng machine learning model (linear regression, random forest, etc.)
```

---

## 🔄 Complete Workflow Example

### Scenario: Tạo Content Từ Layer Đến Publication

**Step-by-Step Flow:**

```
1. [Human] Tạo Layer (L1):
   Human tạo content node → Module 1 lưu vào content_nodes
   - Type: "layer"
   - CreatorType: "human"
   - CreationMethod: "manual"

2. [User] Trigger Workflow:
   User tạo workflow command → Module 2 lưu vào workflow_commands
   - CommandType: "START_WORKFLOW"
   - WorkflowId: "full-content-workflow"
   - RootRefId: ID của Layer vừa tạo
   - RootRefType: "layer"

3. [Bot] Query Commands:
   Bot query workflow_commands queue (filter: status=pending)
   → Bot nhận command → Bot tạo worker

4. [Bot Worker] Start Workflow Run:
   Bot tạo workflow run → Module 2 lưu vào workflow_runs
   - WorkflowId: từ command
   - RootRefId: từ command
   - Status: "running"

5. [Module 2] Execute Step 1: GENERATE STP
   a. Load workflow definition từ workflows collection
   b. Load prompt template cho GENERATE step
   c. Read Layer từ Module 1 (query content node theo rootRefId)
   d. Gọi AI (OpenAI GPT-4) với prompt + context
   e. Parse response → Tạo candidates → Lưu vào generation_batch và candidates
   f. Tạo AI run record (cost, latency, model)

6. [Module 2] Execute Step 2: JUDGE STP
   a. Load prompt template cho JUDGE step
   b. Gọi AI để judge candidates
   c. Parse response → Quality scores
   d. Select candidate tốt nhất (highest score)
   e. Tạo AI run record (JUDGE)

7. [Module 2] Create Draft STP Node:
   Module 2 tạo draft node → Module 1 lưu vào draft_content_nodes
   - Type: "stp"
   - Text: selectedCandidate.Text
   - WorkflowRunId: link về workflow run
   - CreatedByCandidateID: link về candidate

8. [Module 2] Execute Step 3: GENERATE Insight
   a. Read parent draft STP từ Module 1
   b. Gọi AI với prompt + context (STP draft)
   c. Generate candidates → Judge → Select
   d. Create draft Insight node

9. ... Tiếp tục cho các levels: Content Line, Gene, Script

10. [Module 2] Workflow Run Completed:
    → Update workflow_run status = "completed"
    → Bot update command status = "completed"

11. [Human] Review Drafts:
    Human query drafts theo workflowRunId → Review tất cả drafts

12. [Human] Request Approval:
    Human request approval → Module 1 tạo approval_request

13. [Human] Approve:
    Human approve → Module 1 commit tất cả drafts → production
    → Tạo content_nodes, videos, publications (production)

14. [External System] Render Video:
    External system render video từ script
    → Update video status = "ready" trong Module 1

15. [External System] Publish:
    External system tạo publication → Module 1 lưu vào publications
    - VideoId: link về video
    - Platform: "tiktok"
    - Status: "published"

16. [Platform] Update Metrics:
    Platform update MetricsRaw trong publication
    → Module 1 cập nhật metricsRaw

17. [Module 3] Aggregate Metrics:
    a. Đọc publications với metrics từ Module 1
    b. Đọc AI runs (cost, latency, quality) từ Module 2
    c. Tính toán performance metrics
    d. So sánh với experiments
    e. Generate insights và recommendations → Lưu vào collections của Module 3
```

---

## 📊 Data Relationships & References

### Reference Chain

```
Module 1 (Content):
  ContentNode
    ├─ CreatedByRunID → Module 2: WorkflowRun
    ├─ CreatedByStepRunID → Module 2: StepRun
    ├─ CreatedByCandidateID → Module 2: Candidate
    └─ CreatedByBatchID → Module 2: GenerationBatch

Module 2 (AI):
  WorkflowRun
    ├─ WorkflowID → Workflow definition
    └─ StepRuns → Step executions
  
  StepRun
    ├─ StepID → Step definition
    ├─ PromptTemplateID → Prompt template
    └─ GenerationBatchID → Generation batch
  
  Candidate
    ├─ GenerationBatchID → Batch
    ├─ CreatedByAIRunID → AI Run (GENERATE)
    └─ JudgedByAIRunID → AI Run (JUDGE)
  
  AIRun
    ├─ PromptTemplateID → Prompt template
    ├─ ProviderProfileID → AI provider config
    └─ ExperimentID → Experiment (nếu có)

Module 3 (Analytics):
  ContentPerformance
    ├─ ContentNodeID → Module 1: ContentNode
    └─ PublicationIDs → Module 1: Publications
  
  Experiment
    ├─ VariantIDs → Content variants
    └─ PromptTemplateIDs → Module 2: Prompt templates
```

---

## 🔄 Version History

- **v1.0** (2025-01-XX): Initial design document
  - Module 1: Content Storage design
  - Module 2: AI Service design
  - Module 3: Analytics/Learning design
  - Bot integration design
  - Complete workflow examples
  - Logic chi tiết cho metrics aggregation và learning
