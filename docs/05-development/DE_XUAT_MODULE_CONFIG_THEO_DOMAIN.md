# Đề Xuất: Module Config Chung Theo Domain Cho Từng Tổ Chức

**Ngày:** 2025-03-17  
**Mục đích:** Rà soát và đề xuất phương án thêm module config chung cho từng tổ chức, chia theo các domain trong hệ thống.

---

## 1. Hiện Trạng Rà Soát

### 1.1 Cấu hình hiện có

| Thành phần | Collection | Phạm vi | Mô tả |
|------------|------------|---------|-------|
| **Organization Config** | `auth_organization_config_items` | 1 doc per (org, key) | Config chung tổ chức: timezone, businessHours. Flat keys, không phân domain. |
| **Ads Meta Config** | `ads_meta_config` | 1 doc per (adAccountId, ownerOrgID) | Config đặc thù Ads: accountMode, campaign rules, automation. Gắn ad account. |
| **Agent Config** | `agent_configs` | 1 doc per (agentId, version) | Config bot/agent. Không gắn org trực tiếp. |

### 1.2 Domains trong hệ thống

| Domain | Module | Ví dụ config cần |
|--------|--------|------------------|
| **ads** | ads | Đã có `ads_meta_config` (per ad account). Có thể thêm org-level: defaultTimezone, defaultMode. |
| **crm** | crm | bulkJobBatchSize, recalculateInterval, syncCheckpointEnabled, ingestRetryLimit. |
| **cio** | cio | defaultRoutingMode, aiThreshold, planResumeInterval. |
| **report** | report | scheduleCron, dirtyPeriodMinutes, layer3Enabled. |
| **notification** | notification | defaultChannels, routingDomainOverrides. |
| **ruleintel** | ruleintel | defaultParamSetId, traceRetentionDays. |
| **meta** | meta | evaluationMode, layer2CacheTTL. |
| **auth** | auth | timezone, businessHours (đã có trong OrganizationConfigItem). |

### 1.3 Vấn đề cần giải quyết

- **OrganizationConfigItem** hiện tại: keys flat (`timezone`, `businessHours`), không nhóm theo domain.
- **ads_meta_config**: gắn ad account, không phải org-level. Một org có nhiều ad account → mỗi account có config riêng.
- Thiếu **config org-level theo domain** cho: crm, cio, report, notification, ruleintel, meta.
- Cần cơ chế: lấy config theo domain, kế thừa cây tổ chức, metadata (name, dataType, allowOverride).

---

## 2. Các Phương Án Thiết Kế

### Phương án A: Mở rộng OrganizationConfigItem — thêm field Domain

**Ý tưởng:** Thêm field `Domain string` vào `OrganizationConfigItem`. Key vẫn là key đơn (vd: `bulkJobBatchSize`), nhưng nhóm theo domain khi query/API.

**Schema:**
```go
type OrganizationConfigItem struct {
    // ... hiện có
    Domain string `json:"domain" bson:"domain" index:"single:1,compound:owner_domain_key_unique"`
    Key    string `json:"key" bson:"key" index:"compound:owner_domain_key_unique"`
    // ...
}
```

**Index:** `(ownerOrganizationId, domain, key)` unique.

**API:**
- `GET /organization-config?domain=crm` — lấy config theo domain
- `GET /organization-config/resolved?domain=crm` — resolved theo domain
- `PUT /organization-config` — body có `domain`, `key`, `value`

**Ưu điểm:**
- Tái sử dụng collection và service hiện có.
- Chỉ cần thêm field, migration đơn giản (domain="" cho doc cũ = "auth" hoặc "general").
- GetResolvedConfig đã có logic kế thừa cây org.

**Nhược điểm:**
- Key vẫn flat trong DB; domain chỉ là filter.
- Cần migration: gán domain cho doc cũ (vd: `timezone` → domain=`auth`).

---

### Phương án B: Collection mới `org_domain_configs` — 1 doc per (org, domain)

**Ý tưởng:** Mỗi tổ chức có tối đa 1 document cho mỗi domain. Config của domain nằm trong object `config`.

**Schema:**
```go
type OrgDomainConfig struct {
    ID                  primitive.ObjectID            `json:"id" bson:"_id,omitempty"`
    OwnerOrganizationID primitive.ObjectID            `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"compound:owner_domain_unique"`
    Domain              string                        `json:"domain" bson:"domain" index:"compound:owner_domain_unique"`
    Config              map[string]interface{}        `json:"config" bson:"config"`
    ConfigMeta          map[string]ConfigKeyMeta      `json:"configMeta,omitempty" bson:"configMeta,omitempty"`
    CreatedAt           int64                         `json:"createdAt" bson:"createdAt"`
    UpdatedAt           int64                         `json:"updatedAt" bson:"updatedAt"`
}
```

