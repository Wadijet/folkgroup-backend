# Đề Xuất: Tách riêng nhóm khách Block/Spam khỏi đánh giá Customer

> **Vấn đề:** Hệ thống đánh giá Customer đang lẫn cả khách block/spam (dữ liệu từ conversation). Nhóm này không có giá trị kinh doanh và làm nhiễu các chỉ số.

---

## I. Phân tích hiện trạng

### 1. Nguồn dữ liệu Block/Spam

| Nguồn | Field | Mô tả |
|-------|-------|-------|
| **fb_conversations** | `panCakeData.tags[].text` | Tag "Block", "Spam", "Khách BLOCK", "Chặn" |
| **fb_customers** | `panCakeData.is_block`, `panCakeData.tags` | Khách FB bị block |
| **pc_pos_customers** | `isBlock`, `posData.is_block` | Khách POS bị block |
| **crm_customers** | `conversationTags` | Union tags từ conversation (đã sync) |

### 2. Luồng hiện tại (chưa loại trừ block/spam)

```
fb_conversations (tất cả conv) 
    → aggregateConversationMetricsForCustomer (không filter tag)
    → convMetrics.ConversationCount, conversationTags
    → hasConv = true khi có bất kỳ conv nào
    → ComputeClassificationFromMetrics(..., hasConv)
    → journeyStage = "engaged" khi orderCount=0 && hasConv
```

**Hệ quả:**
- Khách chỉ có conversation block/spam → vẫn được xếp **engaged**
- Engaged Volume, Engaged Conversion Rate, Inbox filter "engaged" đều bao gồm khách block/spam
- Làm nhiễu metrics và không có giá trị cho Sale/CEO

### 3. Các điểm cần sửa

| Vị trí | File | Mô tả |
|--------|------|-------|
| **CRM aggregate** | `service.crm.conversation_metrics.go` | `aggregateConversationMetricsForCustomer` — không filter conv block/spam |
| **CRM recalc/merge** | `service.crm.recalculate.go`, `service.crm.merge.go`, `service.crm.metrics.go` | `hasConv` dùng cho classification |
| **Report Inbox** | `service.report.inbox.go` | `computeEngagedStats`, `buildInboxConversationItem`, `loadConversationsForInbox` — không loại trừ conv block/spam |
| **Report Dashboard** | `handler.report.dashboard.go` | JourneyDistribution phân bố visitor/engaged |

---

## II. Phương án đề xuất

### Phương án A: Loại trừ conversation block/spam khi aggregate (khuyến nghị)

**Ý tưởng:** Chỉ aggregate các conversation **không có** tag Block/Spam/Chặn. Khi đó:
- Khách chỉ có conv block/spam → `convMetrics.ConversationCount = 0` → `hasConv = false` → `journeyStage = visitor`
- Khách có cả conv thường và conv block/spam → vẫn tính conv thường (có thể giữ nguyên hoặc loại trừ conv block/spam khỏi count)

**Ưu điểm:**
- Không cần thêm field mới
- Logic đơn giản: conv block/spam = không phải engagement thực
- Tự động: Engaged Volume, Inbox, Dashboard đều không còn khách block/spam (vì journeyStage = visitor)

**Nhược điểm:**
- `conversationTags` vẫn có thể chứa Block/Spam (union từ tất cả conv) — cần quyết định: có loại trừ tags block/spam khỏi union không? Đề xuất: **vẫn giữ** để dễ phân biệt khi cần (audit, filter thủ công).

### Phương án B: Thêm field `isBlockedSpam` và logic riêng

**Ý tưởng:**
- Thêm `crm_customers.isBlockedSpam` (bool)
- Khi có bất kỳ conv nào có tag Block/Spam/Chặn (hoặc fb_customers.is_block, pc_pos_customers.isBlock) → `isBlockedSpam = true`
- Trong classification: `hasConv = hasConv && !isBlockedSpam`
- Khi `isBlockedSpam = true` → `journeyStage = "blocked_spam"` (hoặc visitor)

**Ưu điểm:**
- Có thể filter riêng nhóm blocked_spam trong dashboard
- Có thể hiển thị thống kê "Số khách block/spam" nếu cần

