# Khung khuôn module Intelligence — nguyên tắc chung

**Mục đích:** Cố định **một bộ nguyên tắc dùng chung** khi thiết kế hoặc mở rộng module intelligence (CRM customer intel, order intel, CIX, Meta Ads intel, …): phân tầng dữ liệu tính toán, cờ (flag), cách **lưu kết quả**, **lưu lịch sử**, và **dựng lại trạng thái tại thời điểm trong quá khứ**.

**Tham chiếu triển khai:** module **CRM** (`currentMetrics`, `crm_activity_history`, báo cáo theo snapshot). Các miền khác có thể đơn giản hóa từng lớp nhưng **không phá các nguyên tắc** dưới đây.

**Đọc kèm (bắt buộc khi chạm luồng queue / ingest):**

- [KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md](./KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md) — Ingress, merge L1→L2, job `*_intel_compute`, tách intel khỏi document đồng bộ (mục 1.1–1.3).
- [NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md](./NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md) — CRUD → `EmitDataChanged` → AID; một cửa `applyDatachangedSideEffects`.

**Hợp đồng ID / event:** [unified-data-contract.md](../../docs-shared/architecture/data-contract/unified-data-contract.md), [HUONG_DAN_IDENTITY_LINKS.md](./HUONG_DAN_IDENTITY_LINKS.md).

---

## 1. Hai trục “layer” — không trộn khái niệm

| Trục | Tên gọi trong dự án | Ý nghĩa | Ghi chú |
|------|---------------------|---------|--------|
| **Trục dữ liệu & định danh** | **L1 / L2** (data contract) | **L1:** mirror / thô theo nguồn. **L2:** canonical đã merge (`uid`, `links`, …). | Đây là **lớp persistence**, không phải “layer1 metrics” trong dashboard. |
| **Trục pipeline intelligence** | **raw → layer1 → layer2 → layer3** (và tùy chọn **flag**) | Chuỗi **tính toán / suy diễn** trên đối tượng L2 (hoặc entity tương đương). | Có thể **derive lại** từ raw + thời điểm nếu thiết kế đúng. |

**Quy tắc:** Trong code và tài liệu, khi viết “L1/L2” cần làm rõ là **mirror/canonical** hay **metrics layer1/layer2** để tránh hiểu nhầm.

---

## 2. Khuôn pipeline intelligence trên một entity (L2)

### 2.1 Vai trò từng lớp (khuyến nghị)

| Lớp | Vai trò | Nội dung điển hình | Có derive lại được? |
|-----|---------|-------------------|---------------------|
| **raw** | Facts / aggregate thô | Số liệu từ nguồn đã gom (doanh thu, số đơn, timestamp, đếm tin nhắn, …). Ít “ý nghĩa nghiệp vụ” sẵn. | Là **đầu vào** chuẩn để derive các lớp trên. |
| **layer1** | Gom nhẹ / tiền phân loại | Ví dụ CRM: `journeyStage` sơ bộ, `orderCount` — tùy miền có thể gộp với raw hoặc tách. | Thường derive từ raw + quy tắc đơn giản. |
| **layer2** | Phân loại / classification | Ví dụ CRM: `valueTier`, `lifecycleStage`, `journeyStage`, `channel`, … — có thể chạy qua **Rule Engine** (`LOGIC_CRM_CLASSIFICATION`). | Có — nếu input raw + param set đủ và logic version hóa. |
| **layer3** | Chỉ số chất lượng / diagnostic UI | Ví dụ CRM: First / Repeat / VIP / Inactive / Engaged (package `report/layer3`). | **Nên** derive từ raw + layer1 + layer2 + **thời điểm đánh giá** (`endMs`). |
| **flag** (tùy miền) | Tín hiệu vận hành / rủi ro | Ví dụ: `repeat_gap_risk`, `vip_at_risk` — output của rule “interpretation”, trước khi `flow_trigger`. | Derive từ classification + ngưỡng. |

**Lưu ý:** Tên khóa BSON/JSON (`layer1`, `layer2`, `layer3`) là **quy ước CRM**; miền khác có thể dùng namespace riêng miễn **giữ đúng vai trò** (facts → classification → insight → cờ).

