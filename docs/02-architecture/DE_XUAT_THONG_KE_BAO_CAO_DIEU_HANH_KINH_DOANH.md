# Đề xuất: Tổ chức thống kê & báo cáo phục vụ điều hành kinh doanh

Tài liệu dựa trên dữ liệu mẫu và cấu trúc backend hiện tại, đề xuất cách tổ chức thống kê và báo cáo phục vụ điều hành kinh doanh (FolkForm / Folk Group).

---

## 0. Dữ liệu mẫu (Sample Data)

| Mục | Nội dung |
|-----|----------|
| **Vị trí** | Dữ liệu mẫu **đã có sẵn** tại thư mục `docs-shared/ai-context/folkform/sample-data` (các file `{collection}-sample.json`). |
| **Làm mới (tùy chọn)** | Chạy script `scripts/export_sample_documents.go` (từ thư mục gốc hoặc `api/`) — kết nối MongoDB theo config env, export mỗi collection tối đa 10 document ra thư mục trên. |
| **Phân tích cấu trúc** | Script `scripts/analyze_data_structure.go` in ra cấu trúc thực tế (field, kiểu, độ sâu) của các collection: `pc_pos_*`, `fb_*`, `customers`, v.v. |
| **Collections trong sample-data** | Auth/RBAC, Facebook, POS (pc_pos_*), Content (content_*, content_draft_*), AI (ai_workflows, ai_steps, ai_workflow_commands, ai_workflow_runs, ai_step_runs, …), Notification, Delivery, CTA, Agent, Webhook, Access tokens. |

Khi triển khai báo cáo, nên đối chiếu với model Go trong `api/internal/api/models/mongodb/` và với file JSON mẫu trong thư mục này để dùng đúng tên trường (xem mục 5).

---

## 1. Tổng quan nguồn dữ liệu

| Nhóm | Collection / Nguồn | Mục đích báo cáo |
|------|--------------------|------------------|
| **Bán hàng (POS)** | `pc_pos_orders`, `pc_pos_products`, `pc_pos_shops`, `pc_pos_customers`, `pc_pos_variations`, `pc_pos_warehouses` | Doanh thu, đơn hàng, sản phẩm, shop, khách hàng |
| **Nội dung & Draft** | `content_nodes`, `content_draft_nodes`, `content_draft_publications`, `content_draft_videos`, `content_videos` | Pipeline nội dung, trạng thái duyệt, xuất bản |
| **AI / Workflow** | `ai_workflows`, `ai_steps`, `ai_workflow_commands`, (ai_workflow_runs), `ai_prompt_templates` | Sử dụng workflow, bước AI, lệnh chờ xử lý |
| **Agent** | `agent_activity_logs`, `agent_commands`, `agent_registry`, `agent_configs` | Hoạt động bot, check-in, job |
| **Facebook / Inbox** | `fb_conversations`, `fb_messages`, `fb_message_items`, `fb_customers`, `fb_pages`, `fb_posts` | Hội thoại, tin nhắn, page, bài đăng |
| **Thông báo** | `delivery_history`, `notification_*` | Gửi thông báo, CTA, sự kiện (ví dụ: conversation_unreplied) |
| **Tổ chức & Người dùng** | `auth_organizations`, `auth_users`, `auth_user_roles`, `auth_roles` | Đa tổ chức, phân quyền, filter theo org |

Tất cả báo cáo cần **lọc theo `ownerOrganizationId`** (và có thể theo cây tổ chức `auth_organizations`: group → company → team) để phục vụ từng đơn vị kinh doanh.

---

## 2. Đề xuất nhóm báo cáo (Dashboard / Report nhóm)

### 2.1. Báo cáo Bán hàng (Sales / POS)

**Nguồn chính:** `pc_pos_orders`, `pc_pos_products`, `pc_pos_shops`, `pc_pos_customers`.

- **Chỉ số gợi ý:**
  - Doanh thu: tổng từ `posData.total_price` hoặc `total_price_after_sub_discount` / `transfer_money` (theo quy ước nghiệp vụ), theo ngày/tuần/tháng, theo shop.
  - Số đơn: đếm order theo `status`/`statusName` (ví dụ: new, completed, cancelled), theo shop, theo khoảng thời gian.
  - Đơn đã thanh toán: filter `paidAt > 0` hoặc status tương đương “đã thanh toán”.
  - Sản phẩm bán chạy: aggregate từ `posData.items` (product_id / variation_id, quantity), theo shop, theo thời gian.
  - Khách hàng: số khách mới, số đơn/khách (theo `customerId`), có thể gắn với `fb_customers`/page nếu có link.