**Index:** `(ownerOrganizationId, domain)` unique.

**API:**
- `GET /org-domain-config?domain=crm` — lấy config domain crm của org hiện tại
- `GET /org-domain-config/resolved?domain=crm` — resolved (merge cây org)
- `PUT /org-domain-config` — body: `domain`, `config`, `configMeta`

**Ưu điểm:**
- Cấu trúc rõ ràng: mỗi domain một document.
- Query theo domain hiệu quả.
- Dễ mở rộng: thêm domain mới không ảnh hưởng doc cũ.

**Nhược điểm:**
- Collection mới, service mới, handler mới.
- Logic GetResolvedConfig cần implement lại (merge theo cây org).

---

### Phương án C: Convention key dạng `domain.key` trong OrganizationConfigItem

**Ý tưởng:** Không đổi schema. Dùng key có prefix domain: `crm.bulkJobBatchSize`, `cio.defaultRoutingMode`. GetResolvedConfig trả về nested: `{ "crm": { "bulkJobBatchSize": 200 }, "cio": { ... } }`.

**API:**
- `GET /organization-config/resolved` — response transform thành `{ "crm": {...}, "cio": {...} }`
- `PUT /organization-config` — body: `key: "crm.bulkJobBatchSize"`, `value: 200`

**Ưu điểm:**
- Không migration, không collection mới.
- Chỉ cần convention và transform ở API layer.

**Nhược điểm:**
- Key dài, dễ sai convention.
- Khó validate schema theo domain (mỗi domain có keys khác nhau).
- Filter theo domain phải parse key.

---

### Phương án D: Module config riêng + registry domain schema

**Ý tưởng:** Giống B, nhưng thêm **registry** định nghĩa schema config cho từng domain (keys, dataType, default). Module tự đăng ký schema khi khởi động.

**Ví dụ:**
```go
// api/internal/api/config/registry.go
var DomainSchemas = map[string]DomainSchema{
    "crm": {
        Keys: map[string]KeyMeta{
            "bulkJobBatchSize": {DataType: "number", Default: 200, AllowOverride: true},
            "recalculateInterval": {DataType: "number", Default: 3600},
        },
    },
    "cio": {...},
}
```

**Ưu điểm:**
- Validation theo schema, default value, type-safe.
- UI có thể render form theo schema.

**Nhược điểm:**
- Phức tạp hơn, cần maintain registry.
- Có thể overkill nếu số domain ít.

---

## 3. So Sánh Và Đề Xuất

| Tiêu chí | A | B | C | D |
|----------|---|---|---|---|
| Effort | Thấp | Trung bình | Thấp | Cao |
| Rõ ràng cấu trúc | Trung bình | Cao | Thấp | Cao |
| Tái sử dụng hiện có | Cao | Thấp | Cao | Thấp |
| Validation schema | Thủ công | Thủ công | Khó | Tự động |
| Query theo domain | Tốt | Tốt | Kém | Tốt |

**Đề xuất:** **Phương án A** (mở rộng OrganizationConfigItem) hoặc **Phương án B** (collection mới) tùy mức độ tách biệt mong muốn.

- **Chọn A** nếu: muốn tối thiểu thay đổi, tái dùng logic GetResolvedConfig, migration đơn giản.
- **Chọn B** nếu: muốn cấu trúc rõ ràng, tách biệt config theo domain, dễ mở rộng và query.

---

## 4. Đề Xuất Chi Tiết — Phương Án A (Mở Rộng)

### 4.1 Thay đổi Model

**File:** `api/internal/api/auth/models/model.organization.config.item.go`

```go
type OrganizationConfigItem struct {
    ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
    OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:owner_domain_key_unique"`
    Domain              string             `json:"domain" bson:"domain" index:"single:1,compound:owner_domain_key_unique"` // ads, crm, cio, report, notification, ruleintel, meta, auth
    Key                 string             `json:"key" bson:"key" index:"single:1,compound:owner_domain_key_unique"`
    Value               interface{}        `json:"value" bson:"value"`
    // ... Name, Description, DataType, Constraints, AllowOverride, IsSystem, CreatedAt, UpdatedAt
}
```

**Constants domain:**
```go
// api/internal/api/auth/models/constants.domain.go
const (
    DomainAuth         = "auth"
    DomainAds          = "ads"
    DomainCrm          = "crm"
    DomainCio          = "cio"
    DomainReport       = "report"
    DomainNotification = "notification"
    DomainRuleintel    = "ruleintel"
    DomainMeta        = "meta"
)
```

### 4.2 Migration

- Doc cũ (chưa có `domain`): gán `domain = "auth"` (timezone, businessHours thuộc auth).
- Index mới: `(ownerOrganizationId, domain, key)` unique.

### 4.3 API mở rộng

| Method | Path | Mô tả |
|--------|------|-------|
| GET | `/organization-config?domain=crm` | List config items theo domain |
| GET | `/organization-config/resolved?domain=crm` | Resolved config theo domain (chỉ keys của domain đó) |
| PUT | `/organization-config` | Upsert với body có `domain` |

### 4.4 Service

- `FindByOwnerOrganizationIDAndDomain(ctx, orgID, domain)` — thay vì chỉ FindByOwnerOrganizationID.
- `GetResolvedConfigByDomain(ctx, orgID, domain)` — merge cây org, chỉ trả keys của domain.

---

## 5. Đề Xuất Chi Tiết — Phương Án B (Collection Mới)

### 5.1 Model

**File:** `api/internal/api/config/models/model.org_domain_config.go`

```go
package models

