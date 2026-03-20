# Thiết Kế Collections CIX — Kiến Trúc 3 Lớp

**Ngày:** 2026-03-18  
**Tham chiếu:** [PHUONG_AN_TRIEN_KHAI_CIX.md](./PHUONG_AN_TRIEN_KHAI_CIX.md), [THIET_KE_MODULE_CIO.md](./THIET_KE_MODULE_CIO.md), [identity-links-model](../../docs-shared/architecture/data-contract/identity-links-model.md)

---

## 1. Nguyên Tắc Chốt

| Câu hỏi | Trả lời |
|---------|---------|
| Có collection riêng cho CIX? | **Có** |
| Dùng để merge nhiều nguồn? | **Có** |
| Thay thế collection conversation raw từ Pancake? | **Không** |
| Overwrite intelligence ngược vào raw collection? | **Hạn chế tối đa** — chỉ cache field nhẹ nếu cần search, canonical ở CIX |

**Rule phân tách vai trò:**

| Module | Trả lời câu hỏi |
|--------|-----------------|
| **CIO** | "Đã xảy ra gì?" — ledger, log, event |
| **CIX** | "Cuộc hội thoại này đang có ý nghĩa gì?" — phân tích có ngữ cảnh |
| **Customer Intelligence** | "Khách này là ai, đang ở trạng thái nào?" |
| **Decision Engine** | "Nên làm gì tiếp?" |

---

## 2. Kiến Trúc 3 Lớp

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│ Lớp 1: CIO — Raw Ledger (Source of Truth)                                         │
│ interaction_events, conversations (fb_conversations, zalo_conversations, ...)     │
│ • Đồng bộ từ Pancake, Zalo, webhook                                               │
│ • Message, thread, session, delivery, actor, channel metadata                     │
│ • KHÔNG nhồi NLP/context/business meaning                                         │
└─────────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        │ CIX đọc raw
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ Lớp 2: CIX — Derived / Merged Context                                            │
│ cix_conversations (hoặc conversation_contexts)                                   │
│ • Merged context từ nhiều nguồn                                                  │
│ • Kết quả pipeline Raw → L1 → L2 → L3 → Flag → Action                            │
│ • Phiên bản phân tích, trace rule/LLM                                             │
└─────────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        │ Optional: audit sâu
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ Lớp 3: CIX — Analysis Runs (Tùy chọn)                                            │
│ conversation_analysis_runs                                                        │
│ • Từng lần phân tích / rerun                                                      │
│ • A/B logic, replay, audit                                                        │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Lý Do Tách Riêng

| Vấn đề nếu nhét hết vào raw | Giải pháp với collection CIX riêng |
|-----------------------------|--------------------------------------|
| **Lẫn boundary** — CIO thành intelligence layer | CIO giữ raw; CIX giữ derived |
| **Khó merge đa nguồn** — cùng khách chat Pancake + Zalo + webchat | CIX merge theo conversation_uid/session_uid |
| **Khó versioning** — re-run model/rule cho kết quả khác | CIX lưu analysis_version, rule_version, model_version |
| **Khó trace** — "flag sinh từ rule nào, context nào?" | CIX lưu trace_id, triggeredByRule, sourceRef |
| **Khó scale** — raw và derived có vòng đời, index, query pattern khác | Tách collection, tách index |

---

## 4. Nguyên Tắc Collection CIX

- **Derived, rebuildable** — có thể rebuild từ CIO + Customer Intelligence + Rule Engine
- **Idempotent** — re-run cùng input → cùng output (hoặc version mới)
- **Không phải source of truth cho raw** — raw mất thì CIX không cứu raw
- **Link ngược** — links về raw conversation, external IDs, customer UID

---

## 5. Schema `cix_conversations`

Collection: `cix_conversations` (hoặc `conversation_contexts`)

