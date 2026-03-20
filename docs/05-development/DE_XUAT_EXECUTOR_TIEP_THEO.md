# Đề Xuất Executor — Bước Tiếp Theo

**Ngày:** 2026-03-19  
**Nguồn:** Đối chiếu Vision 08 (Executor) với codebase, [PHUONG_AN_TRIEN_KHAI_EXECUTOR.md](./PHUONG_AN_TRIEN_KHAI_EXECUTOR.md), [DE_XUAT_SUA_CODE_THEO_VISION.md](./DE_XUAT_SUA_CODE_THEO_VISION.md)

---

## 1. Vision 08 — Executor (Tóm tắt từ docs-shared)

**Executor** = Action Control Tower + Execution Runtime + Outcome Tracker

> Mọi module khi tạo action **phải gửi về Executor** để thực thi. Không có đường đi tắt.

### 1.1 7 Sub-layer Vision

| # | Sub-layer | Chức năng |
|---|-----------|-----------|
| 1 | Action Intake | Nhận proposal, chuẩn hóa, gán idempotency |
| 2 | Validation & Normalization | Schema, required fields, adapter tồn tại |
| 3 | Policy / Guardrail / Approval | Policy routing, guardrail check, approval orchestration |
| 4 | Dispatch Runtime | Route adapter, execute, retry, idempotency |
| 5 | Execution Monitoring | State machine, theo dõi lifecycle |
| 6 | Outcome Evaluation | Thu outcome, đánh giá business result |
| 7 | Decision Brain Handoff | Chốt hồ sơ, đẩy khi đủ outcome window |

### 1.2 Approval Modes (Vision)

| Mode | Mô tả |
|------|-------|
| `manual_required` | Luôn chờ người duyệt |
| `auto_by_rule` | Tự động duyệt theo rule (vd: ads ActionRuleConfig.autoApprove) |
| `ai_recommend_human_confirm` | AI gợi ý, người xác nhận |
| `fully_auto` | Tự động hoàn toàn |

---

## 2. Trạng Thái Hiện Tại vs Vision

### 2.1 Đã Có

