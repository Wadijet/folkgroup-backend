# Phương Án: Trích Xuất Intent Từ Nội Dung Tin Nhắn (CRM)

**Ngày:** 2025-03-16  
**Tham chiếu:** [THIET_KE_MODULE_CIO.md](./THIET_KE_MODULE_CIO.md) §12, Phần 3 CIO (docs-shared), [CIO_MODULE_RANG_SOAT.md](./CIO_MODULE_RANG_SOAT.md)

---

## 1. Tổng Quan

### 1.1 Vấn đề

- Nội dung tin nhắn (fb_message_items) chưa được xử lý bởi AI.
- Không có bước đọc intent (complaint, question, order_inquiry, …) và sentiment.
- RULE_CIO_ROUTING_MODE (AI vs Human) cần `[Intent: Complaint] → Human` nhưng chưa có nguồn intent.

### 1.2 Theo Vision

| Nội dung | Phân công (THIET_KE §12.1) |
|----------|----------------------------|
| engagement signals, intent signals, objection signals (Lớp 5) | **CI** (hoặc AI/NLP service) extract từ raw. **CIO** chỉ chuyển raw. |
| [Intent: Complaint] → Human (Lớp 3) | **AI Agent** (hoặc NLP) detect, trả signal. **CIO** nhận signal và routing. |

**Kết luận:** Đặt logic extract intent trong **Customer (CRM)** — CI extract, CIO đọc.

---

## 2. Phạm Vi

| Mục đích | Mô tả |
|----------|-------|
| **Lớp 5 Feedback** | Làm giàu Profile — intent signals vào crm_activity_history, phục vụ aggregate/snapshot. |
| **Lớp 3 Routing** | CIO đọc intent khi routing inbound (AI vs Human) — RULE_CIO_ROUTING_MODE. |
| **Dashboard / Report** | Intent distribution, complaint rate — có thể mở rộng sau. |

---

## 3. Kiến Trúc

### 3.1 Luồng Tổng Quan

```
[Sync / Webhook] 
    → fb_message_items (nội dung tin nhắn)
    → cio_events (raw, OnMessageUpsert)
    → IngestConversationTouchpoint (conversation_started)

[Worker crm_intent]
    → Đọc message mới (từ cio_events hoặc fb_message_items)
    → Load nội dung từ fb_message_items
    → Gọi AI DetectIntent(text)
    → LogActivity(message_received) với metadata.intent

[CIO Routing - Inbound]
    → Cần intent: GetLastIntentForConversation(conversationId)
    → CRM trả intent từ activity message_received mới nhất
    → RULE_CIO_ROUTING_MODE đọc intent → quyết định AI vs Human
```

### 3.2 Nguyên Tắc

| Nguyên tắc | Chi tiết |
|------------|----------|
| **CI extract, CIO đọc** | CRM chạy AI, lưu signal. CIO không gọi AI. |
| **Async worker** | Không block ingestion. Retry khi AI fail. |
| **Activity-based** | Dùng crm_activity_history (message_received) — tái sử dụng framework có sẵn. |
| **Contract rõ** | Intent schema cố định để CIO và Rule Engine đọc. |

---

## 4. Schema Intent

### 4.1 Intent Result (output từ AI)

```json
{
  "intent": "complaint | question | order_inquiry | greeting | product_info | other",
  "sentiment": "negative | neutral | positive",
  "confidence": 0.92,
  "labels": ["urgent", "refund_request"],
  "summary": "Khách phàn nàn giao hàng trễ, yêu cầu hoàn tiền"
}
```

### 4.2 Lưu trong crm_activity_history

- **ActivityType:** `message_received`
- **Source:** `fb`
- **SourceRef:** `{ "conversationId": "...", "messageId": "..." }`
- **Metadata:**
  - `intent`: string
  - `sentiment`: string
  - `confidence`: float
  - `labels`: []string (optional)
  - `summary`: string (optional)
  - `textPreview`: string (50 ký tự đầu, optional)

### 4.3 ActivityTypeToDomain

Đã có `message_received` → `ActivityDomainConversation`. Không cần thay đổi.

---

## 5. Chi Tiết Triển Khai

### 5.1 Service: CrmIntentService

**File:** `api/internal/api/crm/service/service.crm.intent.go`

| Method | Mô tả |
|--------|-------|
| `DetectIntent(ctx, text string) (*IntentResult, error)` | Gọi AI (OpenAI/LLM) để phân tích. Trả intent, sentiment, confidence. |
| `ExtractTextFromMessageItem(item *FbMessageItem) string` | Lấy text từ MessageData (message, text, … theo schema Pancake). |

**Lưu ý:** Cần config API key (env), model (gpt-4o-mini hoặc tương đương), prompt chuẩn.

### 5.2 Worker: crm_intent_worker

**File:** `api/internal/worker/crm_intent_worker.go`

| Bước | Chi tiết |
|------|----------|
| 1. Trigger | Cron mỗi 30s–1 phút, hoặc event-driven từ cio_events. |
| 2. Nguồn | Query cio_events (eventType=message_updated, createdAt > lastRun) chưa có activity message_received tương ứng. Hoặc query fb_message_items mới (có ownerOrganizationId, chưa có activity). |
| 3. Resolve | Với mỗi event: conversationId, customerId → ResolveUnifiedId. Bỏ qua nếu không resolve được. |
| 4. Load content | FbMessageItemService.FindByConversationId(conversationId, page=1, limit=1) — message mới nhất. Lọc message từ customer (from != page). |
| 5. Extract text | ExtractTextFromMessageItem. Bỏ qua nếu text rỗng. |
| 6. Detect | CrmIntentService.DetectIntent(ctx, text). |
| 7. Log | CrmActivityService.LogActivity(message_received, sourceRef: conversationId+messageId, metadata.intent). |

