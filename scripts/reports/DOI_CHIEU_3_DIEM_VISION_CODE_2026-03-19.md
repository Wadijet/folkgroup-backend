# Đối Chiếu 3 Điểm Vision vs Codebase (2026-03-19)

**Mục đích:** Đối chiếu 3 thay đổi lớn (CIO hub sync, AI Decision điều phối event, Approval ⊂ Executor) với code thực tế — đã có gì, cần sửa gì.

---

## 1. CIO = Hub đồng bộ dữ liệu từ mọi nguồn

### Vision
- CIO nhận, chuẩn hóa, lưu trữ mọi nguồn (Interaction, Order, Ads, CRM)
- Emit Decision Events
- Agent gửi raw qua `POST /cio/ingest/*`

### Đã có trong code

| Hạng mục | Vị trí | Ghi chú |
|----------|--------|---------|
| cio_events | Collection | Lưu interaction (conversation_updated, message_updated) |
| cio/ingestion | ingestion.go | OnConversationUpsert → ghi cio_event từ fb_conversation |
| OnCioEventInserted | init.registry.go | Callback → EnqueueAnalysis (CIX) |
| Nguồn hiện tại | FB (fb_conversations sync) | CRM hooks, cio touchpoint gọi ingestion |
| cio_sessions, cio_touchpoint_plans, … | Collections | Plan, routing, execution |

