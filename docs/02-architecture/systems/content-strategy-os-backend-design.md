# Content Strategy Operating System - Backend Design

## 📋 Tổng Quan

**Content Strategy Operating System** là hệ thống quản lý và tạo nội dung tự động với 8 cấp độ nội dung (L1-L8), sử dụng AI để generate và judge content, hỗ trợ A/B testing và learning từ metrics.

**Ngày tạo:** 2025-01-XX  
**Phiên bản:** v1.0  
**Kiến trúc:** 3 Modules độc lập

---

## 🎯 Tóm Tắt Nhanh

**Content Strategy Operating System** là hệ thống quản lý và tạo nội dung tự động với:
- **8 cấp độ nội dung** (L1-L8): Từ Layer đến Publication
- **AI tự động generate và judge** content
- **A/B testing** prompts và models
- **Learning từ metrics** thực tế
- **Kiến trúc 3 Modules độc lập**

### 8 Cấp Độ Nội Dung (Content Levels)

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

### Kiến Trúc 3 Modules

1. **Module 1: Content Storage** - Lưu trữ nội dung (7 collections)
2. **Module 2: AI Service** - Điều phối AI (10 collections)
3. **Module 3: Analytics/Learning** - Phân tích và học hỏi (8 collections)

**👉 Xem chi tiết bên dưới để biết đầy đủ thông tin về từng module, collections, và workflow.**

---

## 📊 Tổng Hợp Collections - Content Strategy OS

### Tổng Số: **25 Collections**

Hệ thống Content Strategy OS có tổng cộng **25 collections** được chia thành 3 modules:

### 📦 Module 1: Content Storage (7 collections)

| Collection | Tác Dụng | Mô Tả |
|------------|----------|-------|
| `content_nodes` | Lưu trữ production content nodes | Content nodes đã được duyệt và commit (L1-L6: Layer, STP, Insight, Content Line, Gene, Script) - Có creator type, creation method |
| `videos` | Lưu trữ production videos | Videos đã được duyệt và commit (L7) - Link với script, có asset URL, metadata |
| `publications` | Lưu trữ production publications | Publications đã được duyệt và commit (L8) - Link với video, platform, **có MetricsRaw (views, likes, shares, comments)** |
| `draft_content_nodes` | Lưu trữ draft nodes | Bản nháp content nodes (L1-L6) - Chưa được duyệt, có approval status, link về workflow run, candidate |
| `draft_videos` | Lưu trữ draft videos | Bản nháp videos (L7) - Chưa được duyệt, link về draft script |
| `draft_publications` | Lưu trữ draft publications | Bản nháp publications (L8) - Chưa được duyệt, link về draft video |
| `draft_approvals` | Quản lý approvals | Approval requests và decisions - Track approval workflow, có status (pending, approved, rejected) |

### 🤖 Module 2: AI Service (10 collections)

| Collection | Tác Dụng | Mô Tả |
|------------|----------|-------|
| `ai_workflows` | Định nghĩa workflows | Workflow definitions với steps, policies, rootRefType, targetLevel |
| `ai_steps` | Định nghĩa steps | Step definitions với input/output schemas, prompt template IDs, targetLevel - **KHÔNG có provider config** (config lưu trong prompt template) |
| `ai_prompt_templates` | Quản lý prompts | Prompt templates với versioning, variables, types (generate, judge, step_generation), **providerProfileId, model, temperature, maxTokens (override từ provider profile)** |
| `ai_provider_profiles` | Quản lý AI providers | Provider profiles với API keys, config, models, pricing, rate limits |
| `ai_workflow_runs` | Lịch sử workflow runs | **1 workflow run = 1 lần chạy workflow** - Status, rootRefId, stepRunIDs[], result - Quản lý toàn bộ workflow execution |
| `ai_step_runs` | Lịch sử step runs | **1 step run = 1 lần chạy 1 step trong workflow** - Link về workflowRunId, stepId - Input/Output (structured data flow giữa các steps) - Quản lý data flow và execution của từng step |
| `ai_generation_batches` | Batches của candidates | Batches chứa nhiều candidates được generate cùng lúc - TargetCount, ActualCount, CandidateIDs |
| `ai_candidates` | Content candidates | Candidates được generate, có judge scores, selected flag - Link về AI runs, generation batch |
| `ai_runs` | Lịch sử AI calls | **1 AI run = 1 lần gọi AI API** - Link về stepRunId, workflowRunId (optional) - Prompt, Response (TEXT), cost, latency, quality, **conversation history** - Chi tiết từng lần gọi AI API |
| `ai_workflow_commands` | Command queue | Queue commands cho bot xử lý (START_WORKFLOW, etc.) - Status, workflowId, params |

### 📊 Module 3: Analytics/Learning (8 collections - dự kiến)

| Collection | Tác Dụng | Mô Tả |
|------------|----------|-------|
| `content_performance` | Performance metrics cho content | Aggregated metrics từ publications, roll-up scores từ lower levels lên higher levels |
| `ai_performance` | Performance metrics cho AI | Cost, latency, quality scores từ AI runs - Phân tích theo model, provider, prompt version |
| `content_experiments` | A/B testing experiments | Experiment definitions, variants, results - So sánh prompts, models, creation methods |
| `content_variants` | Content variants trong experiments | Variants của content để test - Link về experiments |
| `experiment_results` | Kết quả A/B testing | Statistical analysis, winners, significance - So sánh performance giữa variants |
| `learning_insights` | Insights từ data | Insights và patterns được phát hiện từ metrics và experiments |
| `recommendations` | Recommendations | Recommendations cho prompts, models, strategies - Dựa trên performance data |
| `rollup_scores` | Roll-up scores | Scores được roll-up từ lower levels (L8 → L7 → L6 → ... → L1) |

### Tóm Tắt Theo Module

**Module 1 (Content Storage):** 7 collections
- **Production (3):** `content_nodes`, `videos`, `publications`
- **Drafts (3):** `draft_content_nodes`, `draft_videos`, `draft_publications`
- **Approval (1):** `draft_approvals`

**Module 2 (AI Service):** 10 collections
- **Configuration (4):** `ai_workflows`, `ai_steps`, `ai_prompt_templates`, `ai_provider_profiles`
- **Execution (5):** `ai_workflow_runs`, `ai_step_runs`, `ai_generation_batches`, `ai_candidates`, `ai_runs`
- **Queue (1):** `ai_workflow_commands`

**Module 3 (Analytics/Learning):** 8 collections (dự kiến)
- **Performance (2):** `content_performance`, `ai_performance`
- **A/B Testing (3):** `content_experiments`, `content_variants`, `experiment_results`
- **Learning (3):** `learning_insights`, `recommendations`, `rollup_scores`

### Mối Quan Hệ Giữa Các Collections

```
ai_workflows
  ↓ (workflowId)
ai_workflow_runs                    ← 1 workflow run
  ↓ (workflowRunId)                 ↓
ai_step_runs                        ← N step runs (1 cho mỗi step)
  ├─ (stepId) → ai_steps           ↓
  ├─ (generationBatchId) → ai_generation_batches
  └─ (stepRunId) → ai_runs         ← M AI runs (1 step có thể gọi AI nhiều lần)
      ├─ (promptTemplateId) → ai_prompt_templates
      ├─ (providerProfileId) → ai_provider_profiles
      └─ (type: GENERATE) → ai_candidates
          ↓ (createdByCandidateId)
      draft_content_nodes
          ↓ (approve & commit)
      content_nodes
          ↓ (scriptId)
      videos
          ↓ (videoId)
      publications
          ↓ (metricsRaw)
      content_performance (Module 3)
```

### ⚠️ Phân Biệt: ai_workflow_runs vs ai_step_runs vs ai_runs

