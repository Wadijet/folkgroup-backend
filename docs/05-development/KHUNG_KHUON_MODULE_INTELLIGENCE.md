# Khung khuôn module Intelligence — nguyên tắc chung

**Dành cho ai:** Kỹ sư backend / kiến trúc trong team (và người onboard). Đây **không** phải hướng dẫn cho khách hàng dùng sản phẩm.

**Đọc nhanh (khoảng một phút):** Ta tách hai ý — (1) **dữ liệu & định danh** mirror vs canonical (**L1-persist / L2-persist** trong hợp đồng dữ liệu), và (2) **chuỗi suy luận** trên bản canonical: từ số liệu thô → gom nhẹ → phân loại → chỉ số gợi ý (`raw` … `layer3` trong schema CRM). Mỗi lần chạy pipeline intel nên **lưu riêng** (**bản ghi chạy intel**) để có lịch sử và kiểm tra; trên bản ghi chính chỉ giữ **read model intel** (tóm tắt) để UI đọc nhanh. Module **CRM** là ví dụ đầy đủ nhất hiện tại. Xem **mục 0** nếu thấy chữ L1/L2 hoặc “lớp A/B” ở tài liệu khác.

---

**Mục đích:** Cố định **một bộ nguyên tắc dùng chung** khi thiết kế hoặc mở rộng module intelligence (CRM customer intel, order intel, CIX, Meta Ads intel, …): phân tầng dữ liệu tính toán, cờ (flag), cách **lưu kết quả**, **lưu lịch sử**, và **dựng lại trạng thái tại thời điểm trong quá khứ**.

**Tham chiếu triển khai:** module **CRM** (`currentMetrics`, `crm_activity_history`, báo cáo theo snapshot). Các miền khác có thể đơn giản hóa từng lớp nhưng **không phá các nguyên tắc** dưới đây.

**Đọc kèm (bắt buộc khi chạm luồng queue / ingest):**

- [KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md](./KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md) — Ingress, merge **mirror → canonical (L1-persist → L2-persist)**, job `*_intel_compute`, tách intel khỏi document đồng bộ (mục 1.1–1.3).
- [NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md](./NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md) — CRUD → `EmitDataChanged` → AID; một cửa `applyDatachangedSideEffects`.

**Hợp đồng ID / event:** [unified-data-contract.md](../../docs-shared/architecture/data-contract/unified-data-contract.md), [HUONG_DAN_IDENTITY_LINKS.md](./HUONG_DAN_IDENTITY_LINKS.md).

---

## 0. Bảng thuật ngữ (chống nhầm lẫn)

| Mã / tên gọi | Ý nghĩa | Tránh nhầm với |
|--------------|---------|-----------------|
| **Mirror (L1-persist)** / **Canonical (L2-persist)** | Bản ghi theo nguồn vs bản đã merge — data contract §1.7 | Bước **pipeline rule CIX** (L1→L2→L3 trong rule engine); trường BSON CRM `layer1`/`layer2`; **Ads — tầng metric** (Layer 1/2/3) |
| **Pipeline rule CIX** (`L1→L2→L3` trong tài liệu CIX) | Các bước Rule Engine CIX (rule `RULE_CIX_LAYER*` trong code) | Mirror / canonical (L1-persist / L2-persist) |
| **Chuỗi metrics CRM** (`raw` → `layer1` → `layer2` → `layer3`) | Trường tính toán trên customer / snapshot báo cáo | L1-persist / L2-persist; CIO-T* |
| **Bản ghi chạy intel** | Một document mỗi lần job intel kết thúc (`crm_customer_intel_runs`, `cix_analysis_results`, …) | **CIO-T1/T2/T3** (lọc event); **Pha ghi thô** (ingress) |
| **Read model intel** | Tóm tắt trên document canonical (`currentMetrics`, …), có pointer về bản ghi chạy intel | `eventCategory: business` trên CIO |
| **Pha ghi thô / Pha merge / Pha intel** | Ba giai đoạn luồng ingress trong [KHUNG_LUONG…](./KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md) | CIO-T1/T2/T3; roadmap P0/P1 sản phẩm |
| **CIO-T1 / CIO-T2 / CIO-T3** | Ba **tầng lọc** event có được ghi vào `cio_events` — [DE_XUAT_NANG_CAP_CIO_EVENTBASE](./DE_XUAT_NANG_CAP_CIO_EVENTBASE.md) §1.1 | Bản ghi chạy intel; pha ingress |

