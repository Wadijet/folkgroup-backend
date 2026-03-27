# Learning Engine (Decision Brain) — Learning Memory Layer

**Mục đích:** **Bộ nhớ học tập** (learning memory) cho AI Commerce: lưu **một dòng** telemetry đã đóng vòng đời — **context + quyết định + hành động + outcome** — để phân tích, gợi ý rule và audit **trace E2E**.

> **Vision (shared):** [08 - ai-decision.md](../../../../docs-shared/architecture/vision/08%20-%20ai-decision.md) §18 — luồng Executor → `learning_case`. Tài liệu vision chi tiết Learning (nếu có trong bộ shared): `11 - learning-engine.md`.

---

## 1. Tổng quan kiến trúc

```
Activity Log     → Lưu sự kiện (event stream)
State Objects    → Lưu trạng thái hiện tại
Entities         → Xử lý vòng đời nghiệp vụ
Learning Engine  → Lưu case đã đóng (learning_cases) — không tham gia runtime quyết định
```

**Learning Engine KHÔNG phải:** Activity Log, event stream, lifecycle log thuần, hay nơi ra quyết định (việc đó thuộc **AI Decision**).

**Learning Engine LÀ:** lớp **learning memory** — case study sau outcome; hỗ trợ retrieval, evaluation batch, rule suggestion.

---

## 2. Learning case là gì

Một **learning case** là bản ghi **sau khi** nguồn (thường là **Action / Executor**) đã có **outcome** (thành công / từ chối / thất bại).

Cấu trúc tư duy: **Context → Choice → Goal → Outcome → (Lesson / evaluation / rule candidate sau này)**

| Thành phần | Trên `learning_cases` (tương ứng gần đúng) |
|------------|---------------------------------------------|
| Context | `contextSnapshot`, `inputSignals` |
| Choice / Goal | `decision`, `actionType`, `goalCode`, `domain` |
| Outcome | `outcome`, `result`, `actionExecuted` |
| Lesson / học sâu | `evaluation`, `learning` (job sau); `rule_suggestions` (Phase 3) |

---

## 3. Khi nào tạo learning case

**Chỉ khi `action_pending_approval` đóng vòng đời** với một trong các status:

- `executed`
- `rejected`
- `failed`

**Không insert** trong các trường hợp: pending / approved chưa chạy xong / cancelled — trừ khi sau này mở rộng có chủ đích.

**Bỏ qua ghi (điều kiện decision case):** Nếu action gắn `decisionCaseId` và case runtime đóng với `closureType` thuộc nhóm **không đủ ngữ cảnh học đầy đủ** (timeout / manual / proposed), service **không** insert learning case — trừ khi set env **`AI_DECISION_LEARNING_SKIP_INCOMPLETE_CLOSURE=0`**. Chi tiết: `api/internal/api/learning/service/service.learning.vision_policy.go`.

---

## 4. Nguồn tạo case (hiện tại)

| Loại | Collection / nguồn | Ghi chú |
|------|-------------------|---------|
| **Action (Executor)** | `action_pending_approval` | Luồng chính — `OnActionClosed` → `CreateLearningCaseFromAction` |
| **CIO Choice** | (stub) `BuildLearningCaseFromCIOChoice` | Chưa nối production |
| **Content Choice** | — | Tương lai |

---

## 5. MongoDB — collection

**Tên collection:** `learning_cases`  
(`decision_cases` cũ đã thay thế / migrate theo PLATFORM_L1 — code dùng `global.MongoDB_ColNames.LearningCases`.)

---

## 6. Schema tài liệu (trường chính)

Bản ghi được build từ **`BuildLearningCaseFromAction`** (`service.learning.builder.go`). Dưới đây là các nhóm trường quan trọng (BSON camelCase như model Go):

| Nhóm | Trường | Ý nghĩa |
|------|--------|---------|
| Định danh | `_id`, `caseId`, `ownerOrganizationId` | Org-scoped |
| Trace E2E | `decisionCaseId` | Neo `decision_cases_runtime` |
| | `decisionId` | ID quyết định trên payload action |
| | `executionTraceId` | `action_pending.traceId` — rule logs / live trace |
| | `correlationId`, `aidecisionProposeEventId`, `parentEventId`, `rootEventId` | Từ envelope queue / payload (xem §8) |
| Nguồn | `sourceRefType` (= `action_pending`), `sourceRefId` (= hex `_id` action) | Từ action → learning |
| Phân loại | `domain`, `actionType`, `caseType`, `caseCategory`, `goalCode`, `entityType`, `entityId`, `targetType`, `targetId` | Filter / analytics |
| Nội dung học | `contextSnapshot`, `inputSignals`, `rulesApplied`, `paramVersion`, `decision`, `actionExecuted`, `outcome` | Snapshot + kết quả |
| Policy | `decisionCaseClosureType` | Đồng bộ từ runtime khi có (hỗ trợ policy skip) |
| Timeline action | `actionLifecycle` | `proposedAt`, `approvedAt`, `rejectedAt`, `executedAt`, `finalStatus`, `idempotencyKey`, `actionCreatedAt`, `actionUpdatedAt` (Unix **ms**) |
| Sau xử lý | `evaluation`, `learning` | Worker evaluation / learning job |
| Thời gian | `createdAt`, `closedAt` | Learning case (ms) |

