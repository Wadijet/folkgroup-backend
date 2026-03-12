# BÁO CÁO RÀ SOÁT LIÊN KẾT DỮ LIỆU

**Ngày tạo:** 2026-03-09 16:06  
**Database:** folkform_auth

---

## 1. TỔNG QUAN SỐ LƯỢNG

| Collection | Số bản ghi |
|------------|------------|
| pc_pos_orders | 4138 |
| pc_pos_customers | 3621 |
| fb_customers | 38054 |
| fb_conversations | 47062 |
| fb_message_items | 1288607 |
| fb_pages | 5 |
| fb_posts | 6301 |
| meta_ads | 197 |
| crm_customers | 42841 |

## 2. LIÊN KẾT PC_POS_ORDERS (posData)

| Trường | Số đơn có | Tỷ lệ |
|--------|-----------|-------|
| conversation_id / conversationId / conversation_link | 4138 | 100.0% |
| customerId / posData.customer.id / customer_id | 4107 | 99.3% |
| posData.ad_id | 4138 | 100.0% |
| posData.post_id | 4138 | 100.0% |
| posData.page_id | 4138 | 100.0% |

*Tổng đơn hàng: 4138*

## 3. LIÊN KẾT FB_CONVERSATIONS

| Chỉ số | Giá trị |
|--------|--------|
| Tổng conversations | 47062 |
| Conv có customerId | 47062 |
| Tổng fb_message_items | 1288607 |
| Tổng fb_messages | 47040 |
| Số conv có ≥1 message (từ message_items) | 47040 |

## 4. LIÊN KẾT META ADS

| Collection | Số bản ghi |
|------------|------------|
| meta_ads | 197 |
| meta_campaigns | 19 |
| meta_adsets | 29 |
| meta_ad_accounts | 2 |
| meta_ad_insights | 20501 |

## 5. LIÊN KẾT CRM & CUSTOMERS (fb_customers, pc_pos_customers, crm_customers)

| Chỉ số | Giá trị |
|--------|--------|
| fb_customers | 38054 |
| fb_customers có pageId | 38054 |
| pc_pos_customers | 3621 |
| crm_customers | 42841 |
| crm có sourceIds.pos | 4379 |
| crm có sourceIds.fb | 40641 |
| fb_conversations có customerId | 47062 |

### 5.1 CRM merge từ fb_customers + pc_pos_customers

crm_customers được merge từ fb_customers và/hoặc pc_pos_customers qua sourceIds.pos và sourceIds.fb.

| Phân loại | Số lượng | Mô tả |
|------------|-----------|-------|
| **Chỉ POS** (sourceIds.pos có, fb không) | 2200 | Merge từ pc_pos_customers |
| **Chỉ FB** (sourceIds.fb có, pos không) | 38462 | Merge từ fb_customers |
| **Chung (FB + POS)** | 2179 | Đã merge cả hai nguồn — 1 khách = 1 crm |
| *Tổng crm có pos* | 4379 | = Chỉ POS + Chung |
| *Tổng crm có fb* | 40641 | = Chỉ FB + Chung |

## 6. CHI TIẾT RÀ SOÁT TỪNG LIÊN KẾT

### 6.1 pc_pos_orders.posData → collection đích (mẫu 200 đơn)

| Liên kết | Có trong order | Khớp đích | Tỷ lệ khớp |
|----------|----------------|------------|-------------|
| posData.ad_id → meta_ads | 112 | 79 | 70.5% |
| posData.post_id → fb_posts | 114 | 114 | 100.0% |
| posData.conversation_id → fb_conversations | 123 | 123 | 100.0% |
| posData.page_id → fb_pages | 123 | 123 | 100.0% |
| customerId / posData.customer → pc_pos_customers | 198 | 198 | 100.0% |
| customerId / posData.customer → fb_customers | 198 | 0 | 0.0% |

### 6.2 fb_conversations.customerId → fb_customers

Mẫu 100 conversations có customerId: 0 khớp fb_customers (0.0%)

### 6.3 fb_conversations.conversationId → fb_message_items

Mẫu 100 conversations: 100 có ≥1 message trong fb_message_items (100.0%)

### 6.4 fb_pages.shop_id → pc_pos_shops

✅ fb_pages.shop_id (860225178) khớp pc_pos_shops