**Ba collections này KHÔNG trùng, nhưng có mối quan hệ phân cấp:**

| Collection | Mục Đích | Ví Dụ | Dữ Liệu Lưu |
|------------|----------|-------|-------------|
| **ai_workflow_runs** | Lưu lịch sử **chạy workflow** | 1 workflow run = 1 lần chạy workflow từ đầu đến cuối | - Status workflow (pending, running, completed, failed)<br>- Danh sách stepRunIDs[]<br>- RootRefId (link về content)<br>- Result tổng hợp |
| **ai_step_runs** | Lưu lịch sử **chạy từng step** | 1 step run = 1 lần chạy 1 step trong workflow | - Status step (pending, running, completed, failed)<br>- **Input/Output** (structured data flow giữa các steps)<br>- Link về workflowRunId, stepId<br>- GenerationBatchID (nếu step type = GENERATE) |
| **ai_runs** | Lưu lịch sử **gọi AI API** | 1 AI run = 1 lần gọi AI API (GENERATE hoặc JUDGE) | - **Prompt/Response** (TEXT - chi tiết AI call)<br>- Cost, latency, tokens<br>- Conversation history (messages, reasoning)<br>- Link về stepRunId, workflowRunId |

**Mối Quan Hệ:**
- **1 Workflow Run** → **N Step Runs** (1 workflow có N steps)
- **1 Step Run** → **M AI Runs** (1 step có thể gọi AI nhiều lần)
  - Ví dụ: Step GENERATE có thể gọi AI 1 lần để generate 3 candidates → 1 AI run
  - Ví dụ: Step JUDGE có thể gọi AI 3 lần để judge 3 candidates → 3 AI runs

**Tại Sao Cần 3 Collections?**
- **ai_workflow_runs**: Quản lý workflow-level execution (status, progress, result tổng hợp)
- **ai_step_runs**: Quản lý step-level data flow (Input/Output giữa các steps - structured data)
- **ai_runs**: Quản lý AI API-level details (prompt, response, cost, conversation - TEXT và metadata)

---

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
| `steps` | Định nghĩa steps | Step definitions với input/output schemas, prompt template IDs - **KHÔNG có provider config** (config lưu trong prompt template) |
| `prompt_templates` | Quản lý prompts | Prompt templates với versioning, variables, types, **providerProfileId, model, temperature, maxTokens (override từ provider profile)** |
| `provider_profiles` | Quản lý AI providers | Provider profiles với API keys, config, models, pricing |
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
3. Worker execute từng step:
   a. Bot chuẩn bị variables từ step input (từ workflow context, parent content, etc.)
   b. Bot gọi POST /api/v2/ai/steps/:id/render-prompt với variables
   c. Backend render prompt và resolve AI config → trả về rendered prompt + config
   d. Bot gọi AI API với rendered prompt và config
   e. Bot parse response và tạo candidates/AI runs
4. Module 2 tạo drafts trong Module 1
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

### AI Step Standard Schema

Để đảm bảo mapping chính xác giữa output của step này và input của step tiếp theo trong workflow, hệ thống quy định **standard input/output schema** cho từng loại step.

#### ⚠️ QUAN TRỌNG: AI Input/Output vs System Input/Output

**AI Input/Output (CHỈ TEXT):**
- **AI Input (prompt)**: CHỈ là TEXT - được generate từ step input data
- **AI Output (response)**: CHỈ là TEXT - raw response từ AI API (JSON string hoặc plain text)

**System Input/Output (Structured Data):**
- **Step Input**: Dữ liệu đầu vào cho step (layerId, context, candidates, etc.) - dùng để generate prompt
- **Step Output**: Dữ liệu đầu ra của step (candidates[], scores[], etc.) - bao gồm:
  - Parsed output từ AI response text (do system parse)
  - Metadata do system tự bổ sung: timestamps, tokens, model, cost, etc.

**Flow Xử Lý:**
```
Step Input (structured) 
  → Generate Prompt (text) 
  → AI API Call 
  → AI Response (text) 
  → Parse Response (structured) 
  → Add System Metadata 
  → Step Output (structured)
```

#### Nguyên Tắc

1. **Mỗi step type PHẢI có standard schema**: GENERATE, JUDGE, STEP_GENERATION
2. **Required fields không được thiếu**: Khi tạo step, schema phải có đầy đủ required fields theo standard
3. **Cho phép mở rộng**: Có thể thêm fields tùy chọn nhưng không được bỏ required fields
4. **Mapping tự động**: Output của step này sẽ được map tự động vào input của step tiếp theo

#### Standard Schemas

**1. GENERATE Step**

**Input Schema (Required Fields):**
- `layerId` (string, required) - ID của layer cần generate content
- `layerName` (string, required) - Tên của layer
- `targetAudience` (string, required) - B2B, B2C, hoặc B2B2C
- `layerDescription` (string, optional) - Mô tả của layer
- `context` (object, optional) - industry, productType, tone
- `numberOfCandidates` (integer, optional, default: 3) - Số lượng candidates (1-10)

**Output Schema (Required Fields):**
- `candidates[]` (array, **required**) - Danh sách candidates (sẽ được dùng làm input cho JUDGE step)
  - `candidateId` (string) - System tự generate
  - `content` (string) - Từ AI response text (parsed)
  - `title` (string) - Từ AI response text (parsed)
  - `summary` (string) - Từ AI response text (parsed)
  - `metadata` (object) - System tự bổ sung
- `generatedAt` (string, required) - **System tự bổ sung** (không phải từ AI)
- `model` (string, optional) - **System tự bổ sung** (từ AI run record)
- `tokens` (object, optional) - **System tự bổ sung** (từ AI run record)

**Lưu ý**: AI chỉ trả về TEXT, system sẽ parse text đó thành candidates[] và bổ sung metadata

**2. JUDGE Step**

**Input Schema (Required Fields):**
- `candidates[]` (array, **required**) - Từ output của GENERATE step
  - `candidateId` (string)
  - `content` (string)
  - `title` (string)
- `criteria` (object, **required**) - Tiêu chí đánh giá
  - `relevance` (number, 0-10)
  - `clarity` (number, 0-10)
  - `engagement` (number, 0-10)
  - `accuracy` (number, 0-10)
- `context` (object, optional) - targetAudience, industry

**Output Schema (Required Fields):**
- `scores[]` (array, required) - Điểm số từng candidate (từ AI response text - parsed)
  - `candidateId` (string)
  - `overallScore` (number) - Từ AI response
  - `criteriaScores` (object) - Từ AI response
  - `feedback` (string) - Từ AI response
- `rankings[]` (array, required) - Xếp hạng candidates (từ AI response text - parsed)
- `bestCandidate` (object, optional) - Candidate tốt nhất (từ AI response text - parsed)
- `judgedAt` (string, required) - **System tự bổ sung** (không phải từ AI)

**Lưu ý**: AI chỉ trả về TEXT, system sẽ parse text đó thành scores[], rankings[] và bổ sung judgedAt

**3. STEP_GENERATION Step**

**Input Schema (Required Fields):**
- `parentContext` (object, required) - Context từ parent layer/step
  - `layerId` (string)
  - `layerName` (string)
  - `layerType` (string, L1-L8)
  - `content` (string)
- `requirements` (object, required) - Yêu cầu generate steps
  - `numberOfSteps` (integer, 1-10, default: 3)
  - `stepTypes` (array) - GENERATE, JUDGE, STEP_GENERATION
  - `focusAreas` (array)
  - `complexity` (string) - simple, medium, complex
- `targetLevel` (string, required) - L1-L8
- `constraints` (object, optional) - Ràng buộc
- `metadata` (object, optional)

