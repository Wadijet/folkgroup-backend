# Learning Engine (Decision Brain) — Learning Memory Layer

**Mục đích:** Module Decision Brain là **bộ nhớ học tập** cho hệ thống AI Commerce, lưu trữ các quyết định đã hoàn thành để AI học và tối ưu hóa.

> **Canonical reference:** [11 - learning-engine.md](../../../../docs/architecture/vision/11 - learning-engine.md) — Schema đầy đủ, Outcome model, Error attribution, Learning loop

---

## 1. Tổng Quan Kiến Trúc

```
Activity Log     → Lưu sự kiện (event stream)
State Objects    → Lưu trạng thái hiện tại
Entities         → Xử lý vòng đời nghiệp vụ
Decision Brain   → Lưu bộ nhớ quyết định đã hoàn thành (learning memory)
```

**Decision Brain KHÔNG phải:**
- Activity Log
- Event stream
- Lifecycle log
- State database

**Decision Brain LÀ:**
- Learning memory layer
- Lưu trữ case study quyết định đã đóng
- Chuẩn hóa tri thức quyết định
- Hỗ trợ analytics và retrieval cho AI

---

## 2. Decision Case Là Gì

Một **decision case** là quyết định đã hoàn thành chứa tri thức học được.

Cấu trúc: **Context → Choice → Goal → Outcome → Lesson**

| Thành phần | Mô tả |
|------------|-------|
| Context | Tình huống tại thời điểm quyết định |
| Choice | Lựa chọn đã thực hiện |
| Goal | Mục tiêu mong muốn |
| Outcome | Kết quả thực tế |
| Lesson | Bài học rút ra |

---

## 3. Khi Nào Tạo Decision Case

**Chỉ tạo khi entity nguồn đã đóng vòng đời.**

Trạng thái đóng ví dụ:
- `completed`
- `reviewed`
- `finalized`
- `closed`
- `executed` (ActionPending)
- `rejected` (ActionPending)
- `failed` (ActionPending)

**Không tạo** nếu outcome chưa hoàn tất.

---

## 4. Entity Nguồn Có Thể Tạo Case

| Loại | Ví dụ | Collection/Source |
|------|-------|-------------------|
| **Action** | pause campaign, reduce budget, send re-engagement | action_pending_approval (status=executed/rejected/failed) |
| **CIO Choice** | chọn kênh (Zalo vs SMS), lên lịch touchpoint | (tương lai) |
| **Content Choice** | chọn creative, chọn content line | (tương lai) |
| **Governance** | human approval, rejection, manual override | action_pending_approval |

---

## 5. Mongo Collection

**Tên:** `decision_cases`

---

## 6. Mongo Schema

```javascript
{
  _id: ObjectId,
  caseId: String,           // Unique business ID (vd: dc_xxx)
  caseType: String,          // action | cio_choice | content_choice | approval
  caseCategory: String,       // ads | crm | content | notification | ...
  domain: String,            // domain nghiệp vụ

  targetType: String,        // campaign | customer | ad_set | ...
  targetId: String,          // ID của đối tượng bị tác động

  sourceRef: {
    refType: String,         // action_pending | cio_choice | ...
    refId: String            // ID document nguồn
  },

  goalCode: String,         // Mã mục tiêu (vd: reduce_waste, re_engage)
  result: String,            // success | partial | failed | rejected

  summary: {
    primaryMetric: String,
    baselineValue: Number,
    finalValue: Number,
    delta: Number
  },

  text: {
    systemSummary: {
      title: String,
      shortSummary: String
    },
    aiText: {
      situation: String,
      decisionRationale: String,
      intendedGoal: String,
      expectedOutcome: String,
      actualOutcome: String,
      lesson: String,
      nextSuggestion: String
    },
    humanNotes: {
      decisionNote: String,
      reviewNote: String,
      overrideReason: String,
      freeNote: String
    }
  },

  tags: [String],

  ownerOrganizationId: ObjectId,
  sourceClosedAt: Number,    // Unix ms khi entity nguồn đóng
  createdAt: Number,
  updatedAt: Number
}
```

---

## 7. Indexes

| Index | Keys | Mục đích |
|-------|------|----------|
| caseId | caseId | Lookup theo caseId |
| org_domain_created | ownerOrganizationId + domain + createdAt | List theo org, domain |
| org_target | ownerOrganizationId + targetType + targetId | Query theo target |
| caseType | caseType | Filter theo loại |
| caseCategory | caseCategory | Filter theo category |
| goalCode | goalCode | Filter theo mục tiêu |
| result | result | Filter theo kết quả |
| sourceClosedAt | sourceClosedAt | Sort theo thời điểm đóng |
| sourceRef | sourceRef.refType + sourceRef.refId | Lookup theo nguồn |