### 2.2 Chuỗi Rule Intelligence (song song với metrics)

Khi dùng module `ruleintel`, quy ước **layer logic** theo domain (ví dụ CRM):

| `from_layer` | `to_layer` |
|--------------|------------|
| `raw` | `crm_classification` |
| `crm_classification` | `flag` |
| `flag` | `flow_trigger` |

Chi tiết: [PHUONG_AN_CRM_RULE_INTELLIGENCE.md](./PHUONG_AN_CRM_RULE_INTELLIGENCE.md). **Không bắt buộc** mọi miền có đủ bốn bước; bắt buộc là **tách raw khỏi classification** và **version/trace** khi cần audit.

---

## 3. Lưu kết quả tính toán (chuẩn hai lớp A / B)

Khớp [KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md](./KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md) mục **1.3**.

| Lớp lưu | Mã | Vai trò |
|---------|-----|--------|
| **A — Bản ghi mỗi lần chạy / terminal job** | Ví dụ: `cix_analysis_results`, hướng CRM: `crm_customer_intel_runs` | Lịch sử nhiều phiên bản, audit, debug AID, so sánh trước–sau. |
| **B — Read model trên canonical** | Ví dụ: `crm_customers.currentMetrics`, field tóm tắt CIX | UI, sort, context packet; chỉ **denormalize**; luôn **truy ngược A** qua `lastIntelResultId` / `intelRunUid` / `parentJobId` (theo data contract từng miền). |

**Trường tối thiểu gợi ý trên bản ghi lớp A:** `ownerOrganizationId`, khóa entity canonical miền, `computedAt` / `failedAt`, `status` (`success` | `failed` | `skipped`), liên kết `traceId` / `parentJobId` khi có; lỗi: `errorCode`, `errorMessage` tóm tắt.

**Job queue** (`*_intel_compute`) xử lý retry / `processError` **không thay** bản ghi lớp A khi miền cần báo cáo lịch sử lỗi đầy đủ — hai cơ chế bổ trợ.

---

## 4. Lịch sử thay đổi và “điểm thời gian trong quá khứ”

### 4.1 Mục tiêu

- Biết **trạng thái intelligence / classification** của entity **tại mốc T** (báo cáo theo kỳ, xu hướng, phát sinh).
- Không suy đoán ngược từ **trạng thái hiện tại** trên L2 nếu đã có nhiều thay đổi sau T.

### 4.2 Khuôn CRM (tham chiếu)

1. **Activity + snapshot:** `crm_activity_history` (hoặc tương đương) ghi **`activityAt`** (Unix ms) và **`snapshot`** gồm `profileSnapshot`, **`metricsSnapshot`**, `snapshotChanges` (diff so với lần trước). Chỉ tạo bản ghi khi có thay đổi có ý nghĩa (xem `service.crm.snapshot.go`).
2. **Cấu trúc `metricsSnapshot`:** đồng dạng **raw / layer1 / layer2 / layer3** với read model trên customer — để report và UI đọc một schema.
3. **Override theo mốc thời gian:** khi ghi snapshot, dùng **`metricsOverride` / `profileOverride`** nếu cần đảm bảo snapshot khớp **đúng thời điểm sự kiện**, tránh đọc lại entity “bây giờ” sau khi worker khác đã cập nhật.
4. **Truy vấn point-in-time:** hàm kiểu **`GetLastSnapshotPerCustomerBeforeEndMs(org, endMs)`** — lấy snapshot gần nhất với `activityAt <= endMs` — dùng cho **số dư cuối kỳ**, **phát sinh kỳ**, **phân bổ tier/journey theo timeline**.

Luồng derive layer3 từ snapshot tại kỳ: package **`report/layer3`** (`DeriveFromNested`, `DeriveFromMap` với `endMs`).

### 4.3 Nhân rộng sang module khác

| Yêu cầu | Cách làm |
|---------|----------|
| Lịch sử thay đổi insight | Append event có **timestamp nghiệp vụ** + **payload snapshot** (hoặc delta có thể hydrate). |
| Báo cáo “tại ngày D” | API/service đọc **snapshot cuối cùng ≤ D**, không chỉ `updated_at` document L2 hiện tại. |
| Đồng bộ với AID | Sau khi intel terminal, emit handoff (`*_intel_recomputed`, …) theo [KHUNG_LUONG…](./KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md) Pha C. |