**Nhược điểm:**
- Thêm field, migration, logic phức tạp hơn
- Cần cập nhật nhiều nơi (report, dashboard)

### Phương án C: Kết hợp A + B (đầy đủ)

- **Phase 1:** Áp dụng Phương án A — filter conv block/spam khi aggregate
- **Phase 2 (tùy chọn):** Thêm `isBlockedSpam` và `journeyStage = "blocked_spam"` cho khách có tag block/spam (dù có conv thường hay không) — để có thể filter riêng trong UI

---

## III. Chi tiết triển khai (Phương án A)

### 1. Helper: kiểm tra tag block/spam

Tạo file `api/internal/api/crm/service/service.crm.block_spam.go`:

```go
// Package crmvc - Helper kiểm tra tag block/spam từ conversation.
package crmvc

import "strings"

// hasSpamOrBlockTag kiểm tra tags có chứa Block, Spam, Khách BLOCK, Chặn.
// Dùng khi filter conversation khỏi aggregate.
func hasSpamOrBlockTag(tags []string) bool {
	for _, t := range tags {
		lower := strings.ToLower(strings.TrimSpace(t))
		if lower == "block" || lower == "spam" ||
			strings.Contains(lower, "spam") || strings.Contains(lower, "block") || strings.Contains(lower, "chặn") {
			return true
		}
	}
	return false
}

// getTagsFromPanCakeDataTags đọc panCakeData.tags — mảng có thể chứa null và object.
func getTagsFromPanCakeDataTags(m map[string]interface{}, key string) []string {
	// ... (logic từ scripts/audit_visitors_spam_block_tag.go)
}
```

### 2. Sửa `aggregateConversationMetricsForCustomer`

Thêm `$match` loại trừ conversation có tag block/spam **trước** khi aggregate:

```go
// Trong pipeStages, sau $match matchFilter, thêm:
// Loại trừ conversation có tag Block/Spam/Chặn — không tính vào engagement
pipeStages = append(pipeStages, bson.D{{Key: "$match", Value: bson.M{
	"$and": []bson.M{
		{"$or": []bson.M{
			{"panCakeData.tags": bson.M{"$exists": false}},
			{"panCakeData.tags": nil},
			{"panCakeData.tags": bson.A{}},
			// Không có tag nào match block/spam
			{"$nor": []bson.M{
				{"panCakeData.tags.text": bson.M{"$regex": "block", "$options": "i"}},
				{"panCakeData.tags.text": bson.M{"$regex": "spam", "$options": "i"}},
				{"panCakeData.tags.text": bson.M{"$regex": "chặn", "$options": "i"}},
			}},
		}},
	},
}}})
```

**Lưu ý:** Cấu trúc `panCakeData.tags` là mảng object `{text: "Block"}`. Cần dùng `$elemMatch` hoặc `$nor` với path `panCakeData.tags.text` đúng.

**Cách đơn giản hơn:** Thêm điều kiện vào `matchFilter` ban đầu:

```go
matchFilter = bson.M{
	"$and": []bson.M{
		matchFilter,
		{"$nor": []bson.M{
			{"panCakeData.tags.text": bson.M{"$regex": "block", "$options": "i"}},
			{"panCakeData.tags.text": bson.M{"$regex": "spam", "$options": "i"}},
			{"panCakeData.tags.text": bson.M{"$regex": "chặn", "$options": "i"}},
		}},
	},
}
```

### 3. conversationTags: có loại trừ block/spam không?

**Đề xuất:** Vẫn aggregate từ **tất cả** conv (không filter) để ghi `conversationTags` — vì:
- Cần biết khách có tag block/spam để audit
- Có thể filter riêng trong UI nếu cần

→ Cần **2 lần aggregate** hoặc **1 query riêng** cho tags. Hoặc đơn giản: giữ logic hiện tại cho tags (union từ tất cả conv), chỉ filter khi tính `hasConv` và count.

**Cách đơn giản hơn:** 
- Aggregate 1 lần **có** filter block/spam → convMetrics dùng cho hasConv, count
- Aggregate 1 lần **không** filter → chỉ lấy tags (hoặc dùng cursor thứ 2 hiện tại) → tags vẫn đầy đủ