### Cần sửa / bổ sung

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | **POST /cio/ingest/*** | Chưa có. Vision: `POST /cio/ingest/interaction`, `/cio/ingest/order`, `/cio/ingest/ads`, `/cio/ingest/crm` — Agent gửi raw, CIO chuẩn hóa |
| 2 | **Order qua CIO** | Hiện: webhook Pancake → handler.pancake.pos.webhook trực tiếp. Cần: Pancake → CIO ingest → emit commerce.order_* |
| 3 | **Ads qua CIO** | Hiện: meta/ sync trực tiếp. Cần: Ads sync → CIO ingest (hoặc CIO đọc từ meta) → emit ads.data_updated |
| 4 | **CRM qua CIO** | Hiện: crm ingest trực tiếp. Cần: CRM delta → CIO ingest → emit crm.customer_updated |
| 5 | **Đa kênh** | Chỉ FB. Thiếu: Zalo, Web chat, Telegram, Call |

---

## 2. AI Decision quản lý và điều phối mọi event; domain giao tiếp qua AI Decision

### Vision
- AI Decision nhận mọi Decision Event
- AI Decision phân phối event xuống domain (CIX, Ads, Customer, Order)
- Domain xử lý → trả context về AI Decision
- Không có domain gửi trực tiếp cho domain khác

### Đã có trong code

| Hạng mục | Vị trí | Ghi chú |
|----------|--------|---------|
| AI Decision Execute | aidecision/service/engine.go | Nhận CIXPayload, applyPolicy, Propose/ProposeAndApproveAuto |
| ReceiveCixPayload | aidecision/service/cix.go | Entry từ CIX |
| Luồng CIX | CIX worker → AnalyzeSession → ReceiveCixPayload | **Ngược vision**: CIX gọi AI Decision (sync), không phải AI Decision nhận event → gọi CIX |
| CIO → CIX | OnCioEventInserted → EnqueueAnalysis | CIO không emit event → AI Decision. CIO trigger CIX trực tiếp |
| Ads | worker.ads.auto_propose | Ads tự Propose, không qua AI Decision |
| Customer | — | Chưa có event customer.flag_triggered → AI Decision |
| Order | — | Chưa có event commerce.order_* → AI Decision |

### Cần sửa / bổ sung

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | **Luồng event** | Vision: CIO emit event → AI Decision consume → AI Decision gọi CIX. Hiện: CIO → CIX trực tiếp (OnCioEventInserted). Cần: CIO emit → queue/bus → AI Decision consume → AI Decision gọi CIX |
| 2 | **Ads qua AI Decision** | Hiện: Ads worker tự Propose. Vision: Ads emit ads.performance_alert → AI Decision → phân phối Ads Intelligence → tạo Action |
| 3 | **Customer qua AI Decision** | Chưa: customer.flag_triggered → AI Decision. Cần pipeline |
| 4 | **Order qua AI Decision** | Chưa: commerce.order_* → AI Decision. Cần pipeline |
| 5 | **Event Intake** | Chưa: AI Decision chưa có lớp nhận Decision Events từ queue. Hiện chỉ sync ReceiveCixPayload |
| 6 | **Context Aggregation** | Chưa: Chỉ CIXPayload. Cần merge Customer, Ads, Order context |

---

## 3. Approval là một phần của Executor

### Vision
- Executor = Approval + Execution + Outcome
- Không có module approval riêng

### Đã có trong code

| Hạng mục | Vị trí | Ghi chú |
|----------|--------|---------|
| executor/ | api/internal/api/executor/ | Router, handler |
| /executor/actions/* | propose, approve, reject, execute, cancel | ✅ Đúng |
| pkg/approval | Propose, Approve, ProposeAndApproveAuto | Engine nội bộ — không phải module API riêng |
| action_pending_approval | Collection | ✅ |
| executors/cix, ads | Đăng ký Execute | ✅ |

### Cần sửa / bổ sung

| # | Công việc | Chi tiết |
|---|-----------|----------|
| 1 | **ApprovalModeConfig** | Chưa có. Vision: config-driven (domain, scopeKey, mode). Hiện: Ads dùng ShouldAutoApprove trong domain — **vi phạm** |
| 2 | **ResolveImmediate** | Chưa có. Vision: sau Propose, đọc config → auto Approve nếu mode=auto. Hiện: Ads gọi ProposeAndApproveAuto trực tiếp khi ShouldAutoApprove |
| 3 | **Bỏ ShouldAutoApprove** | service.ads.auto_propose.go L495, L562 — logic approval trong Ads. Cần: Ads luôn Propose; Executor/ResolveImmediate quyết định auto |

---

## Tổng Hợp — Cần Sửa Theo Ưu Tiên

### Ưu tiên cao (vi phạm boundary / sai luồng)

| # | Công việc | Module | Effort |
|---|-----------|--------|--------|
| 1 | **Ads: bỏ ShouldAutoApprove** | ads/service | 1 ngày |
| 2 | **ResolveImmediate + ApprovalModeConfig** | pkg/approval, executor | 2–3 ngày |
| 3 | **Delivery: validate source=APPROVAL_GATE** | delivery/handler | 0.5 ngày |

### Ưu tiên trung bình (chưa đúng vision)

| # | Công việc | Module | Effort |
|---|-----------|--------|--------|
| 4 | **POST /cio/ingest/*** | cio/handler | 2 ngày |
| 5 | **CIO emit event → AI Decision** | cio, aidecision | 2–3 ngày (event bus/queue) |
| 6 | **AI Decision Event Intake** | aidecision | 1–2 ngày |
| 7 | **AI Decision Context Aggregation** | aidecision | 1–2 ngày |

### Ưu tiên thấp (mở rộng)

| # | Công việc | Effort |
|---|-----------|--------|
| 8 | Order, Ads, Customer emit event → AI Decision | Dài hạn |
| 9 | Đa kênh CIO (Zalo, web chat) | Dài hạn |

---

## Sơ Đồ Hiện Tại vs Vision

### Hiện tại (code)
```
CIO (OnConversationUpsert) → cio_events
    → OnCioEventInserted → CIX EnqueueAnalysis (trực tiếp)
    → CIX worker → ReceiveCixPayload (CIX gọi AI Decision)
    → AI Decision Execute → Propose/ProposeAndApproveAuto
    → Executor (pkg/approval)
    → executors/cix → delivery

Ads: worker.ads.auto_propose → ShouldAutoApprove? ProposeAndApproveAuto : Propose
    → Executor (vi phạm: logic approval trong Ads)
```

### Vision (mục tiêu)
```
Raw → CIO (nhận + chuẩn hóa + lưu + emit) → Decision Events
    → AI Decision (Event Intake)
    → AI Decision phân phối xuống CIX, Ads, Customer, Order
    → Domain trả context
    → AI Decision (Context Aggregation + Decision Core) → Action
    → Executor (approval + execution)
    → delivery
```

---

## Changelog

- 2026-03-19: Tạo báo cáo đối chiếu 3 điểm vision vs code
