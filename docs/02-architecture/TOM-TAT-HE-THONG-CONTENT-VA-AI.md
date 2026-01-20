# 📋 Tóm Tắt Hệ Thống Content và AI

> **Ngày tạo:** 2025-01-XX  
> **Dựa trên:** Content Strategy Operating System - Backend Design

---

## 🎯 Tổng Quan Hệ Thống

**Content Strategy Operating System** là hệ thống quản lý và tạo nội dung tự động với:
- **8 cấp độ nội dung** (L1-L8): Từ Layer đến Publication
- **AI tự động generate và judge** content
- **A/B testing** prompts và models
- **Learning từ metrics** thực tế
- **Kiến trúc 3 Modules độc lập**

---

## 📊 8 Cấp Độ Nội Dung (Content Levels)

Hệ thống quản lý nội dung theo cấu trúc phân cấp từ tổng quát đến cụ thể:

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

Hệ thống được chia thành **3 modules độc lập**, mỗi module có trách nhiệm riêng:

```
┌─────────────────────────────────────────────────────────────┐
│              Module 1: Content Storage                      │
│         (Pure Storage - Lưu trữ nội dung)                   │
│                                                              │
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

### Collections (7 collections)

| Collection | Tác Dụng | Mô Tả |
|------------|----------|-------|
| `content_nodes` | Lưu trữ production content nodes | Content nodes đã được duyệt và commit (L1-L6: Layer, STP, Insight, Content Line, Gene, Script) - Có creator type, creation method |
| `videos` | Lưu trữ production videos | Videos đã được duyệt và commit (L7) - Link với script, có asset URL, metadata |
| `publications` | Lưu trữ production publications | Publications đã được duyệt và commit (L8) - Link với video, platform, **có MetricsRaw (views, likes, shares, comments)** |
| `draft_content_nodes` | Lưu trữ draft nodes | Bản nháp content nodes (L1-L6) - Chưa được duyệt, có approval status, link về workflow run, candidate |
| `draft_videos` | Lưu trữ draft videos | Bản nháp videos (L7) - Chưa được duyệt, link về draft script |
| `draft_publications` | Lưu trữ draft publications | Bản nháp publications (L8) - Chưa được duyệt, link về draft video |
| `draft_approvals` | Quản lý approvals | Approval requests và decisions - Track approval workflow, có status (pending, approved, rejected) |

### Chức Năng Chính

1. **Content Nodes Management (L1-L6):**
   - Create: Tạo content node (thủ công hoặc từ Module 2)
   - Read: Đọc content node theo ID, type, parent
   - Update: Cập nhật content node
   - Delete: Xóa content node (soft delete)
   - Tree operations: Lấy children, ancestors

2. **Videos Management (L7):**
   - Create: Tạo video record
   - Read: Đọc video theo ID, script ID
   - Update: Cập nhật video (status, asset URL, metadata)
   - Link: Link video với script

3. **Publications Management (L8):**
   - Create: Tạo publication record
   - Read: Đọc publication theo ID, video ID, platform
   - Update: Cập nhật publication (status, metrics)
   - **MetricsRaw**: Lưu raw metrics từ platform (views, likes, shares, comments)

4. **Drafts Management:**
   - Create: Tạo draft node/video/publication
   - Read: Đọc draft theo ID, workflow run ID
   - Update: Cập nhật draft (edit trước khi approve)
   - Commit: Commit draft → production (sau khi approve)
   - Approval: Quản lý approval requests

### Workflow

```
Human tạo content thủ công
  → Module 1 lưu trực tiếp vào content_nodes (không qua draft)
  → CreatorType: "human", CreationMethod: "manual"

AI tạo content (từ Module 2)
  → Module 2 tạo draft node → Module 1 lưu vào draft_content_nodes
  → WorkflowRunId: link về workflow run
  → CreatedByRunId: link về AI run
  → Human review → Approve → Commit → Production
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

### Collections (10 collections)

#### Configuration (4 collections)
| Collection | Tác Dụng | Mô Tả |
|------------|----------|-------|
| `ai_workflows` | Định nghĩa workflows | Workflow definitions với steps, policies, rootRefType, targetLevel |
| `ai_steps` | Định nghĩa steps | Step definitions với input/output schemas, prompt template IDs, targetLevel - **KHÔNG có provider config** (config lưu trong prompt template) |
| `ai_prompt_templates` | Quản lý prompts | Prompt templates với versioning, variables, types (generate, judge, step_generation), **providerProfileId, model, temperature, maxTokens (override từ provider profile)** |
| `ai_provider_profiles` | Quản lý AI providers | Provider profiles với API keys, config, models, pricing, rate limits |

