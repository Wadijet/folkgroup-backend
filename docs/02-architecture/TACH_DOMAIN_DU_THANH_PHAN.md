# Tách domain đủ thành phần (handler, dto, service)

## Hiện trạng

- **Đã tách:** Chỉ **service** (data layer) vào từng domain folder:
  - `api/internal/api/cta/` → package **ctasvc** (library, tracking)
  - `api/internal/api/delivery/` → package **deliverysvc** (queue, history)
- **Chưa tách:** **dto**, **model**, **handler** vẫn nằm chung:
  - `api/internal/api/dto/` – toàn bộ DTO mọi domain
  - `api/internal/api/models/mongodb/` – toàn bộ model
  - `api/internal/api/handler/` – toàn bộ handler

## Mục tiêu (Option A trong đề xuất)

Mỗi domain có **đủ** handler, service, dto trong một cây thư mục; model có thể giữ chung (nhiều nơi tham chiếu).

Ví dụ cấu trúc (Delivery đã áp dụng):

```
api/internal/api/
├── handler/          # Base handler (dùng chung) – BaseHandler, SafeHandlerWrapper, base.crud, base.response
├── services/         # Base service (dùng chung) – BaseServiceMongoImpl, NewBaseServiceMongo
├── delivery/
│   ├── service/      # package deliverysvc – service.delivery.queue.go, service.delivery.history.go (embed BaseServiceMongoImpl)
│   ├── dto/          # package deliverydto – dto.delivery.send.go, dto.delivery.tracking.go
│   └── handler/      # package deliveryhdl – handler.delivery.send.go, handler.delivery.tracking.go (gọi handler.SafeHandlerWrapper)
├── cta/
│   ├── service/      # (ctasvc đã có; có thể thêm folder service/ tương tự)
│   ├── handler/
│   └── dto/
├── models/           # Giữ chung (mongodb)
├── router/
└── middleware/
```

## Lợi ích

- Tìm mọi thứ của một domain trong một folder.
- Thêm/sửa domain ít đụng tới domain khác.
- Chuẩn bị tách service sau: copy cả thư mục domain sang repo mới.

## Base handler và base service (dùng chung)

**Base không tách vào từng domain** – giữ ở package dùng chung để mọi domain dùng lại:

| Thành phần | Vị trí | Cách domain dùng |
|------------|--------|-------------------|
| **Base handler** | `api/internal/api/handler/` (handler.base.go, handler.base.crud.go, handler.base.response.go) | Handler theo domain **import** `handler.SafeHandlerWrapper`, `handler.NewBaseHandler`, embed `handler.BaseHandler[T, CreateInput, UpdateInput]` khi cần CRUD. |
| **Base service** | `api/internal/api/services/` (service..base.mongo.go) | Service theo domain **embed** `*services.BaseServiceMongoImpl[T]`, gọi `services.NewBaseServiceMongo(collection)`, dùng `services.UpdateData` khi cần. |

Domain **không** copy base vào domain folder. Domain handler nằm trong `delivery/handler/` nhưng vẫn import từ `meta_commerce/internal/api/handler`; domain service nằm trong `delivery/service/` và embed từ `meta_commerce/internal/api/services`.

## Lưu ý kỹ thuật khác

- **Model** giữ trong `models/mongodb` để tránh thay đổi import ồ ạt và tránh phụ thuộc vòng.
- **Router** gọi constructor theo domain: `deliveryhdl.NewDeliverySendHandler()`, `deliverysvc.NewDeliveryQueueService()` (import từ `delivery/service`), v.v.

## Quy ước tên file (giữ cấu trúc cũ)

Trong từng domain folder, **giữ tên file** theo cấu trúc cũ để dễ phân biệt với các domain khác:

- **Service:** `service.<domain>.<entity>.go` (vd: `service.delivery.queue.go`, `service.delivery.history.go`)
- **DTO:** `dto.<domain>.<entity>.go` (vd: `dto.delivery.send.go`, `dto.delivery.tracking.go`)
- **Handler:** `handler.<domain>.<entity>.go` (vd: `handler.delivery.send.go`, `handler.delivery.tracking.go`)

Package name có thể là `deliverysvc`, `deliverydto`, `deliveryhdl` tùy thư mục.

## Lộ trình áp dụng

