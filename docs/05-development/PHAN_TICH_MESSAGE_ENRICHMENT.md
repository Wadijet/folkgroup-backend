# Phân Tích Chiến Lược: Message Enrichment — Trích Xuất Giá Trị Từ Nội Dung Tin Nhắn

**Ngày:** 2025-03-16  
**Loại:** Phân tích & Tham mưu  
**Tham chiếu:** [PHUONG_AN_CRM_INTENT_EXTRACTION.md](./PHUONG_AN_CRM_INTENT_EXTRACTION.md), [THIET_KE_MODULE_CIO.md](./THIET_KE_MODULE_CIO.md)

---

## 1. Tổng Quan Vấn Đề

### 1.1 Bối cảnh

- Nội dung tin nhắn (fb_message_items) là nguồn dữ liệu chưa khai thác.
- Vision CIO: `[Intent: Complaint] → Human` — cần signal từ message.
- Vision CI: extract `engagement signals`, `intent signals`, `objection signals` từ raw.
- Một lần gọi AI có thể trả nhiều loại output — cần quyết định scope tối ưu.

### 1.2 Câu hỏi chiến lược

1. **Scope:** Nên extract những gì? Intent only hay mở rộng?
2. **Lưu trữ:** Top-level customer hay currentMetrics.raw?
3. **Thứ tự triển khai:** Phase nào trước, phase nào sau?
4. **ROI vs Cost:** Giá trị thực tế so với chi phí token?

---

## 2. Khung Phân Tích

### 2.1 Tiêu chí đánh giá mỗi extension

| Tiêu chí | Trọng số | Mô tả |
|----------|----------|-------|
| **Business value** | Cao | Tác động trực tiếp đến routing, conversion, retention |
| **Consumption readiness** | Cao | Đã có consumer (CIO, Rule Engine, Report) sẵn sàng dùng |
| **Token efficiency** | Trung bình | Thêm field có tăng token đáng kể không |
| **Implementation complexity** | Trung bình | Độ phức tạp tích hợp, merge logic |
| **Accuracy risk** | Cao | AI extract sai → ảnh hưởng routing/profile |

### 2.2 Ma trận quyết định

```
High Value + Low Risk + Ready Consumer → Làm ngay (Phase 1)
High Value + Medium Risk → Làm có điều kiện (Phase 2)
Medium Value + High Complexity → Làm sau (Phase 3+)
Low Value hoặc No Consumer → Không làm / Defer
```

---

## 3. Phân Tích Từng Extension

### 3.1 Intent + Sentiment (đã có trong phương án)

| Tiêu chí | Đánh giá |
|----------|----------|
| **Business value** | Rất cao — routing AI vs Human, handoff complaint |
| **Consumer** | RULE_CIO_ROUTING_MODE (chưa seed nhưng design có) |
| **Token** | Thấp — 2 field enum |
| **Complexity** | Thấp — schema đơn giản |
| **Accuracy risk** | Trung bình — sentiment đôi khi sai, có thể dùng confidence |

**Kết luận:** **Bắt buộc Phase 1.** Đây là nền tảng.

---

### 3.2 Profile Extracts (phone, email, preference, objection)

| Tiêu chí | Đánh giá |
|----------|----------|
| **Business value** | Cao — làm giàu profile, giảm form thu thập |
| **Consumer** | MergeProfile, currentMetrics.raw, Rule Engine |
| **Token** | Trung bình — prompt dài hơn, output có thể rỗng |
| **Complexity** | Trung bình — cần MergeProfileFromMessageExtracts, validation PII |
| **Accuracy risk** | Cao — phone/email sai → merge nhầm, cần confidence threshold |

**Rủi ro cụ thể:**
- Phone extract sai (vd: "gọi 0912" → parse thành SĐT) → merge vào profile → sai.
- Cần: chỉ merge khi confidence > 0.95, validate format (regex phone VN).

**Kết luận:** **Phase 2 — có điều kiện.** Chỉ merge PII khi confidence cao + validate. Preference/objection ít rủi ro hơn → có thể Phase 1.5.

---

### 3.3 Urgency

| Tiêu chí | Đánh giá |
|----------|----------|
| **Business value** | Trung bình — SLA, thứ tự xử lý |
| **Consumer** | Chưa có — cần xây queue priority, SLA logic |
| **Token** | Thấp |
| **Complexity** | Trung bình — cần tích hợp vào inbound queue (chưa có) |
| **Accuracy risk** | Trung bình |

