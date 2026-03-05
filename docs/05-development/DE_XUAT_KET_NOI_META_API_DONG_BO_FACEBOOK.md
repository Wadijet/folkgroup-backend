# Đề xuất: Kết nối Meta API để đồng bộ dữ liệu từ Facebook

## 📋 Tổng quan

Tài liệu này đề xuất phương án kết nối **Meta API** để đồng bộ dữ liệu từ Facebook vào hệ thống, tương tự cách đang làm với **Pancake** và **Pancake POS**.

### ⚠️ Ưu tiên: Meta Ads trước

**Để làm logic quản lý ads tự động, cần lấy thông tin Ads trước** qua **Meta Marketing API** (Ads Management API). Phần Conversations/Messenger có thể triển khai sau.

| Thứ tự | API | Mục đích |
|--------|-----|----------|
| **1** | **Meta Marketing API** (Ads) | Campaigns, Ad Sets, Ads, Insights — quản lý ads tự động |
| 2 | Meta Graph API (Conversations) | Hội thoại, tin nhắn — bổ sung khi cần |

---

## 📢 Meta Marketing API — Ads (Ưu tiên 1)

### Cấu trúc phân cấp Ads

```
Ad Account (act_xxx)
    └── Campaign (mục tiêu: traffic, conversions, engagement...)
            └── Ad Set (ngân sách, audience, schedule)
                    └── Ad (creative, link, text)
```

### Endpoints cần dùng cho quản lý ads tự động

| Mục đích | Endpoint | Permission |
|----------|----------|------------|
| Danh sách Ad Accounts | `GET /me/adaccounts` hoặc `GET /{business-id}/owned_ad_accounts` | `ads_management`, `ads_read` |
| Campaigns | `GET /act_{ad_account_id}/campaigns` | `ads_read` |
| Ad Sets | `GET /act_{ad_account_id}/adsets` hoặc `GET /{campaign_id}/adsets` | `ads_read` |
| Ads | `GET /act_{ad_account_id}/ads` hoặc `GET /{adset_id}/ads` | `ads_read` |
| **Insights** (hiệu suất) | `GET /act_{ad_account_id}/insights` | `ads_read` |
| Pause/Activate | `POST /{ad_id}` với `status=PAUSED` | `ads_management` |

### Insights API — Metrics quan trọng

```
GET /act_{ad_account_id}/insights?fields=impressions,clicks,spend,reach,cpc,cpm,ctr&date_preset=last_7d
```

| Field | Ý nghĩa |
|-------|---------|
| `impressions` | Số lần hiển thị |
| `clicks` | Số click |
| `spend` | Chi phí (đơn vị account) |
| `reach` | Số người tiếp cận |
| `cpc` | Cost per click |
| `cpm` | Cost per 1000 impressions |
| `ctr` | Click-through rate |

### Token & Permissions

- **User Access Token** hoặc **System User Token** với:
  - `ads_read` — đọc campaigns, adsets, ads, insights
  - `ads_management` — tạo/sửa/dừng ads
- **Ad Account ID** dạng `act_123456789` — lấy từ Business Manager hoặc `me/adaccounts`

### Collections đề xuất cho Ads

| Collection | Nội dung |
|------------|----------|
| `meta_ad_accounts` | Ad accounts (act_xxx) gắn với organization |
| `meta_campaigns` | Campaigns với metaData (raw từ API) |
| `meta_adsets` | Ad sets |
| `meta_ads` | Ads (creative, status, link) |
| `meta_ad_insights` | Insights theo ngày (spend, impressions, clicks...) — dùng cho báo cáo & logic tự động |

### Logic quản lý ads tự động (gợi ý)

1. **Sync định kỳ**: Worker fetch campaigns → adsets → ads → insights, lưu vào collections.
2. **Rules engine**: Dựa trên insights (vd: CPA > ngưỡng, CTR < ngưỡng) → pause ad/adset.
3. **Budget optimization**: So sánh spend vs budget → điều chỉnh hoặc cảnh báo.
4. **Mapping với CRM**: `ad_ids` trong conversation (panCakeData) → map với campaign/ad để phân loại `conversationFromAds`, `ordersFromAds`.