**Output Schema (Required Fields):**
- `generatedSteps[]` (array, required) - Danh sách steps đã generate (từ AI response text - parsed)
  - `stepId` (string) - System tự generate khi tạo step
  - `stepName` (string) - Từ AI response
  - `stepType` (string) - Từ AI response
  - `order` (integer) - Từ AI response
  - `inputSchema` (object) - Từ AI response
  - `outputSchema` (object) - Từ AI response
  - `dependencies` (array) - Từ AI response
- `generationPlan` (object, required) - Kế hoạch generation (từ AI response text - parsed)
- `generatedAt` (string, required) - **System tự bổ sung** (không phải từ AI)
- `model` (string, optional) - **System tự bổ sung** (từ AI run record)
- `tokens` (object, optional) - **System tự bổ sung** (từ AI run record)

**Lưu ý**: AI chỉ trả về TEXT, system sẽ parse text đó thành generatedSteps[], generationPlan và bổ sung metadata

#### Mapping Logic

**GENERATE → JUDGE:**
```go
// Output của GENERATE step
{
  "candidates": [
    {"candidateId": "1", "content": "...", "title": "..."},
    ...
  ],
  "context": {...}
}

// → Input của JUDGE step
{
  "candidates": [...],  // Copy trực tiếp
  "context": {...},    // Copy nếu có
  "criteria": {...}    // Từ workflow config hoặc default
}
```

**JUDGE → STEP_GENERATION:**
```go
// Output của JUDGE step
{
  "bestCandidate": {"candidateId": "1", "score": 9.5, "reason": "..."},
  "scores": [...]
}

// → Input của STEP_GENERATION step
{
  "parentContext": {
    "content": "...",  // Từ bestCandidate hoặc từ candidate content
  },
  "requirements": {...},  // Từ workflow config
  "targetLevel": "L2"     // Từ workflow config
}
```

#### Validation

Khi tạo step, hệ thống sẽ tự động validate:
1. **Schema validation**: Kiểm tra required fields có đầy đủ không
2. **Type validation**: Kiểm tra step type có hợp lệ không
3. **Format validation**: Kiểm tra format của từng field

### AI Provider Profile

Để gọi AI API, hệ thống cần thông tin về provider (OpenAI, Anthropic, Google, etc.) bao gồm API key, config, models, và pricing. Thông tin này được lưu trong collection `ai_provider_profiles`.

#### Cấu Trúc Dữ Liệu

**1. Basic Info:**
- `id`: ID duy nhất của provider profile
- `name`: Tên profile (ví dụ: "OpenAI Production", "Claude Dev")
- `description`: Mô tả profile
- `provider`: Provider type (openai, anthropic, google, cohere, custom)
- `status`: Trạng thái (active, inactive, archived)

**2. Authentication:**
- `apiKey`: API key để gọi provider API (nên được encrypt khi lưu)
- `apiKeyEncrypted`: Flag để biết API key đã được encrypt chưa
- `baseUrl`: Base URL của API (nếu custom provider)
- `organizationId`: Organization ID (cho OpenAI organization billing)

**3. Configuration:**
- `defaultModel`: Model mặc định (ví dụ: "gpt-4")
- `availableModels`: Danh sách models có sẵn
- `defaultTemperature`: Temperature mặc định
- `defaultMaxTokens`: Max tokens mặc định
- `config`: Config bổ sung (timeout, retry, etc.)

**4. Pricing (Optional):**
- `pricingConfig`: Pricing config để tính cost
  ```json
  {
    "gpt-4": {
      "input": 0.03,   // USD per 1K tokens
      "output": 0.06   // USD per 1K tokens
    },
    "gpt-3.5-turbo": {
      "input": 0.0015,
      "output": 0.002
    }
  }
  ```

**5. Rate Limits:**
- `rateLimitRequestsPerMinute`: Rate limit requests per minute
- `rateLimitTokensPerMinute`: Rate limit tokens per minute

**6. Organization:**
- `ownerOrganizationID`: ID của tổ chức sở hữu provider profile

**7. Metadata:**
- `metadata`: Metadata bổ sung

#### Use Cases

**1. Tạo Provider Profile:**
```
POST /api/v1/ai/provider-profiles
Body: {
  name: "OpenAI Production",
  provider: "openai",
  apiKey: "sk-...",
  defaultModel: "gpt-4",
  availableModels: ["gpt-4", "gpt-3.5-turbo"],
  pricingConfig: {...},
  ...
}
```

**2. Sử dụng trong Prompt Template (Override Layer):**
```
Prompt Template có thể override config từ Provider Profile:
{
  providerProfileId: "provider-profile-id",  // Override provider (nếu không có thì dùng default)
  model: "gpt-4",                             // Override defaultModel từ provider
  temperature: 0.7,                          // Override defaultTemperature từ provider
  maxTokens: 2000                            // Override defaultMaxTokens từ provider
}
```

**Logic 2 Lớp Config:**
- **Lớp 1 (Provider Profile)**: Default config (defaultModel, defaultTemperature, defaultMaxTokens)
- **Lớp 2 (Prompt Template)**: Override config (providerProfileId, model, temperature, maxTokens) - override từ lớp 1
- **Step**: Chỉ có promptTemplateId, không có AI config - lấy config từ prompt template

**3. Sử dụng trong AI Run:**
```
Khi tạo AI run, lấy config theo thứ tự ưu tiên:
1. Prompt Template config (nếu có) - override từ Provider Profile
2. Provider Profile default config (nếu prompt template không có)
3. System default (nếu không có cả 2)
```

#### Lưu Ý Bảo Mật

- **API Key Encryption**: API key nên được encrypt trước khi lưu vào database
- **Organization Isolation**: Mỗi organization chỉ thấy provider profiles của mình
- **Access Control**: Chỉ admin của organization mới có thể tạo/update provider profiles

### Prompt Template Rendering

Prompt template chứa variables (ví dụ: `{{layerName}}`, `{{targetAudience}}`) cần được render (thay thế variables bằng giá trị thực tế) trước khi gọi AI API.

#### Nơi Gọi Render

**Prompt rendering được gọi trong Workflow Execution khi execute step:**

```
1. Load step definition từ ai_steps
2. Load prompt template từ ai_prompt_templates
3. Chuẩn bị variables từ step input data
4. **Gọi AIPromptTemplateService.RenderPrompt(template, variables)** ← ĐÂY
5. Nhận prompt TEXT đã được render
6. Gọi AI API với prompt TEXT đã render
```

#### Service Method

**AIPromptTemplateService.RenderPrompt()**

```go
// RenderPrompt render prompt template với variables từ step input
func (s *AIPromptTemplateService) RenderPrompt(
    template *models.AIPromptTemplate, 
    variables map[string]interface{},
) (string, error)
```

**Tham số:**
- `template`: Prompt template cần render (đã load từ database)
- `variables`: Map các biến và giá trị để thay thế (từ step input data)

**Logic:**
1. Lặp qua tất cả variables trong template
2. Với mỗi variable:
   - Lấy giá trị từ `variables` map
   - Nếu không có và variable là `required` → lỗi
   - Nếu không có và variable có `default` → dùng default value
   - Nếu không có và variable là `optional` → để trống
3. Thay thế `{{variableName}}` trong prompt bằng giá trị
4. Trả về prompt TEXT đã được render

**Ví dụ:**

```go
// Prompt template:
prompt: "Generate 3 content candidates for layer '{{layerName}}' targeting {{targetAudience}}..."

// Variables từ step input:
variables := map[string]interface{}{
    "layerName": "Target Audience",
    "targetAudience": "B2C",
}

// Render:
renderedPrompt, err := promptTemplateService.RenderPrompt(template, variables)
// Kết quả: "Generate 3 content candidates for layer 'Target Audience' targeting B2C..."
```

