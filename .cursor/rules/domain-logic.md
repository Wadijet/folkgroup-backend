---
description: Domain layer rules, business logic, OwnerOrganizationID
alwaysApply: false
---

# Domain Logic

## Trách Nhiệm Layer

| Layer | Nên làm | Không làm |
|-------|----------|-----------|
| **Handler** | Parse body; validate input đơn giản; gọi service; trả response | Business logic; query DB trực tiếp |
| **Service** | Validate business rules; logic nghiệp vụ; gọi DB; `ConvertMongoError()` | Parse HTTP; trả response |
| **DTO** | Cấu trúc JSON; struct tags; parse/validate URL | Business logic; gọi DB |
| **Model** | Cấu trúc document; BSON tags; enum | Business logic; gọi DB |

- Handler mỏng: parse → validate cơ bản → gọi service → trả response
- Service chứa logic: validation phức tạp, cross-collection, uniqueness check

## OwnerOrganizationID

- **OwnerOrganizationID**: phân quyền dữ liệu — dữ liệu thuộc tổ chức nào
- **OrganizationID**: logic nghiệp vụ; không thay thế OwnerOrganizationID
- Luôn set `OwnerOrganizationID` từ context khi tạo/update
- Query luôn filter theo `OwnerOrganizationID`
- Không cho client update trực tiếp OwnerOrganizationID để chuyển dữ liệu sang org khác

## Business Rules (tóm tắt)

| Domain | Quy tắc |
|--------|---------|
| RBAC | Role Administrator phải có ít nhất một user |
| Reject/Approval | Khi reject bắt buộc có `decisionNote` |
| Uniqueness | Validate trong service trước khi Insert |
| Share | Không tự ý gán OwnerOrganizationID sang org khác |

## Tham Chiếu

- [docs/02-architecture/business-logic/](../../docs/02-architecture/business-logic/)
- [docs/domain/domain-overview.md](../../docs/domain/domain-overview.md)