---

## 5. Vị trí trong luồng tổng (CIO · Domain · AID)

- **Ingress / merge** đưa dữ liệu về **L1 → L2** (khi miền có merge).
- **Worker domain** chạy **`{domain}_intel_compute`**, cập nhật lớp **B**, ghi lớp **A** khi cần, phát event về AID.
- **AID** debounce / dispatch / case — **không** chứa logic intel nặng; chi tiết [co-cau-module-aid-va-domain-queue.md](../module-map/co-cau-module-aid-va-domain-queue.md).

---

## 6. Tham chiếu code CRM (implementation mẫu)

| Chủ đề | Vị trí |
|--------|--------|
| Cấu trúc `metricsSnapshot` (raw…layer3) | `api/internal/api/crm/service/service.crm.snapshot.go` — `buildMetricsSnapshot`, `ensureLayer3InMetrics` |
| Aggregate → `currentMetrics` | `api/internal/api/crm/service/service.crm.metrics.go` — `BuildCurrentMetricsFromOrderAndConv` |
| Activity sau recalculate / timeline | `api/internal/api/crm/service/service.crm.recalculate.go` — `logRecalculateActivity`, comment `GetLastSnapshotPerCustomerBeforeEndMs` |
| Derive layer3 | `api/internal/api/report/layer3/layer3.go` |
| Báo cáo theo snapshot trước `endMs` | `api/internal/api/report/service/service.report.customers.trend.go`, `service.report.customer.phatsinh.go` |
| Chẩn đoán layer3 / nguồn dữ liệu | [LAYER3_INTELLIGENCE_DIAGNOSTIC.md](./LAYER3_INTELLIGENCE_DIAGNOSTIC.md) |

---

## 7. Checklist khi thêm hoặc mở rộng module intelligence

1. **Tách trục:** Đã phân biệt L1/L2 **dữ liệu** với **raw / layer1 / layer2 / layer3** **tính toán**?
2. **Raw đủ facts:** Có thể derive lại classification / layer3 từ raw + `endMs` (hoặc từ event lịch sử) không?
3. **Hai lớp lưu:** Đã chọn rõ **A** (mỗi run) và **B** (read model) theo mục 3?
4. **Lịch sử / báo cáo:** Có cơ chế **snapshot tại `activityAt`** và truy vấn **last before T** nếu cần trend?
5. **Queue & AID:** Job nặng ở **worker miền**; handoff event đúng contract; không side-effect intel ngoài `applyDatachangedSideEffects` (xem NGUYEN_TAC).
6. **Rule Engine (nếu có):** Khai báo `domain`, `from_layer`, `to_layer`; param version; trace — [rule-intelligence.md](../02-architecture/core/rule-intelligence.md).

---

## 8. Tài liệu liên quan

| Tài liệu | Nội dung |
|----------|----------|
| [KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md](./KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md) | Pha A–B–C, L1/L2 data, persist intel 1.1–1.3 |
| [PHUONG_AN_CRM_RULE_INTELLIGENCE.md](./PHUONG_AN_CRM_RULE_INTELLIGENCE.md) | Layer Rule CRM: raw → classification → flag |
| [LAYER3_INTELLIGENCE_DIAGNOSTIC.md](./LAYER3_INTELLIGENCE_DIAGNOSTIC.md) | Luồng dữ liệu layer3 CRM |
| [rule-intelligence.md](../02-architecture/core/rule-intelligence.md) | Kiến trúc Rule Intelligence (Ads, CRM) |
| [TAB4_CUSTOMER_INTELLIGENCE_DESIGN.md](../02-architecture/TAB4_CUSTOMER_INTELLIGENCE_DESIGN.md) | Tab dashboard CEO (ngữ cảnh sản phẩm) |
| [co-cau-module-aid-va-domain-queue.md](../module-map/co-cau-module-aid-va-domain-queue.md) | AID vs queue miền |

---

**Changelog**

- **2026-04-07:** Bản đầu — gom khuôn raw/layer/flag, lưu A/B, lịch sử & point-in-time theo CRM + KHUNG ingress/intel.
