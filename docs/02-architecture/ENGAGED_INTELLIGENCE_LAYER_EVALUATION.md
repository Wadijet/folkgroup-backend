# Đánh Giá: Engaged Intelligence Layer — Triển Khai Với Dữ Liệu Hiện Tại

> So sánh đề xuất Engaged Intelligence Layer với dữ liệu thực tế có trong hệ thống (fb_conversations, fb_message_items, panCakeData, crm_activity_history).

---

## I. Bảng Ánh Xạ: Đề Xuất vs Dữ Liệu Hiện Có

### 1️⃣ Intent Level (Mức độ ý định mua)

| Nhóm đề xuất | Tiêu chí | Dữ liệu hiện có | Khả năng triển khai |
|--------------|----------|-----------------|----------------------|
| **E1 – Low Intent** | Hỏi chung chung, không hỏi giá/sản phẩm | `panCakeData.last_message.text` hoặc `content` | ✅ **Có** — nhưng cần NLP/keyword |
| **E2 – Product Interest** | Hỏi sản phẩm cụ thể, size, màu | `last_message.text` + product links trong message (nếu có) | ⚠️ **Một phần** — cần parse `messageData` từ fb_message_items |
| **E3 – Price Intent** | Hỏi giá, ưu đãi, phí ship | Keyword: "giá", "bao nhiêu", "ship", "freeship"... | ✅ **Có** — keyword matching trên last_message |
| **E4 – Strong Buying Intent** | Hỏi thanh toán, giữ hàng, thời gian giao | Keyword: "thanh toán", "giữ", "đặt hàng", "giao"... | ✅ **Có** — keyword matching |

**Kết luận Intent:**
- **Có thể triển khai ngay (đơn giản):** E3, E4 bằng keyword trên `last_message.text` + `last_message.content`
- **Cần bổ sung:** E1, E2 — hoặc dùng rule: nếu không match E3/E4 → E1; nếu có tag "quan tâm" từ sale → E2
- **Cần NLP (pha 2):** Phân tích nội dung chat chi tiết hơn

---

### 2️⃣ Conversation Temperature (Nhiệt độ hiện tại)

| Nhóm đề xuất | Tiêu chí | Dữ liệu hiện có | Khả năng triển khai |
|--------------|----------|-----------------|----------------------|
| **Hot** | Chat trong 24h, sale nhắn cuối, khách đang phản hồi | `panCakeUpdatedAt`, `last_sent_by.email` (@facebook.com = khách) | ✅ **Có đủ** |
| **Warm** | 1–3 ngày, trao đổi qua lại | `panCakeUpdatedAt`, `panCakeData.inserted_at` | ✅ **Có đủ** |
| **Cooling** | 3–7 ngày không phản hồi | So sánh `now - panCakeUpdatedAt` | ✅ **Có đủ** |
| **Cold** | > 7 ngày không tương tác | Idem | ✅ **Có đủ** |

**Logic gợi ý:**
```
daysSinceLastInteraction = (now - panCakeUpdatedAt) / 86400
lastFromCustomer = last_sent_by.email contains "@facebook.com" → backlog

Hot:   daysSinceLastInteraction <= 1 && (backlog || lastFromCustomer trong 24h)
Warm:  1 < days <= 3
Cooling: 3 < days <= 7
Cold:   days > 7
```

**Kết luận Temperature:** ✅ **Triển khai ngay được** — chỉ dùng `panCakeUpdatedAt`, `last_sent_by`.

---

### 3️⃣ Engagement Depth (Độ sâu tương tác)

| Nhóm đề xuất | Tiêu chí | Dữ liệu hiện có | Khả năng triển khai |
|--------------|----------|-----------------|----------------------|
| **Light** | 1–3 tin | `panCakeData.message_count` | ✅ **Có** — conversation_metrics đã dùng |
| **Medium** | 4–10 tin | Idem | ✅ **Có** |
| **Deep** | > 10 tin | Idem | ✅ **Có** |