**Idempotent:** SourceRef (conversationId, messageId) → upsert, không tạo duplicate.

### 5.3 API cho CIO: GetLastIntentForConversation

**File:** `api/internal/api/crm/service/service.crm.intent.go` (hoặc service.crm.activity.go)

```go
// GetLastIntentForConversation trả intent của message mới nhất từ khách trong conversation.
// Dùng cho CIO routing (RULE_CIO_ROUTING_MODE).
func (s *CrmActivityService) GetLastIntentForConversation(ctx context.Context, conversationId string, ownerOrgID primitive.ObjectID) (*IntentSignal, error)
```

- Query: `activityType=message_received`, `sourceRef.conversationId=conversationId`, sort by activityAt desc, limit 1.
- Trả: `IntentSignal{ Intent, Sentiment, Confidence }` hoặc nil nếu chưa có.

### 5.4 RULE_CIO_ROUTING_MODE

**Seed mới** trong `seed_rule_cio_system.go`:

- **Input:** cio_context mở rộng: `lastIntent`, `lastSentiment`, `lastIntentConfidence`.
- **Logic:** Nếu `lastIntent === 'complaint'` hoặc `lastSentiment === 'negative'` → `routingMode: 'human'`. VIP → human. Default → AI.
- **Output:** `routing_mode` (ai | human).

**Contract CIO → CRM:** Khi inbound routing, CioRoutingService (hoặc nơi gọi) cần gọi GetLastIntentForConversation trước khi build cio_context cho Rule Engine.

---

## 6. Cấu Trúc File

| File | Vai trò |
|------|---------|
| `api/internal/api/crm/service/service.crm.intent.go` | DetectIntent, ExtractText, GetLastIntentForConversation |
| `api/internal/worker/crm_intent_worker.go` | Worker enrich intent |
| `api/internal/worker/config.go` | Thêm CrmIntentWorker (enable, interval) |
| `api/internal/worker/schedule.go` | Đăng ký worker |
| `ruleintel/migration/seed_rule_cio_system.go` | Seed RULE_CIO_ROUTING_MODE, OUT_CIO_ROUTING_MODE |
| `api/internal/api/cio/service/service.cio.routing.go` | Mở rộng ChooseChannel hoặc thêm ChooseRoutingMode(inbound) — gọi GetLastIntentForConversation |

---

## 7. Extract Text Từ MessageData

Schema Pancake/Facebook Messenger có thể có:

- `message` (text)
- `text`
- `attachments[].payload` (image, file — bỏ qua hoặc extract caption)

**Chiến lược:** Thử lần lượt `messageData["message"]`, `messageData["text"]`, fallback rỗng. Có thể mở rộng cho attachment type sau.

---

## 8. Phase Triển Khai

### Phase 1: Foundation (1–2 tuần)

| # | Công việc | Trạng thái |
|---|-----------|------------|
| 1 | service.crm.intent.go — DetectIntent (mock hoặc OpenAI) | |
| 2 | ExtractTextFromMessageItem | |
| 3 | GetLastIntentForConversation | |
| 4 | Unit test DetectIntent, ExtractText | |

### Phase 2: Worker (1 tuần)

| # | Công việc | Trạng thái |
|---|-----------|------------|
| 1 | crm_intent_worker.go | |
| 2 | Đăng ký worker, config | |
| 3 | LogActivity message_received với metadata.intent | |
| 4 | Test end-to-end: sync message → worker → activity có intent | |

### Phase 3: CIO Integration (1 tuần)

| # | Công việc | Trạng thái |
|---|-----------|------------|
| 1 | Seed RULE_CIO_ROUTING_MODE, OUT_CIO_ROUTING_MODE | |
| 2 | CioRoutingService — thêm ChooseRoutingMode hoặc mở rộng context | |
| 3 | Gọi GetLastIntentForConversation khi inbound | |
| 4 | Test routing: complaint → human | |

### Phase 4: Production (optional)

| # | Công việc | Trạng thái |
|---|-----------|------------|
| 1 | OpenAI API key, model config | |
| 2 | Prompt engineering (Tiếng Việt) | |
| 3 | Rate limit, retry, circuit breaker | |
| 4 | Monitoring, alert khi AI fail | |

---

## 9. Rủi Ro & Giảm Thiểu

| Rủi ro | Giảm thiểu |
|--------|------------|
| AI chậm / timeout | Worker async, retry. Timeout 10s. |
| API key lộ | Env var, không hardcode. |
| Cost | Dùng model rẻ (gpt-4o-mini), batch nếu có thể. |
| Message không có text | Bỏ qua, không log activity. |
| Duplicate | SourceRef (conversationId, messageId) → upsert. |

---

## 10. Tóm Tắt

- **Module:** CRM (Customer Intelligence)
- **Chức năng:** Extract intent từ nội dung tin nhắn qua AI, lưu vào crm_activity_history (message_received).
- **CIO:** Đọc intent qua GetLastIntentForConversation khi routing inbound.
- **Vision:** Khớp — CI extract, CIO nhận signal và routing.
