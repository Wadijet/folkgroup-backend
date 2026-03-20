# Tổng Hợp Thay Đổi CIO Eventbase

**Ngày:** 2026-03-18  
**Nguồn:** [DE_XUAT_NANG_CAP_CIO_EVENTBASE.md](./DE_XUAT_NANG_CAP_CIO_EVENTBASE.md)

---

## 1. Tóm Tắt Thay Đổi

| Hạng mục | Thay đổi |
|----------|----------|
| **Định nghĩa** | Eventbase = `cio_events` — event stream **có chọn lọc**, chỉ lưu event thuộc interaction ledger |
| **Quy tắc lọc** | 3 lớp A/B/C — không lưu pure domain compute (LTV, ads score, rule batch) |
| **Ref model** | 3 tầng: causedBy, links, resultRefs — pointer + snapshot nhỏ, không store full result |
| **Schema mở rộng** | eventCategory, eventScope, domain, tags, causedBy, resultRefs, statusSnapshot |
| **Luồng mới** | Order inject (pc_pos_orders → cio_events), touchpoint_triggered |
| **Thống kê** | domain, tags — aggregate theo domain, filter/drill-down theo tags |

---

## 2. Quy Tắc Lọc Event (3 Lớp)

| Lớp | Mô tả | Lưu CIO? |
|-----|-------|-----------|
| **A. Interaction** | message, conversation, order, touchpoint | ✅ |
| **B. Interaction-adjacent** | agent_assigned, delivery_failed, conversation_viewed | ✅ |
| **C. Pure domain compute** | ltv_recomputed, ads_scored, rule_batch_evaluated | ❌ |

**2 câu hỏi trước ingest:** (1) Gắn customer/session/thread? (2) Mất audit nếu không lưu? → Có/Có = lưu.

---

## 3. Ref Model (3 Tầng)

| Tầng | Field | Mô tả |
|------|-------|-------|
| **1. CausedBy** | `causedBy` | Event xảy ra vì gì? `{ module, entityType, entityUid }` |
| **2. Links** | `links` | Event gắn với gì? conversation, customer, session, order |
| **3. ResultRefs** | `resultRefs` | Event dẫn tới outcome nào? orderUid, deliveryUid, decisionUid |

**Nguyên tắc:** Lưu pointer + statusSnapshot nhỏ. Không copy full order, full execution payload.

---

## 4. Schema CioEvent Mở Rộng

| Field | Loại | Mô tả |
|-------|------|-------|
| eventCategory | string | `interaction` \| `business` |
| eventScope | string | `customer_external` \| `operator_internal` \| `system_operational` |
| domain | string | `cio` \| `crm` \| `pos` \| `delivery` \| `ads` \| `fb` |
| tags | []string | `["re_engage","zalo"]` — phục vụ thống kê |
| causedBy | CausedByRef | module, entityType, entityUid |
| resultRefs | map[string]string | orderUid, deliveryUid, decisionUid |
| statusSnapshot | map[string]interface{} | `{ deliveryStatus: "sent" }` — snapshot nhỏ |

---

## 5. Luồng Cần Triển Khai

| Luồng | Hành động |
|-------|-----------|
| pc_pos_orders | handleCrmDataChange → InjectOrderEvent → cio_events |
| ExecuteTouchpoint | InjectTouchpointTriggered → cio_events |
| OnConversation/OnMessage | Bổ sung eventCategory, eventScope, domain, tags |

---

## 6. Files Backend Cần Sửa

- `api/internal/api/cio/ingestion/ingestion.go` — InjectOrderEvent, InjectTouchpointTriggered
- `api/internal/api/cio/models/model.cio.event.go` — CausedByRef, ResultRefs, StatusSnapshot, Domain, Tags, EventScope, EventCategory
- `api/internal/api/crm/service/service.crm.hooks.go` — Hook pc_pos_orders → InjectOrderEvent
- `api/internal/api/cio/service/service.cio.touchpoint.go` — Gọi InjectTouchpointTriggered
- `api/internal/api/cio/service/service.cio.plan_executor.go` — Gọi InjectTouchpointTriggered