#### Flow Trong Workflow Execution

**Khi execute step:**

```
a. Load step definition → lấy promptTemplateId
b. Load prompt template từ ai_prompt_templates
c. Chuẩn bị variables từ step input data
d. **Render prompt:**
   renderedPrompt := promptTemplateService.RenderPrompt(template, variables)
e. Resolve AI config (provider, model, temperature, maxTokens)
f. Gọi AI API với:
   - Prompt: renderedPrompt (TEXT đã render)
   - Provider, Model, Temperature, MaxTokens: từ config đã resolve
g. Lưu renderedPrompt vào AIRun.prompt (để trace/debug)
```

#### API Endpoint Cho Bot

**Bot gọi API này để lấy prompt đã render và AI config trước khi gọi AI API:**

```
POST /api/v2/ai/steps/:id/render-prompt
Body: {
  variables: {
    layerName: "Target Audience",
    targetAudience: "B2C",
    context: {...}
  }
}

Response: {
  renderedPrompt: "Generate 3 content candidates for layer 'Target Audience' targeting B2C...",
  providerProfileId: "provider-profile-id",
  provider: "openai",
  model: "gpt-4",
  temperature: 0.7,
  maxTokens: 2000,
  variables: {...}  // Variables đã được sử dụng (để trace/debug)
}
```

**Flow Bot Sử Dụng:**
```
1. Bot có workflow command với stepId
2. Bot chuẩn bị variables từ step input (từ workflow context, parent content, etc.)
3. Bot gọi POST /api/v2/ai/steps/:id/render-prompt với variables
4. Backend:
   - Load step → lấy promptTemplateId
   - Load prompt template
   - Load provider profile (nếu có)
   - Resolve AI config (prompt template override provider default)
   - Render prompt với variables
   - Trả về rendered prompt + AI config
5. Bot nhận rendered prompt và AI config
6. Bot gọi AI API với:
   - Prompt: renderedPrompt (TEXT)
   - Provider, Model, Temperature, MaxTokens: từ response
7. Bot lưu AI run với prompt đã render
```

**Lưu Ý:**
- **Bot không biết giá trị variables**: Bot chỉ biết stepId và stepInput (structured data), không biết cách render prompt
- **Backend render prompt**: Backend có logic render và resolve config, bot chỉ cần gọi API
- **Variables từ step input**: Bot cần map step input data thành variables map theo format của prompt template

#### Lưu Ý

- **Render chỉ làm việc với TEXT**: Chỉ thay thế `{{variableName}}` bằng giá trị, không parse JSON hay xử lý logic phức tạp
- **Variables validation**: Kiểm tra required variables có đầy đủ không trước khi render
- **Default values**: Sử dụng default value nếu variable không có trong input
- **Error handling**: Trả về lỗi nếu required variable thiếu

### Thông Tin Một Lượt Gọi AI (AIRun)

Mỗi lần gọi AI API sẽ tạo một record trong collection `ai_runs` với đầy đủ thông tin về request, response, cost, và performance.

#### ⚠️ QUAN TRỌNG: AI Chỉ Làm Việc Với TEXT

**Flow đơn giản:**
```
Step Input (structured data)
  ↓
Generate Prompt (TEXT) ← Đầu vào của AI
  ↓
AI API Call (AI xử lý prompt text)
  ↓
AI Response (TEXT) ← Đầu ra của AI
  ↓
Parse Response (TEXT → structured data)
  ↓
Step Output (structured data)
```

**Điểm quan trọng:**
- **AI Input**: CHỈ là TEXT (prompt) - không phải structured data
- **AI Output**: CHỈ là TEXT (response) - có thể là JSON string hoặc plain text
- **AI không biết gì về structured data** - việc parse và structure hóa là do hệ thống làm

#### Cấu Trúc Dữ Liệu AIRun

**1. Basic Info:**
- `id`: ID duy nhất của AI run
- `type`: Loại AI call (GENERATE hoặc JUDGE)
- `status`: Trạng thái (pending, running, completed, failed)

**2. Prompt Template:**
- `promptTemplateId`: ID của prompt template được sử dụng để generate prompt

**3. AI Provider:**
- `providerProfileId`: ID của provider profile (chứa API key, config)
- `provider`: Tên provider (openai, anthropic, google, etc.)
- `model`: Model cụ thể được sử dụng (gpt-4, claude-3-opus, etc.)

**4. Prompt Data:**
- `prompt`: Prompt text cuối cùng đã được substitute variables - **ĐÂY LÀ ĐẦU VÀO CỦA AI (TEXT)**
- `variables`: Các variables đã được thay thế vào prompt template (dùng để trace/debug)
- `inputSchema`: Schema của step input data (KHÔNG gửi đến AI, chỉ dùng để validate step input)

**5. Response Data:**
- `response`: Raw response TEXT từ AI API - **ĐÂY LÀ ĐẦU RA CỦA AI (TEXT)** - có thể là JSON string hoặc plain text
- `parsedOutput`: Response đã được **HỆ THỐNG parse** thành structured data theo outputSchema (AI không tạo ra cái này, hệ thống tự parse)
- `outputSchema`: Schema của step output (KHÔNG phải schema của AI, mà là schema để hệ thống parse response text)

**5b. Conversation History (QUAN TRỌNG):**
- `messages[]`: Toàn bộ conversation history giữa hệ thống và AI
  - `role`: "system", "user", hoặc "assistant"
  - `content`: Nội dung message (TEXT)
  - `timestamp`: Thời gian message (milliseconds)
  - `metadata`: Metadata bổ sung (tokens, model, etc.)