- **Phân chiều:**
  - Thời gian: ngày, tuần, tháng, quý.
  - Tổ chức: `ownerOrganizationId`, shop (`shopId`), kho (`warehouseId`) nếu cần.
- **API đề xuất:** ví dụ `GET /api/reports/sales/summary`, `GET /api/reports/sales/orders`, `GET /api/reports/sales/top-products` với query `from`, `to`, `shopId`, `ownerOrganizationId`.

---

### 2.2. Báo cáo Nội dung & Draft (Content pipeline)

**Nguồn:** `content_nodes`, `content_draft_nodes`, `content_draft_publications`, `content_draft_videos`.

- **Chỉ số gợi ý:**
  - Số node theo loại: `type` (layer, stp, insight, contentLine, gene, script), theo org, theo thời gian tạo.
  - Pipeline duyệt: đếm draft theo `approvalStatus` (draft, pending, approved, rejected), theo thời gian.
  - Xuất bản: đếm publication theo `platform`, `status`; có thể gắn với draft video.
- **Phân chiều:**
  - Thời gian (createdAt, updatedAt), `ownerOrganizationId`, `type`, `approvalStatus`, `platform`.
- **API đề xuất:** `GET /api/reports/content/summary`, `GET /api/reports/content/drafts-by-status`, `GET /api/reports/content/publications`.

---

### 2.3. Báo cáo AI / Workflow

**Nguồn:** `ai_workflows`, `ai_steps`, `ai_workflow_commands` (và khi có: ai_workflow_runs, ai_step_runs).

- **Chỉ số gợi ý:**
  - Số workflow/step theo trạng thái (`status`: active, archived, draft).
  - Lệnh chờ xử lý: đếm command theo `status` (pending, executing, completed, failed, cancelled), có thể theo workflow/step.
  - Số lần chạy workflow/step (khi có collection runs): thành công / thất bại / đang chạy.
- **Phân chiều:**
  - `ownerOrganizationId`, workflowId, stepId, `status`, thời gian (createdAt, assignedAt, completedAt).
- **API đề xuất:** `GET /api/reports/ai/workflow-summary`, `GET /api/reports/ai/commands-queue`, `GET /api/reports/ai/runs` (khi đã có runs).

---

### 2.4. Báo cáo Inbox / Hội thoại (Facebook)

**Nguồn:** `fb_conversations`, `fb_messages`, `fb_message_items`, `fb_customers`, `fb_pages`.

- **Chỉ số gợi ý:**
  - Số hội thoại theo page, theo khoảng thời gian; số tin nhắn (totalMessages trong conversation hoặc đếm message items).
  - Thời gian phản hồi: cần xác định quy tắc (tin nhắn đầu vào → tin nhắn đầu ra), có thể kết hợp `delivery_history` (eventType conversation_unreplied, severity).
  - Số khách (fb_customers) theo page.
- **Phân chiều:**
  - `ownerOrganizationId`, `pageId`, ngày/tuần/tháng.
- **API đề xuất:** `GET /api/reports/inbox/conversations-summary`, `GET /api/reports/inbox/messages-volume`, `GET /api/reports/inbox/response-delay` (khi đủ dữ liệu).

#### 2.4.1. Dữ liệu trong hội thoại liên quan nhân viên (đã có trong mẫu)

Trong `fb_conversations`, trường `panCakeData` chứa đủ thông tin để thống kê / đánh giá nhân viên sale:

| Dữ liệu | Vị trí trong mẫu | Ý nghĩa |
|--------|-------------------|--------|
| **Tag NV.xx** | `panCakeData.tags[]` — mỗi tag có `text` (ví dụ `"NV9"`, `"NV11"`, `"NV15"`). | Tag nhân viên gắn trên hội thoại (quy ước NV9 = nhân viên 9). Có trong mẫu. |
| **Lịch sử tag** | `panCakeData.tag_histories[]` — mỗi bản ghi có `payload.editor_id`, `payload.editor_name`, `payload.tag.text`. | Ai add/remove tag nào, lúc nào → biết ai gán hội thoại cho NV nào. |
| **Người gửi tin cuối** | `panCakeData.last_sent_by` — có `admin_name` (ví dụ "Tuyết Nhi", "Nguyễn Quỳnh Như"), `uid` (editor id). | Nhân viên gửi tin nhắn cuối trong hội thoại. |
| **Phân công (assignee)** | `panCakeData.assignee_ids`, `current_assign_users`, `assignee_histories`. | Cơ chế phân công chính thức của Pancake; trong mẫu đang là mảng rỗng, nhưng schema có sẵn — khi Pancake sync đầy đủ sẽ có người được assign. |

