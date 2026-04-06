# Rà Soát Vision vs Codebase — 2026-03-19

**Mục đích:** Đối chiếu tài liệu vision mới (00 - ai-commerce-os-platform-l1.md v3.1, các phần CIO/CIX/AI Decision/Executor/Order Intelligence) với codebase folkgroup-backend sau khi đổi tên module.

**Nguồn vision:** `docs/architecture/vision/` (workspace docs)

> **Ghi chú 2026-04-07:** Ký hiệu **Raw→L1→L2→L3** trong báo cáo này = **pipeline rule CIX**, không phải mirror/canonical **L1-persist/L2-persist**. Xem `docs/05-development/KHUNG_KHUON_MODULE_INTELLIGENCE.md` mục 0.

---

## 1. Tổng Quan Trạng Thái

| Thành phần Vision | Code hiện tại | Trạng thái |
|-------------------|---------------|------------|
| **CIO** (Universal Ingestion Hub) | `cio/` — router, handler, service, models | ✅ Đã có |
| **CIX** (Raw→L1→L2→L3→Flags) | `cix/` — pipeline, ReceiveCixPayload | ✅ Đã có |
| **AI Decision** (Event-driven, 3 lớp) | `aidecision/` — Execute, ReceiveCixPayload | ⚠️ Một phần (thiếu Event Intake, Context Aggregation đầy đủ) |
| **Executor** (Approval + Execution) | `executor/` — actions, send, execute, history | ✅ Đã có |
| **Learning Engine** | `learning/` — cases, BuildLearningCaseFromAction | ✅ Đã có |
| **Order Intelligence** | ❌ Chưa có module | Chưa triển khai |
| **Approval Gate thống nhất** | Logic rải rác (ads, CIX_APPROVAL_ACTIONS) | ❌ Thiếu ApprovalModeConfig, ResolveImmediate |
| **Delivery Gate** | allowDirectDeliveryUse, DELIVERY_ALLOW_DIRECT_USE | ⚠️ Chưa validate source=APPROVAL_GATE |

---

## 2. Đối Chiếu Từng Module

### 2.1 CIO — Customer Interaction Orchestrator

| Vision | Code | Ghi chú |
|--------|------|---------|
| Universal Data Ingestion Hub | ✅ cio_events, cio_sessions, cio_touchpoint_plans | Đã có |
| Emit Decision Events | ✅ OnCioEventInserted → cix_pending_analysis | Đã có |
| PlanTouchpoint, ExecuteTouchpoint | ✅ service.cio.touchpoint.go | Đã có |
| RULE_CIO_CHANNEL_CHOICE, FREQUENCY_CHECK | ✅ service.cio.routing.go | Đã có |
| Zalo, Website chat, Telegram, Call | ❌ | Chưa có |
| POST /cio/webhook/:channel | ❌ | Chưa có |
| RULE_CIO_ROUTING_MODE (AI vs Human) | ❌ | Chưa seed |

### 2.2 CIX — Contextual Conversation Intelligence

| Vision | Code | Ghi chú |
|--------|------|---------|
| Raw→L1→L2→L3→Flags | ✅ RULE_CIX_LAYER1_STAGE, LAYER2_*, LAYER3_*, FLAGS, ACTIONS | Đã có |
| CIO → CIX (OnCioEventInserted) | ✅ EnqueueAnalysis | Đã có |
| CIX → Decision (ReceiveCixPayload) | ✅ aidecisionsvc.ReceiveCixPayload | Đã có |
| KHÔNG tạo Action | ✅ Chỉ trả ActionSuggestions | Đúng boundary |

### 2.3 AI Decision Engine

| Vision (08 - ai-decision.md) | Code | Ghi chú |
|------------------------------|------|---------|
| **Lớp 1: Event Intake** | ❌ | Sync ReceiveCixPayload, không consume event |
| **Lớp 2: Context Aggregation** | ⚠️ Một phần | Chỉ CIXPayload, chưa merge Ads/Customer/Order đầy đủ |
| **Lớp 3: Decision Core** | ✅ applyPolicy, proposeCixAction, proposeAndApproveAutoCixAction | Đã có |
| Phân phối event xuống domain | ⚠️ | Chỉ CIX; Ads/Customer chưa được gọi từ AI Decision |
| Arbitration (nhiều action conflict) | ❌ | Chưa có |
| ApprovalModeConfig | ❌ | Dùng env CIX_APPROVAL_ACTIONS |

### 2.4 Executor