**Kết luận:** **Phase 3.** Chờ có inbound queue/SLA system.

---

### 3.4 Complaint Type (delivery, quality, refund)

| Tiêu chí | Đánh giá |
|----------|----------|
| **Business value** | Cao — routing đúng team (support vs sales) |
| **Consumer** | RULE_CIO_ROUTING_MODE có thể mở rộng, hoặc rule mới |
| **Token** | Thấp |
| **Complexity** | Thấp |
| **Accuracy risk** | Trung bình |

**Kết luận:** **Phase 2.** Bổ sung intent chi tiết, ít tốn token.

---

### 3.5 Escalation Signal

| Tiêu chí | Đánh giá |
|----------|----------|
| **Business value** | Cao — "gặp quản lý", "tổng đài" → handoff ngay |
| **Consumer** | RULE_CIO_ROUTING_MODE |
| **Token** | Rất thấp — boolean |
| **Complexity** | Thấp |
| **Accuracy risk** | Thấp — keyword rõ |

**Kết luận:** **Phase 1.5 hoặc Phase 2.** Dễ, giá trị cao.

---

### 3.6 Product Mentions / SKU Interest

| Tiêu chí | Đánh giá |
|----------|----------|
| **Business value** | Trung bình — recommendation, upsell |
| **Consumer** | Chưa có — cần recommendation engine đọc raw |
| **Token** | Cao — cần map text → product_display_id |
| **Complexity** | Cao — fuzzy match sản phẩm, catalog |
| **Accuracy risk** | Cao — "áo thun" có thể map sai SKU |

**Kết luận:** **Phase 4+.** Phụ thuộc catalog, matching logic.

---

### 3.7 Suggested Action (call_back, send_info, escalate)

| Tiêu chí | Đánh giá |
|----------|----------|
| **Business value** | Trung bình — gợi ý cho Sales |
| **Consumer** | Chưa có — cần UI Sales hiển thị |
| **Token** | Trung bình |
| **Complexity** | Trung bình |
| **Accuracy risk** | Trung bình |

**Kết luận:** **Phase 3.** Chờ Sales UI/assist mode.

---

### 3.8 Channel / Time Preference

| Tiêu chí | Đánh giá |
|----------|----------|
| **Business value** | Trung bình — preferred_channel |
| **Consumer** | RULE_CIO_CHANNEL_CHOICE (đã có) |
| **Token** | Thấp |
| **Complexity** | Thấp |
| **Accuracy risk** | Trung bình |

**Kết luận:** **Phase 2 hoặc 3.** Bổ sung raw cho channel choice.

---

### 3.9 Topic / FAQ Category

| Tiêu chí | Đánh giá |
|----------|----------|
| **Business value** | Cao — auto-reply FAQ |
| **Consumer** | Chưa có — cần FAQ/auto-reply engine |
| **Token** | Trung bình |
| **Complexity** | Cao — taxonomy FAQ |
| **Accuracy risk** | Trung bình |

**Kết luận:** **Phase 3+.** Phụ thuộc FAQ system.

---

## 4. Lưu Trữ: Raw vs Top-Level

### 4.1 Khuyến nghị: currentMetrics.raw

| Lý do | Chi tiết |
|-------|----------|
| **Kiến trúc** | raw = input từ nguồn. Message intent = raw signal. |
| **Rule Engine** | RULE_CRM_CLASSIFICATION, RULE_CIO_* đọc từ layers.raw |
| **Không rối schema** | Tránh thêm 10+ field top-level |
| **Pipeline thống nhất** | raw → layer1 → layer2 → layer3 |

### 4.2 Cấu trúc raw mở rộng

```json
{
  "raw": {
    "totalSpent": 5000000,
    "conversationCount": 3,
    "lastConversationAt": 1710000000000,
    "lastIntent": "complaint",
    "lastSentiment": "negative",
    "lastIntentAt": 1710000100000,
    "lastIntentConfidence": 0.92,
    "lastEscalationSignal": true,
    "lastComplaintType": "delivery",
    "profileExtractsFromMessage": {
      "objection": "giao trễ",
      "sizePreference": "M"
    }
  }
}
```

**Lưu ý:** Profile PII (phone, email) merge vào `CrmCustomer.Profile`, không để trong raw. Chỉ preference, objection → raw.

---

## 5. Rủi Ro Tổng Thể

### 5.1 Rủi ro kỹ thuật