---

## 8. Services

- `CreateDecisionCase`
- `FindDecisionCaseById`
- `ListDecisionCases`
- `QueryDecisionCasesByTarget`
- `QueryDecisionCasesByCaseType`
- `QueryDecisionCasesByCategory`
- `QueryDecisionCasesByGoal`
- `QueryDecisionCasesByResult`

---

## 9. Builders

| Builder | Nguồn | Trách nhiệm |
|---------|-------|-------------|
| BuildDecisionCaseFromAction | ActionPending (executed/rejected/failed) | Đọc entity, extract, tạo case |
| BuildDecisionCaseFromCIOChoice | (tương lai) | Tương tự |
| BuildDecisionCaseFromContentChoice | (tương lai) | Tương tự |
| BuildDecisionCaseFromApproval | ActionPending (approved/rejected) | Tương tự |

---

## 10. Khác Biệt Với Activity Log

| | Activity Log | Decision Brain |
|---|--------------|----------------|
| Mục đích | Ghi sự kiện, timeline | Lưu case study học tập |
| Thời điểm | Mỗi event xảy ra | Chỉ khi lifecycle đóng |
| Nội dung | Event + snapshot | Context + Choice + Goal + Outcome + Lesson |
| AI dùng | Timeline, audit | Learning, retrieval, clustering |

---

## 11. Ví Dụ Decision Case

### Action (pause campaign)

```json
{
  "caseId": "dc_674a1b2c_1710316800",
  "caseType": "action",
  "caseCategory": "ads",
  "domain": "ads",
  "targetType": "campaign",
  "targetId": "123456789",
  "sourceRef": { "refType": "action_pending", "refId": "674a1b2c..." },
  "goalCode": "pause_campaign",
  "result": "success",
  "summary": { "primaryMetric": "spend", "baselineValue": 500, "finalValue": 0, "delta": -500 },
  "text": {
    "systemSummary": { "title": "pause_campaign - ads", "shortSummary": "CH cao, tạm dừng campaign" },
    "aiText": {
      "situation": "CH cao 3 ngày liên tiếp",
      "decisionRationale": "Giảm lãng phí ngân sách",
      "actualOutcome": "Đã tạm dừng thành công",
      "lesson": "Pause sớm giúp tiết kiệm"
    }
  }
}
```

### CIO Choice (chọn kênh — tương lai)

```json
{
  "caseType": "cio_choice",
  "caseCategory": "crm",
  "goalCode": "choose_channel",
  "targetType": "customer",
  "targetId": "crm_xxx",
  "result": "success",
  "text": {
    "aiText": {
      "situation": "Khách chưa mua 30 ngày",
      "decisionRationale": "Zalo có tỷ lệ mở cao hơn SMS",
      "intendedGoal": "Re-engage",
      "actualOutcome": "Đã gửi Zalo, mở sau 2h"
    }
  }
}
```

### Content Choice (chọn creative — tương lai)

```json
{
  "caseType": "content_choice",
  "caseCategory": "content",
  "goalCode": "select_creative",
  "targetType": "campaign",
  "result": "success",
  "text": {
    "aiText": {
      "situation": "3 creative A/B test",
      "decisionRationale": "Creative B có CTR cao hơn 20%",
      "actualOutcome": "Chọn B, CTR tăng"
    }
  }
}
```

### Approval (human rejection)

```json
{
  "caseType": "approval",
  "caseCategory": "ads",
  "goalCode": "reduce_budget",
  "result": "rejected",
  "text": {
    "humanNotes": { "decisionNote": "Giữ budget để chạy thêm 2 ngày" },
    "aiText": { "lesson": "Lý do từ chối: chờ đủ thời gian đánh giá" }
  }
}
```

---

## 12. Điểm Tích Hợp (Integration)

Để tạo decision case khi ActionPending đóng:

- **Trong `approval.NotifyExecuted`** (sau khi worker execute thành công): gọi `BuildDecisionCaseFromAction(doc)` → `CreateDecisionCase`
- **Trong `approval.NotifyFailed`** (sau khi hết retry): gọi `BuildDecisionCaseFromAction(doc)` → `CreateDecisionCase`
- **Trong `approval.Reject`** (khi user reject): gọi `BuildDecisionCaseFromAction(doc)` → `CreateDecisionCase`

---

## Changelog

- 2025-03-13: Tạo tài liệu thiết kế ban đầu