---

## 1. Hai trục “layer” — không trộn khái niệm

| Trục | Tên gọi trong dự án | Ý nghĩa | Ghi chú |
|------|---------------------|---------|--------|
| **Trục dữ liệu & định danh** | **L1-persist / L2-persist** (mirror / canonical) | **L1-persist:** mirror / thô theo nguồn. **L2-persist:** canonical đã merge (`uid`, `links`, …). | **Không** phải trường BSON `layer1`/`layer2` CRM, **không** phải bước rule CIX “L1/L2/L3”, **không** phải Ads Layer 1/2. |
| **Trục pipeline intelligence (CRM)** | **`raw` → `layer1` → `layer2` → `layer3`** (và tùy chọn **flag**) | Chuỗi **tính toán / suy diễn** trên entity **canonical (L2-persist)** (hoặc tương đương). | Có thể **derive lại** từ raw + thời điểm nếu thiết kế đúng. |

**Quy tắc:** Trong code và tài liệu, nếu chỉ viết “L1/L2” hãy thêm ngữ cảnh: **mirror/canonical (persist)** hay **pipeline rule CIX** hay **trường `layer1`/`layer2` CRM** hay **Ads metric layer** — hoặc dùng đúng mã trong bảng mục 0.

---

## 2. Khuôn pipeline intelligence trên một entity canonical (L2-persist)

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

## 3. Lưu kết quả tính toán (bản ghi chạy intel + read model intel)

Khớp [KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md](./KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md) mục **1.3**.

| Thành phần | Ví dụ | Vai trò |
|------------|-------|--------|
| **Bản ghi chạy intel** | `cix_analysis_results`, `crm_customer_intel_runs` | Lịch sử mỗi lần job terminal, audit, debug AID, so sánh trước–sau. |
| **Read model intel** | `crm_customers.currentMetrics`, field tóm tắt CIX | UI, sort, context packet; chỉ **denormalize**; luôn **truy ngược bản ghi chạy intel** qua pointer miền (CRM: `intelLastRunId` / `intelLastComputedAt` / `intelSequence`; API profile: `intelSummary`). Tên generic trong contract có thể là `lastIntelResultId` / `parentJobId` tuỳ miền. |

**Trường tối thiểu gợi ý trên bản ghi chạy intel:** `ownerOrganizationId`, khóa entity canonical miền, `computedAt` / `failedAt` (audit — lúc worker kết thúc), `status` (`success` | `failed` | `skipped`), liên kết `traceId` / `parentJobId` khi có; lỗi: `errorCode`, `errorMessage` tóm tắt.

**Khi cần sort lịch sử đúng thứ tự nghiệp vụ** (merge/event đến lệch thời gian so với worker): thêm **mốc thay đổi nguồn** trên run (ví dụ `causalOrderingAt`, nguồn từ payload `causalOrderingAtMs` trên event/job) và **số thứ tự monotonic** trên canonical (ví dụ `intelSequence` trên `crm_customers`) để tie-break — tham chiếu CRM trong [KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md](./KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md) mục 1.3.

**Job queue** (`*_intel_compute`) xử lý retry / `processError` **không thay** bản ghi chạy intel khi miền cần báo cáo lịch sử lỗi đầy đủ — hai cơ chế bổ trợ.

