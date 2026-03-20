# Phương Án: Tích Hợp Decision Engine với CIX

**Ngày:** 2026-03-18  
**Tham chiếu:** [PHUONG_AN_TRIEN_KHAI_CIX.md](./PHUONG_AN_TRIEN_KHAI_CIX.md) §6.4, [07 - ai-decision](../../docs-shared/architecture/vision/07%20-%20ai-decision.md) (nếu có), Executor (approval là phần của executor)

---

## 1. Tổng Quan

CIX phân tích hội thoại → Layer1/2/3, Flags, ActionSuggestions. Decision Engine nhận payload, ra quyết định, tạo Execution Plan. Một số action cần **approval** (duyệt người) trước khi thực thi.

**Tận dụng sẵn có:**
- `pkg/approval` — Propose → Approve/Reject → Execute
- `internal/approval` — bridge MongoDB + notifytrigger
- Domain `ads` đã đăng ký Executor, dùng deferred execution (worker)
- `DecisionEngineService.Execute` — khung xương, nhận `CIXPayload`
- `BuildDecisionCaseFromAction` — chuyển ActionPending → DecisionCase
- `delivery` — nhận `ExecutionActionInput`, route SEND_MESSAGE → queue

---

## 2. Luồng Đề Xuất

```
CIX AnalyzeSession
    │
    ├─► cix_analysis_results (đã có)
    ├─► OnCixSignalUpdate → CRM (đã có)
    │
    └─► ReceiveCixPayload (mới)
            │
            ▼
    DecisionEngine.Execute(ExecuteRequest{CIXPayload, ...})
            │
            ├─► Policy: action cần approval?
            │       │
            │       ├─ CÓ (escalate_to_senior, assign_to_human_sale)
            │       │   └─► approval.Propose(domain="cix", ...)
            │       │       └─► ActionPending (pending) → notify → user approve/reject
            │       │
            │       └─ KHÔNG (trigger_fast_response, prioritize_followup)
            │           └─► Build ExecutionActionInput[] → trả trong ExecuteResponse
            │
            └─► Caller (CIX worker/handler) gọi Delivery.Execute(actions) nếu có actions auto
```

---

## 3. Mapping CIX Actions → Execution / Approval

| CIX ActionSuggestion | Cần Approval? | ExecutionActionType | Ghi chú |
|----------------------|---------------|---------------------|---------|
| `escalate_to_senior` | **Có** | — | Propose → user duyệt chuyển conversation cho senior |
| `assign_to_human_sale` | **Có** | — | Propose → user duyệt assign agent |
| `prioritize_followup` | Không (tùy config) | `TAG_CUSTOMER` hoặc priority | Có thể auto: tag, tăng priority |
| `trigger_fast_response` | Không | `SEND_MESSAGE` | Auto: gửi template phản hồi nhanh |

**Policy config (env hoặc DB):**
- `CIX_APPROVAL_ACTIONS` — danh sách action bắt buộc approval, mặc định: `escalate_to_senior,assign_to_human_sale`

---

## 4. Chi Tiết Triển Khai

### 4.1 DecisionEngineService — Mở rộng Execute

**File:** `api/internal/api/decision/service/service.decision.engine.go`

```go
// Execute nhận context (CIX payload), ra quyết định.
// - Action cần approval → Propose(domain=cix), không trả action
// - Action auto → build ExecutionActionInput, trả trong response
func (s *DecisionEngineService) Execute(ctx, req, ownerOrgID) (*ExecuteResponse, error) {
    cixPayload := req.CIXPayload
    if cixPayload == nil {
        return s.executeEmpty(ctx, req, ownerOrgID)
    }
    actions := req.CIXPayload["actionSuggestions"] // []string
    // Policy: tách actions cần approval vs auto
    needApproval, autoActions := s.applyPolicy(actions)
    for _, a := range needApproval {
        s.proposeCixAction(ctx, a, req, ownerOrgID, baseURL)
    }
    execActions := s.buildExecutionActions(autoActions, req)
    return &ExecuteResponse{DecisionID, TraceID, execActions}, nil
}
```

### 4.2 ReceiveCixPayload — Entry point từ CIX

**File:** `api/internal/api/decision/service/service.decision.cix.go` (mới)

```go
// ReceiveCixPayload nhận CIX result, gọi Decision Engine, xử lý approval + auto actions.
func (s *DecisionEngineService) ReceiveCixPayload(ctx, result *CixAnalysisResult, ownerOrgID, baseURL) error {
    req := &ExecuteRequest{
        SessionUid:  result.SessionUid,
        CustomerUid: result.CustomerUid,
        CIXPayload:  toMap(result), // Layer1/2/3, Flags, ActionSuggestions
        TraceID:     result.TraceID,
    }
    resp, err := s.Execute(ctx, req, ownerOrgID)
    if err != nil { return err }
    // Gọi Delivery cho actions auto (nếu có)
    if len(resp.Actions) > 0 {
        deliverySvc.Execute(ctx, resp.Actions, ownerOrgID)
    }
    return nil
}
```

### 4.3 Domain CIX — Đăng ký Approval

**File:** `api/internal/api/cix/executor.go` (mới, tương tự `ads/executor.go`)

```go
func init() {
    approval.RegisterExecutor(DomainCix, pkgapproval.ExecutorFunc(executeCixAction))
    approval.RegisterEventTypes(DomainCix, map[string]string{
        "executed": "cix_action_executed",
        "rejected": "cix_action_rejected",
        "failed":   "cix_action_failed",
    })
    // CIX có thể dùng queue (deferred) hoặc execute ngay — tùy chọn
    // pkgapproval.RegisterDeferredExecutionDomain(DomainCix)
}

func executeCixAction(ctx, doc *ActionPending) (map[string]interface{}, error) {
    // Payload: actionType, sessionUid, customerUid, cixResultRef, ...
    // Map sang ExecutionActionInput (ASSIGN_TO_AGENT, SEND_MESSAGE, ...) → gọi Delivery
    return cixsvc.ExecuteCixAction(ctx, doc)
}
```

