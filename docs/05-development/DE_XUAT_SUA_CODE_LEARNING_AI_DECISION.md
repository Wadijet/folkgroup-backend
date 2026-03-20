# Đề Xuất Phương Án Sửa Code — Learning & AI Decision

**Ngày:** 2026-03-19  
**Dựa trên:** [PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md](./PHUONG_AN_TRIEN_KHAI_AI_DECISION_VA_LEARNING.md)

---

## 1. Tóm Tắt Đối Chiếu Vision vs Codebase

### 1.1 Đã Khớp Vision

| Thành phần | Vị trí | Ghi chú |
|------------|--------|---------|
| API routes | `/ai-decision/execute`, `/learning/cases`, `/executor/actions/*`, `/executor/send`, `/executor/execute`, `/executor/history` | ✅ Đúng theo vision |
| pkg/approval | `Propose`, `Approve`, `Reject`, `ProposeAndApproveAuto`, `ExecuteOne` | ✅ Có đủ |
| Executor API | `handler.executor.action.go` | ✅ Có |
| AI Decision Engine | `aidecision/` — Execute, ReceiveCixPayload | ✅ Có |
| Learning engine | `learning/` — BuildLearningCaseFromAction, CreateLearningCaseFromAction | ✅ Có |
| CIX → AI Decision | `service.cix.analysis` → `aidecisionsvc.ReceiveCixPayload` | ✅ Có |
| Domain cix Executor | `executors/cix/` | ✅ Có |
| BuildLearningCase khi executed/failed/rejected | worker.ads.execution, handler.executor.action | ✅ Có |

### 1.2 Chưa Có / Cần Sửa

| Thành phần | Trạng thái | Ưu tiên |
|------------|------------|---------|
| **ApprovalModeConfig** | ❌ Chưa có model, collection, service | Phase 1 |
| **ResolveImmediate** | ❌ pkg/approval chưa có | Phase 1 |
| **GetApprovalMode** | ❌ Chưa có service đọc config | Phase 1 |
| **Ads scheduler** | Logic auto-approve nằm trong scheduler (ShouldAutoApprove → ProposeAndApprove) | Phase 1 |
| **Delivery Gate** | Chưa validate `source=APPROVAL_GATE` / `actionPendingId` | Phase 2 |
| **Context Aggregation** | AI Decision Engine chưa merge đầy đủ context | Phase 3 |
| **Policy / Arbitration / Action Builder** | Chỉ TODO trong comment | Phase 3 |
| **CIO migration** | CIO dùng flow riêng, chưa qua pkg/approval | Phase 4 |

---

## 2. Phase 1: Config & Resolve (2–3 ngày)

### 2.1 Tạo ApprovalModeConfig

**File mới:** `api/internal/api/approval/models/model.approval_mode_config.go`

```go
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ApprovalModeConfig cấu hình chế độ duyệt theo domain/org.
// Collection: approval_mode_config
type ApprovalModeConfig struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty"`
	Domain              string             `bson:"domain"`      // ads | cio | cix | delivery
	ScopeKey            string             `bson:"scopeKey"`     // adAccountId | planId | "" (org-level)
	OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
	Mode                string             `bson:"mode"`         // human | auto | ai
	ActionOverrides     map[string]string   `bson:"actionOverrides,omitempty"` // actionType -> mode
	CreatedAt           int64              `bson:"createdAt"`
	UpdatedAt           int64              `bson:"updatedAt"`
}
```

**Collection:** `approval_mode_config` — đăng ký trong `init.data.go` / `init.registry.go`.

**Index:** `(ownerOrganizationId, domain, scopeKey)` unique.

---

### 2.2 Service GetApprovalMode

**File mới:** `api/internal/approval/config.go`

```go
package approval

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ApprovalModeResolver interface để resolve mode. App inject implementation.
type ApprovalModeResolver interface {
	GetApprovalMode(ctx context.Context, domain, scopeKey, actionType string, ownerOrgID primitive.ObjectID) (mode string)
}