**Lưu ý:** `message_count` là tổng tin trong hội thoại (cả 2 chiều). Nếu cần đếm riêng tin từ khách → phải aggregate từ `fb_message_items` (đã có logic `IsFromCust` trong loadResponseTimes).

**Kết luận Depth:** ✅ **Triển khai ngay** — dùng `panCakeData.message_count`. Có thể bổ sung `customerMessageCount` từ fb_message_items nếu cần chính xác hơn.

---

### 4️⃣ Source Quality (Chất lượng nguồn)

| Nhóm đề xuất | Tiêu chí | Dữ liệu hiện có | Khả năng triển khai |
|--------------|----------|-----------------|----------------------|
| **Organic inbox** | Không từ ads | `panCakeData.ad_ids` rỗng | ✅ **Có** — conversation_metrics đã dùng `hasAdIds` |
| **Ads campaign** | Có ad_ids | `panCakeData.ad_ids` length > 0 | ✅ **Có** |
| **Retargeting** | — | Không có field riêng | ⚠️ **Chưa** — cần campaign/ad metadata |
| **Old customer** | Khách cũ quay lại | Cần join crm_customers (có đơn) | ✅ **Có** — filter engaged chưa có đơn = mới |

**Phân loại đơn giản:**
- `fromAds = len(panCakeData.ad_ids) > 0`
- `organic = !fromAds`
- Retargeting: chưa có data; có thể map ad_ids với campaign sau này.

**Kết luận Source:** ✅ **Organic vs Ads** — triển khai ngay. Retargeting cần bổ sung.

---

## II. Engaged Score (0–100)

| Thành phần | Trọng số đề xuất | Nguồn dữ liệu | Ghi chú |
|------------|------------------|---------------|---------|
| IntentScore (0–40) | 40% | last_message.text, tags | Keyword rules; tag "hot"/"quan tâm" từ sale |
| TemperatureScore (0–20) | 20% | panCakeUpdatedAt, last_sent_by | Hot=20, Warm=15, Cooling=8, Cold=0 |
| DepthScore (0–20) | 20% | message_count | Deep=20, Medium=12, Light=5 |
| SourceScore (0–20) | 20% | ad_ids | Ads=15, Organic=20 (hoặc tùy chiến lược) |

**Có thể triển khai:** ✅ Công thức trên dùng toàn bộ dữ liệu có sẵn.

---

## III. Các Chỉ Số CEO

| Chỉ số đề xuất | Nguồn dữ liệu | Khả năng |
|----------------|---------------|----------|
| **Engaged Volume** | fb_conversations where journey=engaged (chưa có đơn) | ✅ Filter crm_customers journeyStage=engaged + chưa link đơn |
| **Engaged Conversion Rate** | engaged → first purchase | ✅ Join conversation.customerId với pc_pos_orders (đã có loadConvertedCustomers) |
| **Time to Conversion** | First conversation → first order | ⚠️ Cần `firstConversationAt` từ conv + order insertedAt |
| **Engaged Aging** | Engaged kẹt > 1d, > 3d, > 7d | ✅ panCakeUpdatedAt |
| **Sale Performance** | Assigned engaged, converted, conversion rate | ✅ current_assign_users, loadConvertedCustomers |

**Ghi chú Time to Conversion:**
- `firstConversationAt`: lấy từ `panCakeData.inserted_at` hoặc aggregate min(updatedAt) — conversation_metrics đã có
- First order: `pc_pos_orders` min(insertedAt) where customerId match
- Cần join conversation → customerId → order (crm_customers.unifiedId hoặc fb_conversations.customerId)

---

## IV. Dữ Liệu Thiếu / Cần Bổ Sung