**Import trong main:** Đảm bảo package `cix` được import để `init()` chạy (ví dụ qua `cixrouter`).

### 4.4 Propose từ Decision Engine

**File:** `api/internal/api/decision/service/service.decision.engine.go`

```go
func (s *DecisionEngineService) proposeCixAction(ctx, actionType string, req *ExecuteRequest, ownerOrgID primitive.ObjectID, baseURL string) error {
    return approval.Propose(ctx, "cix", approval.ProposeInput{
        ActionType:       actionType,
        Reason:           buildReason(req.CIXPayload),
        Payload:          map[string]interface{}{
            "sessionUid":   req.SessionUid,
            "customerUid":  req.CustomerUid,
            "cixPayload":   req.CIXPayload,
            "traceId":      req.TraceID,
        },
        EventTypePending: "cix_action_pending",
    }, ownerOrgID, baseURL)
}
```

### 4.5 BuildDecisionCaseFromCix — Decision Brain

**File:** `api/internal/api/decision/service/service.decision.builder.go`

Thêm:

```go
// CixAnalysisInput input cho BuildDecisionCaseFromCix.
type CixAnalysisInput struct {
    SessionUid        string
    CustomerUid       string
    Layer1            CixLayer1
    Layer2            CixLayer2
    Layer3            CixLayer3
    Flags             []CixFlag
    ActionSuggestions []string
    Outcome           string  // "proposed" | "executed" | "rejected" | "skipped"
    SourceRefID       string  // cix_analysis_result ID hoặc action_pending ID
    SourceClosedAt    int64
    OwnerOrganizationID primitive.ObjectID
}

// BuildDecisionCaseFromCix chuyển CIX analysis (có outcome) thành DecisionCase.
// Gọi khi: (a) đã Propose và user approve/reject, hoặc (b) đã execute auto actions.
func BuildDecisionCaseFromCix(in *CixAnalysisInput) (*models.DecisionCase, error)
```

**CaseType:** Thêm `CaseTypeCixAnalysis = "cix_analysis"` trong `model.decision.case.go`.

---

## 5. Gọi ReceiveCixPayload từ CIX

**Vị trí:** Sau `AnalyzeSession` thành công, trong `service.cix.analysis.go` hoặc `cix_analysis_worker.go`.

```go
// Trong AnalyzeSession hoặc worker (sau InsertOne, OnCixSignalUpdate)
if len(result.ActionSuggestions) > 0 {
    decSvc, _ := decisionsvc.NewDecisionEngineService()
    baseURL := os.Getenv("BASE_URL")
    _ = decSvc.ReceiveCixPayload(ctx, result, ownerOrgID, baseURL)
}
```

**Lưu ý:** Fire-and-forget, không block. Có thể dùng goroutine nếu cần.

---

## 6. ExecutionActionInput — Action Types Cần Thêm

**File:** `api/internal/api/delivery/dto/dto.execution.action.go`

| ActionType | Mô tả | Delivery xử lý |
|------------|-------|----------------|
| `ASSIGN_TO_AGENT` | Chuyển conversation cho agent | TODO: integrate với inbox routing |
| `TAG_CUSTOMER` | Gắn tag ưu tiên | TODO: gọi CRM tag |
| `SEND_MESSAGE` | Đã có | Queue → notification |

---

## 7. Checklist Triển Khai

| # | Công việc | File |
|---|-----------|------|
| 1 | Mở rộng `DecisionEngineService.Execute` — parse CIXPayload, policy, propose | `service.decision.engine.go` |
| 2 | Thêm `ReceiveCixPayload` | `service.decision.cix.go` (mới) |
| 3 | Tạo `cix/executor.go` — RegisterExecutor, ExecuteCixAction | `api/internal/api/cix/executor.go` |
| 4 | Import cix trong main để init executor | `main.go` hoặc qua router |
| 5 | Gọi `ReceiveCixPayload` từ CIX (AnalyzeSession hoặc worker) | `service.cix.analysis.go` |
| 6 | `BuildDecisionCaseFromCix` | `service.decision.builder.go` |
| 7 | Thêm `CaseTypeCixAnalysis` | `model.decision.case.go` |
| 8 | Env `CIX_APPROVAL_ACTIONS`, `BASE_URL` | config |

---

## 8. Thứ Tự Ưu Tiên

1. **Phase 1 (tối thiểu):** ReceiveCixPayload + Execute parse CIXPayload, trả ExecutionActionInput (chưa Propose). Gọi Delivery nếu có SEND_MESSAGE.
2. **Phase 2:** Propose cho action cần approval, đăng ký Executor domain cix.
3. **Phase 3:** BuildDecisionCaseFromCix, lưu DecisionCase khi có outcome.

---

## 9. Rủi Ro & Mitigation

| Rủi ro | Mitigation |
|--------|------------|
| BASE_URL rỗng → approveUrl sai | Fallback env, validate trước Propose |
| Delivery chưa hỗ trợ ASSIGN_TO_AGENT | Trả unsupported, log; mở rộng sau |
| CIX gọi Decision đồng bộ → chậm | Fire-and-forget goroutine, timeout |
| Duplicate propose cùng session | IdempotencyKey = sessionUid + actionType + window |

---

## Changelog

- 2026-03-18: Tạo phương án tích hợp Decision Engine với CIX, tận dụng module approval