// GetApprovalModeDefault trả mode mặc định khi không có config.
const GetApprovalModeDefault = "human"

// approvalModeResolver global — set bởi InitConfig.
var approvalModeResolver ApprovalModeResolver

// SetApprovalModeResolver inject resolver.
func SetApprovalModeResolver(r ApprovalModeResolver) {
	approvalModeResolver = r
}

// GetApprovalMode gọi resolver. Trả human|auto|ai.
func GetApprovalMode(ctx context.Context, domain, scopeKey, actionType string, ownerOrgID primitive.ObjectID) string {
	if approvalModeResolver == nil {
		return GetApprovalModeDefault
	}
	mode := approvalModeResolver.GetApprovalMode(ctx, domain, scopeKey, actionType, ownerOrgID)
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode != "auto" && mode != "ai" {
		return "human"
	}
	return mode
}
```

**Implementation:** `api/internal/approval/bridge/config_resolver.go` — đọc từ MongoDB `approval_mode_config`, fallback `ads_meta_config.actionRuleConfig` (migration từ config cũ).

---

### 2.3 ResolveImmediate trong pkg/approval

**File:** `api/pkg/approval/engine.go`

**Thêm interface:**

```go
// ApprovalModeResolver (trong interfaces.go hoặc engine)
type ApprovalModeResolver interface {
	GetApprovalMode(ctx context.Context, domain, scopeKey, actionType string, ownerOrgID primitive.ObjectID) string
}
```

**Thêm vào Engine struct:**

```go
type Engine struct {
	storage   Storage
	notifier  Notifier
	resolver  ApprovalModeResolver  // optional, nil = luôn human
}
```

**Thêm hàm ResolveImmediate:**

```go
// ResolveImmediate đọc config, nếu mode=auto thì Approve ngay. Gọi sau Propose.
// mode=human: không làm gì (giữ pending)
// mode=ai: TODO Phase 5
func (e *Engine) ResolveImmediate(ctx context.Context, doc *ActionPending, scopeKey string) error {
	if doc.Status != StatusPending {
		return nil
	}
	if e.resolver == nil {
		return nil
	}
	mode := e.resolver.GetApprovalMode(ctx, doc.Domain, scopeKey, doc.ActionType, doc.OwnerOrganizationID)
	if mode != "auto" {
		return nil
	}
	_, err := e.approveAndExecute(ctx, doc, time.Now().UnixMilli())
	return err
}
```

**Cập nhật NewEngine:** Thêm param `resolver ApprovalModeResolver` (có thể nil).

---

### 2.4 Luồng Propose Mới: Propose + ResolveImmediate

**Option A — Gọi ResolveImmediate trong Propose (sau Insert):**

Trong `pkg/approval/engine.go` → `Propose()`:
- Sau `e.storage.Insert(ctx, doc)`
- Gọi `e.ResolveImmediate(ctx, doc, scopeKey)` — cần thêm `scopeKey` vào `ProposeInput`

**Option B — Caller gọi Propose rồi ResolveImmediate:**

- `Propose()` giữ nguyên
- Caller (Ads scheduler, AI Decision Engine) gọi `Propose()` xong gọi `ResolveImmediate()`
- Cần truyền `scopeKey` từ caller

**Đề xuất:** Option B — ít thay đổi Propose, linh hoạt hơn. Caller chịu trách nhiệm gọi ResolveImmediate.

---

### 2.5 Refactor Ads Scheduler

**File:** `api/internal/api/ads/service/service.ads.auto_propose.go`

**Hiện tại (dòng 495–518, 562–585):**

```go
if metaCfg != nil && ShouldAutoApprove(ruleCode, metaCfg) {
	_, err = ProposeAndApprove(ctx, &ProposeInput{...}, c.OwnerOrganizationID)
} else {
	_, err = Propose(ctx, &ProposeInput{...}, c.OwnerOrganizationID, baseURL)
}
```

**Thay bằng:**

```go
doc, err := Propose(ctx, &ProposeInput{...}, c.OwnerOrganizationID, baseURL)
if err != nil {
	return proposed, fmt.Errorf("propose campaign %s: %w", c.CampaignId, err)
}
// ResolveImmediate: đọc ApprovalModeConfig, nếu mode=auto thì Approve ngay
scopeKey := c.AdAccountId
_ = approval.ResolveImmediate(ctx, doc, scopeKey)
proposed++
```

**Lưu ý:**
- Bỏ hoàn toàn `ProposeAndApprove` trong scheduler
- `ResolveImmediate` sẽ đọc config: ưu tiên `approval_mode_config`, fallback `ads_meta_config.actionRuleConfig` (để backward compat)
- Migration: script copy config từ `ads_meta_config` sang `approval_mode_config` cho các org đang dùng

---

### 2.6 Migration Config

**File:** `api/internal/api/approval/migration/migrate_ads_to_approval_mode_config.go`

- Đọc `ads_meta_config` có `actionRuleConfig` (KillRules, DecreaseRules, IncreaseRules)
- Với mỗi rule có `AutoApprove: true` → tạo/update `approval_mode_config`: domain=ads, scopeKey=adAccountId, actionOverrides[ruleCode]=auto
- Chạy một lần khi deploy Phase 1

---

## 3. Phase 2: Delivery Gate (1–2 ngày)

### 3.1 Luồng Hiện Tại

- **Executor (in-process):** Approve → `executors[domain].Execute(doc)` → CIX/Ads executor gọi `deliverysvc.ExecuteActions()` trực tiếp — **không qua HTTP**, không bị gate.
- **HTTP:** POST `/executor/execute` → `HandleExecute` → gate `allowDirectDeliveryUse()`: nếu `false` → 403 cho **mọi** request.

**Kết luận:** Gate chỉ áp dụng cho HTTP. Executor nội bộ luôn đi qua (gọi service trực tiếp).

### 3.2 Vấn Đề Hiện Tại

- Khi `DELIVERY_ALLOW_DIRECT_USE != "true"` → 403 cho mọi HTTP request (kể cả request hợp lệ có `actionPendingId`).
- Không có cơ chế cho HTTP caller chứng minh request đến từ luồng đã approve.

### 3.3 Đề Xuất Sửa

**ExecutionActionInput** đã có `Source`. Thêm `ActionPendingID`:

```go
ActionPendingID string `json:"actionPendingId,omitempty"` // ID từ action_pending_approval — chứng minh đã qua Approval Gate
```

**Logic gate mới (handler):**

```go
// Cho phép khi: (1) có actionPendingId (đã qua approve), (2) hoặc Source=APPROVAL_GATE, (3) hoặc DELIVERY_ALLOW_DIRECT_USE (deprecated)
func isRequestAllowed(req ExecuteRequest) bool {
	if allowDirectDeliveryUse() {
		return true
	}
	for _, a := range req.Actions {
		if a.ActionPendingID != "" || a.Source == "APPROVAL_GATE" {
			return true
		}
	}
	return false
}
```

**Executor khi gọi Delivery (nội bộ):** Đã gọi service trực tiếp — không cần sửa. Có thể set `Source: "APPROVAL_GATE"`, `ActionPendingID: doc.ID.Hex()` trong payload để audit.

---

## 4. Phase 3: AI Decision Engine Mở Rộng (2–3 ngày)

### 4.1 Context Aggregation

**File mới:** `api/internal/api/aidecision/service/service.aidecision.context.go`

```go
type AggregatedContext struct {
	SessionUid    string
	CustomerUid   string
	Domain        string
	Layer1        map[string]interface{}
	Layer2        map[string]interface{}
	Layer3        map[string]interface{}
	Flags         []string
	CustomerCtx   map[string]interface{}
	CIXActions   []string
	RuleOutputs   []RuleOutput
	CIOContext    map[string]interface{}
	AdsContext    map[string]interface{}
	TraceID       string
	CorrelationID string
}