**Tài liệu tham khảo:**
- [Marketing API Overview](https://developers.facebook.com/docs/marketing-api/overview)
- [Ad Account Reference](https://developers.facebook.com/docs/marketing-api/reference/ad-account)
- [Insights API](https://developers.facebook.com/docs/marketing-api/insights)

---

## 📋 Thứ tự triển khai đề xuất

| Phase | Nội dung | Lý do |
|-------|----------|-------|
| **Phase 1** | Meta Marketing API + Ads sync | Cần cho logic quản lý ads tự động |
| Phase 2 | Meta Conversations API | Bổ sung khi cần sync hội thoại trực tiếp |

---

## 🔄 Kiến trúc hiện tại (Pancake & POS)

### Luồng dữ liệu hiện có

```
┌─────────────────┐     Webhook      ┌──────────────────┐     SyncUpsert     ┌─────────────────┐
│    Pancake      │ ───────────────► │  folkgroup API   │ ◄───────────────── │  Pancake POS    │
│ (FB/Messenger)  │   Push realtime  │                  │   Push batch/API   │  (external)     │
└─────────────────┘                  └────────┬─────────┘                    └─────────────────┘
                                              │
                                              ▼
                                    ┌──────────────────┐
                                    │ fb_customers      │
                                    │ fb_conversations  │
                                    │ fb_messages       │
                                    │ pc_pos_*          │
                                    └────────┬──────────┘
                                             │
                                             │ Event hooks
                                             ▼
                                    ┌──────────────────┐
                                    │ crm_pending_ingest │
                                    │ → crm_customers   │
                                    └──────────────────┘
```

### Đặc điểm chính

| Nguồn | Cơ chế | Endpoint | Collection đích |
|-------|--------|----------|------------------|
| **Pancake** | Webhook (push) | `POST /api/v1/pancake/webhook` | fb_conversations, fb_messages, fb_customers, pc_orders |
| **Pancake POS** | Sync-upsert (push) | `POST /pancake-pos/*/sync-upsert-one` | pc_pos_orders, pc_pos_customers, pc_pos_shops, ... |

- **Pancake**: Dữ liệu lưu trong `panCakeData` (map), có `panCakeUpdatedAt` để so sánh khi sync.
- **POS**: Dữ liệu lưu trong `posData`, có `posUpdatedAt`.
- **Sync logic**: `DoSyncUpsert` — chỉ ghi khi dữ liệu mới hơn (`updated_at`) hoặc document chưa tồn tại.
- **CRM**: Merge từ `fb_customers` + `pc_pos_customers` → `crm_customers` qua `SyncAllCustomers` và event hooks.

---

## 🎯 Phương án kết nối Meta API

### Khác biệt: Pull vs Push

| Nguồn | Hướng | Cơ chế |
|-------|-------|--------|
| Pancake | Push | Pancake gửi webhook khi có thay đổi |
| POS | Push | Hệ thống bên ngoài gọi sync-upsert |
| **Meta API** | **Pull** | Backend **chủ động gọi** Meta Graph API định kỳ |

Meta không push dữ liệu trực tiếp tới backend. Cần **worker/job** chạy định kỳ để fetch và đồng bộ.

---

## 📐 Kiến trúc đề xuất

```
┌─────────────────┐                    ┌──────────────────┐
│  Meta Graph API │ ◄── HTTP GET ───── │  Meta Sync Job   │
│ (graph.fb.com)  │                    │  (worker/cron)   │
└─────────────────┘                    └────────┬─────────┘
                                                │
                                                │ Map response → panCakeData format
                                                │ SyncUpsertOne (giống Pancake)
                                                ▼
                                       ┌──────────────────┐
                                       │ fb_customers      │
                                       │ fb_conversations  │
                                       │ fb_messages       │
                                       └────────┬──────────┘
                                                │
                                                │ Event hooks (đã có)
                                                ▼
                                       ┌──────────────────┐
                                       │ crm_customers     │
                                       └──────────────────┘
```

### Lợi ích

1. **Tái sử dụng** toàn bộ logic CRM merge, hooks, ingest — không cần sửa.
2. **Đồng nhất format**: Map Meta response → `panCakeData` format giống Pancake.
3. **Có thể chạy song song** với Pancake: nếu có Pancake thì dùng webhook realtime; nếu không thì dùng Meta sync job.

---

## 🔧 Chi tiết triển khai

### Bước 1: Meta Graph API — Endpoints cần dùng

| Mục đích | Endpoint | Permissions |
|----------|----------|-------------|
| Danh sách conversations | `GET /{page-id}/conversations` | `pages_messaging`, `pages_manage_metadata` |
| Chi tiết conversation | `GET /{conversation-id}?fields=id,messages,participants,updated_time` |同上 |
| Messages trong conversation | `GET /{conversation-id}/messages` | 同上 |
| User profile (PSID) | `GET /{psid}?fields=name,profile_pic` | `pages_messaging` |

**Tài liệu tham khảo:**
- [Conversations API](https://developers.facebook.com/docs/graph-api/reference/conversation/)
- [Conversations API for Messenger](https://developers.facebook.com/docs/messenger-platform/conversations/)
- [Page Access Token](https://developers.facebook.com/docs/pages/access-tokens)

### Bước 2: Cấu trúc package mới

```
api/internal/
├── api/
│   └── meta/                          # Package mới
│       ├── client/
│       │   └── client.meta.graph.go   # HTTP client gọi Meta Graph API
│       ├── service/
│       │   └── service.meta.sync.go   # Logic đồng bộ: fetch → map → upsert
│       └── dto/
│           └── dto.meta.graph.go      # Struct parse response Meta
└── worker/
    └── meta_sync_worker.go            # Job chạy định kỳ
```

### Bước 3: Meta Graph Client

```go
// client.meta.graph.go
package meta

const GraphAPIBase = "https://graph.facebook.com/v21.0"

type MetaGraphClient struct {
    httpClient *http.Client
}

// GetConversations lấy danh sách conversations của page
func (c *MetaGraphClient) GetConversations(ctx context.Context, pageID, accessToken string, limit int) (*ConversationsResponse, error)

// GetConversationDetail lấy chi tiết conversation (messages, participants)
func (c *MetaGraphClient) GetConversationDetail(ctx context.Context, conversationID, accessToken string) (*ConversationDetailResponse, error)

// GetUserProfile lấy thông tin user theo PSID (optional)
func (c *MetaGraphClient) GetUserProfile(ctx context.Context, psid, accessToken string) (*UserProfileResponse, error)
```

### Bước 4: Map Meta response → panCakeData format

Meta API trả về format khác Pancake. Cần **mapper** chuyển sang format mà `FbConversation`, `FbCustomer`, `FbMessage` đang dùng:

| Meta API field | panCakeData / FbConversation |
|----------------|------------------------------|
| `id` (conversation) | `conversationId`, `panCakeData.id` |
| `participants[].id` (PSID) | `customerId` (pageId_psid hoặc id nội bộ), `panCakeData.psid` |
| `participants[].name` | `panCakeData.name` |
| `updated_time` | `panCakeData.updated_at`, `panCakeUpdatedAt` |
| `messages[].id` | `messageData.id` trong fb_message_items |
| `messages[].created_time` | `messageData.created_time` |

**Lưu ý:** Pancake dùng `customerId` là ID nội bộ của Pancake. Meta dùng `pageId_psid` (page-scoped ID). Cần quy ước:
- **customerId** = `pageId + "_" + psid` (format `pageId_psid`) khi sync từ Meta trực tiếp.
- Hoặc tạo mapping table nếu cần đồng bộ với Pancake.

### Bước 5: Meta Sync Service

```go
// service.meta.sync.go
package metasvc

// SyncPageConversations đồng bộ conversations + messages của một page
// - Lấy page từ fb_pages (pageId, pageAccessToken, ownerOrganizationId)
// - Gọi Meta API: conversations → từng conversation detail (messages, participants)
// - Map → FbConversation, FbCustomer, FbMessage
// - Gọi FbConversationService.SyncUpsertOne, FbCustomerService.SyncUpsertOne, FbMessageService.UpsertMessages
func (s *MetaSyncService) SyncPageConversations(ctx context.Context, pageID string) (convCount, msgCount int, err error)

// SyncAllPages chạy SyncPageConversations cho tất cả fb_pages có pageAccessToken
func (s *MetaSyncService) SyncAllPages(ctx context.Context) (totalConvs, totalMsgs int, err error)
```

### Bước 6: Worker / Cron Job

```go
// meta_sync_worker.go (trong api-worker hoặc service riêng)
// Chạy mỗi 5–15 phút (tùy rate limit Meta)
func RunMetaSyncJob(ctx context.Context) {
    svc, _ := metasvc.NewMetaSyncService()
    totalConvs, totalMsgs, err := svc.SyncAllPages(ctx)
    // Log kết quả
}
```

Đăng ký trong `api-worker` tương tự `crm_ingest_worker` hoặc job khác.

---

## 📋 Checklist triển khai

### Phase 1: Chuẩn bị

- [ ] Tạo Meta App trên [developers.facebook.com](https://developers.facebook.com)
- [ ] Cấu hình permissions: `pages_messaging`, `pages_manage_metadata`, `pages_read_engagement`
- [ ] Lấy Page Access Token (long-lived) cho từng page
- [ ] Lưu token vào `fb_pages.pageAccessToken` (đã có sẵn)

### Phase 2: Code

- [ ] Tạo package `api/internal/api/meta/`
- [ ] Implement `MetaGraphClient` (GetConversations, GetConversationDetail)
- [ ] Implement mapper Meta → panCakeData format
- [ ] Implement `MetaSyncService.SyncPageConversations`
- [ ] Tích hợp `SyncUpsertOne` cho FbConversation, FbCustomer, FbMessage
- [ ] Tạo `meta_sync_worker` và đăng ký cron

### Phase 3: Cấu hình & vận hành

- [ ] Cấu hình interval sync (ví dụ: 10 phút)
- [ ] Xử lý rate limit Meta (429) — backoff, giới hạn request/page
- [ ] Log và monitor lỗi token hết hạn
- [ ] Cập nhật `PUT /facebook/page/update-token` khi token refresh

---

## ⚠️ Lưu ý quan trọng

### 1. Trùng lặp với Pancake

Nếu **cả Pancake và Meta sync** cùng chạy cho cùng page:
- Có thể trùng conversation/message.
- **Giải pháp**: Dùng `SyncUpsertOne` (ghi khi mới hơn) — đã có sẵn.
- Hoặc: Chỉ bật Meta sync cho pages **không** dùng Pancake.

### 2. Rate limit Meta

- Meta giới hạn số request/giờ theo app.
- Cần: throttle, batch, tránh gọi quá nhiều conversation cùng lúc.
- Cân nhắc: sync từng page theo thứ tự, delay giữa các page.

### 3. Token hết hạn

- Page Access Token có thể hết hạn (60 ngày nếu long-lived).
- Cần flow refresh token và cập nhật `fb_pages.pageAccessToken`.
- Log cảnh báo khi API trả 190 (token expired).

### 4. Format participant

- Meta: `participants` có `id` (PSID), `name`, `email` (page messaging).
- Pancake: `customer.id`, `customers[0].id` — có thể là ID nội bộ.
- Cần mapping rõ: `customerId` = `pageId_psid` khi sync từ Meta.

---

## 📚 Tài liệu tham khảo

- [Meta Graph API - Conversation](https://developers.facebook.com/docs/graph-api/reference/conversation/)
- [Conversations API for Messenger](https://developers.facebook.com/docs/messenger-platform/conversations/)
- [Page Access Tokens](https://developers.facebook.com/docs/pages/access-tokens)
- [Webhook Pancake (hiện tại)](../03-api/webhook-pancake.md)
- [Sync flow Pancake/POS](../02-architecture/systems/) — service.crm.sync.go, handler.pancake.webhook.go

---

## 📝 Tóm tắt

### Meta Ads (ưu tiên)

| Hạng mục | Nội dung |
|----------|----------|
| **API** | Meta Marketing API (Ads Management) |
| **Token** | User/System token với `ads_read`, `ads_management` |
| **Cấu trúc** | Ad Account → Campaign → Ad Set → Ad |
| **Insights** | `GET /act_xxx/insights` — spend, impressions, clicks, cpc, ctr... |
| **Collections** | meta_ad_accounts, meta_campaigns, meta_adsets, meta_ads, meta_ad_insights |
| **Logic tự động** | Rules (pause khi CPA/CTR vượt ngưỡng), budget optimization, mapping ad_ids → CRM |

### Meta Conversations (phase 2)

| Hạng mục | Nội dung |
|----------|----------|
| **Cơ chế** | Pull (worker gọi Meta API định kỳ) |
| **Format lưu** | Map Meta response → `panCakeData` (giống Pancake) |
| **Collections** | fb_conversations, fb_customers, fb_messages |
| **CRM** | Dùng lại hooks/merge hiện có, không đổi |
| **Token** | `fb_pages.pageAccessToken` |