**CRM — một khách vs đa khách (tránh hiểu nhầm khi backfill / đọc API):**

- **Job một khách** (`refresh`, `recalculate_one`, …): sau khi worker terminal, ghi **một** dòng `crm_customer_intel_runs` (filter `unifiedId`), cập nhật pointer trên `crm_customers`, có thể dùng `GET …/intel-runs`.
- **Job đa khách** (`recalculate_all`, `recalculate_batch`, `classification_refresh`, …): thường **một** bản ghi chạy intel kiểu `multiCustomerJob` cho cả job; **không** cập nhật `intelLastRunId` từng khách trong cùng lần persist đó. Metrics trên từng `crm_customers` vẫn có thể đã được tính lại bên trong batch — nhưng **lịch sử intel theo từng khách** cần thêm lần chạy **một khách** (hoặc mở rộng code sau này).

**Payload `causalOrderingAtMs`:** merge/defer/refresh debounce có thể set theo nguồn; với `crm.intelligence.compute_requested`, nếu payload chưa có mốc thì **gán lúc emit** (wall-clock) để backfill/API vẫn có thứ tự nghiệp vụ ổn định.

---

## 4. Lịch sử thay đổi và “điểm thời gian trong quá khứ”

### 4.1 Mục tiêu

- Biết **trạng thái intelligence / classification** của entity **tại mốc T** (báo cáo theo kỳ, xu hướng, phát sinh).
- Không suy đoán ngược từ **trạng thái hiện tại** trên canonical (L2-persist) nếu đã có nhiều thay đổi sau T.

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
| Báo cáo “tại ngày D” | API/service đọc **snapshot cuối cùng ≤ D**, không chỉ `updated_at` document canonical hiện tại. |
| Đồng bộ với AID | Sau khi intel terminal, emit handoff (`*_intel_recomputed`, …) theo [KHUNG_LUONG…](./KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md) **Pha intel**. |

---

## 5. Vị trí trong luồng tổng (CIO · Domain · AID)

- **Ingress / merge** đưa dữ liệu về **mirror → canonical (L1-persist → L2-persist)** (khi miền có merge).
- **Worker domain** chạy **`{domain}_intel_compute`**, cập nhật **read model intel**, ghi **bản ghi chạy intel** khi cần, phát event về AID.
- **AID** debounce / dispatch / case — **không** chứa logic intel nặng; chi tiết [co-cau-module-aid-va-domain-queue.md](../module-map/co-cau-module-aid-va-domain-queue.md).

---

## 6. Tham chiếu code CRM (implementation mẫu)

| Chủ đề | Vị trí |
|--------|--------|
| Persist run intel khách (bản ghi chạy intel) + causal / sequence | `api/internal/api/crm/service/service.crm.intel_run.go`; payload `causalOrderingAtMs`: `aidecision/crmqueue` (mặc định lúc emit nếu thiếu); debounce max causal: `aidecision/eventintake/crm_intel_after_ingest_defer.go` |
| API lịch sử + tóm tắt profile | `crm/handler/handler.crm.customer.go` — `HandleListIntelRuns`; `service.crm.fullprofile.go` — `intelSummary` |
| Cấu trúc `metricsSnapshot` (raw…layer3) | `api/internal/api/crm/service/service.crm.snapshot.go` — `buildMetricsSnapshot`, `ensureLayer3InMetrics` |
| Aggregate → `currentMetrics` | `api/internal/api/crm/service/service.crm.metrics.go` — `BuildCurrentMetricsFromOrderAndConv` |
| Activity sau recalculate / timeline | `api/internal/api/crm/service/service.crm.recalculate.go` — `logRecalculateActivity`, comment `GetLastSnapshotPerCustomerBeforeEndMs` |
| Derive layer3 | `api/internal/api/report/layer3/layer3.go` |
| Báo cáo theo snapshot trước `endMs` | `api/internal/api/report/service/service.report.customers.trend.go`, `service.report.customer.phatsinh.go` |
| Chẩn đoán layer3 / nguồn dữ liệu | [LAYER3_INTELLIGENCE_DIAGNOSTIC.md](./LAYER3_INTELLIGENCE_DIAGNOSTIC.md) |