```json
{
  "_id": ObjectId,
  "uid": "ctx_507f1f77bcf86cd799439011",
  "ownerOrganizationId": ObjectId,

  "links": {
    "customer": {
      "uid": "cust_xxx",
      "externalRefs": [{ "source": "facebook", "id": "psid123" }],
      "status": "resolved"
    },
    "session": {
      "uid": "sess_xxx",
      "status": "resolved"
    },
    "conversation": {
      "uid": "conv_xxx",
      "externalRefs": [{ "source": "fb", "id": "conv_123" }],
      "status": "resolved"
    }
  },

  "linkedSources": [
    { "source": "fb", "sourceId": "conv_123", "channel": "messenger" },
    { "source": "zalo", "sourceId": "zalo_conv_456", "channel": "zalo" }
  ],

  "participants": [
    { "role": "customer", "uid": "cust_xxx", "channelId": "psid123" },
    { "role": "agent", "uid": "user_xxx" }
  ],

  "mergedTranscript": {
    "windowStart": 1710000000000,
    "windowEnd": 1710003600000,
    "turns": [
      { "from": "customer", "content": "...", "timestamp": 1710000100000, "channel": "messenger" },
      { "from": "agent", "content": "...", "timestamp": 1710000200000, "channel": "messenger" }
    ]
  },

  "customerSnapshotAtAnalysis": {
    "valueTier": "top",
    "lifecycleStage": "active",
    "journeyStage": "repeat",
    "flags": ["vip_at_risk"]
  },

  "layer1": { "stage": "negotiating" },
  "layer2": {
    "intentStage": "high",
    "urgencyLevel": "high",
    "riskLevelRaw": "warning",
    "riskLevelAdj": "danger",
    "adjustmentRule": "ADJUST_RISK_VIP_v1"
  },
  "layer3": {
    "buyingIntent": "ready_to_buy",
    "objectionLevel": "soft_objection",
    "sentiment": "negative"
  },

  "flags": [
    { "name": "vip_at_risk", "severity": "critical", "triggeredByRule": "FLAG_VIP_RISK_v2" }
  ],

  "actionSuggestions": ["escalate_to_senior"],

  "trace": {
    "traceId": "trace_abc123",
    "correlationId": "corr_xyz789",
    "analysisVersion": "4.1",
    "ruleVersion": "v2",
    "modelVersion": "gpt-4-2024"
  },

  "createdAt": 1710003600000,
  "updatedAt": 1710003600000
}
```

---

## 6. Chi Tiết Schema (Go Struct)

```go
// CixConversation document trong collection cix_conversations.
// Derived, rebuildable. Không phải source of truth cho raw.
type CixConversation struct {
    ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
    Uid                 string             `json:"uid" bson:"uid" index:"single:1"`
    OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`

    // Links — ngược về raw, customer, session
    Links map[string]identity.LinkItem `json:"links,omitempty" bson:"links,omitempty"`

    // LinkedSources — các nguồn đã merge (fb, zalo, webchat)
    LinkedSources []CixLinkedSource `json:"linkedSources" bson:"linkedSources"`

    // Participants — normalized
    Participants []CixParticipant `json:"participants" bson:"participants"`

    // MergedTranscript — window + turns (snapshot tại thời điểm phân tích)
    MergedTranscript CixMergedTranscript `json:"mergedTranscript" bson:"mergedTranscript"`

    // CustomerSnapshotAtAnalysis — profile tại thời điểm chạy pipeline
    CustomerSnapshotAtAnalysis map[string]interface{} `json:"customerSnapshotAtAnalysis" bson:"customerSnapshotAtAnalysis"`

    // Pipeline output
    Layer1            CixLayer1   `json:"layer1" bson:"layer1"`
    Layer2            CixLayer2   `json:"layer2" bson:"layer2"`
    Layer3            CixLayer3   `json:"layer3" bson:"layer3"`
    Flags             []CixFlag   `json:"flags" bson:"flags"`
    ActionSuggestions []string    `json:"actionSuggestions" bson:"actionSuggestions"`

    // Trace — audit, replay
    Trace CixTrace `json:"trace" bson:"trace"`

    CreatedAt int64 `json:"createdAt" bson:"createdAt" index:"single:-1"`
    UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`
}

type CixLinkedSource struct {
    Source  string `json:"source" bson:"source"`   // fb | zalo | webchat
    SourceID string `json:"sourceId" bson:"sourceId"`
    Channel string `json:"channel" bson:"channel"` // messenger | zalo | website_chat
}

type CixParticipant struct {
    Role     string `json:"role" bson:"role"`         // customer | agent
    Uid      string `json:"uid" bson:"uid"`           // cust_xxx | user_xxx
    ChannelID string `json:"channelId,omitempty" bson:"channelId,omitempty"` // psid, zalo userId
}

type CixMergedTranscript struct {
    WindowStart int64           `json:"windowStart" bson:"windowStart"`
    WindowEnd   int64           `json:"windowEnd" bson:"windowEnd"`
    Turns       []CixTranscriptTurn `json:"turns" bson:"turns"`
}

type CixTranscriptTurn struct {
    From      string `json:"from" bson:"from"`           // customer | agent
    Content   string `json:"content" bson:"content"`
    Timestamp int64  `json:"timestamp" bson:"timestamp"`
    Channel   string `json:"channel" bson:"channel"`
}

type CixTrace struct {
    TraceID         string `json:"traceId" bson:"traceId"`
    CorrelationID   string `json:"correlationId" bson:"correlationId"`
    AnalysisVersion string `json:"analysisVersion" bson:"analysisVersion"`
    RuleVersion     string `json:"ruleVersion" bson:"ruleVersion"`
    ModelVersion    string `json:"modelVersion" bson:"modelVersion"`
}
```