func (s *AIDecisionService) AggregateContext(ctx context.Context, req *ExecuteRequest, ownerOrgID primitive.ObjectID) (*AggregatedContext, error) {
	// Merge CIXPayload, CustomerCtx, gọi Rule/CRM/Ads nếu cần
	// ...
}
```

### 4.2 Policy Evaluation — Dùng ApprovalModeConfig

Thay vì env `CIX_APPROVAL_ACTIONS` cố định, gọi `GetApprovalMode(domain=cix, scopeKey="", actionType)` → nếu `auto` thì Propose + ResolveImmediate.

### 4.3 Arbitration (đơn giản)

Khi nhiều action cùng target (vd: campaignId) và conflict → chọn theo priority (Manual > CIX > Rule > Default), hoặc bỏ qua và log.

---

## 5. Phase 4: CIO Migration (2–3 ngày)

- `CreateExecutionRequest` → gọi `approval.Propose(domain=cio, ...)` thay vì tạo `cio_plan_executions` status=pending_approval
- Đăng ký Executor domain=cio: load execution, gọi `RunExecution`
- `ApproveExecution` / `RejectExecution` → map sang `approval.Approve` / `approval.Reject`

---

## 6. Checklist Thực Hiện

### Phase 1
- [ ] Tạo model `ApprovalModeConfig`, collection `approval_mode_config`
- [ ] Tạo `internal/approval/config.go` + `GetApprovalMode`, `ApprovalModeResolver`
- [ ] Implement resolver đọc từ DB (fallback ads_meta_config)
- [ ] Thêm `ResolveImmediate` vào `pkg/approval`
- [ ] Inject `ApprovalModeResolver` vào Engine
- [ ] Refactor Ads scheduler: chỉ `Propose` + `ResolveImmediate`
- [ ] Migration script ads_meta_config → approval_mode_config

### Phase 2
- [ ] Thêm `Source`, `ActionPendingID` vào `ExecutionActionInput`
- [ ] Sửa Delivery gate: cho phép khi `Source=APPROVAL_GATE` hoặc `ActionPendingID` có giá trị
- [ ] Executor khi execute: set `Source`, `ActionPendingID` trong payload gửi Delivery

### Phase 3
- [ ] `AggregateContext` trong AI Decision Engine
- [ ] Policy dùng `GetApprovalMode` thay env
- [ ] Arbitration (đơn giản)

### Phase 4
- [ ] CIO CreateExecutionRequest → Propose
- [ ] Executor domain=cio
- [ ] ApproveExecution/RejectExecution → approval.Approve/Reject

---

## 7. Rủi Ro & Lưu Ý

| Rủi ro | Mitigation |
|--------|------------|
| Ads backward compat | Phase 1: nếu không có `approval_mode_config`, resolver fallback đọc `ads_meta_config` — hành vi giữ như cũ |
| Delivery gate chặn Executor | Phase 2: Executor phải set `Source=APPROVAL_GATE` khi gọi Delivery |
| Config trùng lặp | Migration gộp dần vào `approval_mode_config`, deprecate ads_meta_config.actionRuleConfig (dài hạn) |

---

## 8. Thứ Tự Ưu Tiên Đề Xuất

1. **Phase 1** — Config & Resolve: tách logic auto-approve khỏi scheduler, config-driven
2. **Phase 2** — Delivery Gate: validate nguồn, chuẩn bị cho Phase 3
3. **Phase 3** — AI Decision Engine: Context Aggregation, Policy từ config
4. **Phase 4** — CIO: tùy nhu cầu, có thể trì hoãn