| Vision (09 - executor.md) | Code | Ghi chú |
|---------------------------|------|---------|
| 7 sub-layer | ⚠️ Một phần | Có Validation, Policy/Approval, Dispatch, chưa đủ Outcome Registry, Guardrail |
| Propose, Approve, Reject, Execute | ✅ executor/router, handler.executor.action | Đã có |
| /executor/actions/* | ✅ | Đã đổi từ /approval/actions/* |
| /executor/send, /executor/execute | ✅ | Đã đổi từ /delivery/* |
| ResolveImmediate (config-driven auto) | ❌ | Ads dùng ShouldAutoApprove trong domain — vi phạm boundary |

### 2.5 Learning Engine

| Vision | Code | Ghi chú |
|--------|------|---------|
| Chỉ học khi lifecycle kết thúc | ✅ CreateLearningCaseFromAction (executed/rejected/failed) | Đúng |
| decision_cases collection | ✅ `decision_cases` (learning module) | Đã có |
| /learning/cases | ✅ | Đã đổi từ /decision/cases |
| BuildLearningCaseFromAction | ✅ service.learning.builder.go | Đã có |
| CIO Choice cases | ✅ BuildDecisionCaseFromCIOChoice | Đã có |

### 2.6 Order Intelligence (Vision 07 - order-intelligence.md)

| Vision | Code | Ghi chú |
|--------|------|---------|
| Module Raw→L1→L2→L3→Flags | ❌ | Chưa có module orderintel/ |
| AI Decision gọi khi cần context order | ❌ | Chưa có |
| Trace creative/keyword/conversation → order | ⚠️ Một phần | posData.ad_id có; keyword chưa (Google Ads chưa có) |

### 2.7 Content OS

| Vision | Code | Ghi chú |
|--------|------|---------|
| Content Node L1–L8 | ✅ model.content.node, video, publication | Đã có |
| Insight từ Customer/Ads/Learning | ❌ | Chưa có pipeline tự động |
| Input Factory | ❌ | Chưa có |

### 2.8 Ads Intelligence

| Vision | Code | Ghi chú |
|--------|------|---------|
| Meta Ads | ✅ meta/, ads/ | Đầy đủ |
| Google Ads | ❌ | Chưa có |
| Cross Ads (creative winner → Content) | ❌ | Chưa có |

### 2.9 Customer Intelligence

| Vision | Code | Ghi chú |
|--------|------|---------|
| Classification, valueTier, lifecycleStage | ✅ crm/ | Đã có |
| Intent (I1–I4), Psychographic (Lớp 4) | ❌ | Chưa có |
| next_best_action, churn_risk (Lớp 5) | ❌ | Chưa có |
| Segment API động | ⚠️ | Có filter, chưa segment definition |

---

## 3. Cần Làm Tiếp Theo (Theo Thứ Tự Ưu Tiên)

### Phase 1: Approval Gate Thống Nhất (2–3 ngày) — **Ưu tiên cao**

| # | Công việc | File | Chi tiết |
|---|-----------|------|----------|
| 1 | ApprovalModeConfig | model mới (executor hoặc internal/approval) | domain, scopeKey, ownerOrganizationId, mode, actionOverrides |
| 2 | GetApprovalMode | internal/approval/config.go | Fallback: ads_meta_config, CIX_APPROVAL_ACTIONS |
| 3 | ResolveImmediate | pkg/approval/resolver.go | Sau Propose: đọc config → auto Approve nếu mode=auto |
| 4 | Ads refactor | service.ads.auto_propose.go | Bỏ ShouldAutoApprove; luôn Propose; Engine gọi ResolveImmediate |

### Phase 2: Delivery Gate Cứng (1–2 ngày)

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | Validate source | handler.delivery.execute: chỉ nhận source=APPROVAL_GATE, actionPendingId |
| 2 | Deprecation | DELIVERY_ALLOW_DIRECT_USE: log warning khi true |

### Phase 3: AI Decision 3 Lớp Đầy Đủ (2–3 ngày)

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | Event Intake | Nhận ExecuteRequest như Decision Event envelope; normalize, filter |
| 2 | Context Aggregation | Merge CIXPayload + CustomerCtx + gọi CRM/Ads nếu cần |
| 3 | Arbitration | Resolve conflict khi nhiều action cùng target |
| 4 | Policy từ config | Dùng GetApprovalMode thay env |

### Phase 4: Cập Nhật Docs (1 ngày)

| File | Cần sửa |
|------|---------|
| docs/api/api-overview.md | Bảng Modules: approval→executor, decision→ai-decision+learning, delivery→executor |
| docs/05-development/RASOAT_MODULE_KHUNG_XUONG.md | decision→aidecision+learning; approval→executor |
| 00 - ai-commerce-os-platform-l1.md | Sửa link 15 - order-intelligence → 07 - order-intelligence |

### Phase 5: Module Mới (Dài hạn)

| Module | Ưu tiên | Ghi chú |
|--------|---------|---------|
| **Order Intelligence** | Trung bình | Raw→L1→L2→L3→Flags cho order; AI Decision gọi khi cần |
| **Google Ads** | Trung bình | Module mới, cấu trúc tương tự meta/ |
| **Input Factory** | Trung bình | Service trong content/ hoặc module mới |
| **Cross Ads** | Thấp | service.ads.cross_intel.go + worker |

---

## 4. Luồng Đã Khép Vòng (CIO → CIX → AI Decision → Executor → Delivery)

```
cio_events (OnCioEventInserted)
    → cix_pending_analysis
    → CIX worker AnalyzeSession
    → ReceiveCixPayload (khi có ActionSuggestions)
    → aidecisionsvc.Execute
    → Propose / ProposeAndApproveAuto
    → executor (actions)
    → executors/cix → delivery.ExecuteActions
```

**Kết luận:** Luồng chính đã chạy. Cần bổ sung Approval Gate thống nhất và Delivery Gate validate.

---

## 5. Tài Liệu Tham Chiếu

| Loại | Đường dẫn |
|------|-----------|
| Vision Platform L1 | docs/architecture/vision/00 - ai-commerce-os-platform-l1.md |
| AI Decision | docs/architecture/vision/08 - ai-decision.md |
| Executor | docs/architecture/vision/09 - executor.md |
| Order Intelligence | docs/architecture/vision/07 - order-intelligence.md |
| Rà soát chi tiết | docs/architecture/vision/13 - ra-soat-trien-khai-vision.md |
| Đề xuất sửa code | docs/05-development/DE_XUAT_SUA_CODE_THEO_VISION.md |
| Kế hoạch phase | docs/architecture/vision/14 - ke-hoach-trien-khai-phase.md |

---

## Changelog

- 2026-03-19: Tạo báo cáo rà soát vision vs code sau cập nhật vision v3.1