---

## 7. Schema `conversation_analysis_runs` (Tùy chọn)

Dùng khi cần audit sâu, A/B logic, replay.

```json
{
  "_id": ObjectId,
  "uid": "run_507f1f77bcf86cd799439012",
  "ownerOrganizationId": ObjectId,
  "cixConversationUid": "ctx_xxx",
  "sessionUid": "sess_xxx",
  "triggeredBy": "worker" | "api" | "rerun",
  "inputSnapshot": { "conversationWindow": {...}, "customerContext": {...} },
  "outputSnapshot": { "layer1": {...}, "layer2": {...}, "flags": [...] },
  "traceId": "trace_abc123",
  "ruleVersion": "v2",
  "modelVersion": "gpt-4-2024",
  "experimentId": "exp_cix_layer3_ab",
  "variant": "A",
  "createdAt": 1710003600000
}
```

---

## 8. Schema `cix_pending_analysis` (Hàng đợi — Option B)

Collection: `cix_pending_analysis` — queue cho worker CIX. CIO event → enqueue → worker poll.

```json
{
  "_id": ObjectId,
  "conversationId": "conv_123",
  "customerId": "psid_456",
  "channel": "messenger",
  "ownerOrganizationId": ObjectId,
  "cioEventUid": "evt_xxx",
  "cioEventId": ObjectId,
  "eventType": "conversation_updated",
  "eventAt": 1710003600000,
  "processedAt": null,
  "processError": "",
  "retryCount": 0,
  "createdAt": 1710003600000
}
```

**Dedupe:** Cùng `conversationId` + `ownerOrganizationId` → upsert (chỉ giữ job mới nhất).

**Index:** `{ ownerOrganizationId: 1, processedAt: 1, createdAt: 1 }` — worker query `processedAt: null` sort `createdAt`.

---

## 9. Quan Hệ Với Collection Hiện Có

| Collection hiện tại | Vai trò | Quan hệ với CIX |
|---------------------|---------|-----------------|
| `cix_analysis_results` | Kết quả phân tích (đơn giản) | Có thể **mở rộng** thành `cix_conversations` hoặc **map 1:1** — cix_conversations = phiên bản đầy đủ |
| `fb_conversations` | Raw Messenger | CIO source of truth; CIX đọc qua links |
| `cio_events` | Event stream | CIX trigger từ message_in |
| `cio_sessions` | Session state | CIX link sessionUid |

**Đề xuất migration:** Giữ `cix_analysis_results` cho Phase 1–3 (đơn giản). Khi cần merge đa nguồn, versioning, trace đầy đủ → thêm `cix_conversations` và có thể deprecate/alias.

---

## 10. Index Đề Xuất

```javascript
// cix_conversations
{ ownerOrganizationId: 1, "links.customer.uid": 1 }
{ ownerOrganizationId: 1, "links.session.uid": 1 }
{ ownerOrganizationId: 1, createdAt: -1 }
{ "trace.traceId": 1 }
{ "linkedSources.source": 1, "linkedSources.sourceId": 1 }
```

```javascript
// cix_pending_analysis
{ ownerOrganizationId: 1, processedAt: 1, createdAt: 1 }
{ conversationId: 1, ownerOrganizationId: 1 }  // unique cho dedupe upsert
```

---

## Changelog

- 2026-03-18: Tạo tài liệu thiết kế collections CIX theo kiến trúc 3 lớp
- 2026-03-18: Thêm §8 schema cix_pending_analysis (hàng đợi Option B)