**Lưu ý:** `actionExecuted` thường chứa cả `payload` gốc của action (kèm các khóa trace đã merge từ queue).

---

## 7. Trace E2E: queue → action → learning

Để tra được **từ đầu đến cuối** khi propose đi qua **`executor.propose_requested`**:

1. Bản ghi **`decision_events_queue`** có `eventId`, `traceId`, `correlationId`, …
2. Trước khi gọi `approval.Propose`, consumer chạy **`MergeQueueEnvelopeIntoProposePayload`** — đưa vào **payload** của đề xuất (chỉ nếu payload **chưa** có khóa đó): `traceId`, `correlationId`, `aidecisionProposeEventId` (= `eventId` của event queue vừa xử lý), `parentEventId`, `rootEventId`.
3. `action_pending` lưu `traceId`, `decisionCaseId`, `decisionId` ở **top-level** và payload đầy đủ.
4. Khi action đóng → **`learning_cases`** copy các trường trace + `actionLifecycle`.

**Tra cứu nhanh:** `GET /api/v1/learning/cases?aidecisionProposeEventId=evt_...` hoặc `traceId=...` hoặc `decisionCaseId=...` (cùng org).

**Bổ sung 2026-03-26:** Chuỗi **`traceId` / `correlationId`** trên envelope queue từ hook datachanged + consumer + debounce + **`decision_cases_runtime`**; CIX có **`pipelineRuleTraceIds`** (nhiều bước rule) — xem [NGUYEN_TAC §9](../../05-development/NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md), [unified-data-contract §2.5b](../../../docs-shared/architecture/data-contract/unified-data-contract.md#contract-25b-trace-queue).

---

## 8. Code tham chiếu (folkgroup-backend)

| Việc | Vị trí |
|------|--------|
| Đăng ký hook đóng action | `api/cmd/server/init.registry.go` — `pkgapproval.OnActionClosed` |
| Tạo + policy skip | `api/internal/api/learning/service/service.learning.case.go` — `CreateLearningCaseFromAction` |
| Builder | `api/internal/api/learning/service/service.learning.builder.go` — `BuildLearningCaseFromAction` |
| Merge trace queue → payload | `api/internal/api/aidecision/service/service.aidecision.propose_trace.go` — `MergeQueueEnvelopeIntoProposePayload` |
| Gọi merge | `api/internal/api/aidecision/worker/worker.aidecision.consumer.go` — `processExecutorProposeRequested` |
| Model | `api/internal/api/learning/models/model.learning.case.go` |
| Rule execution theo trace | `service.learning.rules_applied.go` — `FetchRulesAppliedFromTraceID` |
| Evaluation batch | `service.learning.evaluation.go` — `RunEvaluationBatch` |
| Rule suggestion | `service.learning.rule_suggestion.go`, `worker.learning.rule_suggestion.go` |

---

## 9. API (HTTP)

Đăng ký route: `api/internal/api/learning/router/routes.go`. Tóm tắt:

| Method | Path | Mô tả |
|--------|------|--------|
| GET | `/learning/cases` | List (filter: `domain`, `caseType`, `goalCode`, `result`, `targetType`, `targetId`, `sourceRefId`, **`decisionCaseId`**, **`traceId`** → `executionTraceId`, **`correlationId`**, **`aidecisionProposeEventId`**, …) |
| GET | `/learning/cases/:id` | Chi tiết theo `_id` |
| POST | `/learning/cases` | Tạo thủ công (DTO riêng — không thay thế luồng tự động từ action) |
| GET/PATCH | `/learning/rule-suggestions` | Phase 3 — gợi ý rule |

Chi tiết endpoint: [docs/api/api-overview.md](../../api/api-overview.md).

---

## 10. Khác biệt với Activity Log

| | Activity Log | Learning Engine |
|---|--------------|-----------------|
| Mục đích | Timeline sự kiện | Case học / audit quyết định đã đóng |
| Thời điểm | Mỗi event | Khi action có outcome |
| Trace | Tuỳ nguồn | Chuẩn hóa `decisionCaseId` + `executionTraceId` + queue `eventId` |

---

## 11. Ví dụ JSON (rút gọn — action ads)

```json
{
  "caseId": "lc_674a1b2c_1710316800",
  "caseType": "action",
  "domain": "ads",
  "decisionCaseId": "dc_xxx",
  "decisionId": "dec_yyy",
  "executionTraceId": "trace_zzz",
  "aidecisionProposeEventId": "evt_aaa",
  "correlationId": "corr_bbb",
  "sourceRefType": "action_pending",
  "sourceRefId": "674a1b2c3d4e5f6789012345",
  "goalCode": "pause_campaign",
  "result": "success",
  "actionLifecycle": {
    "proposedAt": 1710316800000,
    "approvedAt": 1710316810000,
    "executedAt": 1710316820000,
    "finalStatus": "executed"
  },
  "outcome": {
    "technical": { "status": "success", "delivery": "delivered" },
    "direct": true
  }
}
```

---

## Changelog

- **2026-03-25:** Đồng bộ với triển khai: collection **`learning_cases`**, schema + **trace E2E** (queue → payload → learning), hook **`OnActionClosed`**, policy skip closure, API filter, file code tham chiếu.
- **2025-03-13:** Tài liệu thiết kế ban đầu (decision_cases / schema cũ — đã lỗi thời).
