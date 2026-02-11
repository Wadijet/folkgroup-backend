# Cấu trúc mới đã áp dụng & Chuẩn các công ty lớn

## 1. Cấu trúc mới đã làm trong dự án

### 1.1 Router tách theo domain (đã làm)

- **Trước:** Một file `routes.go` ~680 dòng, đăng ký toàn bộ route.
- **Sau:** Một file `routes.go` chỉ giữ types, config, `SetupRoutes`; từng nhóm route nằm trong file riêng theo domain:

| File | Domain |
|------|--------|
| `auth_routes.go` | Admin, System, Auth, RBAC, Init |
| `facebook_routes.go` | Facebook, Pancake, Webhook, Report |
| `notification_routes.go` | Notification, Tracking |
| `cta_routes.go` | CTA Library |
| `delivery_routes.go` | Delivery Send, History |
| `agent_routes.go` | Agent Management |
| `content_routes.go` | Content Storage |
| `ai_routes.go` | AI Service |

**Lợi ích:** Dễ tìm, giảm conflict khi nhiều người sửa, thêm domain mới chỉ thêm file mới.

### 1.2 Tách domain CTA (đã làm, đủ service/ + dto/ + handler/)

- **api/internal/api/cta/service/** (package **ctasvc**): Service – `service.cta.library.go`, `service.cta.tracking.go` (CTALibraryService, CTATrackingService). Embed BaseServiceMongoImpl từ api/services.
- **api/internal/api/cta/dto/** (package **ctadto**): DTO – `dto.cta.library.go` (CTALibraryCreateInput, CTALibraryUpdateInput, CTAActionParams).
- **api/internal/api/cta/handler/** (package **ctahdl**): Handler – `handler.cta.library.go` (CTALibraryHandler). Gọi handler.NewBaseHandler, SetFilterOptions.
- **api/internal/cta/** (package **cta**): Logic CTA – render, track click (renderer.go, helpers.go). Import ctasvc từ `api/internal/api/cta/service`.
- **Base:** Giữ ở api/handler, api/services; ctahdl dùng SetFilterOptions để cấu hình filter.

### 1.2b Tách domain Delivery (đã làm, đủ service/ + dto/ + handler/, base giữ chung)

- **api/internal/api/delivery/service/** (package **deliverysvc**): Service – `service.delivery.queue.go`, `service.delivery.history.go` (folder riêng, đối xứng với dto/, handler/). Embed `services.BaseServiceMongoImpl` từ api/services.
- **api/internal/api/delivery/dto/** (package **deliverydto**): DTO – `dto.delivery.send.go`, `dto.delivery.tracking.go`.
- **api/internal/api/delivery/handler/** (package **deliveryhdl**): Handler – `handler.delivery.send.go`, `handler.delivery.tracking.go`. Gọi `handler.SafeHandlerWrapper` từ api/handler. Handler History (CRUD delivery history) giữ trong `handler` vì dùng BaseHandler + filterOptions.
- **api/internal/delivery/** (package **delivery**): Logic delivery – Queue, Processor, channels. Import deliverysvc từ `api/internal/api/delivery/service`.
- **Base handler / base service:** Không tách vào domain; giữ ở **api/handler** (BaseHandler, SafeHandlerWrapper) và **api/services** (BaseServiceMongoImpl). Domain handler import handler.*, domain service embed services.*.

### 1.2c Tách domain Report (đã làm, đủ service/ + dto/ + handler/)

- **api/internal/api/report/service/** (package **reportsvc**): Service – `service.report.go`, `service.report.engine.go` (ReportService: definitions, snapshots, dirty_periods; Compute, MarkDirty, FindSnapshotsForTrend, GetUnprocessedDirtyPeriods, SetDirtyProcessed). Không embed BaseServiceMongoImpl.
- **api/internal/api/report/dto/** (package **reportdto**): DTO – `dto.report.go` (ReportTrendQuery, ReportRecomputeBody).
- **api/internal/api/report/handler/** (package **reporthdl**): Handler – `handler.report.go` (HandleTrend, HandleRecompute). Gọi handler.SafeHandlerWrapper.
- Router (facebook_routes.go) dùng reporthdl.NewReportHandler(); worker report_dirty_worker và service.pc.pos.order import reportsvc từ api/internal/api/report/service.

### 1.3 Cấu trúc thư mục hiện tại (tóm tắt)

Đã đổi **core** → **internal** theo chuẩn golang-standards (code private, không cho module khác import).

```
api/
├── cmd/server/           # Entry point, init, main
├── config/              # Cấu hình
├── internal/            # Code private (trước đây là core/)
│   ├── api/             # Layer API (HTTP)
│   │   ├── cta/         # service/ (ctasvc), dto/ (ctadto), handler/ (ctahdl)
│   │   ├── delivery/    # service/ (deliverysvc), dto/ (deliverydto), handler/ (deliveryhdl)
│   │   ├── report/      # service/ (reportsvc), dto/ (reportdto), handler/ (reporthdl)
│   │   ├── dto/
│   │   ├── handler/
│   │   ├── middleware/
│   │   ├── models/
│   │   ├── router/      # routes.go + *_routes.go theo domain
│   │   └── services/
│   ├── cta/             # package cta – CTA render, track
│   ├── delivery/
│   ├── notification/
│   ├── database/, global/, logger/, utility/, registry/, worker/, ...
│   └── common/
└── docs/
```

Import: toàn bộ `meta_commerce/internal/...` đã được thay bằng `meta_commerce/internal/...`.

---

## 2. Cách các công ty / cộng đồng Go thường làm

### 2.1 Chuẩn tham khảo: golang-standards/project-layout

Đây **không phải** chuẩn chính thức của Go team, nhưng được dùng rộng rãi và nhiều công ty áp dụng.

| Thư mục | Ý nghĩa |
|---------|---------|
| **/cmd** | Điểm vào ứng dụng (main). Mỗi app một thư mục con (vd: `cmd/server`, `cmd/worker`). Code trong đây nên mỏng, gọi vào internal/pkg. |
| **/internal** | Code **private** – không cho project/module khác import. Go toolchain thực sự cấm import từ bên ngoài. Dùng cho toàn bộ logic nghiệp vụ, adapter, shared code nội bộ. |
| **/pkg** | Code **public** – cho phép project khác import. Chỉ đặt ở đây khi chủ đích cho người ngoài dùng. |
| **/api** | Định nghĩa API: OpenAPI/Swagger, proto, schema. |
| **/configs** | File cấu hình mẫu / default. |
| **/scripts, /build, /deployments** | Script build, CI, deploy. |
| **/docs** | Tài liệu thiết kế, hướng dẫn. |