Thực tế: cursor thứ 2 trong `aggregateConversationMetricsForCustomer` dùng `matchFilter` — nếu đổi matchFilter thì tags cũng bị filter. 

**Giải pháp:** Tách riêng:
- `matchFilterForEngagement` = matchFilter + $nor block/spam
- `matchFilterForTags` = matchFilter (không filter tag)
- Aggregate count dùng matchFilterForEngagement
- Cursor lấy tags dùng matchFilterForTags (hoặc ngược lại: cursor tags dùng matchFilter gốc)

### 4. Report Inbox: loại trừ conv block/spam

Cần sửa `loadConversationsForInbox` — thêm filter loại trừ conv có tag block/spam:

```go
filter["$nor"] = []bson.M{
	{"panCakeData.tags.text": bson.M{"$regex": "block", "$options": "i"}},
	{"panCakeData.tags.text": bson.M{"$regex": "spam", "$options": "i"}},
	{"panCakeData.tags.text": bson.M{"$regex": "chặn", "$options": "i"}},
}
```

Và `computeEngagedStats` — đã dùng convs từ loadConversationsForInbox, nên sau khi filter loadConversationsForInbox thì tự động không còn conv block/spam.

### 5. Tóm tắt thay đổi

| File | Thay đổi |
|------|----------|
| `service.crm.conversation_metrics.go` | Thêm filter block/spam vào matchFilter khi aggregate (cho count, lastMessageFromCustomer). Cursor thứ 2 lấy tags: cần quyết định giữ tags block/spam hay không. |
| `service.report.inbox.go` | `loadConversationsForInbox`: thêm $nor filter loại trừ conv có tag block/spam |
| **Không cần** | recalculate, merge, metrics — vì đã aggregate đúng từ conversation_metrics |

---

## IV. Lưu ý kỹ thuật

### MongoDB regex với tag

`panCakeData.tags` có cấu trúc: `[{text: "Block"}, {text: "Đã mua"}, ...]`

- `panCakeData.tags.text` match khi **bất kỳ** phần tử có `text` match
- `$nor` với `panCakeData.tags.text: {$regex: "block"}` = loại trừ conv có ít nhất 1 tag chứa "block"

### Conv vừa có tag thường vừa có tag block/spam

Nếu 1 conv có cả "Đã mua" và "Block" → loại trừ khỏi engagement (đúng vì đã block).

### Khách có nhiều conv: 1 block, 1 thường

- Chỉ aggregate conv thường → count = 1, hasConv = true → engaged (đúng)
- Tags vẫn có thể chứa Block/Spam từ conv kia (nếu dùng cursor riêng cho tags)

---

## V. Migration / Recalculate

Sau khi triển khai:
1. Chạy `RecalculateAllCustomers` hoặc `RecalculateMismatchCustomers` để cập nhật lại classification cho khách đã có
2. Hoặc chạy worker recalc theo batch

---

## VI. Checklist triển khai (đã hoàn thành — phiên bản mới)

**Phiên bản mới:** Khách block/spam đưa vào nhóm `blocked_spam` trong hành trình — vẫn tính toán, thống kê bình thường.

- [x] Thêm `journeyStage = "blocked_spam"` khi orderCount=0 và conversationTags có Block/Spam/Chặn
- [x] Thêm `HasSpamOrBlockTag` helper, `ComputeClassificationFromMetrics` nhận conversationTags
- [x] Cập nhật dashboard, report, JourneyDistribution, JourneyLTV cho blocked_spam
- [x] Inbox: Engaged count và filter "engaged" loại trừ conv block/spam (nhóm riêng blocked_spam)
- [ ] Chạy recalc batch cho org cần thiết (sau khi deploy)

---

## VII. Tài liệu tham khảo

- `scripts/audit_visitors_spam_block_tag.go` — logic kiểm tra tag block/spam
- `scripts/find_conv_with_block_spam_tag.go` — query conv có tag
- `docs/02-architecture/ENGAGED_INTELLIGENCE_LAYER_EVALUATION.md`
- `api/internal/api/crm/service/service.crm.classification.go` — ComputeJourneyStage
