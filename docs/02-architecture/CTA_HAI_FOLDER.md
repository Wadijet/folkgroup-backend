# Hai folder CTA – có hợp lý không?

## Hiện trạng

| Vị trí | Package | Nội dung | Ai dùng |
|--------|---------|----------|--------|
| **api/internal/api/cta/** | `cta` | CTALibraryService, CTATrackingService (CRUD, data access) | handler.cta.library, handler.admin.init, core/cta/renderer (qua alias `ctasvc`) |
| **api/internal/cta/** | `cta` | Renderer (render CTA), helpers (GetSystemOrganizationID), TrackCTAClick | handler.tracking, handler.notification.trigger, notification/template |

## Vấn đề

1. **Trùng tên package** – Cả hai đều `package cta` → khi `core/cta` import `api/cta` phải dùng alias `ctasvc "meta_commerce/internal/api/cta"`.
2. **Vai trò khác nhau** nhưng tên giống nhau:
   - **api/cta** = CTA **data/service** (truy cập DB, phục vụ API CRUD).
   - **core/cta** = CTA **application logic** (render, track, helper).

## Kết luận: tách hai folder là hợp lý, nhưng nên đổi tên để rõ nghĩa

- **api/internal/api/cta** = module CTA trong API (service layer cho CTA).
- **api/internal/cta** = logic CTA ở core (render, track), không phụ thuộc HTTP/Fiber.

Để tránh nhầm và trùng tên package, nên đổi **tên package** (và/hoặc tên folder) cho rõ:

- **Đề xuất 1:** Đổi package **api/internal/api/cta** thành **ctasvc** (folder giữ `api/cta`).  
  → Import: `import "meta_commerce/internal/api/cta"` và dùng `cta.CTALibraryService` (tên package trong file là `ctasvc` thì dùng `ctasvc.XXX`). Thực tế nếu đổi package name thành `ctasvc` thì mọi chỗ import `meta_commerce/internal/api/cta` sẽ dùng prefix `ctasvc.` – không còn trùng với package `cta` ở core/cta.

- **Đề xuất 2:** Đổi **core/cta** thành **core/ctarender** (folder + package `ctarender`).  
  → Rõ đây là module “CTA render”, không nhầm với “CTA service” trong api.

Đã áp dụng **Đề xuất 1** trong code: package `api/internal/api/cta` đổi thành `ctasvc` để hết trùng tên và dùng alias rõ ràng.