| Hạng mục | Trạng thái | Vị trí |
|----------|------------|--------|
| Propose, Approve, Reject, Execute | ✅ | executor/router, handler.executor.action |
| /executor/actions/*, /executor/send, /executor/execute | ✅ | executor/router |
| CIX Executor, Ads Executor, CIO Executor | ✅ | executors/cix, ads/executor, executors/cio |
| Delivery gate (block direct khi DELIVERY_ALLOW_DIRECT_USE=false) | ✅ | handler.delivery.execute |
| BuildLearningCaseFromAction (executed/rejected/failed) | ✅ | worker.ads.execution, handler.executor.action |
| Deferred execution (ads) | ✅ | RegisterDeferredExecutionDomain("ads") |
| Idempotency key trong payload | ✅ | decision_case_id:action_type:version |

### 2.2 Chưa Có / Chưa Đủ

| Hạng mục Vision | Trạng thái | Ghi chú |
|-----------------|------------|---------|
| **Action Policy Registry** | ❌ | Chưa có registry actionType → policy |
| **Guardrail Engine** | ❌ | Chưa có guardrail (rate limit, budget cap, conflict) |
| **ApprovalModeConfig** | ❌ | Config rải rác: ads_meta_config, CIX_APPROVAL_ACTIONS env |
| **ResolveImmediate** | ❌ | Ads dùng ShouldAutoApprove trong domain — vi phạm boundary |
| **Conflict / Concurrency** | ❌ | Chưa xử lý conflict khi nhiều action cùng target |
| **Outcome Registry + Evaluation** | ❌ | Chưa có outcome evaluation, business result |
| **State machine đầy đủ** | ⚠️ | Có pending/queued/executed/rejected, chưa đủ trạng thái |
| **Explainability Snapshot** | ❌ | Chưa lưu snapshot lý do quyết định |
| **Idempotency enforce** | ⚠️ | Payload có key, chưa enforce trùng trước Execute |

---

## 3. Phương Án Triển Khai (Theo Thứ Tự Ưu Tiên)

### Phase 1: Approval Gate Thống Nhất (2–3 ngày) — **Ưu tiên cao nhất**

**Mục tiêu:** Tách logic auto-approve khỏi domain (Ads), đưa vào Executor/Approval Gate. Config-driven.

| # | Công việc | File | Chi tiết |
|---|-----------|------|----------|
| 1 | **ApprovalModeConfig** | Model mới (executor hoặc internal/approval) | Struct: domain, scopeKey, ownerOrganizationId, mode, actionOverrides. Collection `approval_mode_config` |
| 2 | **GetApprovalMode** | `internal/approval/config.go` (mới) | `GetApprovalMode(ctx, domain, scopeKey, actionType) (mode string, err error)`. Fallback: ads_meta_config, CIX_APPROVAL_ACTIONS |
| 3 | **ResolveImmediate** | `pkg/approval/resolver.go` (mới) | Sau Propose: đọc GetApprovalMode; mode=auto → Approve nội bộ |
| 4 | **ProposeWithResolve** | `pkg/approval/engine.go` | Propose → Insert → ResolveImmediate (gọi trong Propose) |
| 5 | **Ads refactor** | `service.ads.auto_propose.go` | Bỏ `ShouldAutoApprove`; luôn gọi `Propose` (không ProposeAndApprove). Engine gọi ResolveImmediate |
| 6 | **Migration config** | Script (tùy chọn) | Map ads_meta_config.ActionRules[].autoApprove → approval_mode_config |

**Backward compat:** Nếu không có ApprovalModeConfig, fallback đọc ads_meta_config (giữ hành vi cũ).

### Phase 2: Delivery Gate Cứng (1 ngày)

| # | Công việc | File | Chi tiết |
|---|-----------|------|----------|
| 1 | **Validate source** | `handler.delivery.execute.go` | Chỉ nhận khi payload có `source=APPROVAL_GATE` và `actionPendingId` |
| 2 | **Deprecation** | Cùng file | Khi DELIVERY_ALLOW_DIRECT_USE=true: log warning |

### Phase 3: Idempotency Enforce (0.5–1 ngày)

| # | Công việc | File | Chi tiết |
|---|-----------|------|----------|
| 1 | **Check idempotency trước Execute** | `pkg/approval/engine.go` hoặc executor | Trước Execute: kiểm tra idempotency_key đã xử lý chưa → skip nếu trùng |
| 2 | **Index idempotency_key** | action_pending_approval | Index unique hoặc lookup nhanh |

### Phase 4: Action Policy Registry (1–2 ngày) — Tùy chọn

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | **Policy Registry** | Collection hoặc config: actionType → requiredApproval, guardrailIds |
| 2 | **Validation trước Propose** | Đọc registry, validate actionType hợp lệ |

### Phase 5: Guardrail Engine (2–3 ngày) — Dài hạn

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | **Guardrail interface** | Check(ctx, action) (allowed bool, reason string) |
| 2 | **Rate limit guardrail** | Số action/ngày theo domain, scopeKey |
| 3 | **Budget cap guardrail** | Ads: tổng budget không vượt cap |
| 4 | **Hook vào Propose** | Trước Insert: chạy guardrails, reject nếu fail |

---

## 4. Lộ Trình Đề Xuất

```
Phase 1 (2–3 ngày)     Phase 2 (1 ngày)      Phase 3 (0.5–1 ngày)
─────────────────     ─────────────────     ─────────────────
ApprovalModeConfig  →  Delivery gate      →  Idempotency enforce
ResolveImmediate       validate source
Ads: chỉ Propose       deprecation
GetApprovalMode
```

**Phase 4–5:** Thực hiện khi có nhu cầu (policy phức tạp, guardrail).

---

## 5. Chi Tiết Phase 1 (Approval Gate)

### 5.1 ApprovalModeConfig Schema

```go
type ApprovalModeConfig struct {
    ID                  primitive.ObjectID `bson:"_id,omitempty"`
    OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
    Domain              string             `bson:"domain"`      // ads | cix | cio
    ScopeKey            string             `bson:"scopeKey"`    // adAccountId, planId, "" (default)
    Mode                string             `bson:"mode"`       // manual_required | auto_by_rule | fully_auto
    ActionOverrides     map[string]string   `bson:"actionOverrides,omitempty"` // actionType -> mode
}
```

### 5.2 GetApprovalMode Logic

1. Query `approval_mode_config` theo (ownerOrgId, domain, scopeKey)
2. Nếu có ActionOverrides[actionType] → dùng
3. Else dùng Mode
4. Fallback: domain=ads → đọc ads_meta_config.ActionRules; domain=cix → env CIX_APPROVAL_ACTIONS

### 5.3 ResolveImmediate Flow

```
Propose(doc) → Insert(doc) → GetApprovalMode(doc) 
  → if mode in [auto_by_rule, fully_auto]: Approve(doc) nội bộ (sync)
  → else: Notify pending, return
```

### 5.4 Ads Refactor

- `service.ads.auto_propose.go`: Xóa `ShouldAutoApprove`, luôn gọi `Propose(domain="ads", ...)`
- Engine/Propose: Sau Insert gọi ResolveImmediate
- ResolveImmediate: Đọc GetApprovalMode(domain=ads, scopeKey=adAccountId, actionType) → nếu auto thì Approve

---

## 6. Rủi Ro & Mitigation

| Rủi ro | Mitigation |
|--------|------------|
| Breaking Ads auto-propose | Fallback đọc ads_meta_config nếu chưa có approval_mode_config |
| Import cycle (approval ↔ ads) | GetApprovalMode trong internal/approval, inject hoặc lazy load ads config |
| ResolveImmediate gọi Approve sync | Approve đã có logic Execute ngay (cix, cio) hoặc queued (ads) |

---

## 7. Tài Liệu Tham Chiếu

| Tài liệu | Đường dẫn |
|----------|-----------|
| Vision Executor | docs-shared/architecture/vision/08 - executor.md |
| Phương án Executor | [PHUONG_AN_TRIEN_KHAI_EXECUTOR.md](./PHUONG_AN_TRIEN_KHAI_EXECUTOR.md) |
| Đề xuất sửa code | [DE_XUAT_SUA_CODE_THEO_VISION.md](./DE_XUAT_SUA_CODE_THEO_VISION.md) |
| AI Decision & Learning | [PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md](./PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md) |

---

## Changelog

- 2026-03-19: Tạo đề xuất Executor tiếp theo dựa trên Vision 08 và gap analysis