Trong `fb_message_items`, mỗi tin nhắn có `messageData.from`:

- Tin từ **page (nhân viên)**: `from.admin_name`, `from.uid` (ví dụ "Tuyết Nhi", "0d485290-3899-4e1d-b4d2-934b7615cf2b").
- Tin từ **khách**: `from.id`, `from.name` (customer).

→ Có thể **đếm số tin nhắn do từng nhân viên gửi** (theo `uid` hoặc `admin_name`), theo hội thoại / page / khoảng thời gian.

#### 2.4.2. Báo cáo đánh giá nhân viên sale (Inbox)

**Có thể thống kê đánh giá làm việc của nhân viên sale** với dữ liệu hiện tại:

- **Theo tag NV.xx (hội thoại “gán” cho NV):**
  - Đếm số hội thoại có tag `text` match pattern NV.xx (NV9, NV11, NV15, …) → số hội thoại đang gán cho từng mã NV.
  - Có thể kết hợp `tag_histories`: ai add tag NV.xx (editor_id / editor_name) → thống kê ai gán tag cho ai.
- **Theo người chat (last_sent_by):**
  - Số hội thoại mà nhân viên X là người gửi tin cuối (`panCakeData.last_sent_by.uid` hoặc `admin_name`), theo page, theo thời gian.
- **Theo từng tin nhắn (fb_message_items):**
  - Đếm số tin nhắn do từng nhân viên gửi: filter `messageData.from.uid` (hoặc `admin_name`) có giá trị, group by uid → số tin / NV, theo conversation, page, ngày.
- **Khi có assignee:** Khi `assignee_histories` / `current_assign_users` được sync đầy đủ từ Pancake, có thể thêm chỉ số: số hội thoại được assign cho từng nhân viên, thời gian xử lý.

**Lưu ý triển khai:**

- Cần **mapping tag "NV9" ↔ nhân viên thật** (user id / email trong `auth_users` hoặc config) nếu muốn báo cáo theo tên/account thống nhất.
- Backend lưu toàn bộ trong `panCakeData` (map), nên aggregate MongoDB trên `panCakeData.tags`, `panCakeData.last_sent_by`, `panCakeData.tag_histories`; với message items dùng `messageData.from.uid` / `admin_name`.

**API đề xuất:** `GET /api/reports/inbox/staff-summary` (số hội thoại/tin theo NV, theo tag NV.xx hoặc theo uid), `GET /api/reports/inbox/staff-messages` (số tin nhắn theo nhân viên, theo khoảng thời gian).

---

### 2.5. Báo cáo Thông báo & Cảnh báo

**Nguồn:** `delivery_history`, `notification_*`.

- **Chỉ số gợi ý:**
  - Số gửi theo `channelType` (telegram, email, …), `status` (sent, failed), `domain`, `eventType` (ví dụ conversation_unreplied).
  - Tỷ lệ mở/click (openCount, clickCount, ctaClicks) nếu dùng cho đánh giá hiệu quả thông báo.
- **Phân chiều:**
  - `ownerOrganizationId`, kênh, loại sự kiện, ngày.
- **API đề xuất:** `GET /api/reports/notifications/summary`, `GET /api/reports/notifications/events`.

---

### 2.6. Báo cáo Agent (Vận hành hệ thống)

**Nguồn:** `agent_activity_logs`, `agent_commands`, `agent_registry`.

- **Chỉ số gợi ý:**
  - Số hoạt động theo `activityType` (check_in, job_completed, …), theo agent, theo thời gian.
  - Trạng thái agent: online/offline dựa trên check-in gần nhất (từ registry hoặc activity).
  - Hàng đợi lệnh: đã có trong nhóm AI workflow commands.
- **Phân chiều:**
  - agentId, activityType, ngày/giờ.
- **API đề xuất:** `GET /api/reports/agents/activity-summary`, `GET /api/reports/agents/status`.

---

## 3. Tổ chức API và module backend

- **Prefix chung:** ví dụ `/api/reports/...` hoặc `/api/stats/...`, bảo vệ bằng RBAC (ví dụ permission `reports:read` hoặc theo từng nhóm báo cáo).
- **Query chuẩn:**
  - `from`, `to` (timestamp hoặc date ISO).
  - `ownerOrganizationId` (bắt buộc hoặc lấy từ context); có thể hỗ trợ `includeChildren=true` để gộp cả công ty con/team con.
  - Tùy báo cáo: `shopId`, `pageId`, `workflowId`, v.v.