| Mục | Cần gì | Cách bổ sung |
|-----|--------|--------------|
| **Nội dung tin nhắn đầy đủ** | Full chat để NLP | Đã có trong fb_message_items — cần aggregate theo conversationId |
| **Product items gửi trong chat** | Attachments, product links | Cần kiểm tra structure messageData từ Pancake API |
| **Intent từ keyword** | Rules E1–E4 | Implement keyword matching trên last_message |
| **ad_ids → campaign name** | Phân loại Retargeting | Cần API/table ads metadata (phase 2) |
| **Lọc Engaged thuần** | Chỉ khách chưa có đơn | Filter: có conversation, không có order (qua crm_customers journeyStage) |

---

## V. Roadmap Triển Khai Đề Xuất

### Phase 1 — Có thể làm ngay (dùng 100% data hiện có)

1. **Conversation Temperature** (Hot/Warm/Cooling/Cold) — từ panCakeUpdatedAt, last_sent_by
2. **Engagement Depth** (Light/Medium/Deep) — từ message_count
3. **Source** (Organic/Ads) — từ ad_ids
4. **Care Priority** — backlog + waitingMinutes + (Hot/Warm ưu tiên)
5. **Engaged Aging** — buckets > 1d, > 3d, > 7d
6. **Engaged Volume** — Count conversation của khách journey=engaged (chưa đơn)
7. **Engaged Conversion Rate** — Mở rộng logic loadConvertedCustomers cho engaged

### Phase 2 — Cần ít bổ sung

8. **Intent Level (đơn giản)** — Keyword matching trên last_message.text (giá, ship, đặt, thanh toán...)
9. **Engaged Score** — Kết hợp Temperature + Depth + Source + Intent (keyword)
10. **Time to Conversion** — firstConversationAt (panCakeData.inserted_at) → first order
11. **Sale Performance cho Engaged** — Tách metric theo engaged vs all

### Phase 3 — Cần mở rộng data / NLP

12. **Intent chi tiết** — NLP phân tích nội dung chat
13. **Product interest** — Parse product links/attachments trong messages
14. **Source Retargeting** — Join ad_ids với campaign metadata
15. **Auto-detect** — Price objection, gift buyer, VIP prospect

---

## VI. Kết Luận

| Hạng mục | Đánh giá |
|----------|----------|
| **Temperature, Depth, Source** | ✅ Triển khai được ngay |
| **Intent (keyword-based)** | ✅ Phase 2 |
| **Engaged Score** | ✅ Phase 2 |
| **CEO metrics (Volume, Conversion, Aging)** | ✅ Phase 1–2 |
| **Intent (NLP)** | ⏳ Phase 3 |

**Khuyến nghị:** Bắt đầu với Phase 1 — Temperature, Depth, Source, Care Priority, Engaged Aging. Đây là nền tảng ổn để phân bổ nguồn lực và tạo Engaged Intelligence Panel. Sau đó bổ sung Intent (keyword) và Engaged Score ở Phase 2.

---

## VII. Phase 1 Đã Triển Khai (2025-02)

| Thành phần | File | Mô tả |
|------------|------|-------|
| Temperature | service.report.inbox.go | `computeTemperature` — hot/warm/cooling/cold theo daysSinceLast |
| Engagement Depth | service.report.inbox.go | `computeEngagementDepth` — light/medium/deep theo message_count |
| Source | service.report.inbox.go | `computeSourceType` — organic/ads theo ad_ids |
| Care Priority | service.report.inbox.go | `computeCarePriority` — P0..P4 theo backlog, unassigned, waiting |
| Engaged Volume & Aging | service.report.inbox.go | `computeEngagedStats`, `loadCustomersWithOrders` |
| InboxConversationItem | dto.report.inbox.go | Thêm engaged: { temperature, engagementDepth, sourceType, carePriority }, isEngaged, customerId |
| InboxSummary | dto.report.inbox.go | Thêm engagedCount, engagedAging1d, engagedAging3d, engagedAging7d |
| Filter | service.report.inbox.go | filter=engaged, query engaged=true |
| Sort | service.report.inbox.go | sort=care_priority |