type OrgDomainConfig struct {
    ID                  primitive.ObjectID       `json:"id" bson:"_id,omitempty"`
    OwnerOrganizationID primitive.ObjectID       `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"compound:owner_domain_unique"`
    Domain              string                   `json:"domain" bson:"domain" index:"compound:owner_domain_unique"`
    Config              map[string]interface{}   `json:"config" bson:"config"`
    ConfigMeta          map[string]ConfigKeyMeta `json:"configMeta,omitempty" bson:"configMeta,omitempty"`
    CreatedAt           int64                   `json:"createdAt" bson:"createdAt"`
    UpdatedAt           int64                   `json:"updatedAt" bson:"updatedAt"`
}

type ConfigKeyMeta struct {
    Name          string `json:"name" bson:"name"`
    Description   string `json:"description" bson:"description"`
    DataType       string `json:"dataType" bson:"dataType"`
    Constraints    string `json:"constraints,omitempty" bson:"constraints,omitempty"`
    AllowOverride bool   `json:"allowOverride" bson:"allowOverride"`
}
```

### 5.2 Module mới: `config`

- `api/internal/api/config/` — handler, service, router, models, dto.
- Router: `/config` hoặc `/org-domain-config`.
- Permission: `OrgDomainConfig.Read`, `OrgDomainConfig.Update`, `OrgDomainConfig.Delete`.

### 5.3 API

| Method | Path | Mô tả |
|--------|------|-------|
| GET | `/config/org/:orgId/domain/:domain` | Config raw của domain |
| GET | `/config/org/:orgId/domain/:domain/resolved` | Config resolved (merge cây) |
| PUT | `/config/org/:orgId/domain/:domain` | Upsert config domain |

---

## 6. Phân Chia Domain Và Keys Gợi Ý

| Domain | Keys gợi ý | Mô tả |
|--------|-------------|-------|
| **auth** | timezone, businessHours | Đã có. Giữ nguyên. |
| **ads** | defaultTimezone, defaultMode | Org-level default (ads_meta_config vẫn per ad account). |
| **crm** | bulkJobBatchSize, recalculateInterval, syncCheckpointEnabled | Config bulk job, sync, recalculate. |
| **cio** | defaultRoutingMode, aiThreshold, planResumeInterval | Config CIO routing, plan. |
| **report** | scheduleCron, dirtyPeriodMinutes, layer3Enabled | Config report schedule, layer3. |
| **notification** | defaultChannels | Override channel mặc định theo domain. |
| **ruleintel** | traceRetentionDays, defaultParamSetId | Config rule engine. |
| **meta** | evaluationMode | Mode đánh giá (nếu cần org-level). |

---

## 7. Lộ Trình Triển Khai

### Giai đoạn 1 (Phương án A)
1. Thêm field `Domain` vào `OrganizationConfigItem`.
2. Migration: gán `domain="auth"` cho doc cũ.
3. Tạo index `(ownerOrganizationId, domain, key)` unique.
4. Mở rộng service: `FindByOwnerOrganizationIDAndDomain`, `GetResolvedConfigByDomain`.
5. Mở rộng API: query param `domain`, response filter theo domain.

### Giai đoạn 2
1. Seed config mặc định cho từng domain (crm, cio, report, ...).
2. Cập nhật module crm, cio, report để đọc config từ OrgDomainConfig thay vì hardcode.

### Giai đoạn 3 (tùy chọn)
1. Nếu cần validation schema mạnh → xem xét Phương án D (registry).
2. UI quản lý config theo domain.

---

## 8. Tài Liệu Tham Chiếu

- [organization-config-plan.md](../09-ai-context/organization-config-plan.md)
- [organization-config.md](../03-api/organization-config.md)
- [backend-module-map.md](../module-map/backend-module-map.md)
- [domain-logic.md](../../.cursor/rules/domain-logic.md)
