# BÁO CÁO KHÁM PHÁ DỮ LIỆU ZALO TRONG DATABASE

**Ngày tạo:** 2026-03-17  
**Database:** folkform_auth  
**Script:** `scripts/discover_zalo_data.go`

---

## TÓM TẮT

Database đã có **dữ liệu Zalo** được Pancake đồng bộ qua webhook. Zalo dùng chung collections với Messenger (`fb_conversations`, `fb_customers`, `fb_pages`) và được phân biệt qua **pageId bắt đầu `pzl_`** (personal_zalo).

---

## 1. FB_PAGES (Zalo)

| Chỉ số | Giá trị |
|--------|---------|
| Tổng fb_pages | 6 |
| Page Zalo (platform=personal_zalo) | 1 |

**Chi tiết page Zalo:**
- **pageId:** `pzl_712413543211467438`
- **platform:** `personal_zalo` (trong panCakeData)
- **username:** `pzl_84965656066`
- **inserted_at:** 2026-03-16 (mới thêm gần đây)
- **name:** Folk Form

---

## 2. FB_CONVERSATIONS (Zalo)

| Chỉ số | Giá trị |
|--------|---------|
| Tổng conversations | 51.752 |
| **Conv Zalo** (pageId bắt đầu `pzl_`) | **843** |
| Conv Messenger | 50.909 |

**Định dạng conversation Zalo:**
- `conversationId`: `pzl_u_712413543211467438_<user_id>` (prefix `pzl_u_`)
- `pageId`: `pzl_712413543211467438`
- `panCakeData.type`: INBOX (tương tự Messenger)

---

## 3. FB_CUSTOMERS (Zalo)

| Chỉ số | Giá trị |
|--------|---------|
| Tổng fb_customers | 40.900 |
| **Khách Zalo** (pageId bắt đầu `pzl_`) | **1.125** |
| Khách Messenger | 39.775 |

**Lưu ý:** fb_customers dùng chung cho cả Messenger và Zalo. Phân biệt qua `pageId`:
- Zalo: `pageId` bắt đầu `pzl_`
- Messenger: `pageId` là số (Facebook Page ID)

---

## 4. CÁCH PHÂN BIỆT ZALO TRONG CODE

```go
// Conversation Zalo
pageId := conv.PageId
isZalo := strings.HasPrefix(pageId, "pzl_")

// Customer Zalo
isZaloCustomer := strings.HasPrefix(fbCustomer.PageId, "pzl_")

// Page Zalo
// fb_pages.panCakeData.platform == "personal_zalo"
```

---

## 5. SO SÁNH VỚI BÁO CÁO TRƯỚC (2026-03-09)

| Collection | 2026-03-09 | 2026-03-17 | Ghi chú |
|------------|------------|------------|---------|
| fb_pages | 5 | 6 | +1 page Zalo |
| fb_conversations | 47.062 | 51.752 | +4.690 (trong đó 843 Zalo) |
| fb_customers | 38.054 | 40.900 | +2.846 (trong đó 1.125 Zalo) |

---

## 6. GỢI Ý CẬP NHẬT

1. **Báo cáo rà soát** (`report_data_linkage.go`): Thêm mục phân tách Zalo vs Messenger.
2. **CRM merge:** Kiểm tra `sourceIds.fb` có dùng cho khách Zalo không — hiện crm merge từ fb_customers, khách Zalo cũng vào sourceIds.fb.
3. **CIO module:** Đã có design cho channel `zalo` — dữ liệu thực tế đã có, có thể bật ingestion từ fb_conversations (filter pageId pzl_).
4. **Docs:** Cập nhật `docs/03-api/webhook-pancake.md` — Pancake gửi cả Zalo qua cùng webhook.