#### Execution (5 collections)
| Collection | Tác Dụng | Mô Tả |
|------------|----------|-------|
| `ai_workflow_runs` | Lịch sử workflow runs | **1 workflow run = 1 lần chạy workflow** - Status, rootRefId, stepRunIDs[], result - Quản lý toàn bộ workflow execution |
| `ai_step_runs` | Lịch sử step runs | **1 step run = 1 lần chạy 1 step trong workflow** - Link về workflowRunId, stepId - Input/Output (structured data flow giữa các steps) - Quản lý data flow và execution của từng step |
| `ai_generation_batches` | Batches của candidates | Batches chứa nhiều candidates được generate cùng lúc - TargetCount, ActualCount, CandidateIDs |
| `ai_candidates` | Content candidates | Candidates được generate, có judge scores, selected flag - Link về AI runs, generation batch |
| `ai_runs` | Lịch sử AI calls | **1 AI run = 1 lần gọi AI API** - Link về stepRunId, workflowRunId (optional) - Prompt, Response (TEXT), cost, latency, quality, **conversation history** - Chi tiết từng lần gọi AI API |

#### Queue (1 collection)
| Collection | Tác Dụng | Mô Tả |
|------------|----------|-------|
| `ai_workflow_commands` | Command queue | Queue commands cho bot xử lý (START_WORKFLOW, etc.) - Status, workflowId, params |

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

### Chức Năng Chính

1. **Workflow Management:**
   - Define workflows (sequence of steps)
   - Dynamic step generation (AI tạo steps tiếp theo dựa trên context)
   - Step types: GENERATE, JUDGE, STEP_GENERATION

2. **Prompt Template Management:**
   - Versioned prompt templates
   - Variable substitution
   - Strict JSON input/output schemas
   - Types: `generate`, `judge`, `step_generation`

3. **Workflow Execution:**
   - Execute workflows (tạo workflow runs)
   - Generate content candidates
   - Judge candidates (scoring)
   - Select best candidates
   - Create draft nodes trong Module 1

4. **Command Queue:**
   - Queue cho bot (folkgroup-agent) xử lý
   - Bot query commands và tạo workers
   - Process commands async

5. **AI Run Tracking:**
   - Log tất cả AI calls (prompt, model, cost, latency, quality score)
   - Traceability: link từ content → candidate → AI run

### Two-Step Level Transition (GENERATE/JUDGE)

Mỗi level transition (ví dụ: Layer → STP) phải có **2 bước riêng biệt**:

1. **GENERATE Step:**
   - AI generate content candidates
   - Tạo nhiều candidates (batch)
   - Lưu vào `ai_candidates` collection

2. **JUDGE Step:**
   - AI judge/scoring candidates
   - Tính quality score cho mỗi candidate
   - Select candidate tốt nhất
   - Commit candidate → draft node trong Module 1

**Lý do:**
- Tách biệt generation và judging để A/B testing
- Có thể test prompt versions riêng cho GENERATE và JUDGE
- Có thể so sánh judge scores với actual performance

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

### Bot xử lý workflow command (Chi tiết)

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

---

## 📊 Module 3: Analytics/Learning

### Mục Đích
**Module 3 là hệ thống phân tích và học hỏi:**
- Aggregated metrics từ publications
- Roll-up scores từ lower levels lên higher levels
- A/B testing experiments
- Learning insights và recommendations
- **KHÔNG** lưu trữ content
- **KHÔNG** gọi AI

### Collections (8 collections - dự kiến)

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

---

## 🔄 Mối Quan Hệ Giữa Các Collections

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

---

## 📝 Luồng Hoạt Động Tổng Thể

### Ví Dụ: Tạo Content Từ Layer Đến Publication

```
1. [Human] Tạo Layer (L1) thủ công
   → Module 1 lưu vào content_nodes

2. [Human] Trigger Workflow
   → Tạo workflow_command trong Module 2

3. [Bot] Query workflow_commands
   → Bot tạo worker để xử lý

4. [Module 2] Tạo Workflow Run
   → Status: "running"

5. [Module 2] Execute Step 1: GENERATE STP
   a. Load workflow definition từ ai_workflows
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

16. [External System] Update Metrics:
    External system update MetricsRaw trong publications
    → Module 1 cập nhật metricsRaw
    → Module 3 đọc MetricsRaw để tính toán performance
```

---

## 🔑 Điểm Quan Trọng

### 1. Phân Biệt Draft vs Production
- **Draft**: Chưa được duyệt, có thể edit, link về workflow run
- **Production**: Đã được duyệt và commit, không thể edit trực tiếp

### 2. Traceability
- Mọi content đều có thể trace về:
  - Workflow run → Step run → AI run → Prompt template → Provider
  - Candidate → Generation batch → AI run

### 3. AI Config Resolution
- AI config được resolve từ 2 lớp:
  1. Provider Profile (default config)
  2. Prompt Template (override config)
- Config bao gồm: providerProfileId, model, temperature, maxTokens

### 4. Standard Input/Output Schema
- Mỗi step type có standard input/output schema
- Đảm bảo mapping chính xác giữa output của step này và input của step tiếp theo

### 5. Module Independence
- 3 modules độc lập, giao tiếp qua HTTP API
- Module 1: Pure storage
- Module 2: AI orchestration
- Module 3: Analytics & learning

---

## 📚 Tài Liệu Tham Khảo

- **Design Document**: `docs/02-architecture/content-strategy-os-backend-design.md`
- **API Context**: `docs/ai-context/folkform/api-context.md`
- **Models**: `api/core/api/models/mongodb/`

---

**Lưu ý:** Tài liệu này là tóm tắt ngắn gọn. Để biết chi tiết đầy đủ, vui lòng tham khảo design document chính thức.
