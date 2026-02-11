# Plan: Cấu hình tổ chức (Organization Config)

Tóm tắt mục tiêu, thiết kế và trạng thái triển khai. Chi tiết API: `docs/03-api/organization-config.md`.

---

## Mục tiêu

- Config riêng cho từng tổ chức, lưu trong collection `auth_organization_configs`.
- Config theo cây: tổ chức cấp dưới kế thừa từ cấp trên; key có `allowOverride: false` thì cấp dưới không ghi đè.
- Config hệ thống (root) không cho xóa.
- Mỗi key có metadata: tên, mô tả, loại dữ liệu, ràng buộc, `allowOverride`.

---

## Thiết kế (đã triển khai)

| Hạng mục | Nội dung |
|----------|----------|
| Collection | `auth_organization_configs`, 1 doc / 1 org (unique `ownerOrganizationId`) |
| Model | `api/internal/api/models/mongodb/model.organization.config.go` — OrganizationConfig, ConfigKeyMeta |
| Global / init | `OrganizationConfigs` trong global.vars, init.go (index), init.registry.go (đăng ký collection) |
| Service | `api/internal/api/services/service.organization.config.go` — GetByOwnerOrganizationID, UpsertByOwnerOrganizationID, GetResolvedConfig, DeleteByOwnerOrganizationID, ValidateBeforeDelete, validateLockedKeysOnUpdate |
| DTO | `api/internal/api/dto/dto.organization.config.go` — ConfigKeyMetaInput, OrganizationConfigUpdateInput |
| Handler | `api/internal/api/handler/handler.organization.config.go` — GetConfig, GetResolvedConfig, UpdateConfig, DeleteConfig (custom handler, :id = org id) |
| Route | GET/PUT/DELETE `/organization/:id/config`, GET `/organization/:id/config/resolved` — registerRouteWithMiddleware trong registerRBACRoutes |
| Permission | OrganizationConfig.Read, OrganizationConfig.Update, OrganizationConfig.Delete — thêm trong InitialPermissions (service.admin.init.go) |
| Tài liệu | `docs/03-api/organization-config.md` — API, cấu trúc, quy tắc nghiệp vụ |

---

## Trạng thái

- **Đã xong:** Model, global/init/registry, service, DTO, handler, route, permission, tài liệu API.
- **Sau deploy:** Gọi init permissions (hoặc init/all) để tạo 3 permission mới; gán OrganizationConfig.Read / Update / Delete cho role cần quản lý config.

---

**Cập nhật:** 2025-01-30
