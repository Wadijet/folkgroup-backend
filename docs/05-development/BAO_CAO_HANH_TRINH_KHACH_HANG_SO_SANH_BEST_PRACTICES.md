# Báo Cáo: So Sánh Hành Trình Khách Hàng FolkForm với Best Practices

> Ngày: 2025-03-11  
> Mục đích: Đánh giá hành trình khách hàng hiện tại đã hợp lý chưa, còn thiếu gì.

---

## 1. Hành Trình FolkForm Hiện Tại (2025)

**Journey (Lớp 1):** `visitor` | `engaged` | `blocked_spam` | `first` | `repeat` | `inactive`

| Stage | Định nghĩa |
|-------|------------|
| **visitor** | Chưa có conversation, chưa mua |
| **engaged** | Có conversation, chưa mua |
| **blocked_spam** | Có conv tag Block/Spam/Chặn, chưa mua |
| **first** | Mua lần đầu, còn active (≤90 ngày từ đơn cuối) |
| **repeat** | Mua ≥2 lần, còn active |
| **inactive** | Đã mua nhưng >90 ngày không mua |

**Lớp 2 (Value, Lifecycle, Loyalty, Momentum, Channel, CeoGroup):** Đã có đầy đủ, không phạm vi báo cáo này.

---

## 2. Mô Hình Chuẩn Trên Thị Trường

### 2.1 E-commerce / Retail (Shopify, Semrush, Salesforce 2024–2025)

| Stage | Mô tả |
|-------|-------|
| **Awareness** | Khách biết đến brand |
| **Consideration** | So sánh, đánh giá sản phẩm |
| **Acquisition/Purchase** | Mua lần đầu |
| **Service** | Hỗ trợ sau mua |
| **Loyalty** | Mua lại, trung thành |

### 2.2 B2C Lifecycle (Klaviyo, Blueshift)

| Stage | Mô tả |
|-------|-------|
| **Awareness** | Khám phá brand |
| **Acquisition/Purchase** | Mua lần đầu |
| **Retention/Engagement** | Mua lại, tương tác |
| **Advocacy** | Giới thiệu, review, referral |

### 2.3 Conversational Commerce (Messaging)

| Stage | Mô tả |
|-------|-------|
| **Inspiration** | Nghiên cứu, tìm hiểu |
| **Decision** | Quyết định mua |
| **Purchase** | Mua hàng |
| **Usage** | Sử dụng, hỗ trợ |
| **New Purchase** | Mua lại / mua thêm |

### 2.4 RFM / Segmentation (Behavioral)

- **Champions** (5-5-5): Mua gần, mua nhiều, chi tiêu cao
- **Loyal Customers**: Tần suất cao, giá trị cao
- **At Risk**: Khách cũ đang lãng quên
- **Hibernating (1-1-1)**: Chưa mua lâu, gần như churned

### 2.5 Lifecycle Status (Insider, Omnisend)

- **Visitors / Prospects**: Mới, chưa mua
- **Active Customers**: Mua đều định kỳ
- **At-Risk**: Có dấu hiệu rời bỏ
- **Inactive/Dormant**: Không hoạt động lâu
- **Churned**: Không còn mua

---

## 3. Đánh Giá FolkForm

### 3.1 Điểm Hợp Lý (Best Practices Đã Đáp Ứng)

| Aspect | Đánh giá |
|--------|----------|
| **visitor → engaged** | Tương đương Awareness → Consideration (trong messaging: chưa trò chuyện → đã trò chuyện) |
| **first → repeat** | Đúng Acquisition → Retention |
| **blocked_spam** | Có trong mô hình messaging: xử lý khách chặn/spam, không làm nhiễu funnel |
| **inactive** | Đúng với Inactive/Dormant/Churned |
| **Tách Lớp 1 (Journey) và Lớp 2 (Value, Lifecycle, Loyalty)** | Tránh trùng lặp, RFM/Value được tách ở Lớp 2 |
| **VIP ở valueTier** | Không nhét VIP vào Journey, tránh trùng với repeat |

### 3.2 Có Thể Cần Xem Xét

| Vấn đề | Mô tả | Đề xuất |
|--------|-------|---------|
| **Inactive** | Trùng với `lifecycleStage` (inactive, dead). Journey có thể chỉ nên là "hành trình trưởng thành" (engagement → purchase). | Cân nhắc bỏ `inactive` khỏi Journey, dùng `lifecycleStage`; chưa sửa code. |
| **Advocate / Promoter** | Stage cuối trong chuẩn: khách giới thiệu, review, referral. | Khó đo từ dữ liệu transaction/conversation hiện tại. Cần thêm: referral tracking, NPS, reviews. |
| **At-risk** | Khách đang cooling (30–90 ngày). | Đã có trong `lifecycleStage` (cooling). Không cần thêm vào Journey. |

### 3.3 Không Khuyến Nghị Thêm

| Stage | Lý do |
|-------|-------|
| **Service** | Giai đoạn sau mua, không phải stage khách hàng trong Journey. |
| **Usage** | Tương tự, dùng trong support flow. |
| **At-risk** | Đã có trong lifecycleStage. |

---

## 4. Kết Luận

### 4.1 Tổng Quan

- **Hành trình hiện tại đã hợp lý** với các mô hình chuẩn và phù hợp với mô hình messaging/chat commerce.
- **Bổ sung blocked_spam** là hợp lý cho môi trường messaging.
- **Tách VIP** sang valueTier là đúng hướng.

### 4.2 Đã Thực Hiện (Version 3.88 - 2025-03-11)

1. **Bỏ inactive khỏi Journey** — inactive dùng lifecycleStage (Lớp 2). Khách >90 ngày vẫn là first/repeat theo orderCount.
2. **Thêm promoter** — Stage placeholder chờ dữ liệu referral/NPS. Logic: `getReferralCount(c) > 0` → promoter (hiện luôn 0).

### 4.3 Khuyến Nghị

- **Giữ nguyên** visitor, engaged, blocked_spam, first, repeat, promoter.
- **Khi có dữ liệu referral:** Thêm field `referralCount` hoặc `isPromoter` vào crm_customers, cập nhật `getReferralCount()`.

---

## 5. Tài Liệu Tham Khảo

- [Shopify Retail Customer Journey](https://www.shopify.com/uk/retail/retail-customer-journey)
- [Klaviyo Customer Lifecycle Marketing](https://klaviyo.com/blog/customer-lifecycle-marketing)
- [Semrush Customer Journey](https://www.semrush.com/blog/customer-journey/)
- [Salesforce Customer Journey](https://www.salesforce.com/marketing/customer-journey)
- [Sinch Conversational Commerce](https://sinch.com/blog/conversational-commerce-messaging-apps-customer-journey/)
- [RFM Segmentation](https://mcpanalytics.ai/articles/rfm-segmentation-practical-guide-for-data-driven-decisions)
- [Customer Advocacy Stage](https://emfluence.com/blog/customer-journey-understanding-the-advocacy-stage)
