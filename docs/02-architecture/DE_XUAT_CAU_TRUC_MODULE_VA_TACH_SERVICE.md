# Đề xuất cấu trúc module và hướng tách service

Tài liệu rà soát hiện trạng và đề xuất cách tổ chức lại code để dễ quản lý, mở rộng và tách service về sau.

---

## 1. Rà soát hiện trạng

### 1.1 Cấu trúc thư mục hiện tại (flat monolith)

```
api/internal/api/
├── handler/     # ~44 file, tất cả domain trong 1 package
├── services/     # ~60+ file, tất cả domain trong 1 package
├── models/       # ~59 file (mongodb)
├── dto/          # ~55 file
├── router/       # routes.go ~680 dòng, đăng ký toàn bộ route
├── middleware/
└── ...
```

**Vấn đề:**

- Một package quá nhiều file, khó định vị và review theo domain.
- Thêm entity mới = sửa nhiều chỗ: `init.go`, `init.registry.go`, `global.vars.go`, `routes.go`, handler, service, model, dto.
- Không có ranh giới rõ theo domain → tách một “cục” ra thành service riêng rất tốn công.

### 1.2 Phụ thuộc giữa các service (coupling)

Nhiều service gọi trực tiếp `New*Service()` của service khác trong constructor:

| Service | Phụ thuộc trong New*() |
|--------|-------------------------|
| **InitService** | User, Role, Permission, RolePermission, UserRole, Organization, NotificationSender/Template/Channel/Routing, OrganizationShare, CTALibrary, AIProviderProfile, AIPromptTemplate, AIStep, AIWorkflow, AIWorkflowCommand (17 service) |
| **AdminService** | User, Role, Permission, UserRole, RolePermission |
| **UserRoleService** | User, Role |
| **OrganizationService** | Role |
| **OrganizationConfigItemService** | Organization |
| **FbPostService** | FbPage |
| **FbMessageService** | FbPage, FbMessageItem |
| **FbConversationService** | FbPage, FbMessage |
| **AgentConfigService** | AgentRegistry |
| **AgentManagementService** | AgentRegistry, AgentConfig, AgentCommand, AgentActivity |
| **AIStepService** | AIPromptTemplate, AIProviderProfile (trong method) |
| **AIWorkflowCommandService** | AIStep, AIWorkflow, AIWorkflowRun |
| **AIWorkflowRunService** | ContentNode, DraftContentNode |
| **NotificationRoutingService** | Organization |
| **DraftContentNodeService** | ContentNode |
| **RolePermissionService** | Role, Permission |
| **OrganizationHelper** | UserRole, Role, RolePermission, Permission, Organization (và lặp lại trong nhiều hàm) |

**Hệ quả:**

- Không có Dependency Injection (DI): handler/middleware gọi `services.New*Service()` trực tiếp; nhiều chỗ tạo service mới trong từng request.
- Chuỗi khởi tạo dài (ví dụ InitService kéo theo 17 service), khó unit test và khó thay implementation (mock).
- Muốn tách một domain (ví dụ AI, Notification) ra service riêng thì phải cắt đứt các New*() này và thay bằng gọi qua interface/API.

### 1.3 Điểm tập trung (single point of change)

- **Router** (`routes.go`): biết mọi handler, gọi `handler.New*Handler()` cho từng domain → file rất dài, dễ conflict khi nhiều người thêm domain.
- **Init** (`cmd/server/init.go`): khai báo tên collection + tạo index cho toàn bộ model; thêm entity = sửa init.
- **Registry** (`init.registry.go`): danh sách collection cố định; thêm collection = sửa list.
- **Global** (`global.vars.go`): struct tên collection; thêm entity = thêm field + chỗ dùng.

→ Mọi domain đều “chạm” vào vài file dùng chung → khó tách một domain ra độc lập.

### 1.4 Tạo service/handler mỗi request

Một số handler/middleware tạo service bên trong hàm xử lý request (không phải trong `New*Handler()`), ví dụ:

- Handler: User, Organization, NotificationTrigger, DeliverySend, PancakeWebhook, … gọi `services.New*Service()` trong handler.
- Middleware: Auth, OrganizationContext gọi `NewUserService()`, `NewRoleService()`, … mỗi request.

**Hệ quả:** Tốn tài nguyên, khó gắn mock cho test, không tận dụng singleton/cache nếu sau này cần.

---

## 2. Nguyên tắc đề xuất

1. **Chia theo domain (bounded context)** để dễ tìm code, review và sau này tách từng “cục” ra service.
2. **Giảm coupling**: service không “new” service khác trong constructor; ưu tiên inject interface.
3. **Router và init theo từng domain**: mỗi domain đăng ký route/collection của mình, giảm sửa file dùng chung.
4. **Chuẩn bị tách service**: định nghĩa interface ở biên (port), implementation trong từng module; sau có thể thay bằng gRPC/HTTP client.

---

## 3. Đề xuất cấu trúc theo module (domain)