| Rủi ro | Mức độ | Giảm thiểu |
|--------|--------|------------|
| AI extract sai intent | Trung bình | Confidence threshold, rule-based fallback |
| Merge PII sai | Cao | Chỉ merge khi confidence > 0.95, validate format |
| Token cost vượt budget | Trung bình | Filter trước AI, chỉ tin gần đây, model rẻ |
| Worker lag / backlog | Trung bình | Batch, scale worker, watermark |

### 5.2 Rủi ro nghiệp vụ

| Rủi ro | Mức độ | Giảm thiểu |
|--------|--------|------------|
| Complaint bị route AI thay vì Human | Cao | Rule: sentiment=negative → human, không tin confidence thấp |
| Profile bị merge dữ liệu sai | Cao | Audit log merge, rollback mechanism |

---

## 6. Khuyến Nghị Phân Phase

### Phase 1: Foundation (4–6 tuần)

**Mục tiêu:** Có intent + sentiment phục vụ routing, không over-engineer.

| Hạng mục | Chi tiết |
|----------|----------|
| **Output** | intent, sentiment, confidence |
| **Lưu trữ** | currentMetrics.raw (lastIntent, lastSentiment, lastIntentAt) |
| **Consumer** | RULE_CIO_ROUTING_MODE (seed mới) |
| **Không làm** | profileExtracts, urgency, complaintType, productMentions |

**Lý do:** Validate pipeline trước. Intent + sentiment đủ cho 80% giá trị routing.

---

### Phase 2: Mở rộng có điều kiện (2–3 tuần sau Phase 1)

**Điều kiện:** Phase 1 chạy ổn, có data thực tế, có nhu cầu rõ.

| Hạng mục | Chi tiết |
|----------|----------|
| **Thêm output** | escalationSignal, complaintType, profileExtracts (chỉ objection, preference — không PII) |
| **PII merge** | Chỉ khi confidence > 0.95, validate phone/email, log audit |
| **Consumer** | Mở rộng RULE_CIO_ROUTING_MODE, MergeProfileFromMessageExtracts |

---

### Phase 3: Tối ưu (khi có consumer)

| Hạng mục | Chi tiết |
|----------|----------|
| **urgency** | Khi có inbound queue / SLA |
| **suggestedAction** | Khi có Sales assist UI |
| **channelPreference, timePreference** | Khi RULE_CIO_CHANNEL_CHOICE cần |
| **topic** | Khi có FAQ/auto-reply |

---

### Phase 4+: Defer

| Hạng mục | Lý do defer |
|----------|-------------|
| **productMentions, skuInterest** | Cần catalog, matching phức tạp |
| **competitorMention** | Giá trị insight, chưa có consumer |
| **language** | Có thể thêm sau nếu cần |

---

## 7. Tiêu Chí Thành Công

### 7.1 Phase 1

| Chỉ số | Mục tiêu |
|--------|----------|
| Complaint → Human routing | 100% (khi sentiment=negative) |
| Token cost | < $50/tháng (10k tin) |
| Worker latency | P95 < 2 phút từ message → raw updated |
| False positive (complaint nhầm) | < 5% |

### 7.2 Phase 2

| Chỉ số | Mục tiêu |
|--------|----------|
| Profile merge từ message | Số merge thành công / tổng extract |
| PII merge accuracy | 100% (không merge sai) |

---

## 8. Tóm Tắt Tham Mưu

### 8.1 Quyết định chính

| Câu hỏi | Khuyến nghị |
|---------|-------------|
| **Làm ngay?** | Có — Phase 1 (intent + sentiment) |
| **Lưu ở đâu?** | currentMetrics.raw |
| **Mở rộng profileExtracts?** | Phase 2 — chỉ objection/preference Phase 2; PII có điều kiện |
| **Mở rộng urgency, complaintType, escalation?** | Phase 2 (escalation, complaintType), Phase 3 (urgency) |
| **productMentions, topic?** | Defer đến khi có consumer |

### 8.2 Nguyên tắc vàng

1. **Start small, validate, then expand** — Phase 1 đủ để chứng minh giá trị.
2. **Raw = source of truth** — Mọi extension vào raw, Rule Engine đọc từ đó.
3. **PII merge = high bar** — Confidence + validation + audit.
4. **No consumer = no extract** — Không extract field chưa có ai dùng.

### 8.3 Điều kiện Go/No-Go Phase 2

- Phase 1 chạy ổn ít nhất 2 tuần.
- Có data: intent distribution, complaint rate.
- Có yêu cầu rõ: "cần complaintType để route team" hoặc "cần merge phone từ chat".