- `reasoning`: Reasoning/thinking process của AI (nếu có, ví dụ: Claude's thinking)
- `intermediateSteps[]`: Các bước trung gian trong quá trình xử lý (nếu AI có nhiều bước)

**6. Cost & Performance:**
- `cost`: Chi phí tính bằng USD (tính từ tokens và model pricing)
- `latency`: Thời gian phản hồi (milliseconds) - từ lúc gửi request đến nhận response
- `inputTokens`: Số lượng tokens trong prompt
- `outputTokens`: Số lượng tokens trong response
- `qualityScore`: Điểm chất lượng (0.0-1.0) - từ judge step hoặc human rating

**7. Error:**
- `error`: Thông báo lỗi ngắn gọn
- `errorDetails`: Chi tiết lỗi (code, message, stack trace, etc.)

**8. References:**
- `stepRunId`: ID của step run (link đến step execution)
- `workflowRunId`: ID của workflow run (link đến workflow execution)
- `experimentId`: ID của experiment (link đến Module 3 - A/B testing)

**9. Timestamps:**
- `startedAt`: Thời gian bắt đầu gọi AI API
- `completedAt`: Thời gian nhận được response
- `createdAt`: Thời gian tạo record

**10. Organization:**
- `ownerOrganizationId`: ID của tổ chức sở hữu AI run (dùng cho phân quyền)

**11. Conversation History:**
- `messages[]`: Array các messages trong conversation (system, user, assistant)
- `reasoning`: Reasoning/thinking process của AI (nếu có)
- `intermediateSteps[]`: Các bước trung gian trong quá trình xử lý

**12. Metadata:**
- `metadata`: Các thông tin bổ sung tùy chỉnh (temperature, maxTokens, custom fields, etc.)

#### Flow Tạo AIRun

```
1. Tạo AIRun record (status: "pending")
   ↓
2. Set startedAt = now
   Update status: "running"
   ↓
3. Generate prompt TEXT từ step input data
   ↓
4. Gọi AI API với prompt TEXT (AI chỉ nhận TEXT)
   ↓
5. Nhận response TEXT từ AI (AI chỉ trả về TEXT)
   ↓
6. HỆ THỐNG parse response text → parsedOutput (structured data)
   ↓
7. Tính toán cost từ tokens
   ↓
8. Tính latency = completedAt - startedAt
   ↓
9. Update AIRun:
   - prompt: prompt text (đầu vào của AI)
   - response: raw response text (đầu ra của AI)
   - parsedOutput: structured data (hệ thống tự parse)
   - messages: conversation history (nếu có nhiều lượt chat)
   - reasoning: reasoning process (nếu AI có)
   - intermediateSteps: các bước trung gian (nếu có)
   - cost, latency, tokens
   - status: "completed"
   - completedAt: now
```

**Lưu ý:**
- AI chỉ nhận và trả về TEXT
- Việc parse TEXT → structured data là do hệ thống làm
- `inputSchema` và `outputSchema` là của step, không phải của AI

#### Conversation History

**Mục đích:** Lưu lại toàn bộ quá trình làm việc của AI để:
- Debug: Xem lại từng bước AI đã làm
- Learning: Phân tích cách AI reasoning để cải thiện prompts
- Transparency: Hiểu rõ quá trình AI tạo ra output

**Ví dụ Conversation History:**

```json
{
  "messages": [
    {
      "role": "system",
      "content": "You are a content generation assistant...",
      "timestamp": 1705123456789
    },
    {
      "role": "user",
      "content": "Generate 3 content candidates for layer 'Target Audience'...",
      "timestamp": 1705123456790
    },
    {
      "role": "assistant",
      "content": "I'll analyze the requirements and generate candidates...",
      "timestamp": 1705123457000,
      "metadata": {
        "tokens": 50
      }
    },
    {
      "role": "user",
      "content": "Please provide the candidates in JSON format",
      "timestamp": 1705123457100
    },
    {
      "role": "assistant",
      "content": "{\"candidates\": [...]}",
      "timestamp": 1705123457500,
      "metadata": {
        "tokens": 300
      }
    }
  ],
  "reasoning": "I first analyzed the target audience requirements, then generated multiple candidates with different approaches...",
  "intermediateSteps": [
    {
      "step": "analyze_requirements",
      "result": "Identified B2C audience, E-commerce industry",
      "timestamp": 1705123456800
    },
    {
      "step": "generate_candidates",
      "result": "Created 3 candidates with different angles",
      "timestamp": 1705123457200
    }
  ]
}
```

**Use Cases:**

1. **Debug AI Output:**
   - Xem lại conversation để hiểu tại sao AI tạo ra output như vậy
   - Phát hiện lỗi trong reasoning process

2. **Improve Prompts:**
   - Phân tích conversation để tối ưu prompts
   - Hiểu cách AI interpret prompts

3. **Quality Analysis:**
   - So sánh reasoning process giữa các AI runs
   - Tìm patterns trong cách AI xử lý

#### Lợi Ích

1. **Traceability**: Có thể trace lại mọi AI call với đầy đủ context
2. **Cost Tracking**: Theo dõi chi phí từng AI call
3. **Performance Monitoring**: Theo dõi latency, tokens usage
4. **Quality Analysis**: Theo dõi quality score qua thời gian
5. **Debugging**: Có thể xem lại prompt, response, và conversation history để debug
6. **A/B Testing**: Link đến experiment để so sánh kết quả
7. **Transparency**: Hiểu rõ quá trình AI reasoning và tạo output

### Nơi Lưu Trữ Content Draft và Bản Chấm Điểm

Khi AI Workflow thực thi, có 2 loại dữ liệu chính được tạo ra:
1. **Content Draft**: Bản nháp nội dung được AI tạo ra (sau khi chọn candidate tốt nhất)
2. **Bản Chấm Điểm**: Kết quả judge/scoring của các candidates

#### 1. Content Draft - Collection: `draft_content_nodes` (Module 1)

**Bản content draft mà AI tạo ra được lưu trong collection `draft_content_nodes` thuộc Module 1.**

**Cấu Trúc Dữ Liệu:**
```go
type DraftContentNode struct {
    ID primitive.ObjectID
    
    // ===== CONTENT HIERARCHY =====
    Type     string              // Loại: layer, stp, insight, contentLine, gene, script
    ParentID *primitive.ObjectID  // ID của parent node
    Text     string               // Nội dung text (từ candidate đã chọn)
    
    // ===== WORKFLOW LINK =====
    WorkflowRunID        *primitive.ObjectID  // Link về ai_workflow_runs (Module 2)
    CreatedByRunID       *primitive.ObjectID  // Link về ai_runs (Module 2)
    CreatedByStepRunID  *primitive.ObjectID  // Link về ai_step_runs (Module 2)
    CreatedByCandidateID *primitive.ObjectID   // Link về ai_candidates (Module 2) - QUAN TRỌNG
    CreatedByBatchID     *primitive.ObjectID  // Link về ai_generation_batches (Module 2)
    
    // ===== APPROVAL STATUS =====
    ApprovalStatus string  // pending, approved, rejected, draft
    
    // ===== ORGANIZATION =====
    OwnerOrganizationID primitive.ObjectID
    
    // ===== METADATA =====
    Metadata map[string]interface{}
    
    CreatedAt int64
    UpdatedAt int64
}
```

**Quy Trình Tạo Draft:**
```
1. [Module 2] GENERATE Step:
   - AI generate nhiều candidates
   - Lưu vào collection ai_candidates
   
2. [Module 2] JUDGE Step:
   - AI judge các candidates
   - Update JudgeScore cho từng candidate
   - Select candidate tốt nhất (highest score)
   
3. [Module 2] Tạo Draft Node:
   POST /api/v1/drafts/nodes
   Body: {
     type: "stp",
     text: selectedCandidate.Text,  // Text từ candidate đã chọn
     parentId: parentNodeID,
     workflowRunId: workflowRunID,
     createdByCandidateId: candidateID,  // Link về candidate
     ...
   }
   
4. [Module 1] Lưu vào draft_content_nodes:
   - Collection: draft_content_nodes
   - Text: từ candidate đã chọn
   - CreatedByCandidateID: link về ai_candidates
```

#### 2. Bản Chấm Điểm - Collection: `ai_candidates` (Module 2)

**Bản chấm điểm (judge scores) được lưu trong collection `ai_candidates` thuộc Module 2.**

**Cấu Trúc Dữ Liệu:**
```go
type AICandidate struct {
    ID primitive.ObjectID
    
    // ===== REFERENCES =====
    GenerationBatchID primitive.ObjectID  // Batch chứa candidate này
    StepRunID         primitive.ObjectID   // Step run tạo ra candidate
    
    // ===== CONTENT =====
    Text     string                 // Nội dung text của candidate
    Metadata map[string]interface{} // Metadata bổ sung
    
    // ===== JUDGING (QUAN TRỌNG) =====
    JudgeScore      *float64                // Quality score từ AI judge (0.0 - 1.0)
    JudgeReasoning  string                  // Lý do judge score
    JudgedByAIRunID *primitive.ObjectID     // ID của AI run thực hiện judge
    JudgeDetails    map[string]interface{}  // Chi tiết judge (tùy chọn)
    
    // ===== SELECTION =====
    Selected bool  // Candidate này đã được chọn hay chưa
    
    // ===== AI RUN REFERENCES =====
    CreatedByAIRunID primitive.ObjectID  // ID của AI run tạo ra candidate (GENERATE)
    
    CreatedAt int64
    OwnerOrganizationID primitive.ObjectID
}
```

**Quy Trình Chấm Điểm:**
```
1. [Module 2] GENERATE Step:
   - AI generate nhiều candidates
   - Tạo documents trong ai_candidates:
     {
       text: "Gen Z, 18-25, thích TikTok",
       judgeScore: null,  // Chưa có điểm
       selected: false
     }
   
2. [Module 2] JUDGE Step:
   - Load candidates từ generation batch
   - Gọi AI để judge
   - Parse response:
     {
       "scores": [
         {"candidateId": "candidate-1", "score": 0.95, "reasoning": "Phù hợp với Gen Z..."},
         ...
       ]
     }
   
3. [Module 2] Update Candidates:
   - Update JudgeScore cho từng candidate
   - Update JudgeReasoning
   - Update JudgedByAIRunID (link về AI run judge)
   - Select candidate tốt nhất: Selected = true
```

#### 3. Traceability - Liên Kết Giữa Draft và Judge Score

**Flow Traceability:**
```
DraftContentNode (Module 1)
  ↓ createdByCandidateId
AICandidate (Module 2)
  ├─ JudgeScore: 0.95
  ├─ JudgeReasoning: "..."
  ├─ JudgedByAIRunID → AIRun (JUDGE)
  └─ CreatedByAIRunID → AIRun (GENERATE)
```

**Tóm Tắt:**
- **Content Draft**: Collection `draft_content_nodes` (Module 1) - Text từ candidate đã chọn
- **Bản Chấm Điểm**: Collection `ai_candidates` (Module 2) - JudgeScore, JudgeReasoning, JudgedByAIRunID
- **Quan Hệ**: Mỗi draft node link về 1 candidate đã chọn, mỗi candidate có judge score và reasoning

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

### ⚠️ QUAN TRỌNG: Vị Trí Xử Lý Command AI

**Câu hỏi:** Việc chạy command AI (workflow commands) nên đặt ở đâu?

**Phân tích 3 lựa chọn:**

#### Option 1: Trong Module 2 (AI Service Backend) ❌ KHÔNG KHUYẾN NGHỊ

**Cách triển khai:**
- Module 2 có background service/job poll `workflow_commands` queue
- Service tự động xử lý commands và execute workflows

**Ưu điểm:**
- ✅ Đơn giản, không cần bot riêng
- ✅ Tất cả logic AI ở một nơi

**Nhược điểm:**
- ❌ **Tight coupling**: Module 2 vừa là API server vừa là worker
- ❌ **Scalability**: Khó scale riêng biệt (API server vs workers)
- ❌ **Resource conflict**: API requests và AI processing dùng chung resources
- ❌ **Deployment**: Phải restart API server khi update worker logic
- ❌ **Monitoring**: Khó tách biệt metrics giữa API và worker

#### Option 2: Trong folkgroup-agent (Sync Agent) ⚠️ TẠM THỜI OK

**Cách triển khai:**
- Thêm "Workflow job" vào folkgroup-agent
- Job query commands và tạo workers để xử lý

**Ưu điểm:**
- ✅ Tận dụng infrastructure có sẵn (check-in, command system)
- ✅ Tách biệt với Module 2 API server
- ✅ Dễ scale riêng biệt

**Nhược điểm:**
- ⚠️ **Mixed responsibilities**: Sync agent vừa sync data vừa xử lý AI
- ⚠️ **Different patterns**: Sync jobs (scheduled) vs AI commands (on-demand worker pool)
- ⚠️ **Dependencies**: AI agent cần AI SDKs, sync agent không cần
- ⚠️ **Config complexity**: Mỗi agent có config khác nhau

#### Option 3: Trong folkgroup-ai-agent riêng ✅ KHUYẾN NGHỊ

**Cách triển khai:**
- Tạo agent riêng `folkgroup-ai-agent` chỉ để xử lý AI commands
- Agent query `workflow_commands` queue và tạo worker pool
- Share common code (check-in, command handler base) với sync agent

**Ưu điểm:**
- ✅ **Separation of concerns**: Mỗi agent có một nhiệm vụ rõ ràng
- ✅ **Different execution patterns**: 
  - Sync agent: Scheduled jobs (cron-based)
  - AI agent: Worker pool (command-driven, on-demand)
- ✅ **Independent scaling**: Scale AI workers riêng biệt với sync jobs
- ✅ **Independent deployment**: Update AI agent không ảnh hưởng sync agent
- ✅ **Clean dependencies**: AI agent chỉ cần AI SDKs, không cần sync logic
- ✅ **Better monitoring**: Metrics riêng biệt cho từng agent

**Nhược điểm:**
- ⚠️ Cần maintain 2 agents (nhưng có thể share common code)

**Kết luận:** ✅ **Nên tách riêng thành `folkgroup-ai-agent`** vì:
1. Execution pattern khác nhau (Scheduled Jobs vs Worker Pool)
2. Dependencies khác nhau (AI SDKs vs sync logic)
3. Scaling requirements khác nhau
4. Separation of concerns rõ ràng hơn

**Lưu ý:** Có thể share common infrastructure code (check-in service, base command handler, API client) giữa 2 agents.

---

### Kiến Trúc

#### Option 2: Trong folkgroup-agent (Hiện Tại - Tạm Thời)

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
                        │  - Workflow job  │ ← Job mới (tạm thời)
                        └──────────────────┘
                                ↓
                        Workflow Job:
                        - Query commands
                        - Tạo workers
                        - Xử lý từng yêu cầu
```

#### Option 3: Trong folkgroup-ai-agent riêng (Khuyến Nghị - Tương Lai)

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
        ┌───────────────────────────┴───────────────────────────┐
        ↓                                                         ↓
┌──────────────────┐                                  ┌──────────────────┐
│  Sync Agent      │                                  │  AI Agent        │
│  (folkgroup-agent)│                                  │  (folkgroup-ai-  │
│                  │                                  │   agent)         │
│  - Check-in job  │                                  │                  │
│  - Sync jobs     │                                  │  - Check-in job  │
│  (Scheduled)     │                                  │  - Workflow job  │
└──────────────────┘                                  │  (Worker Pool)   │
                                                      └──────────────────┘
                                                               ↓
                                                      Workflow Job:
                                                      - Query commands
                                                      - Worker pool manager
                                                      - Xử lý commands async
```

**Lưu ý:** 
- Hiện tại sử dụng Option 2 (trong folkgroup-agent) như giải pháp tạm thời
- Nên migrate sang Option 3 (folkgroup-ai-agent riêng) để có kiến trúc tốt hơn

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

## 🔄 Toàn Bộ Quy Trình Từ Đầu Đến Cuối

### Tổng Quan

Quy trình hoàn chỉnh từ setup đến production bao gồm 4 giai đoạn chính:

```
┌─────────────────────────────────────────────────────────────┐
│  PHASE 1: SETUP (Một lần)                                  │
│  - Tạo Provider Profiles                                    │
│  - Tạo Prompt Templates                                     │
│  - Tạo AI Steps                                             │
│  - Tạo AI Workflows                                         │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│  PHASE 2: EXECUTION (Mỗi lần chạy workflow)                │
│  - Bot trigger workflow                                     │
│  - Module 2 execute workflow                                │
│  - AI generate & judge                                      │
│  - Tạo draft nodes                                          │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│  PHASE 3: REVIEW (Human review)                             │
│  - Human review drafts                                      │
│  - Human request approval                                   │
│  - Human approve/reject                                      │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│  PHASE 4: PRODUCTION (Commit & Publish)                     │
│  - Commit drafts → production                               │
│  - Render video (nếu cần)                                   │
│  - Publish to platforms                                     │
└─────────────────────────────────────────────────────────────┘
```

### PHASE 1: Setup (Một Lần - Configuration)

**Mục đích:** Chuẩn bị tất cả components cần thiết để chạy workflow

#### 1.1. Tạo Provider Profile

```
POST /api/v1/ai/provider-profiles
Body: {
  name: "OpenAI Production",
  provider: "openai",
  apiKey: "sk-...",
  defaultModel: "gpt-4",
  availableModels: ["gpt-4", "gpt-3.5-turbo"],
  pricingConfig: {
    "gpt-4": {"input": 0.03, "output": 0.06},
    "gpt-3.5-turbo": {"input": 0.0015, "output": 0.002}
  },
  ...
}
```

**Kết quả:** Provider profile được lưu trong `ai_provider_profiles` collection

#### 1.2. Tạo Prompt Templates

```
POST /api/v1/ai/prompt-templates
Body: {
  name: "Generate STP Content",
  type: "generate",
  version: "1.0.0",
  prompt: "Generate 3 content candidates for layer '{{layerName}}'...",
  variables: [
    {name: "layerName", required: true},
    {name: "targetAudience", required: true}
  ],
  providerProfileId: "provider-profile-id",  // Override provider (tùy chọn)
  model: "gpt-4",                            // Override defaultModel từ provider (tùy chọn)
  temperature: 0.7,                          // Override defaultTemperature từ provider (tùy chọn)
  maxTokens: 2000                            // Override defaultMaxTokens từ provider (tùy chọn)
}
```

**Kết quả:** Prompt template được lưu trong `ai_prompt_templates` collection

#### 1.3. Tạo AI Steps

```
POST /api/v1/ai/steps
Body: {
  name: "Generate Content - Layer L1",
  type: "GENERATE",
  promptTemplateId: "prompt-template-id",  // Reference đến prompt template (chứa AI config)
  inputSchema: {...},  // Standard schema
  outputSchema: {...}, // Standard schema
  targetLevel: "L2",
  parentLevel: "L1"
  // KHÔNG có providerProfileId, model, temperature, maxTokens - lấy từ prompt template
}
```

**Kết quả:** Step được lưu trong `ai_steps` collection

#### 1.4. Tạo AI Workflow

```
POST /api/v1/ai/workflows
Body: {
  name: "Content Generation Workflow",
  version: "1.0.0",
  steps: [
    {stepId: "generate-step-id", order: 0, policy: {...}},
    {stepId: "judge-step-id", order: 1, policy: {...}}
  ],
  rootRefType: "layer",
  targetLevel: "L1",
  ...
}
```

**Kết quả:** Workflow được lưu trong `ai_workflows` collection

### PHASE 2: Execution (Mỗi Lần Chạy Workflow)

**Mục đích:** Thực thi workflow để tạo content

#### 2.1. Bot Trigger Workflow

```
1. Bot (folkgroup-agent) query workflow_commands queue
   GET /api/v1/ai/workflow-commands?status=pending
   
2. Bot tạo worker để xử lý command
   
3. Bot gọi Module 2 API:
   POST /api/v1/ai/workflow-runs
   Body: {
     workflowId: "workflow-id",
     rootRefId: "layer-123",  // ID của Layer L1 từ Module 1
     rootRefType: "layer",
     params: {
       organizationId: "...",
       userId: "..."
     }
   }
```

**Kết quả:** Workflow run được tạo trong `ai_workflow_runs` collection (status: "pending")

#### 2.2. Module 2 Execute Workflow

```
1. Load workflow definition từ ai_workflows
2. Load root content từ Module 1 (GET /api/v1/content/nodes/:id)
3. Update workflow run status = "running"
4. Execute từng step theo thứ tự:
```

#### 2.3. Execute Step 1: GENERATE

```
a. Load step definition từ ai_steps
b. Load prompt template từ ai_prompt_templates (chứa AI config: providerProfileId, model, temperature, maxTokens)
c. Load provider profile từ ai_provider_profiles (dùng để lấy default config nếu prompt template không có)
d. Resolve AI config (logic 2 lớp):
   - Nếu prompt template có providerProfileId → dùng provider đó
   - Nếu prompt template có model → dùng model đó (override từ provider defaultModel)
   - Nếu prompt template có temperature → dùng temperature đó (override từ provider defaultTemperature)
   - Nếu prompt template có maxTokens → dùng maxTokens đó (override từ provider defaultMaxTokens)
   - Nếu prompt template không có → dùng default từ provider profile
e. Chuẩn bị input data từ step input:
   {
     layerId: "layer-123",
     layerName: "Target Audience",
     targetAudience: "B2C",
     context: {...}
   }
f. Render prompt TEXT:
   - Gọi AIPromptTemplateService.RenderPrompt(template, variables)
   - Variables lấy từ step input data (bước e)
   - Thay thế {{variableName}} trong prompt template bằng giá trị thực tế
   - Kết quả: Prompt TEXT đã được render (ví dụ: "Generate 3 content candidates for layer 'Target Audience'...")
g. Tạo AIRun record (status: "pending")
   ↓
h. Gọi AI API với prompt TEXT:
   - Provider: Từ providerProfileId (đã resolve ở bước d)
   - Model: Từ prompt template hoặc provider default (đã resolve ở bước d)
   - Temperature: Từ prompt template hoặc provider default (đã resolve ở bước d)
   - MaxTokens: Từ prompt template hoặc provider default (đã resolve ở bước d)
   - Prompt: "Generate 3 content candidates..."
   ↓
i. Nhận response TEXT từ AI:
   "{\"candidates\": [{\"content\": \"Gen Z, 18-25...\", ...}, ...]}"
   ↓
j. HỆ THỐNG parse response text → parsedOutput:
   {
     candidates: [
       {candidateId: "auto-id-1", content: "Gen Z, 18-25...", ...},
       {candidateId: "auto-id-2", content: "Millennials, 26-35...", ...},
       {candidateId: "auto-id-3", content: "Gen X, 36-50...", ...}
     ]
   }
   ↓
k. Tạo generation_batch:
   - BatchID: new ObjectID
   - StepRunID: link đến step run
   ↓
l. Tạo candidates trong ai_candidates:
   - GenerationBatchID: link đến batch
   - Text: candidate content
   - CreatedByAIRunID: link đến AI run
   - Selected: false
   ↓
l. Update AIRun:
   - response: raw response text
   - parsedOutput: structured data
   - messages: conversation history (nếu có)
   - reasoning: reasoning process (nếu có)
   - cost: tính từ tokens
   - latency: thời gian response
   - status: "completed"
   ↓
m. Tạo step run:
   - Status: "completed"
   - GenerationBatchID: link đến batch
   - Output: parsedOutput
```

**Kết quả:**
- AIRun record trong `ai_runs` (type: "GENERATE")
- Generation batch trong `ai_generation_batches`
- 3 candidates trong `ai_candidates`
- Step run trong `ai_step_runs`

#### 2.4. Execute Step 2: JUDGE

```
a. Load step definition (type: "JUDGE")
b. Load prompt template cho JUDGE (chứa AI config: providerProfileId, model, temperature, maxTokens)
c. Load provider profile (dùng để lấy default config nếu prompt template không có)
d. Resolve AI config (logic 2 lớp):
   - Nếu prompt template có providerProfileId → dùng provider đó
   - Nếu prompt template có model → dùng model đó (override từ provider defaultModel)
   - Nếu prompt template có temperature → dùng temperature đó (override từ provider defaultTemperature)
   - Nếu prompt template có maxTokens → dùng maxTokens đó (override từ provider defaultMaxTokens)
   - Nếu prompt template không có → dùng default từ provider profile
e. Lấy candidates từ step GENERATE trước:
   - Query candidates theo GenerationBatchID
f. Chuẩn bị input data từ step input:
   {
     candidates: [
       {candidateId: "auto-id-1", content: "...", ...},
       {candidateId: "auto-id-2", content: "...", ...},
       {candidateId: "auto-id-3", content: "...", ...}
     ],
     criteria: {
       relevance: 10,
       clarity: 10,
       engagement: 10,
       accuracy: 10
     }
   }
g. Render prompt TEXT:
   - Gọi AIPromptTemplateService.RenderPrompt(template, variables)
   - Variables lấy từ step input data (bước f)
   - Thay thế {{variableName}} trong prompt template bằng giá trị thực tế
   - Kết quả: Prompt TEXT đã được render (ví dụ: "Judge these candidates based on criteria...")
h. Tạo AIRun record (status: "pending")
   ↓
i. Gọi AI API để judge:
   - Provider: Từ providerProfileId (đã resolve ở bước d)
   - Model: Từ prompt template hoặc provider default (đã resolve ở bước d)
   - Temperature: Từ prompt template hoặc provider default (đã resolve ở bước d)
   - MaxTokens: Từ prompt template hoặc provider default (đã resolve ở bước d)
   - Prompt: judge prompt với candidates
   ↓
j. Nhận response TEXT:
   "{\"scores\": [{\"candidateId\": \"auto-id-1\", \"score\": 0.95, ...}, ...]}"
   ↓
k. HỆ THỐNG parse response text → parsedOutput:
   {
     scores: [
       {candidateId: "auto-id-1", overallScore: 0.95, ...},
       {candidateId: "auto-id-2", overallScore: 0.72, ...},
       {candidateId: "auto-id-3", overallScore: 0.58, ...}
     ],
     rankings: [...],
     bestCandidate: {candidateId: "auto-id-1", score: 0.95, ...}
   }
   ↓
l. Update candidates:
   - Update JudgeScore cho từng candidate
   - Update JudgeReasoning
   - Update JudgedByAIRunID
   - Select candidate tốt nhất: Selected = true
   ↓
m. Update AIRun:
   - response: raw response text
   - parsedOutput: structured data
   - messages: conversation history
   - reasoning: reasoning process
   - cost, latency, tokens
   - status: "completed"
   ↓
n. Tạo step run:
   - Status: "completed"
   - Output: judge results
```

**Kết quả:**
- AIRun record trong `ai_runs` (type: "JUDGE")
- Candidates được update với judge scores
- Candidate tốt nhất được select (Selected = true)
- Step run trong `ai_step_runs`

#### 2.5. Create Draft Node

```
Module 2 gọi Module 1 API:
POST /api/v1/drafts/nodes
Body: {
  type: "stp",
  text: selectedCandidate.Text,  // "Gen Z, 18-25, thích TikTok"
  parentId: "layer-123",
  workflowRunId: "workflow-run-id",
  createdByCandidateId: "candidate-auto-id-1",
  createdByRunId: "ai-run-judge-id",
  createdByStepRunId: "step-run-judge-id",
  ...
}
```

**Kết quả:** Draft node được lưu trong `draft_content_nodes` (Module 1)

#### 2.6. Tiếp Tục Các Steps Tiếp Theo

```
Step 3: GENERATE (STP → Insight)
- Read parent draft STP từ Module 1
- Gọi AI với prompt + context (STP draft)
- Generate candidates → Judge → Select
- Create draft Insight node

Step 4: GENERATE (Insight → Content Line)
- Tương tự...

Step 5: GENERATE (Content Line → Gene)
- Tương tự...

Step 6: GENERATE (Gene → Script)
- Tương tự...
```

#### 2.7. Workflow Run Completed

```
1. Tất cả steps đã hoàn thành
2. Update workflow run:
   - Status: "completed"
   - CompletedAt: timestamp
   - Result: tổng hợp kết quả
3. Bot update command:
   - Status: "completed"
   - WorkflowRunID: link đến workflow run
```

**Kết quả:**
- Tất cả draft nodes đã được tạo trong Module 1
- Workflow run status = "completed"

### PHASE 3: Review (Human Review)

**Mục đích:** Human review và approve drafts

#### 3.1. Human Query Drafts

```
GET /api/v1/drafts/nodes?workflowRunId=workflow-run-id
```

**Kết quả:** Danh sách tất cả draft nodes của workflow run

#### 3.2. Human Review Drafts

```
Human xem lại từng draft:
- STP draft
- Insight draft
- Content Line draft
- Gene draft
- Script draft

Human có thể:
- Xem candidate đã chọn
- Xem judge score và reasoning
- Xem conversation history của AI
- Xem reasoning process
```

#### 3.3. Human Request Approval

```
POST /api/v1/drafts/approval-requests
Body: {
  draftIds: ["draft-stp-id", "draft-insight-id", ...],
  workflowRunId: "workflow-run-id",
  ...
}
```

**Kết quả:** Approval request được tạo trong `draft_approvals` collection

#### 3.4. Human Approve/Reject

```
POST /api/v1/drafts/approve
Body: {
  approvalRequestId: "approval-request-id",
  action: "approve" | "reject",
  comments: "..."
}
```

**Nếu approve:**
- Module 1 commit tất cả drafts → production
- Tạo content_nodes, videos, publications (production)

**Nếu reject:**
- Drafts vẫn ở trạng thái draft
- Human có thể chỉnh sửa và request approval lại

### PHASE 4: Production (Commit & Publish)

**Mục đích:** Đưa content vào production và publish

#### 4.1. Commit Drafts → Production

```
Module 1 tự động commit khi approve:
- Copy draft nodes → content_nodes (production)
- Link về candidates, AI runs, workflow runs
- Update status = "published"
```

**Kết quả:** Content nodes được tạo trong `content_nodes` collection (production)

#### 4.2. Render Video (Nếu Cần)

```
External system render video từ script:
- Read script từ content_nodes
- Render video
- Update video status = "ready" trong Module 1
```

**Kết quả:** Video được lưu trong `videos` collection

#### 4.3. Publish to Platforms

```
External system tạo publication:
POST /api/v1/publications
Body: {
  videoId: "video-id",
  platform: "facebook",
  status: "published",
  ...
}
```

**Kết quả:** Publication được lưu trong `publications` collection

#### 4.4. Track Metrics

```
External system update metrics:
PUT /api/v1/publications/:id
Body: {
  metricsRaw: {
    views: 1000,
    likes: 50,
    shares: 10,
    comments: 5
  }
}
```

**Kết quả:** Metrics được lưu, Module 3 sẽ đọc để tính toán performance

### Traceability Flow

```
Content Node (Production)
  ↓ createdByCandidateId
AI Candidate
  ├─ JudgeScore: 0.95
  ├─ JudgeReasoning: "..."
  ├─ CreatedByAIRunID → AIRun (GENERATE)
  │   ├─ prompt: prompt text
  │   ├─ response: response text
  │   ├─ messages: conversation history
  │   ├─ reasoning: reasoning process
  │   ├─ cost, latency, tokens
  │   └─ parsedOutput: structured data
  └─ JudgedByAIRunID → AIRun (JUDGE)
      ├─ prompt: judge prompt
      ├─ response: judge response
      ├─ messages: conversation history
      ├─ reasoning: reasoning process
      └─ parsedOutput: scores, rankings
        ↓ stepRunId
AI Step Run
  ↓ workflowRunId
AI Workflow Run
  ↓ workflowId
AI Workflow
```

### Tóm Tắt

**Setup (1 lần):**
1. Tạo Provider Profile (API keys, config)
2. Tạo Prompt Templates (prompts với variables)
3. Tạo AI Steps (input/output schemas)
4. Tạo AI Workflows (sequence of steps)

**Execution (Mỗi lần):**
1. Bot trigger workflow
2. Module 2 execute workflow
3. Mỗi level transition: GENERATE → JUDGE → Create Draft
4. Lưu conversation history, reasoning process
5. Workflow run completed

**Review:**
1. Human review drafts
2. Human request approval
3. Human approve/reject

**Production:**
1. Commit drafts → production
2. Render video (nếu cần)
3. Publish to platforms
4. Track metrics

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
