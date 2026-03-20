# Spec Code-Level — AI Decision Module (Phase 1)

**Ngày:** 2026-03-19  
**Nguồn:** [PLATFORM_L1_EVENT_DECISION_SUPPLEMENT.md](../../docs/architecture/vision/PLATFORM_L1_EVENT_DECISION_SUPPLEMENT.md)  
**Mục đích:** Đủ chi tiết để bắt đầu code

---

## 1. Mapping event_type → case_type

| event_type | case_type | required_contexts |
|------------|-----------|-------------------|
| `conversation.message_inserted` | `conversation_response_decision` | `["cix","customer"]` |
| `message.batch_ready` | `conversation_response_decision` | `["cix","customer"]` |
| `cio_event.inserted` | `conversation_response_decision` | `["cix","customer"]` |
| `cix.analysis_completed` | (update case) | — |
| `customer.context_ready` | (update case) | — |
| `customer.updated` | `customer_state_decision` | `["customer"]` |
| `order.inserted`, `order.updated` | `order_risk_decision` | `["order"]` |
| `ads.updated` | `ads_optimization_decision` | `["ads"]` |

---

## 2. Mapping event_type → priority, lane

| event_type | priority | lane |
|------------|----------|------|
| `conversation.message_inserted` | high | fast |
| `message.batch_ready` | high | fast |
| `cio_event.inserted` | high | fast |
| `cix.analysis_completed` | high | fast |
| `customer.context_ready` | high | fast |
| `customer.updated` | normal | normal |
| `order.inserted`, `order.updated` | high | fast |
| `ads.updated` | normal | batch |

---

## 3. ResolveOrCreate — Merge Rule

**Attach vào case cũ nếu:**
- `org_id` trùng
- `entity_refs.conversation_id` trùng (hoặc `customer_id` nếu case_type customer_state)
- `case_type` trùng
- `status` chưa closed (không trong `closed`, `cancelled`, `expired`, `dropped`)
- Trong time window: `opened_at` >= now - 30 phút (config `merge_window_sec`)

**Nếu tìm thấy case cũ:** Update `trigger_event_ids`, `latest_event_id`, `status` → `context_collecting` nếu cần

**Nếu không:** Tạo case mới với `status=opened`

---

## 4. Event Queue Service — Lease Pattern

**Lease:** Worker gọi `LeaseOne(ctx, lane, workerID, leaseDurationSec)`:
- `findOneAndUpdate`: `status=pending`, `lane=lane`, `scheduled_at <= now` (hoặc null)
- Set `status=leased`, `leased_by=workerID`, `leased_until=now+leaseDurationSec`
- Return document

**Complete:** `UpdateStatus(ctx, eventID, "completed")`

**Fail:** `UpdateStatus(ctx, eventID, "failed_retryable")` hoặc `failed_terminal`:
- Nếu retryable: `attempt_count++`, `scheduled_at = now + backoffDelay`, `status=pending`

**Retry backoff:** Attempt 1→5s, 2→30s, 3→2 phút, 4→10 phút

---

## 5. Worker Polling

- **Interval:** 2 giây (config)
- **Lane:** Worker chạy từng lane (fast trước, normal, batch)
- **Query:** `findOne` với sort `{priority:1, created_at:1}` filter `status=pending`, `lane=X`, `scheduled_at <= now` (hoặc null)
- **Concurrency:** 1 event per worker (có thể scale worker process)

---

## 6. API POST /ai-decision/events

**Request:**
```json
{
  "eventType": "conversation.message_inserted",
  "eventSource": "cio",
  "entityType": "message",
  "entityId": "msg_xxx",
  "orgId": "org_xxx",
  "priority": "high",
  "lane": "fast",
  "traceId": "trace_xxx",
  "correlationId": "corr_xxx",
  "payload": {
    "conversationId": "conv_xxx",
    "customerId": "cust_xxx"
  }
}
```

**Response:** 200 OK
```json
{
  "code": 200,
  "message": "Đã nhận event",
  "data": {
    "eventId": "evt_xxx",
    "status": "pending"
  }
}
```

**Logic:** Validate → emit vào `decision_events_queue` (status=pending) → return

---

## 7. Idempotency Key

Format: `{decision_case_id}:{action_type}:{version}`

- `version` = `created_at` của decision_case (hoặc timestamp khi tạo decision_packet)

---

## 8. Hook Injection — Nơi emit event

| Vị trí | event_type | Khi nào |
|--------|------------|---------|
| `cio/ingestion` — sau Insert cio_event | `cio_event.inserted` | Khi ghi cio_events |
| `OnCioEventInserted` (init.registry) | (giữ EnqueueAnalysis) | Phase 2: thêm emit event song song |
| `fb_messages` insert hook | (Phase 2) | Khi có message mới |

**Phase 1:** Chỉ cần API `POST /ai-decision/events` — caller có thể emit thủ công. Hook chưa bắt buộc.

---

## 9. Debounce Worker (Phase 1 đơn giản)

**Collection:** `decision_debounce_state` (tạm)

```json
{
  "debounce_key": "org_001:conv_123:message_burst",
  "last_message_at": 1234567890,
  "last_event_id": "evt_xxx",
  "created_at": 1234567890
}
```

**Logic:** Worker chạy mỗi 5s, tìm các key có `last_message_at` + 30s < now → emit `message.batch_ready` với payload gom event_ids, xóa state.

**Critical pattern:** Nếu payload chứa keyword "huỷ đơn", "cancel" → flush ngay, không chờ 30s.

---

## 10. Constants

```go
// Event status
const (
    EventStatusPending        = "pending"
    EventStatusLeased         = "leased"
    EventStatusProcessing    = "processing"
    EventStatusCompleted     = "completed"
    EventStatusFailedRetryable = "failed_retryable"
    EventStatusFailedTerminal = "failed_terminal"
    EventStatusDeferred       = "deferred"
)

// Case status
const (
    CaseStatusOpened           = "opened"
    CaseStatusContextCollecting = "context_collecting"
    CaseStatusReadyForDecision = "ready_for_decision"
    CaseStatusDecided          = "decided"
    CaseStatusActionsCreated   = "actions_created"
    CaseStatusExecuting        = "executing"
    CaseStatusOutcomeWaiting   = "outcome_waiting"
    CaseStatusClosed           = "closed"
)

// Closure type
const (
    ClosureComplete = "closed_complete"
    ClosureTimeout  = "closed_timeout"
    ClosureManual   = "closed_manual"
)
```

---

## Changelog

- 2026-03-19: Tạo spec code-level