1. **Thí điểm 1 domain (Delivery) – đã làm (đủ service/ + dto/ + handler/, tên file cấu trúc cũ):**
   - **api/internal/api/delivery/service/** (package `deliverysvc`): `service.delivery.queue.go`, `service.delivery.history.go` – folder riêng cho service, đối xứng với dto/, handler/.
   - **api/internal/api/delivery/dto/** (package `deliverydto`): `dto.delivery.send.go`, `dto.delivery.tracking.go`.
   - **api/internal/api/delivery/handler/** (package `deliveryhdl`): `handler.delivery.send.go`, `handler.delivery.tracking.go`. Handler History (NotificationHistoryHandler) giữ trong `handler` vì gắn BaseHandler + filterOptions nội bộ.
   - Import deliverysvc: `meta_commerce/internal/api/delivery/service`. Base handler ở `api/handler`, base service ở `api/services` (domain service embed BaseServiceMongoImpl từ đây).
   - Router: gọi `deliveryhdl.NewDeliverySendHandler()`, `deliveryhdl.NewTrackingHandler()`, History vẫn `handler.NewNotificationHistoryHandler()`.
2. **Domain CTA – đã làm (đủ service/ + dto/ + handler/):**
   - **api/internal/api/cta/service/** (package ctasvc): service.cta.library.go, service.cta.tracking.go.
   - **api/internal/api/cta/dto/** (package ctadto): dto.cta.library.go.
   - **api/internal/api/cta/handler/** (package ctahdl): handler.cta.library.go (dùng handler.SetFilterOptions).
   - Router dùng ctahdl.NewCTALibraryHandler(); internal/cta/renderer và handler.admin.init dùng ctasvc từ cta/service.
4. **Domain Report – đã làm (đủ service/ + dto/ + handler/):**
   - **api/internal/api/report/service/** (package reportsvc): service.report.go, service.report.engine.go (ReportService, Compute, MarkDirty, FindSnapshotsForTrend, GetUnprocessedDirtyPeriods, SetDirtyProcessed). Không embed BaseServiceMongoImpl (dùng 3 collection: definitions, snapshots, dirty_periods).
   - **api/internal/api/report/dto/** (package reportdto): dto.report.go (ReportTrendQuery, ReportRecomputeBody).
   - **api/internal/api/report/handler/** (package reporthdl): handler.report.go (HandleTrend, HandleRecompute). Gọi handler.SafeHandlerWrapper.
   - Router (facebook_routes.go) dùng reporthdl.NewReportHandler(); worker report_dirty_worker và service.pc.pos.order dùng reportsvc từ report/service.
   - Đã xóa api/handler/handler.report.go, api/services/service.report.go, service.report.engine.go, api/dto/dto.report.go.
5. **Domain Webhook – đã làm (đủ service/ + dto/ + handler/):**
   - **api/internal/api/webhook/service/** (package `webhooksvc`): `service.webhook.log.go` (WebhookLogService, CreateWebhookLog, UpdateProcessedStatus). Embed `*services.BaseServiceMongoImpl`, dùng `s.Collection()` thay vì `s.collection` khi gọi UpdateOne.
   - **api/internal/api/webhook/dto/** (package `webhookdto`): `dto.webhook.log.go` (WebhookLogCreateInput, WebhookLogUpdateInput).
   - **api/internal/api/webhook/handler/** (package `webhookhdl`): `handler.webhook.log.go` (WebhookLogHandler). Router (facebook_routes) dùng `webhookhdl.NewWebhookLogHandler()`. Handler pancake webhook và pancake pos webhook import `webhooksvc` để gọi CreateWebhookLog.
   - Đã xóa api/handler/handler.webhook.log.go, api/services/service.webhook.log.go, api/dto/dto.webhook.log.go.
6. **Nhân rộng:** Mỗi domain có **service/**, **dto/**, **handler/** (folder riêng); base handler và base service giữ ở api/handler và api/services.

## Các domain còn lại cần tách (checklist)

Áp dụng cùng quy trình: tạo `<domain>/service/`, `<domain>/dto/`, `<domain>/handler/`, đổi package sang `<domain>svc`, `<domain>dto`, `<domain>hdl`, import base từ `handler` và `services`, cập nhật router và mọi chỗ tham chiếu, xóa file cũ.

| Domain | Service files (→ api/internal/api/<domain>/service/) | DTO files (→ dto/) | Handler files (→ handler/) | Router / tham chiếu |
|--------|------------------------------------------------------|--------------------|----------------------------|----------------------|
| **Auth** | service.auth.user, role, permission, role_permission, user_role, organization, organization.config.item, organization.helper, organization.share | dto.auth.*, dto.organization.* | handler.auth.*, handler.organization.* | auth_routes.go; handler.admin, handler.admin.init, middleware.auth, middleware.organization_context dùng authsvc |
| **Agent** | service.agent.activity, command, config, management, registry (+ job.helper) | dto.agent.* | handler.agent.* | agent_routes.go |
| **AI** | service.ai.* (workflow, step, prompt.template, provider.profile, run, step.run, step.mapping, generation.batch, candidate, workflow.command) | dto.ai.* | handler.ai.* | ai_routes.go |
| **Content** | service.content.*, service.draft.* | dto.content.*, dto.draft.* | handler.content.*, handler.draft.* | content_routes.go |
| **Notification** | service.notification.channel, routing, sender, template (không có trigger service?) | dto.notification.* | handler.notification.* (history có thể giữ trong handler chung nếu dùng BaseHandler) | notification_routes.go |
| **Facebook + PC** | service.fb.*, service.pc.* (page, post, conversation, message, customer; access_token, order; pos.*) | dto.fb.*, dto.pc.*, dto.pancake.* | handler.fb.*, handler.pc.*, handler.pancake.* | facebook_routes.go; handler.pancake.* đã dùng webhooksvc |

**Lưu ý khi tách Auth:** UserService gọi `services.NewInitService()`; authsvc cần import `meta_commerce/internal/api/services` cho Base, UpdateData và NewInitService. organization.helper dùng nhiều auth service → nằm trong authsvc.