Nguyên tắc họ nhấn mạnh:

- Với PoC/sản phẩm nhỏ: không cần đủ thư mục, có thể chỉ `main.go` + `go.mod`.
- Khi dự án lớn hơn, nhiều người làm: cần cấu trúc rõ để tránh dependency ẩn và global state.
- **internal** là công cụ quan trọng: refactor thoải mái bên trong mà không làm vỡ người dùng bên ngoài.

### 2.2 Cách tổ chức bên trong internal

Hai hướng thường gặp:

**A) Chia theo layer (horizontal):**

```
internal/
├── app/          # Ứng dụng (orchestration)
├── domain/       # Domain model, interface (ports)
├── handler/      # HTTP handler
├── service/      # Business logic
├── repository/   # Data access
└── pkg/          # Shared nội bộ (util, logger, ...)
```

**B) Chia theo domain (vertical / DDD-style):**

```
internal/
├── auth/
│   ├── handler.go
│   ├── service.go
│   └── repository.go
├── order/
│   ├── handler.go
│   ├── service.go
│   └── repository.go
└── shared/       # Code dùng chung nhiều domain
```

Công ty lớn thường:

- Dùng **internal** cho toàn bộ code app (trừ thư viện thật sự public thì mới đưa vào **pkg**).
- Kết hợp **cmd** (nhiều binary: server, worker, migrate) + **internal** (logic chung).
- Khi nhiều domain, hay tách **internal/domain_name** (DDD-style) để dễ sở hữu code theo team và dễ tách microservice sau này.

### 2.3 Go module chính thức (go.dev)

- **cmd/** và **internal/** được nhắc trong [Organizing a Go module](https://go.dev/doc/modules/layout).
- **internal**: package bên trong chỉ import được bởi module chứa nó (và cây thư mục cha).
- Cấu trúc gợi ý: một repo một module, trong module có `cmd/<app>` và phần còn lại (thường là internal hoặc pkg).

---

## 3. So sánh với dự án hiện tại và hướng tiến hóa

| Chuẩn / Công ty | Dự án hiện tại | Gợi ý tiến hóa |
|------------------|----------------|----------------|
| **cmd/** cho entry point | Có `api/cmd/server` | Giữ. Có thể thêm `cmd/worker` nếu tách worker. |
| **internal** cho code private | Chưa dùng tên `internal`; code đang ở `core/` | Có thể đổi `core/` → `internal/` (hoặc đặt `core` bên trong `internal`) để rõ “private” và chuẩn hơn. |
| **pkg** cho code public | Chưa có; không có thư viện public | Chỉ thêm khi thật sự có package cho project khác import. |
| **api** cho OpenAPI/spec | Có `api/internal/api` (layer API) + `router/api_routes*.yaml` | Giữ. Có thể đặt thêm `api/openapi/` nếu có spec OpenAPI. |
| Router/route theo domain | Đã tách `*_routes.go` | Đã đúng hướng. Có thể tiếp tục tách domain khác (notification, report, …) như CTA. |
| Domain tách package | Đã có `api/cta` (ctasvc) + `core/cta` (cta) | Mô hình tốt. Có thể nhân bản cho auth, notification, content, ai (từng bước). |

---

## 4. Tóm tắt ngắn

- **Cấu trúc mới đã áp dụng:** Router tách file theo domain; domain CTA tách thành hai package rõ ràng (ctasvc = data, cta = logic render/track).
- **Cách công ty lớn / chuẩn Go thường làm:** Dùng **cmd** + **internal** (+ **pkg** khi cần public); bên trong internal có thể chia theo layer hoặc theo domain (DDD); **internal** là chuẩn để bảo vệ code private.
- **Hướng đi hợp lý cho dự án:** Giữ cấu trúc hiện tại, có thể đổi `core/` sang `internal/` nếu muốn sát chuẩn; tiếp tục tách từng domain (service/handler) giống CTA khi module đủ lớn.

Tài liệu tham khảo:

- [golang-standards/project-layout](https://github.com/golang-standards/project-layout)
- [Organizing a Go module (go.dev)](https://go.dev/doc/modules/layout)