---

## 7. Checklist khi thêm hoặc mở rộng module intelligence

1. **Tách trục:** Đã phân biệt **L1-persist / L2-persist** với **raw / layer1 / layer2 / layer3** (metrics CRM) và với **pipeline rule CIX** / **Ads metric layer**?
2. **Raw đủ facts:** Có thể derive lại classification / layer3 từ raw + `endMs` (hoặc từ event lịch sử) không?
3. **Hai lớp lưu intel:** Đã chọn rõ **bản ghi chạy intel** (mỗi run) và **read model intel** theo mục 3?
4. **Lịch sử / báo cáo:** Có cơ chế **snapshot tại `activityAt`** và truy vấn **last before T** nếu cần trend?
5. **Queue & AID:** Job nặng ở **worker miền**; handoff event đúng contract; không side-effect intel ngoài `applyDatachangedSideEffects` (xem NGUYEN_TAC).
6. **Rule Engine (nếu có):** Khai báo `domain`, `from_layer`, `to_layer`; param version; trace — [rule-intelligence.md](../02-architecture/core/rule-intelligence.md).
7. **Lịch sử run không FIFO:** Có truyền mốc nghiệp vụ qua queue và ghi vào **bản ghi chạy intel** + tie-break monotonic (pattern CRM) nếu UI/audit cần sort đúng thứ tự thay đổi nguồn?

---

## 8. Tài liệu liên quan

| Tài liệu | Nội dung |
|----------|----------|
| [KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md](./KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md) | Pha ghi thô / merge / intel; mirror–canonical; persist intel 1.1–1.3 |
| [PHUONG_AN_CRM_RULE_INTELLIGENCE.md](./PHUONG_AN_CRM_RULE_INTELLIGENCE.md) | Layer Rule CRM: raw → classification → flag |
| [LAYER3_INTELLIGENCE_DIAGNOSTIC.md](./LAYER3_INTELLIGENCE_DIAGNOSTIC.md) | Luồng dữ liệu layer3 CRM |
| [rule-intelligence.md](../02-architecture/core/rule-intelligence.md) | Kiến trúc Rule Intelligence (Ads, CRM) |
| [TAB4_CUSTOMER_INTELLIGENCE_DESIGN.md](../02-architecture/TAB4_CUSTOMER_INTELLIGENCE_DESIGN.md) | Tab dashboard CEO (ngữ cảnh sản phẩm) |
| [co-cau-module-aid-va-domain-queue.md](../module-map/co-cau-module-aid-va-domain-queue.md) | AID vs queue miền |

---

**Changelog**

- **2026-04-07:** Bản đầu — gom khuôn raw/layer/flag, bản ghi chạy intel + read model intel, lịch sử & point-in-time theo CRM + KHUNG ingress/intel.
- **Cập nhật:** Mục 3 + checklist §7 + bảng §6 — pattern sort lịch sử intel (`causalOrderingAt`, `intelSequence`, payload `causalOrderingAtMs`); khớp KHUNG_LUONG mục 1.3.
- **Cập nhật:** Sau mục 3 — CRM job một khách vs đa khách; pointer read model intel đặt tên thực tế; causal mặc định lúc emit; bảng §6 thêm API intel-runs / fullprofile.
- **2026-04-07:** Mục 0 — bảng thuật ngữ; thống nhất từ **L1-persist/L2-persist**, **bản ghi chạy intel**, **read model intel**, **Pha ghi thô/merge/intel**, **CIO-T***.
- **Cập nhật:** Mở đầu — “Dành cho ai” + “Đọc nhanh” (ngôn từ đỡ khô, rõ đối tượng đọc).