### 3.1 Option A: Chia thư mục theo domain (khuyến nghị nếu muốn tách service rõ)

Giữ một repo monolith nhưng tổ chức lại theo domain. Mỗi domain có handler, service, model, dto trong cùng “cây” (hoặc tối thiểu cùng prefix).

Ví dụ cấu trúc:

```
api/internal/api/
├── common/                    # Base handler, base service, response, error
│   ├── handler/
│   │   ├── base.go
│   │   ├── base.crud.go
│   │   └── base.response.go
│   └── service/
│       └── base.mongo.go
├── auth/                      # RBAC, User, Role, Permission, Organization
│   ├── handler/
│   ├── service/
│   ├── model/                 # hoặc tham chiếu api/models/mongodb với prefix auth
│   └── dto/
├── facebook/                  # FbPage, FbPost, FbConversation, FbMessage, FbCustomer
│   ├── handler/
│   ├── service/
│   └── dto/
├── pancake/                   # PcOrder, PcPos*, Webhook
│   ├── handler/
│   ├── service/
│   └── dto/
├── notification/              # Channel, Template, Routing, Sender, Trigger
│   ├── handler/
│   ├── service/
│   └── dto/
├── delivery/                  # Queue, History, Send
│   ├── handler/
│   ├── service/
│   └── dto/
├── content/                   # ContentNode, Video, Publication, Draft*
│   ├── handler/
│   ├── service/
│   └── dto/
├── ai/                        # Workflow, Step, PromptTemplate, ProviderProfile, Run, Command, ...
│   ├── handler/
│   ├── service/
│   └── dto/
├── agent/                     # Registry, Config, Command, Activity, Management
│   ├── handler/
│   ├── service/
│   └── dto/
├── report/                    # ReportDefinition, ReportSnapshot, ReportDirtyPeriod (phase 1)
│   ├── handler/
│   ├── service/
│   └── dto/
├── router/
│   ├── routes.go              # Chỉ gọi register từng domain
│   ├── auth.go
│   ├── facebook.go
│   ├── notification.go
│   ├── delivery.go
│   ├── content.go
│   ├── ai.go
│   ├── agent.go
│   └── ...
├── middleware/
└── models/                    # Có thể giữ chung hoặc rải vào từng domain
    └── mongodb/
```

**Lợi ích:**

- Tìm mọi thứ của một domain trong một cây thư mục.
- Thêm/sửa domain ít đụng vào code domain khác.
- Tách service sau: copy/refactor cả thư mục `ai/` hoặc `notification/` sang repo mới, thay gọi nội bộ bằng API/gRPC.

**Lưu ý:** Model/dto có thể giữ trong `api/internal/api/models/mongodb` và `dto` với naming `model.<domain>.*` nếu không muốn đổi import ồ ạt; bước đầu có thể chỉ tách handler + service theo domain.

### 3.2 Option B: Giữ flat nhưng nhóm file bằng prefix và router tách file

Không đổi cấu trúc thư mục handler/services/dto/models, chỉ:

- Giữ naming hiện tại: `handler.auth.*`, `service.auth.*`, …
- Tách `routes.go` thành nhiều file theo domain, ví dụ:
  - `router/routes.go` chỉ gọi `RegisterAuthRoutes`, `RegisterFacebookRoutes`, …
  - `router/auth.go`, `router/facebook.go`, … mỗi file đăng ký route (và handler) của một domain.

Kèm theo:

- **Registry/init theo domain**: mỗi domain có hàm `RegisterCollections()` / `RegisterIndexes()` và đăng ký vào global thay vì một list cố định trong `init.go` / `init.registry.go`.

**Lợi ích:** Ít thay đổi nhất, vẫn cải thiện được việc “một file quá to” và “init/registry quá tập trung”. Phù hợp làm bước đầu.

**Hạn chế:** Vẫn một package handler, một package services → coupling và dependency graph giữa service không thay đổi nhiều.

---

## 4. Giảm coupling và chuẩn bị tách service

### 4.1 Dependency Injection (DI) cho service

- **Hiện tại:** Service A trong `NewAService()` gọi `NewBService()`, `NewCService()`.
- **Hướng mong muốn:** Service A nhận interface của B, C qua constructor (hoặc setter); ai gọi A (router, main, container) sẽ tạo B, C và inject vào A.

Ví dụ:

```go
// Thay vì FbPostService tự gọi NewFbPageService()
type FbPostService struct {
    *BaseServiceMongoImpl[models.FbPost]
    pageService FbPageServiceReader // interface
}
func NewFbPostService(pageService FbPageServiceReader) (*FbPostService, error) { ... }
```

- Định nghĩa interface nhỏ (theo nhu cầu thực sự dùng), ví dụ `FbPageServiceReader` với `FindOneById`, `FindOneByPageID`.
- Ở điểm khởi tạo (main hoặc router/container), tạo từng service “gốc” (không phụ thuộc service khác) trước, rồi tạo service phụ thuộc và inject.