- **Định dạng trả về:** JSON thống nhất (theo chuẩn dự án), có thể hỗ trợ `groupBy=day|week|month` cho các báo cáo theo thời gian.

Gợi ý cấu trúc thư mục:

- `api/internal/api/handler/handler.report.*.go` (hoặc `handler.stats.*.go`) — từng nhóm báo cáo.
- `api/internal/api/services/service.report.*.go` — logic aggregate, gọi repository.
- `api/internal/api/models/mongodb` — tái sử dụng model hiện có; nếu cần có thể thêm model read-only cho view/aggregation.

---

## 4. Giai đoạn triển khai đề xuất

1. **Phase 1 – Cốt lõi điều hành**
   - Báo cáo bán hàng (doanh thu, số đơn, theo shop, theo thời gian).
   - Báo cáo tổng quan inbox (số hội thoại, số tin nhắn theo page/ngày).
   - Endpoint tổng quan: một dashboard summary (số liệu rút gọn cho 1 org, 1 khoảng thời gian).

2. **Phase 2 – Nội dung & AI**
   - Content pipeline (draft theo status, publication).
   - AI: workflow/command queue, khi có runs thì thêm báo cáo runs.

3. **Phase 3 – Chi tiết & tối ưu**
   - Thời gian phản hồi inbox, báo cáo thông báo/CTA, báo cáo agent.
   - Cache/aggregation định kỳ nếu volume lớn (ví dụ collection pre-aggregate theo ngày/tuần).

---

## 5. Lưu ý với dữ liệu mẫu và đối chiếu model backend

### 5.1. Nguồn dữ liệu mẫu

- **pc_pos_orders (model: `PcPosOrder`):** Model có sẵn: `ShopId`, `Status`, `StatusName`, `InsertedAt`, `PaidAt`, `TotalDiscount`, `OrderItems`, `OwnerOrganizationID`, `CreatedAt`. Doanh thu cần lấy từ **`posData`** (map): `posData["total_price"]`, `posData["total_price_after_sub_discount"]`, `posData["transfer_money"]` (theo quy ước nghiệp vụ). Sản phẩm bán chạy: aggregate từ `OrderItems` hoặc `posData["order_items"]` (product_id / variation_id, quantity). Đủ để báo cáo doanh thu/đơn theo shop và thời gian.
- **content_nodes (model: `ContentNode`):** Có `Type` (pillar, stp, insight, contentLine, gene, script), `OwnerOrganizationID`, `CreatedAt`, `Status` — đủ để thống kê theo loại và org.
- **content_draft_* (model: `DraftContentNode`):** Có `ApprovalStatus` (draft, pending, approved, rejected), `Type`, `OwnerOrganizationID`, `CreatedAt`. Draft publication/video có `status`, `platform` — đủ cho pipeline duyệt và xuất bản.
- **delivery_history (model: `DeliveryHistory`):** Có `ChannelType`, `EventType`, `Status`, `SentAt`, `Severity`, `Domain`, `OpenCount`, `ClickCount`, `CTAClicks` — đủ cho thống kê gửi thông báo và sự kiện (ví dụ conversation_unreplied), tỷ lệ mở/click.
- **auth_organizations:** Có cấu trúc `parentId`, `path`, `type` (system, team, group, company) — dùng để filter hoặc roll-up theo tổ chức con.
- **fb_conversations / fb_messages:** Có `totalMessages`, `pageId`, `ownerOrganizationId`; trong conversation có `panCakeData` (tags, last_sent_by, tag_histories, assignee) — đủ cho volume hội thoại/tin nhắn và báo cáo nhân viên sale; thời gian phản hồi cần thêm quy tắc và trường (nếu chưa có).

### 5.2. Collections đã có trong code

- **ai_workflow_runs / ai_step_runs:** Đã có trong script export và registry; khi dùng được trong API thì thêm ngay nhóm báo cáo AI runs vào Phase 2.

---

## 6. Tóm tắt

- **6 nhóm báo cáo:** Sales (POS), Content & Draft, AI/Workflow, Inbox, Notifications, Agents.
- **Chuẩn hóa:** Lọc theo `ownerOrganizationId` (và cây org), query `from`/`to`, RBAC.
- **Triển khai:** Bắt đầu bằng Sales + Inbox summary + dashboard tổng quan (Phase 1), sau đó Content + AI (Phase 2), cuối cùng chi tiết và tối ưu (Phase 3).

Tài liệu này có thể dùng làm baseline để thiết kế API, permission và backlog cho từng phase.