**Áp dụng từng bước:** Ưu tiên các service “trung tâm” (InitService, AdminService, NotificationTrigger, DeliverySend, AIWorkflowCommand, …) và các service bị gọi chéo nhiều (Organization, User, Role, FbPage).

### 4.2 Interface ở biên (port) cho domain

Để sau này thay implementation bằng gRPC/HTTP:

- Đặt interface (port) ở package dùng (handler hoặc domain gọi), implementation (adapter) trong package domain cung cấp.
- Ví dụ: package `notification` định nghĩa `OrganizationResolver` interface; package `auth` implement. Khi tách auth ra service riêng, chỉ cần thay implementation bằng client gọi API auth.

Có thể bắt đầu với 1–2 domain có khả năng tách sớm (ví dụ AI, Notification/Delivery).

### 4.3 Giảm “god object” InitService

- Chia nhỏ theo domain: mỗi domain có “init” riêng (ví dụ `auth.InitDefaultPermissions()`, `notification.InitDefaultTemplates()`), được gọi từ một orchestration (có thể vẫn là InitService nhưng chỉ gọi từng block, không hold 17 service).
- Hoặc dùng registry: từng domain đăng ký hàm init của mình; lúc chạy init all, loop và gọi từng hàm. Như vậy thêm domain mới không cần sửa InitService.

---

## 5. Đăng ký route và collection theo domain

### 5.1 Router tách file

- `router/routes.go`: chỉ khởi tạo router và gọi lần lượt:
  - `RegisterInitRoutes`
  - `RegisterAuthRoutes`
  - `RegisterAdminRoutes`
  - `RegisterFacebookRoutes`
  - `RegisterNotificationRoutes`
  - …
- Mỗi nhóm route nằm trong file riêng (`router/auth.go`, `router/facebook.go`, …), giảm conflict và dễ tìm.

### 5.2 Collection và index theo domain

- Trong `init.go` / `init.registry.go`: thay vì một list cố định, có thể:
  - Gọi theo từng nhóm: `auth.RegisterCollections(db)`, `facebook.RegisterCollections(db)`, …
  - Hoặc mỗi domain export slice tên collection + model (cho index), một hàm chung loop và đăng ký/tạo index.

Mục tiêu: thêm entity mới trong domain X chỉ sửa file trong domain X (và global nếu vẫn dùng struct tên collection tập trung).

---

## 6. Singleton service thay vì tạo mỗi request

- Ở nơi khởi tạo app (main hoặc router setup): tạo một lần các service dùng chung (User, Role, Permission, …).
- Handler và middleware nhận service qua dependency (struct field được inject khi tạo handler), không gọi `services.New*Service()` trong từng request.
- Nếu chưa có DI container, có thể tạm dùng holder (ví dụ `global.GetUserService()`) được set một lần lúc startup, sau đó chuyển dần sang inject qua constructor.

---

## 7. Lộ trình gợi ý

| Giai đoạn | Nội dung | Mục đích |
|-----------|----------|----------|
| **1. Ngắn hạn** | Tách `routes.go` thành nhiều file theo domain; đăng ký collection/index theo từng nhóm domain (hoặc từng file router). | Giảm file quá lớn, giảm conflict, dễ tìm route/init theo domain. |
| **2. Trung hạn** | Áp dụng DI cho vài service “trung tâm” (Init, Admin, NotificationTrigger, DeliverySend) và các service bị gọi chéo (Organization, User, Role, FbPage). Định nghĩa interface nhỏ cho dependency. | Giảm coupling, dễ test, chuẩn bị thay implementation. |
| **3. Trung hạn** | Chia InitService thành từng block theo domain (auth init, notification init, …) hoặc registry pattern. | Giảm god object, thêm domain mới ít đụng InitService. |
| **4. Dài hạn (tùy chọn)** | Chuyển sang cấu trúc Option A (thư mục theo domain). | Ranh giới rõ, tách service sau này đơn giản hơn. |
| **5. Khi cần tách service** | Chọn domain (ví dụ AI, Notification): định nghĩa port (interface) ở biên, thay implementation bằng client gọi service mới. | Tách từng bounded context ra microservice mà không đảo lộn toàn bộ. |

---

## 8. Tóm tắt

- **Hiện trạng:** Flat monolith, nhiều service gọi chéo trong constructor, router/init/global tập trung, một số chỗ tạo service mỗi request.
- **Đề xuất:** (1) Chia route và đăng ký collection/index theo domain; (2) Giảm coupling bằng DI và interface; (3) Giảm god object InitService; (4) Cân nhắc cấu trúc thư mục theo domain nếu hướng tới tách service.
- **Lộ trình:** Làm từng bước từ tách file router/init → DI cho service trung tâm → chia InitService → (tùy chọn) tổ chức lại thư mục theo domain → khi cần thì tách từng domain thành service riêng.

Nếu bạn chọn bước đầu (ví dụ chỉ tách `routes.go` và đăng ký collection theo domain), có thể triển khai chi tiết ngay trong repo hiện tại mà không đổi cấu trúc handler/service.
