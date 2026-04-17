# Bảng giai đoạn — bước — sự kiện (đầu-cuối: CIO → Domain → AID → Executor → Learning)

**Toàn bộ tài liệu sắp theo khung sáu trường** (cùng «ngôn ngữ» với **một bước** trên timeline — xem mục 4.1): `label` · `purpose` · `inputSummary` · `logicSummary` · `resultSummary` · `nextStepHint`.

### Khung tham chiếu vs thực tế

Chuỗi **G1–G6** và catalog API (`e2e-reference-catalog`) là **khung tiêu chuẩn tham chiếu toàn trình** — mô tả đủ đường đi để đối chiếu code, UI swimlane và audit. **Trên runtime**, từng sự kiện / job có thể **vào hoặc ra** luồng ở **bất kỳ điểm** (bắt đầu giữa chừng, kết thúc sớm, nhảy pha, lặp nhánh, bỏ bước…). `e2eStage` / `e2eStepId` **neo vị trí gần đúng** trên khung; **không** suy ra mọi bản ghi đều đi hết mọi bước trong bảng.


| Trường              | Phần trong file                                                                                                        | Nội dung chính                                                                                                          |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `**label`**         | [§1](#1-label--tên-và-quy-ước)                                                                                         | **Trục vụ** (pha/bước theo dõi vụ); quy ước G1–G6 / `Gx-Syy`; **§1.1–1.4** từ vựng miền, mã lưu đồ, owner, meta job miền. |
| `**purpose`**       | [§2](#2-purpose--vì-sao-có-luồng-này)                                                                                  | Vì sao cần trace/audit; ràng buộc emit/publish; hai lớp envelope vs consumer.                                           |
| `**inputSummary`**  | [§3](#3-inputsummary--đầu-vào-và-tham-chiếu-code) (kèm [§3.1](#31-api-catalog-e2e-json-cho-frontend) API catalog JSON) | Collection, file code, hàm resolve, link hợp đồng — **đầu vào** để đọc luồng; GET `e2e-reference-catalog` cho frontend. |
| `**logicSummary`**  | [§4](#4-logicsummary--cách-hệ-thống-chạy)                                                                              | Khung sáu trường **từng bước**, Publish, `processTrace` consumer, Phương án B, khung mốc + liên kết.                    |
| `**resultSummary`** | [§5](#5-resultsummary--đầu-ra-và-tra-cứu)                                                                              | **Sáu pha chính** + neo G1–G6; [gom bước](#gom-buoc-truc-vu) §5.2; `outcome*`; §5.3; mức độ khớp code.                  |
| `**nextStepHint`**  | [§6](#6-nextstephint--đọc-tiếp-và-vận-hành)                                                                            | Mapping P01–P13, ghi chú vận hành, changelog.                                                                           |


---

## 1. `label` — Tên và quy ước

`**label` (cấp tài liệu):** Bảng tra **giai đoạn — bước — sự kiện** end-to-end; tên kỹ thuật giữ nguyên theo code.

**Nguyên tắc ngôn ngữ:** Mô tả tiếng Việt; giữ nguyên `eventType`, `eventSource`, `pipelineStage`, tên hàm, collection, endpoint.

**Cách chia pha:** Trên **trục vụ** đọc **sáu pha chính** — pha ghi thô, pha merge, pha intel, pha ra quyết định, pha thực thi, pha học ([§5.2](#52-bảng-giai-đoạn-lớn-đọc-trước)). **Pha ghi thô (G1)** trên máy chỉ **CIO ingress → L1 → `EmitDataChanged` → enqueue** (`G1-S01`…`G1-S04`; xem `E2EStageCatalog`). **Pha merge (G2)** — trên **trục vụ**, sau khi L1 đổi, **mục tiêu nghiệp vụ** là **gom và merge canonical (L2)**; có **debounce** (cửa sổ trì hoãn) và **gấp** (chỉ bỏ hoặc rút ngắn bước **gom**, ưu tiên xử lý ngay và trước — **vẫn đủ** luồng `applyDatachangedSideEffects` → routing → `dispatchConsumerEvent`). **Consumer một cửa** (`G2-S01` lease một job — **điển hình** trên trục merge là `**l1_datachanged`** sau G1-S04; cùng bước lease cho mọi job queue khác, `G2-S02` toàn bộ `processEvent` — **gom** + **gấp** điều chỉnh gom, không tắt luồng) có thể **xếp job merge cho worker miền** (minh hoạ CRM: `crm_pending_merge`); **worker miền** (`G2-S03…S05`) gộp L1→L2 rồi **emit lại** `decision_events_queue` cho AID (minh hoạ: `crm.intelligence.recompute_requested`, `eventSource` `crm_merge_queue` qua `aidecision/crmqueue`) — cùng thuộc pha merge; nhiều mốc timeline consumer có thể cùng neo **G2-S02** (catalog); worker merge neo **G2-S03…S05** (§5.3). Trong **code / resolver / API** có **sáu** giai đoạn **G1–G6** (`e2eStage`, bảng §5.3, trường `stages` của `e2e-reference-catalog`); trong đó có **bước (S)** và **sự kiện (E)**. Pha nhỏ cũ P01–P13 gom ở [§6](#6-nextstephint--đọc-tiếp-và-vận-hành).

**Quy ước ID:** `Gx-Syy`; chi tiết `Gx-Syy-Ezz` khi cần.

### Trục vụ — mục đích của pha và bước

**Pha (sáu pha chính; neo G1–G6) và bước (`Gx-Syy`) là trục vụ:** dùng để **theo dõi một vụ** theo hành trình nghiệp vụ đầu–cuối — từ lúc dữ liệu vào hệ thống (CIO / kênh ingress), qua ghi miền và event, điều phối AID, intel miền, case và quyết định, thực thi, tới outcome, `**learning_cases`** và vòng feedback. Catalog E2E, `e2eStage` / `e2eStepId` trên timeline và bảng §5 phục vụ **câu hỏi «vụ này đang ở đoạn nào của quy trình?»**, không phải danh sách mọi hàng đợi vật lý.

**Tách khỏi trục triển khai (nhiều queue / worker):** runtime có **nhiều** collection job và worker ở module khác nhau (ví dụ `decision_events_queue`, debounce, merge pending, `*_intel_compute`, …). Một **pha trục vụ** có thể tương ứng **chuỗi job** qua vài queue; **không** quy định mỗi queue là một dòng `Gx-Syy` trong bảng. Việc đi qua queue nào, worker nào — mô tả ở **Publish** (`refs`, `businessDomain`), cây `**processTrace`**, `**TraceStep`** (`inputRef` / `outputRef`), và tài liệu vận hành.

**Sự kiện (`eventType`, envelope):** là **cầu nối** giữa trục vụ và máy (map trong `e2e_reference.go`); bảng chi tiết §5.3 vẫn liệt kê đủ để tra code, nhưng **đọc cho người** ưu tiên theo **vị trí trên trục vụ** (Gx / nhãn bước), rồi mới drill-down kỹ thuật. **Gom bước theo trục vụ:** [§5.2 — sáu pha chính + đoạn gom](#gom-buoc-truc-vu).

### 1.1. Lớp kiến trúc (swimlane) — mã cho lưu đồ dòng chảy

**Mục đích:** Một **hàng ngang** (swimlane) trên lưu đồ = một lớp trách nhiệm; **không** trộn với tên module Go hay `eventSource`. Cột **«Nhóm trách nhiệm»** ở [§5.3](#bang-catalog-chi-tiet-e2e) khớp cột **«Nhóm trách nhiệm (cột §5.3)»** trong bảng dưới; cột **«Tên đầy đủ (VN)»** dùng làm nhãn swimlane cho người đọc (song song với mã `ING`…`FBK`).


| Mã lưu đồ | Tên đầy đủ (VN)                                             | Nhóm trách nhiệm (cột §5.3)                  | Giai đoạn Gx điển hình                            |
| --------- | ----------------------------------------------------------- | -------------------------------------------- | ------------------------------------------------- |
| `**ING`** | Tiếp nhận / kênh vào (Ingress)                              | `CIO` (catalog G1; tương đương swimlane ING) | Pha ghi thô — G1 (webhook, sync kênh)             |
| `**DOM`** | Miền dữ liệu (hợp nhất L2)                                  | `DomainData`                                 | Pha merge — G2 (L1→L2)                            |
| `**AID`** | AI Decision (bus `decision_events_queue`, consumer một cửa) | `AID`                                        | Pha merge (consumer) + pha ra quyết định — G2, G4 |
| `**INT`** | Intel miền (job `*_intel_compute`, handoff `*_recomputed`)  | `DomainIntel`                                | Pha intel — G3                                    |
| `**EXC`** | Thực thi (Executor)                                         | `Executor`                                   | Pha thực thi — G5                                 |
| `**OUT`** | Kết quả giao / phản hồi kênh (outcome kỹ thuật)             | `Outcome`                                    | Pha học — G6 (đoạn đầu)                           |
| `**LRN`** | Học tập (learning cases, đánh giá)                          | `Learning`                                   | Pha học — G6                                      |
| `**FBK`** | Phản hồi cải tiến (gợi ý rule/policy)                       | `Feedback`                                   | Pha học — G6 (đoạn cuối)                          |


**Tiêu đề file** (*CIO → Domain → AID → Executor → Learning*) đọc theo swimlane: `**ING`/`DOM` → `AID` → `EXC` → `LRN`** (có thể lồng `**INT`** giữa `DOM` và `AID`, `**OUT`/`FBK`** sau `EXC`).

### 1.2. Miền nghiệp vụ (bounded context) — tên gọi thống nhất

**Mục đích:** Trong **subgraph** bên trong swimlane `DOM` / `INT`, dùng **một** bộ tên dưới đây (cột «Tên trên lưu đồ»); tránh xen kẽ «CRM / crm / khách», «Ads / Meta / quảng cáo» không có quy ước.


| Mã miền            | Tên trên lưu đồ (VN)                    | Package Go (tham chiếu)           | Ghi chú ngắn                                                                                          |
| ------------------ | --------------------------------------- | --------------------------------- | ----------------------------------------------------------------------------------------------------- |
| `**cio`**          | CIO (điều phối đa kênh)                 | `cio/`                            | Ingress có kiểm soát; không thay cho toàn bộ `ING`.                                                   |
| `**pc`**           | Pancake (PC)                            | `pc/`                             | Ingress POS/Pages → L1.                                                                               |
| `**fb`**           | Facebook                                | `fb/`                             | Ingress Page/hội thoại → L1.                                                                          |
| `**webhook`**      | Webhook                                 | `webhook/`                        | HTTP ngoài → thường chuyển tiếp sync.                                                                 |
| `**meta`**         | Meta Ads (thực thể & insight L1)        | `meta/`                           | Tiền đề pipeline quảng cáo; job intel Meta.                                                           |
| `**ads`**          | Ads (API/rule phía Ads)                 | `ads/`                            | Phối hợp với `meta` trên luồng intel/đề xuất.                                                         |
| `**crm`**          | CRM (khách, merge, intel CRM)           | `crm/`                            | L2 canonical, `crm_pending_merge`, `crm_intel_*`.                                                     |
| `**order`**        | Đơn hàng (commerce)                     | `order/`, `orderintel/`           | Đồng bộ đơn + `order_intel_*`.                                                                        |
| `**conversation**` | Hội thoại (mirror messaging)            | `conversation/`, `fb/` (tin nhắn) | Sự kiện `conversation.*` / `message.*` trên bus.                                                      |
| `**cix**`          | CIX (intel hội thoại theo pipeline CIX) | `cix/`, `conversationintel/`      | `cix.analysis_requested`, `cix_intel_*`; **không** gọi chung là «AI Decision» khi ý chỉ pipeline CIX. |
| `**report`**       | Báo cáo                                 | `report/`                         | Dirty/snapshot; side-effect sau datachanged.                                                          |
| `**notification`** | Thông báo                               | `notification/`                   | Kênh/template — ngoài trục E2E chính nếu không vẽ.                                                    |


**Quy tắc vẽ:**

1. **Swimlane** chỉ dùng mã `**ING` … `FBK`** (§1.1).
2. **Ô / subgraph miền** dùng **«Tên trên lưu đồ (VN)»** + khi cần chú thích nhỏ mã `**crm`**, `**cix`**, … trong ngoặc (để khớp repo).
3. `**eventSource**` trên envelope (vd. `crm_intel`, `order_intel`, `meta_ads_intel`) là **nhãn máy** — giữ nguyên trong bảng §5.3; trên lưu đồ người đọc ưu tiên **tên miền §1.2**, kèm chú thích `eventSource` nếu cần audit.

**Timeline / API (`DecisionLiveEvent`):** field `**businessDomain`** (JSON) — **module đang xử lý mốc** (queue/worker phát `Publish`), không đồng nhất với «chủ nghiệp vụ của payload» trên envelope: milestone consumer `**decision_events_queue`** (pha `queue_processing` / `queue_done` / `queue_error` / `datachanged_effects`, …) → `**aidecision`** dù `eventType` là `crm_*` hay `pos_*`; milestone worker intel miền (`intel_domain_*` + `refs.intelDomain`) → `**crm` | `order` | `cix` | `ads`**; pipeline execute/orchestrate/engine trong AID → `**aidecision`**. Mã vẫn nằm trong cùng bảng «Mã miền» §1.2 (`crm`, `order`, `meta`, `pc`, `cix`, `aidecision`, `unknown`, …) để lọc UI. Kèm `**businessDomainLabelVi**` (nhãn tiếng Việt + mã trong ngoặc, vd. «AI Decision (aidecision)») — UI cột swimlane «module xử lý» phải dùng hai field này, không dùng `**feedSourceLabelVi**` (chip «Nguồn»; giá trị «Khác» chỉ nghĩa nhóm nguồn dữ liệu chưa xếp loại, không phải tên module). Emitter ghi đè hợp lệ qua `**refs.businessDomain`**. Persist org-live: cột phẳng `**businessDomain`**, `**businessDomainLabelVi**`, `refs`. Logic: `api/internal/api/aidecision/decisionlive/business_domain_enrich.go`. Lưu đồ nghiệp vụ theo chủ đề payload → dùng khái niệm `**ownerDomain**` ở [§1.3](#13-owner-domain-của-event-để-vẽ-lưu-đồ-nghiệp-vụ), tách với `businessDomain`.

**Đọc thêm (ngữ cảnh kiến trúc):** `eventtypes/names.go`, `pipeline_stage.go`, `**e2e_reference.go`**, `aidecision/crmqueue/domain_queue_bus.go` (meta job miền — [§1.4](#14-meta-trên-document-job-hàng-đợi-miền-đồng-bộ-bus--chuẩn-gộp-một-nguồn)). Hợp đồng: [unified-data-contract 2.5e](../../docs-shared/architecture/data-contract/unified-data-contract.md#contract-25e-e2e-reference); [api-context 4.14](../../docs-shared/ai-context/folkform/api-context.md#version-414). Khung ingress: `docs/05-development/KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md`. Nhóm module B–D: [co-cau-module-aid-va-domain-queue.md](../module-map/co-cau-module-aid-va-domain-queue.md).

### 1.3. Owner domain của event (để vẽ lưu đồ nghiệp vụ)

**Mục đích:** Khi theo dõi một event trên `decision_events_queue`, cần phân biệt:

- `**ownerDomain` (miền nghiệp vụ sở hữu ý nghĩa event)**: dùng để vẽ lưu đồ nghiệp vụ.
- `**consumerDomain` (miền thực thi bước điều phối)**: thường là `aidecision` ở **mốc consumer** queue (neo **G2** — `G2-S01`…`S02`), không phải owner business.

**Quy tắc chốt để vẽ lưu đồ:**

1. **Vẽ theo `ownerDomain`**, không vẽ theo tên worker tiêu thụ trước mắt.
2. Nếu mốc là lifecycle của queue consumer (`QueueMilestone*`) thì giữ owner theo `**refs.eventType` gốc**, không đổi sang `aidecision`.
3. Với event handoff `*_intel_recomputed`, owner là **miền intel phát ra** (`crm_intel`, `order_intel`, `cix_intel`, `meta_ads_intel`), dù AID là nơi nhận và quyết định tiếp.


| Dấu hiệu event                                          | `ownerDomain`                                                                     | Gợi ý package/module         |
| ------------------------------------------------------- | --------------------------------------------------------------------------------- | ---------------------------- |
| `conversation.*`, `message.*`                           | `conversation`                                                                    | `fb/`, `conversation/`       |
| `order.*`, `commerce.order_*`                           | `order`                                                                           | `order/`, `orderintel/`      |
| `crm.*`, `customer.context_*`, `customer.flag_*`        | `crm`                                                                             | `crm/`                       |
| `cix.*`, `cix_intel_*`                                  | `cix`                                                                             | `cix/`, `conversationintel/` |
| `ads.*`, `campaign_*`, `meta_ad_*`, `meta_ad_insight.*` | `ads` hoặc `meta` (xem nguồn phát)                                                | `ads/`, `meta/`              |
| `*_intel_recomputed`                                    | miền intel phát event (`crm_intel`, `order_intel`, `cix_intel`, `meta_ads_intel`) | worker miền tương ứng        |
| `aidecision.*`, `executor.propose_requested`            | `aidecision`                                                                      | `aidecision/`                |
| `execution.*`                                           | `executor`                                                                        | `executor/`                  |
| `learning.*`                                            | `learning`                                                                        | `learning/`                  |


**Quy tắc ưu tiên khi suy ra `ownerDomain` từ bản ghi queue (theo thứ tự):**

1. `payload.ownerDomain` (nếu đã có và hợp lệ theo bảng trên).
2. Map từ `eventType` theo bảng trên.
3. Nếu mơ hồ, dùng `eventSource` để chốt miền phát (`crm_intel`, `order_intel`, `meta_ads_intel`, ...).
4. Cuối cùng mới fallback `aidecision` (chỉ khi event thực sự thuộc orchestration của AID).

**Áp dụng vào lưu đồ:** Mỗi node event nên có nhãn dạng `**[ownerDomain] eventType`** (ví dụ: `[crm] crm.intelligence.recompute_requested`) để nhìn thấy rõ event đang được tạo bởi miền nào.

### 1.4. Meta trên document job hàng đợi miền (đồng bộ bus — chuẩn gộp một nguồn)

**Mục đích:** `decision_events_queue` là bus điều phối AID; các collection job miền (`crm_intel_compute`, `cix_intel_compute`, `order_intel_compute`, `ads_intel_compute`, `crm_pending_merge`, …) **không** thay bus, nhưng khi **gộp mọi event / job về một store** (tra cứu, dashboard, audit chéo) cần **cùng một bộ khóa** để biết: envelope nghiệp vụ, chủ nghiệp vụ, ai thực thi, ai phát lệnh enqueue, neo E2E.

**Một nguồn struct trong code:** `api/internal/api/aidecision/crmqueue/domain_queue_bus.go` — `DomainQueueBusFields` + `DomainQueueBusFieldsPtrFromDecisionEvent` (copy từ `DecisionEvent` + `payload.ownerDomain`) + `CompleteDomainJobBus` (gắn thêm `processorDomain` và `enqueueSourceDomain` theo đường enqueue).

**Các field lưu trên document job miền** (BSON/JSON `omitempty` — bản ghi cũ có thể thiếu):

| Trường (BSON) | Ý nghĩa |
| --- | --- |
| `eventType`, `eventSource`, `pipelineStage` | Bản sao envelope bus AID (đồng bộ `eventtypes` + `pipeline_stage.go`). |
| `ownerDomain` | Chủ nghiệp vụ payload — ưu tiên từ `payload.ownerDomain` khi emit bus; khớp tinh thần [§1.3](#13-owner-domain-của-event-để-vẽ-lưu-đồ-nghiệp-vụ). |
| `processorDomain` | Module/worker **thực thi** job (`crm` \| `order` \| `cix` \| `ads`) — hằng số `ProcessorDomain*` trong `domain_queue_bus.go`. |
| `enqueueSourceDomain` | Module/lớp hệ thống **phát lệnh ghi** job (`aidecision`, `crm_datachanged`, `orderintel`, `conversationintel`, `system_debounce`, …) — hằng số `EnqueueSource*`. |
| `e2eStage`, `e2eStepId` | Neo catalog G1–G6 copy từ `DecisionEvent` khi job sinh ra từ bus AID; job chỉ từ datachanged nội bộ có thể rỗng hoặc bù sau. |

**Ghi chú:** `processorDomain` / `enqueueSourceDomain` **không** thay cho `businessDomain` trên timeline live ([§1.2](#12-miền-nghiệp-vụ-bounded-context--tên-gọi-thống-nhất) — cột swimlane «module xử lý mốc»); chúng phục vụ **document queue Mongo** và kịch bản **hợp nhất dữ liệu** sau này.

**Vị trí trong cơ cấu module:** nhóm D và vai trò `crmqueue` — xem [co-cau-module-aid-va-domain-queue.md](../module-map/co-cau-module-aid-va-domain-queue.md) (mục **Meta job miền — đồng bộ envelope bus AID**).

---

## 2. `purpose` — Vì sao có luồng này

`**purpose`:** Cho phép **trace** và **audit** thống nhất — cùng một việc nhìn được trên queue, timeline live, Mongo org-live; neo G1–G6 không mơ hồ.

**Ràng buộc vận hành:** Mọi emit `decision_events_queue` gán `e2eStage` / `e2eStepId` / `e2eStepLabelVi` (và `e2e*` trong payload) theo `ResolveE2EForQueueEnvelope`. Mọi `decisionlive.Publish` làm giàu `DecisionLiveEvent` qua `enrichPublishE2ERef` (refs hoặc `phase`); timeline consumer gán **G2-S01** (bắt đầu) hoặc **G2-S02** (các mốc sau — gồm `datachanged_done`, `handler_done`, …) qua `ResolveE2EForQueueConsumerMilestone`. Persist `decision_org_live_events` có cột phẳng `e2e*`, `**businessDomain`**, `**outcomeKind`**, `**outcomeAbnormal**`, `**outcomeLabelVi**`, `**processTrace**`, `payload` đầy đủ.

**Hai lớp đọc trên cùng một job queue:** (1) **Envelope** — nghĩa nghiệp vụ theo `eventType` (G1 enqueue từ L1, G2 merge / intel tiếp theo, G3 intel, G4 quyết định…); (2) **Mốc consumer live** — neo **G2** (pha merge — consumer một cửa) khi worker xử lý job. Một trace có thể có cả mốc **G2** (consumer) và **G4** (engine / case).

---

## 3. `inputSummary` — Đầu vào và tham chiếu code

`**inputSummary`:** **Vào** hệ thống gồm envelope queue, org, `traceId`, miền dữ liệu — tra theo bảng dưới và bảng chi tiết [§5](#5-resultsummary--đầu-ra-và-tra-cứu).

### Bảng tham chiếu E2E trong code (một nguồn)


| Thành phần                           | Vai trò                                                                                                                                                                                                                                                                                                                                                                              |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `**eventtypes/e2e_reference.go`**    | Nguồn sự thật máy: `ResolveE2EForQueueEnvelope`, `ResolveE2EForQueueConsumerMilestone`, `ResolveE2EForLivePhase`, `MergePayloadE2E`. Đổi bảng G/S/E ở doc phải cập nhật map tương ứng trong file này.                                                                                                                                                                                |
| `**decision_events_queue`**          | Trường top-level: `e2eStage`, `e2eStepId`, `e2eStepLabelVi`; payload lặp khóa `e2eStage`, `e2eStepId`, `e2eStepLabelVi` (emit qua `EmitEvent` / `eventemit.EmitDecisionEvent`). Bản ghi cũ trước khi triển khai có thể thiếu các trường này.                                                                                                                                         |
| `**decisionlive.Publish`**           | Sau enrich trace/feed: `enrichPublishE2ERef` — nếu mốc đã có `e2eStepId` (ví dụ timeline consumer) thì **không ghi đè**; nếu chưa có: ưu tiên map từ `refs.eventType` (+ `eventSource`, `pipelineStage` trong refs), không đủ thì map từ `phase`. Sau đó: `CapDecisionLiveProcessTrace`, `**EnrichLiveOutcomeMetadata`** (`outcomeKind` / `outcomeAbnormal` / `outcomeLabelVi`).     |
| **Timeline consumer (queue → live)** | `decisionlive/livecopy/BuildQueueConsumerEvent`: gắn **G2-S01** hoặc **G2-S02** theo mốc vòng đời worker (nhiều mốc sau lease neo **G2-S02**), **độc lập** với nghĩa nghiệp vụ của `eventType` (ví dụ job `crm_intel_recomputed` vẫn hiển thị **G2** trên mốc consumer). `**TraceStep`** (Phương án B): `inputRef` / `outputRef` / `reasoning` — xem `livecopy/queue_trace_step.go`. |
| `**decision_org_live_events`**       | `BuildOrgLivePersistDocument` ghi cột phẳng `e2eStage`, `e2eStepId`, `e2eStepLabelVi`, `outcomeKind`, `outcomeAbnormal`, `outcomeLabelVi`, `processTrace` (cùng `payload` JSON đầy đủ). Index Mongo: xem struct `AIDecisionOrgLiveEvent`.                                                                                                                                            |
| `**aidecision/crmqueue/domain_queue_bus.go`** | Meta **job miền**: `DomainQueueBusFields`, `DomainQueueBusFieldsPtrFromDecisionEvent`, `CompleteDomainJobBus`, `OwnerDomainFromDecisionPayload`; field đồng bộ trên model `CrmIntelComputeJob`, `CixIntelComputeJob`, `OrderIntelComputeJob`, `AdsIntelComputeJob`, `CrmPendingMerge` — xem [§1.4](#14-meta-trên-document-job-hàng-đợi-miền-đồng-bộ-bus--chuẩn-gộp-một-nguồn). |


### 3.1. API catalog E2E (JSON cho frontend)

**Mục đích:** Frontend (lưu đồ swimlane, tooltip, legend, bảng tra cứu) lấy **cùng nội dung** với §5.2–5.3 dưới dạng JSON — không cần hard-code bảng G1–G6 trong app.

**HTTP:** **GET** `/v1/ai-decision/e2e-reference-catalog` — quyền `**MetaAdAccount.Read`** + **org context** (cùng pattern các GET `/ai-decision/*` khác). Handler: `api/internal/api/aidecision/handler/handler.aidecision.e2e_catalog.go`.

**Response** (`status: success`): object `data` gồm:


| Khóa              | Nội dung                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| ----------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `schemaVersion`   | Số nguyên (`eventtypes.E2ECatalogSchemaVersion`) — tăng khi đổi shape hoặc cột; client có thể cache theo version. **v35:** **bỏ** bước catalog **G4-S04** — gộp execute/propose (**G4-S03-E01…E03**) vào **G4-S03**; `ResolveE2EForQueueEnvelope` / `ResolveE2EForLivePhase` (`queued`, `propose`, `done`, `error`) theo **G4-S03** / **G4-S03-E\***; `stages` **G4** — **bốn bước** trục vụ; **v34:** `steps` — **bỏ** dòng catalog **G4-S03** cũ (`message.batch_ready`); đánh lại **G4-S04→G4-S03**, **G4-S05→G4-S04** (**E01–E03**); envelope **`message.batch_ready`** → **`e2eStepId` G2-S02** (`ResolveE2EForQueueEnvelope`); `stages` **G4** — **năm bước** trục vụ (intel → context → logic → execute/propose); **v33:** `steps` — **G4-S02** gộp **một dòng** (`*.context_requested` + `*.context_ready`); `**e2eStepId` resolver** = **G4-S02** (không còn G4-S02-E01…E04); **v32:** `stages` **G4** — **bốn bước trục vụ** (trước v34; sau v34 là năm bước và đánh lại S03/S04) trong `summaryVi` / `userSummaryVi` (G4-S01 xử lý `<domain>_intel_recomputed` → G4-S02 yêu cầu / nhận `***.context_ready`** → G4-S04 đủ ngữ cảnh + policy → G4-S05 sang Executor); **v31:** envelope `**<domain>_intel_recomputed`** trên `decision_events_queue` → `**e2eStepId` G4-S01** (AID nhận — case); catalog **G3-S06** = bước **miền phát** handoff; **v30:** **G3-S02** AID **xếp job** `*_intel_compute`; handoff emit catalog **G3-S06**; **livePhase** `intel_domain_compute_done` → **G3-S05**; **v29:** **G3-S05** gộp **một dòng** intel_recomputed (trước v30; từ v30 là **G3-S06**); **v28:** **G3-S01** — điển hình **báo đổi L2** (`l2_datachanged`), cùng cách nói với G2-S01 và `l1_datachanged`; ghi chú resolver G2-S05-E01 vs gom một dòng intel; **v27:** **G2-S01** — điển hình **một job `l1_datachanged`** (trục merge sau G1-S04); cùng lease cho mọi job queue; `queueMilestones` + `ResolveE2EForQueueConsumerMilestone` / `ResolveE2EForLivePhase` (`queue_processing`) đồng bộ chữ; **v26:** **G3** **ba bước** (nhận queue kể `**l2_datachanged`** → `***_intel_compute`** → `***_intel_recomputed`** về AID); **v25:** **G3-S01** gộp **một dòng** (prefix miền `crm.*` / `ads.*` / `order.*` / `cix.*`); **v24:** **G3** — mô tả chuỗi **L2-datachanged → AID (gom/gấp) → hàng đợi *_intel_compute miền → _intel_recomputed* lại `decision_events_queue`; v23: G2-S05 — wire `**l2_datachanged`** + `**<prefix>.changed**` (+ emit `EmitAfterL2MergeForCrmIntel`, nhánh consumer `IsPostL2MergeCrmIntelEnvelope`); **v22:** G2-S05 / G1-S04 — cùng vai trò enqueue «báo thay đổi»; **v21:** G2-S05 wire vs G1-S04; **v20:** `crm_pending_merge` → `crmqueue`; **v19:** **gấp** vs **gom**; **v18:** **G2-S02** **gom** + **gấp**; **v17:** **G2** gộp consumer; **v15:** G2 trục vụ L1→merge; **v14:** wire G1 `<prefix>.changed`; **v13:** catalog `.changed`; **v12:** G1-S04 wire cũ; **v11:** G1-S04 một dòng; **v10:** hai cột mô tả + `userSummaryVi` + `userLabelVi`. |
| `docRef`          | Chuỗi tham chiếu tài liệu (§5.2–5.3 file này).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `stages`          | Mảng **sáu** giai đoạn G1…G6 — khớp bảng **G1–G6** trong §5.2 (`E2EStageCatalog`): `id`, `swimlaneCode`, `titleVi`, `summaryVi` (kỹ thuật), `**userSummaryVi`** (thân thiện end-user), `ingressHint`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| `steps`           | Mảng bước chi tiết — khớp **§5.3**: `stageId`, `stepId`, `eventDetailId`, `**descriptionTechnicalVi`**, `**descriptionUserVi`**, `eventType`, `eventSource`, `pipelineStage`, `responsibilityGroup`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| `queueMilestones` | Mảng mốc consumer (**G2**): `key`, `stageId`, `stepId`, `labelVi` (kỹ thuật), `**userLabelVi`** (thân thiện) — khớp `ResolveE2EForQueueConsumerMilestone`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `livePhaseMap`    | Mảng map `DecisionLiveEvent.phase` → `e2eStage`, `e2eStepId`, `e2eStepLabelVi` — sinh từ `ResolveE2EForLivePhase` (`decisionlive/e2e_live_phase_catalog.go`).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |


**Nguồn code bảng tĩnh:** `api/internal/api/aidecision/eventtypes/e2e_catalog.go` — **bắt buộc đồng bộ** khi sửa bảng Markdown §5.2–5.3. Khi thêm **phase** timeline mới: thêm hằng trong `decisionlive/types.go`, cập nhật `e2e_reference.go` (`ResolveE2EForLivePhase`) và danh sách phase trong `E2ELivePhaseCatalog()`.

---

## 4. `logicSummary` — Cách hệ thống chạy

`**logicSummary`:** **Kiểm tra / suy luận / thứ tự** — Publish, consumer, cây `processTrace`, Phương án B, liên kết giữa các mốc. **Từng micro-bước** trên cây nên điền theo khung sáu trường dưới đây (gói trong `labelVi` + `detailVi` hoặc mở rộng schema sau).

### 4.1. Khung sáu trường — một bước logic (product, trace, audit)


| Trường                  | Ý nghĩa                                           | Yêu cầu                                                                                  |
| ----------------------- | ------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| **1 — `label`**         | Tên bước ngắn — đọc một lần biết **đang làm gì**. | Tiếng Việt; trùng ý `labelVi` trên nút cây.                                              |
| **2 — `purpose`**       | Bước này **tồn tại để làm gì** trong pipeline.    | Một câu — «vì sao có bước này».                                                          |
| **3 — `inputSummary`**  | **Đầu vào** (ref rút gọn).                        | `eventType`, `sourceCollection`, `entityId`, `orgId`, … — không raw PII / full document. |
| **4 — `logicSummary`**  | Đã **kiểm tra / so sánh / suy luận** gì.          | **Quan trọng nhất** cho audit — «vì sao hệ thống làm vậy».                               |
| **5 — `resultSummary`** | **Kết quả trực tiếp** của bước.                   | Khác `nextStepHint`: output của bước hiện tại.                                           |
| **6 — `nextStepHint`**  | **Sau bước này** đi đâu.                          | Nối chuỗi nhân quả giữa nút cây / mốc timeline.                                          |


**Ví dụ `label`:** Nhận hội thoại mới · Đọc lại dữ liệu vừa thay đổi · Kiểm tra chính sách xử lý · Chọn luồng xử lý · Lên lịch xử lý tiếp theo · Đẩy job vào hàng chờ.

**Ví dụ `purpose`:** Xác nhận hệ thống đã nhận sự kiện mới · Đọc snapshot mới nhất của bản ghi · Xác định cần chạy side effect nào · Quyết định tuyến xử lý phù hợp theo loại dữ liệu.

**Ví dụ `inputSummary`:** `eventType: conversation.changed`, `sourceCollection: fb_conversations`, `entityId: …`, `orgId: …`.

**Ví dụ `logicSummary`:** Kiểm tra loại entity là conversation · Kiểm tra policy cho phép cập nhật customer / report / ads · Kiểm tra có cần defer để gom event hay không.

**Ví dụ `resultSummary`:** Xác định cần chạy customer merge · Cần cập nhật report · Defer 90 giây · Đã enqueue job thành công.

**Ví dụ `nextStepHint`:** Chuyển sang bước route collection · Chờ cửa sổ defer · Đẩy sang queue nội bộ · Kết thúc phase đồng bộ, chờ nghiệp vụ chính.

**Ánh xạ sang code hiện tại:**


| Khung sáu trường | `processTrace` (`labelVi`, `detailVi`)         | `TraceStep`                               |
| ---------------- | ---------------------------------------------- | ----------------------------------------- |
| `label`          | `labelVi`                                      | `title`                                   |
| `purpose`        | Mở đầu `detailVi`                              | Đầu `reasoning` (có thể)                  |
| `inputSummary`   | Đoạn ref trong `detailVi` / đồng bộ `inputRef` | `inputRef`                                |
| `logicSummary`   | **Giữa `detailVi`**                            | `**reasoning**`                           |
| `resultSummary`  | Cuối `detailVi` (kết quả bước)                 | `outputRef`                               |
| `nextStepHint`   | Câu kết `detailVi` / bước con kế               | Cuối `reasoning` hoặc `outputRef["next"]` |


---

### 4.2. Publish live event — đọc nhanh

**Một câu (vai trò):** `decisionlive.Publish` là **điểm ghi nhật ký timeline** cho một luồng đã có `traceId`: nó đẩy **một mốc** (`DecisionLiveEvent`) vào ring bộ nhớ, (nếu bật live) broadcast WebSocket và ghi Mongo org-live. Nó **không** enqueue thêm job trên `decision_events_queue` — việc tạo công việc thuộc `EmitEvent` / intake / worker khác.

**Tại sao tách hẳn khỏi queue**


| Khái niệm                            | Ý nghĩa vận hành                                                                                                                                                                                                                     |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Queue (`decision_events_queue`)**  | «Việc cần làm» — consumer lease, xử lý, có thể lỗi / retry.                                                                                                                                                                          |
| **Publish (`decisionlive.Publish`)** | «Điểm đánh dấu trên timeline» — để người vận hành / UI / CHI **nhìn thấy** tiến trình theo **cùng** `traceId`. Một job queue có thể sinh **nhiều** lần Publish (ví dụ: bắt đầu xử lý → xong side-effect datachanged → xong handler). |


**Luồng xử lý một lần gọi `Publish`** (khớp `decisionlive/publish.go`)

```mermaid
flowchart LR
  subgraph in["Đầu vào"]
    A["org + traceId + DecisionLiveEvent"]
  end
  subgraph enrich["Làm giàu"]
    B["tier / feed / E2E / cap processTrace / outcome"]
  end
  subgraph out["Đầu ra"]
    C{"AI_DECISION_LIVE_ENABLED?"}
    C -->|0| D["Chỉ metrics trung tâm chỉ huy"]
    C -->|1 hoặc mặc định| E["Ring theo trace"]
    E --> F["WS theo trace"]
    E --> G["Feed org + WS org"]
    E --> H["Persist org-live async"]
  end
  A --> B --> C
```




| Bước trong code    | Việc làm                                                                                                                                 | Vì sao cần                                                                                                                    |
| ------------------ | ---------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| Điều kiện          | Bỏ qua nếu thiếu `ownerOrgID` hoặc `traceId`                                                                                             | Mọi fan-out (replay, WS, persist) đều bám **cặp org + trace**; không có thì không có chỗ «gắn» mốc.                           |
| Chuẩn hóa envelope | `schemaVersion`, `stream`, `tsMs`, `severity`, gắn `traceId` / `org`                                                                     | Client, metrics và persist đọc **cùng một khung** sự kiện.                                                                    |
| Làm giàu           | `enrichLiveEventOpsTier`, `enrichLiveEventFeedSource`, `enrichPublishE2ERef`, `CapDecisionLiveProcessTrace`, `EnrichLiveOutcomeMetadata` | Neo mốc với bảng G1–G6 (`e2e*`), giới hạn độ lớn cây `processTrace`, gắn **bình thường / bất thường** (`outcome*`) để lọc UI. |
| Nhánh live tắt     | `AI_DECISION_LIVE_ENABLED=0`                                                                                                             | Tiết kiệm RAM ring, WS và ghi Mongo; vẫn bump gauge CHI để vận hành biết **pha** đang chạy.                                   |
| Nhánh live bật     | Append ring → metrics → **hai** broadcast WS → persist async                                                                             | **Replay** REST/WS theo trace; **màn org** xem tổng hợp; **audit** lâu dài trên `decision_org_live_events`.                   |


**Hai kênh WebSocket sau một lần Publish**

1. **Theo trace** (`orgHex:traceId`) — người đang «mở một vụ» theo dõi end-to-end.
2. **Theo tổ chức** (`__org_feed__`) — màn live gộp mọi trace; bản ghi có thể được `globalOrgFeed` chỉnh thêm field dẫn xuất trước khi broadcast / persist (xem comment trong `publish.go`).

**Người đọc timeline nên hiểu theo ba lớp nội dung** (không trộn một cục)


| Lớp                                  | Field / chỗ hiển thị                                                                                                               | Câu hỏi trả lời                                                                                           |
| ------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| **1 — Tiếng người**                  | `summary`, `reasoningSummary`, bullets, `phaseLabelVi`, nhãn trên nút `processTrace`                                               | «Chuyện gì vừa xảy ra?» / «Kết quả ra sao?» — ưu tiên **tiếng Việt ngắn**, tránh nhồi tên hàm/collection. |
| **2 — Vị trí trong quy trình chuẩn** | `e2eStage`, `e2eStepId`, `e2eStepLabelVi` + dòng đầu `DetailBullets` **«Trong quy trình: Gx-Syy — …»** (sau `enrichPublishE2ERef`) | «Mốc này tương ứng **bước nào** trong bảng G1–G6?» — đối chiếu doc và `e2e_reference.go`.                 |
| **3 — Chứng cứ / audit**             | `refs`, `processTrace` (cây), `step` (`TraceStep`: `inputRef` / `reasoning` / `outputRef`), accordion **«Thông tin thêm»** (queue) | «Tra log/Mongo bằng gì?» / «Đầu vào–đầu ra rút gọn là gì?»                                                |


**Quy ước trùng lặp có chủ đích:** `summary` và tiêu đề một dòng (`step.title` → `uiTitle`) **không** mang tiền tố kiểu `G1 —` / `G2 —` vì vị trí Gx-Syy đã có ở lớp 2; `phase` / `phaseLabelVi` vẫn phục vụ **lọc** và badge. `ResolveE2EForLivePhase` map `phase` engine (orchestrate, execute_ready, queued, parse, propose, …) sang bước **G4** cụ thể khi refs chưa đủ.

**Nội dung Publish (timeline) — quy tắc kỹ thuật bổ sung:** `Publish` luôn gắn `e2e*`; nếu chưa có dòng tương tự thì **chèn một dòng đầu** vào `DetailBullets`: `**Trong quy trình: Gx-Syy — …`**. Tránh trùng khi dòng đầu đã chứa `E2E:`, `E2E` , `Trong quy trình:` hoặc `Tham chiếu E2E`. **Phân loại kết quả** (`outcome*`) — [§5.1](#51-phân-loại-kết-quả-outcome-và-cây-processtrace). **Khung mốc + liên kết** — [§4.5](#45-khung-mốc-timeline--liên-kết-trace--audit).

---

### 4.3. Phân loại kết quả (`outcome*`) và cây `processTrace`

**Mục đích:** Tách **bình thường** vs **bất thường / cần chú ý** trên từng mốc timeline; hỗ trợ lọc UI, cảnh báo vận hành và truy vấn Mongo.


| Thành phần            | Vai trò                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| --------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `**outcomeKind`**     | Mã ổn định (string) — lọc/thống kê. Định nghĩa hằng trong `decisionlive/outcome.go`.                                                                                                                                                                                                                                                                                                                                                                        |
| `**outcomeAbnormal`** | `true` nếu **không** thuộc nhóm bình thường (`nominal`, `success`).                                                                                                                                                                                                                                                                                                                                                                                         |
| `**outcomeLabelVi`**  | Nhãn ngắn tiếng Việt (chip); điền bởi `OutcomeLabelViForKind` khi trống.                                                                                                                                                                                                                                                                                                                                                                                    |
| `**processTrace`**    | Cây `DecisionLiveProcessNode` do worker consumer `decision_events_queue` dựng theo thời gian; chi tiết từng bước và mốc Publish — xem mục **«Quy trình dựng `processTrace` (consumer queue)»** ngay dưới. Giới hạn: `CapDecisionLiveProcessTrace` (`decisionlive/process_trace.go`). **I/O audit có cấu trúc (đầu vào / vì sao / đầu ra)** theo **Phương án B** — dùng `DecisionLiveEvent.step` (`TraceStep`), xem mục **«Phương án B»** sau phần consumer. |


**Luồng gán:** Builder (`livecopy/queue.go`, `engine_llm.go`, `execute.go`, `cix_orchestrate.go`, `ads.go`, …) có thể gán `outcomeKind` sẵn; `EnrichLiveOutcomeMetadata` (gọi từ `Publish` và `backfillLiveEventsDerivedFields`) suy luận dự phòng từ `phase` / `severity` / `sourceKind` nếu thiếu, rồi luôn gắn `outcomeAbnormal` và `outcomeLabelVi`.

**Bảng mã `outcomeKind` (tóm tắt)**


| `outcomeKind`               | Ý nghĩa                                                                                |
| --------------------------- | -------------------------------------------------------------------------------------- |
| `nominal`                   | Mốc trung gian bình thường (đang xử lý).                                               |
| `success`                   | Hoàn tất mong đợi (vd. queue xong job, engine done, propose thành công).               |
| `processing_error`          | Lỗi kỹ thuật / handler / queue error.                                                  |
| `policy_skipped`            | Bỏ qua theo quy tắc routing (noop có chủ đích).                                        |
| `unsupported`               | Chưa có handler cho loại sự kiện (trừ luồng mirror khách chỉ đồng bộ → vẫn `nominal`). |
| `data_incomplete`           | Thiếu dữ liệu đầu vào (vd. engine bỏ qua vì chưa có phân tích hội thoại).              |
| `no_actions`                | Sau phân tích không còn hành động đề xuất; hoặc cảnh báo tương đương (ads).            |
| `proposal_failed`           | Không tạo được đề xuất / việc cần làm.                                                 |
| `partial_failure`           | Một phần luồng lỗi (vd. orchestrate đơ — không xếp hàng intel).                        |
| `queue_skipped_unspecified` | Dự phòng suy luận: `PhaseSkipped` + nguồn queue nhưng thiếu `outcomeKind` từ builder.  |


#### Quy trình dựng `processTrace` (consumer queue — neo G2)

**Phạm vi:** Hiện tại code **chỉ** dựng cây đầy đủ trong **AI Decision consumer** (`worker.aidecision.consumer` → `processEvent` trong `worker.aidecision.consumer.go` → `dispatchConsumerEvent` trong `worker.aidecision.consumer_dispatch.go`). Các mốc timeline khác (engine, CIX, …) có thể để `processTrace` rỗng hoặc bổ sung sau; struct và `kind` nút nằm ở `decisionlive/types.go`.

**Hình dạng cây:** Hàm `wrapQueueConsumerRoot` bọc các bước trong **một** nút gốc (`kind=branch`, `key=queue_consumer`, nhãn tiếng Việt «Xử lý việc đã xếp hàng»). Các bước thực tế nằm trong `children`, được `queueProcessTracer` (`worker/queue_process_trace.go`) **append theo thứ tự thời gian**; mỗi lần Publish mốc queue, worker truyền `**snapshotTree()`** tại thời điểm đó — vì vậy **mốc sau có thể có nhiều nút hơn mốc trước** (tiến trình tích lũy), trừ mốc «bắt đầu» xem dòng dưới.

**1) Ngay sau lease, trước `processEvent`**

- `publishQueueConsumerLifecycleStart` (`worker/publish_queue_live.go`) gọi `queueTraceForProcessingStart(evt)`.
- **Mốc timeline:** `QueueMilestoneProcessingStart` (consumer — neo **G2-S01** trong `e2e_reference`).
- **Cây gửi đi:** chỉ **một** lá con `lease_acquired` (nhãn thân thiện: «Đã bắt đầu xử lý yêu cầu của bạn»); `detailVi` tiếng Việt: loại cập nhật (nhãn gọn) + mã tham chiếu / mã luồng khi cần hỗ trợ — **không** nhồi tên field kỹ thuật. **Không** dùng chung `queueProcessTracer` với các bước sau (cây này độc lập).

**2) Trong `processEvent` — khởi tạo tracer**

- `tr := newQueueProcessTracer(evt)` luôn đẩy bước đầu: `queue_envelope` (nhãn «Nhận: …» theo `QueueFriendlyEventLabel`); `detailVi` giải thích bằng tiếng Việt dễ hiểu (hàng chờ an toàn, xử lý lần lượt).

**3) Nhánh L1 datachanged (`l1_datachanged` hoặc `datachanged` bản ghi cũ — `IsL1DatachangedEventSource`)**

- Gọi `applyDatachangedSideEffects`, rồi `tr.noteDatachangedSideEffects()` — bước `datachanged_side_effects` (nhãn thân thiện: đồng bộ sau khi lưu; các bước con trong cây dùng tiếng Việt, tránh tên hàm/collection trên UI — chi tiết kỹ thuật nằm ở `TraceStep` / log / vận hành).
- `publishQueueDatachangedEffectsDone` → **mốc** `QueueMilestoneDatachangedDone`, `processTrace` = snapshot lúc này: thường là `**queue_envelope` → `datachanged_side_effects`**.

**4) Nhánh routing noop**

- Nếu `AIDecisionService.ShouldSkipDispatchForRoutingRule` → `tr.noteRoutingSkipped()` (`routing_noop`, kind `decision`).
- `publishQueueRoutingSkipped` → mốc `QueueMilestoneRoutingSkipped`.
- `processEvent` **return** `ConsumerCompletionKindRoutingSkipped` và `traceForEnd == nil`, nên `**publishQueueConsumerLifecycleEnd` không publish** `HandlerDone` / `HandlerError` (đã có mốc routing riêng).

**5) Nhánh dispatch (vào `dispatchConsumerEvent`)**

- `tr.noteRoutingAllowDispatch()` → bước `routing_allow`.
- `tr.noteDispatchLookup()` → `dispatch_lookup` (`detailVi` = `eventType` để tra registry).
- `consumerreg.Lookup(eventType)`:
  - **Không có handler:** `noteNoHandlerRegistered` → `no_handler` (kind `outcome`); `publishQueueNoRegisteredHandler` → `QueueMilestoneNoHandler`; handler return `NoHandler` → **LifecycleEnd bỏ qua HandlerDone** (giống routing).
  - **Có handler:** `noteHandlerInvoke` → `handler_run`; chạy `h(ctx, svc, evt)`; thành công → `noteHandlerSuccess` → `handler_ok` (outcome), lỗi → `noteHandlerError` → `handler_error` (kind `error`, `detailVi` cắt tối đa ~400 rune).

**6) Kết thúc job (đường chạy handler)**

- `publishQueueConsumerLifecycleEnd` nhận `tr.snapshotTree()` đầy đủ:
  - Lỗi handler → **mốc** `QueueMilestoneHandlerError` + cây có `handler_error`.
  - Thành công (và không thuộc `RoutingSkipped` / `NoHandler`) → **mốc** `QueueMilestoneHandlerDone` + cây có `handler_ok`.
- Thứ tự lá điển hình khi xử lý xong (có datachanged):  
`queue_envelope` → `datachanged_side_effects` → `routing_allow` → `dispatch_lookup` → `handler_run` → `handler_ok` | `handler_error`.

**Ngoại lệ — không có chuỗi mốc consumer:** `shouldSkipConsumerLiveSpan` (`publish_queue_live.go`) — với `eventType == aidecision.execute_requested` thì **không** publish ProcessingStart / Datachanged / Handler… (timeline execute do engine/livecopy khác).

`**kind` trên nút:** `branch`, `step`, `decision`, `outcome`, `skip`, `error` (`ProcessTraceKind*` trong `decisionlive/types.go`).

**Giới hạn khi Publish:** `CapDecisionLiveProcessTrace` — tối đa **64** nút tổng cộng, độ sâu **12**; vượt thì cắt theo duyệt cây (`decisionlive/process_trace.go`).

**Bảng `key` (consumer) — tra nhanh**


| `key`                          | Vai trò trong luồng                                  |
| ------------------------------ | ---------------------------------------------------- |
| `queue_consumer`               | Nút gốc nhánh (bọc toàn bộ bước con).                |
| `lease_acquired`               | Chỉ mốc ProcessingStart (trước `processEvent`).      |
| `queue_envelope`               | Bước đầu mọi snapshot sau khi vào `processEvent`.    |
| `datachanged_side_effects`     | Sau `applyDatachangedSideEffects` (chỉ datachanged). |
| `routing_noop`                 | Routing rule bắt noop — không gọi handler.           |
| `routing_allow`                | Cho phép đi tiếp tới dispatch.                       |
| `dispatch_lookup`              | Tra `consumerreg` theo `eventType`.                  |
| `no_handler`                   | Chưa đăng ký handler cho `eventType`.                |
| `handler_run`                  | Chuẩn bị gọi handler đã đăng ký.                     |
| `handler_ok` / `handler_error` | Kết quả chạy handler.                                |


**Code:** `decisionlive/outcome.go`, `decisionlive/types.go` (field + `ProcessTraceKind*`), `decisionlive/persist_org_audit.go` (BSON), `decisionlive/process_trace.go`, `worker/queue_process_trace.go`, `worker/publish_queue_live.go`, `worker.aidecision.consumer.go`, `worker.aidecision.consumer_dispatch.go`.

---

### 4.4. Phương án B — Lưu «quá trình thật» qua `TraceStep` (không mở rộng từng nút `processTrace`)

**Ý tưởng cốt lõi:** Coi `**processTrace`** là **khung điều hướng / thứ tự nhánh** (cây ngắn, `key` ổn định, `labelVi`/`detailVi` tóm tắt). Toàn bộ **đầu vào — xử lý — tại sao — đầu ra** có cấu trúc audit nằm ở `**DecisionLiveEvent.step` (`TraceStep`)** trên **từng mốc** timeline, tái sử dụng field đã có trong `decisionlive/types.go` (`Kind`, `Title`, `Reasoning`, `InputRef`, `OutputRef`, `Index`).

**Vì sao không nhồi chi tiết sâu vào `processTrace`:** Tránh trùng lặp hai nguồn sự thật; `TraceStep` đã thiết kế cho **một bước logic** với **reasoning** và **ref I/O**; persist org-live lưu **cả** `payload` JSON (gồm `step`) nên có thể phục hồi đủ cho audit; cột phẳng Mongo hiện chỉ tách `stepKind` / `stepTitle` — tra cứu sâu `inputRef`/`outputRef` là **đọc trong `payload`** (hoặc bổ sung index sau nếu cần).

**Khung sáu trường cho từng micro-bước:** đã nêu ở [§4.1](#41-khung-sáu-trường--một-bước-logic-product-trace-audit).

---

#### B.1 — Cấu trúc `TraceStep` trong code (tham chiếu)


| Field           | Kiểu (Go)                | Vai trò trong «quá trình thật»                                                                                                       |
| --------------- | ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------ |
| `**index`**     | `int`                    | Thứ tự bước **trong cùng một mốc** khi sau này có nhiều bước (xem B.5); mặc định `0` = bước chính của mốc.                           |
| `**kind`**      | `string`                 | Phân loại máy: `queue`                                                                                                               |
| `**title`**     | `string`                 | **Tên bước** hiển thị một dòng (đã dùng làm `uiTitle` qua persist).                                                                  |
| `**reasoning`** | `string`                 | **Tại sao** — giải thích nhánh, policy, rule id, điều kiện skip (tiếng Việt ngắn + mã kỹ thuật nếu cần).                             |
| `**inputRef`**  | `map[string]interface{}` | **Đầu vào** đã chuẩn hoá: **chỉ ref** (id, collection, `eventType`, `traceId`, …) — **không** nhét nội dung PII/raw document đầy đủ. |
| `**outputRef`** | `map[string]interface{}` | **Đầu ra** đã chuẩn hoá: job đã enqueue, `parentEventId`, tên worker miền, trạng thái, lỗi rút gọn.                                  |


**Giới hạn an toàn:** Mọi giá trị trong `inputRef`/`outputRef` nên **ngắn**, có **cắt độ dài** (tương tự cắt lỗi `processTrace`); tránh key tự do không tài liệu — nên có **bảng khóa** (vd. `sourceCollection`, `normalizedRecordUid`, `enqueue`, `jobType`).

---

#### B.2 — Ánh xạ trực tiếp: câu hỏi vận hành → field


| Câu hỏi                            | Ghi ở đâu                                                                                |
| ---------------------------------- | ---------------------------------------------------------------------------------------- |
| **Đầu vào là gì?** (từ đâu tới)    | `inputRef`: envelope queue, payload rút gọn, id bản ghi, org.                            |
| **Đang / đã xử lý thế nào?**       | `kind` + `title` (hành động chính); có thể thêm `inputRef["handler"]` = tên hàm đăng ký. |
| **Tại sao như vậy?** (nhánh, rule) | `reasoning`: routing noop, dedupe CRM, `ruleId` định tuyến collection, urgency.          |
| **Đầu ra là gì?**                  | `outputRef`: đã enqueue loại job nào, không enqueue vì lý do gì, `ok` / `errorCode`.     |


---

#### B.3 — Một mốc timeline = một «bước chính» + khung `processTrace`

- Mỗi lần `Publish` tạo **một** `DecisionLiveEvent` → **một** `TraceStep` đại diện **ý chính** của mốc đó (vd. mốc *DatachangedDone*: bước «đã chạy một cửa side-effect»).
- `**processTrace`** trên **cùng mốc** giữ **cây các micro-bước** (consumer đã có: envelope → datachanged → routing → dispatch → handler). Quan hệ: `**TraceStep` = tóm tắt có cấu trúc cho cả mốc**; `**processTrace` = chi tiết thứ tự / nhánh** (có thể rút gọn `detailVi` dài sang `reasoning`/`inputRef`/`outputRef` của `TraceStep` để tránh trùng).

**Ví dụ quy ước (mốc `HandlerDone` sau datachanged):**

- `step.kind`: `queue`
- `step.title`: «Đã xử lý xong job trên hàng đợi» (hoặc theo `DomainNarrative` hiện tại).
- `step.reasoning`: «Routing cho phép dispatch · handler `processCixAnalysisRequested` · không lỗi».
- `step.inputRef`: `{ "eventId", "eventType", "eventSource", "sourceCollection?", "normalizedRecordUid?" }`
- `step.outputRef`: `{ "action": "enqueue", "target": "cix_intel_compute", "note": "…" }` (chỉ khi handler thật sự enqueue — điền trong code handler hoặc wrapper).

---

#### B.4 — Chuỗi thời gian nhiều mốc (consumer queue)

Consumer publish **nhiều mốc** cho cùng job (`ProcessingStart` → có thể `DatachangedDone` → … → `HandlerDone`). Phương án B coi **mỗi mốc là một snapshot**:

- **ProcessingStart:** `TraceStep` mô tả **đầu vào lease** (`inputRef`: eventId, eventType, org); `outputRef` có thể rỗng hoặc `{ "phase": "leased" }`.
- **DatachangedDone:** `TraceStep` mô tả **kết quả một cửa datachanged**; `reasoning` = tóm policy + `ruleId`; `outputRef` = các hành động đã kích hoạt (merge defer, enqueue …) ở mức **tóm tắt** (không cần lặp toàn bộ cây con trong `processTrace` nếu đã chuyển sang ref).
- **HandlerDone / HandlerError:** `TraceStep` mô tả **handler**; lỗi đặt vào `reasoning` hoặc `outputRef.error` (chuỗi cắt).

**Gom trace theo người dùng:** UI/API gom theo `traceId` + sắp `seq`/`tsMs` — được **chuỗi `TraceStep`** theo thời gian, không cần một document ghép.

---

#### B.5 — Khi một mốc cần **nhiều** bước có cấu trúc (mở rộng sau)

**Hiện trạng struct:** `DecisionLiveEvent` chỉ có `**step` đơn** (`*TraceStep`), không có `steps []TraceStep`.

**Hướng mở rộng tương thích Phương án B (chọn một khi triển khai):**

1. **Thêm `steps []TraceStep`** (optional): mốc có thể mang **danh sách** bước; `step` giữ bước **đầu tiên** hoặc bước **đại diện** để client cũ không gãy.
2. **Giữ một `TraceStep`**: đặt **danh sách con** trong `outputRef["substeps"]` hoặc `inputRef["substeps"]` (mảng object nhỏ có `title`/`reasoning`/`ref`) — không đổi struct, nhưng cần **quy ước schema** trong doc + validator nhẹ khi Publish.
3. **Tách mốc timeline:** mỗi micro-bước thành một `Publish` riêng (thường **không** khuyến nghị cho consumer vì đã có `processTrace`).

Khuyến nghị thiết kế dài hạn: `**steps []TraceStep`** nếu audit cần query/filter theo từng bước; nếu chỉ hiển thị — `substeps` trong ref đủ.

---

#### B.6 — Đồng bộ với engine / CIX / Ads (đã có `Step` trong livecopy)

Các file `livecopy/engine_llm.go`, `cix_orchestrate.go`, `execute.go`, `ads.go` **đã gán** `TraceStep` (kind/title/reasoning, đôi khi input/output). Phương án B yêu cầu **cùng một quy ước** ref:

- `**inputRef`:** id case, trace, loại ngữ cảnh, **không** raw LLM prompt đầy đủ trong persist (nếu cần debug sâu → log nội bộ hoặc ledger riêng).
- `**outputRef`:** số action đề xuất, mode, mã lỗi business.

Consumer queue: `BuildQueueConsumerEvent` gọi `buildQueueConsumerTraceStep` (`livecopy/queue_trace_step.go`) — điền `**Kind`/`Title`**, `**reasoning`** (narrative + bổ sung kỹ thuật theo mốc), `**inputRef**` (envelope + subset payload datachanged / id thực thể), `**outputRef**` (`consumerPhase`, `resultVi`/`policyVi`, lỗi handler nếu có). Giới hạn: tối đa **28** khóa/ref, chuỗi cắt **320** rune; chỉ kiểu string/bool/int (bỏ map lồng).

---

#### B.7 — Checklist triển khai (Phương án B)

1. **Chốt danh sách khóa** `inputRef` / `outputRef` cho luồng queue + datachanged + enqueue intel (tài liệu + comment Go).
2. **Điền `TraceStep` đầy đủ** trên các mốc consumer (ít nhất `HandlerDone`, `DatachangedDone`, `HandlerError`, `RoutingSkipped`, `NoHandler`).
3. Với **mỗi nút con** trong `processTrace`, tự kiểm **khung sáu trường** (`label`, `purpose`, `inputSummary`, `**logicSummary`**, `resultSummary`, `nextStepHint`) — ít nhất **trong đầu người viết copy**; khi gộp vào `detailVi` vẫn giữ **thứ tự ý**: mục đích → đầu vào → **logic / vì sao** → kết quả → bước sau.
4. **Giảm trùng lặp** giữa `processTrace.detailVi` dài và `step.reasoning` — ưu tiên một nơi là «nguồn sự thật» cho «tại sao» (thường `**logicSummary` → `reasoning` / phần giữa `detailVi`**).
5. **Cap + redact** trước `Publish` (middleware chung hoặc helper `SanitizeTraceStepRefs`).
6. (Tuỳ chọn) **Version schema** trong `inputRef["traceStepSchema"] = "1"` để client/ETL phân biệt.

---

### 4.5. Khung mốc timeline — liên kết trace & audit

**Mục tiêu:** Một mốc `DecisionLiveEvent` (và một dòng `decision_org_live_events`) phải đồng thời (1) **hiển thị được** cho người dùng, (2) **nối được** với mốc trước/sau và với hệ thống phía sau (queue, case, entity), (3) **audit được** sau thời gian dài mà không cần đoán. Khung dưới đây là **một bảng tra** cho product, frontend, backend và vận hành — trùng với ba lớp «tiếng người / E2E / chứng cứ» ở mục Publish nhưng **mở rộng** phần liên kết và thứ tự chuỗi mốc.

---

#### Sáu khối trên mỗi mốc


| Khối                               | Nhóm field (điển hình)                                                                                                                                                                                                                                                                                                                       | Câu hỏi phải trả lời được                                                                                                                                                            |
| ---------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **1 — Định danh luồng & thứ tự**   | `ownerOrganizationId` (persist), `traceId`, `stream`, `tsMs`, `seq`, `feedSeq`, `schemaVersion` / `docSchemaVersion`                                                                                                                                                                                                                         | Mốc này thuộc **org** nào, **luồng** nào, đứng **thứ mấy** trên timeline?                                                                                                            |
| **2 — Neo quy trình chuẩn (E2E)**  | `e2eStage`, `e2eStepId`, `e2eStepLabelVi` (top-level + trong `refs`); `phase`, `phaseLabelVi`; dòng đầu `DetailBullets` **«Trong quy trình: Gx-Syy — …»**                                                                                                                                                                                    | Mốc này khớp **bước nào** trong bảng G1–G6 / `e2e_reference.go`?                                                                                                                     |
| **3 — Narrative người đọc**        | Trên JSON timeline (`DecisionLiveEvent`): `phaseLabelVi`, `uiTitle`, `uiSummary` (enrich Publish + backfill — **mỗi node swimlane** có tiêu đề + tóm tắt ngắn trước khi mở chi tiết); thêm `stepTitle` (từ `step`), `summary`, `reasoningSummary`, `DetailBullets`, `DetailSections`; `sourceKind`, `sourceTitle`, `feedSource*`, `opsTier*` | **Chuyện gì** xảy ra, **vì sao**, **tiếp theo là gì** (tiếng Việt ngắn)?                                                                                                             |
| **4 — Liên kết trace (đồ thị id)** | Bảng riêng ngay dưới — `**refs` + span W3C** là lõi                                                                                                                                                                                                                                                                                          | Làm sao **nối** mốc này với job queue, case, entity và **mốc khác**?                                                                                                                 |
| **5 — Audit có cấu trúc**          | `step` (`TraceStep`: `inputRef`, `reasoning`, `outputRef`); `processTrace` (cây); trên persist thêm `**payload`** JSON/BSON đầy đủ                                                                                                                                                                                                           | **Đầu vào / lý do / đầu ra** rút gọn (ref, không PII thô)? **Thứ tự micro-bước** (consumer)? **Từng bước con** bám [§4.1](#41-khung-sáu-trường--một-bước-logic-product-trace-audit). |
| **6 — Phân loại kết quả**          | `outcomeKind`, `outcomeAbnormal`, `outcomeLabelVi`, `severity`                                                                                                                                                                                                                                                                               | Mốc này **bình thường** hay **cần chú ý**; lọc dashboard / cảnh báo?                                                                                                                 |


**Quy ước tránh trùng:** `summary` / `uiTitle` **không** gắn tiền tố `Gx —`; vị trí Gx-Syy để ở khối 2 (`e2e*` + dòng đầu `DetailBullets`).

---

#### Bảng liên kết — user thấy sự nối giữa các event (trace & audit)

Mỗi mốc timeline là **một nút**; các khóa dưới đây là **cạnh** nối tới nút khác hoặc tới collection ngoài timeline.


| Khóa / nhóm                                                      | Vai trò liên kết                                                                                            | Cách dùng khi trace hoặc audit                                                                                                             |
| ---------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `**traceId`**                                                    | Trục chính của **một vụ** end-to-end (cùng org)                                                             | Lọc mọi mốc: `decision_org_live_events` hoặc API Timeline theo `(org, traceId)`; sắp `seq` / `tsMs`.                                       |
| `**correlationId`**                                              | Gom **một phiên xử lý** / request tới nhiều service (log, vendor)                                           | Grep log; đối chiếu khi `traceId` dài hoặc tách nhánh — **không** thay thế `traceId` trên timeline.                                        |
| `**w3cTraceId`**                                                 | Chuỗi trace chuẩn quan sát được (OTel-style)                                                                | Đồng bộ với APM/log tập trung; có thể **rộng hơn** một tiến trình app.                                                                     |
| `**spanId` / `parentSpanId`**                                    | **Nối thứ tự publish** giữa các mốc: mốc sau có `parentSpanId` = `spanId` mốc trước (khi pipeline gắn đúng) | Dựng chuỗi span trên timeline; chứng minh **thứ tự** khi `seq` cần đối chiếu.                                                              |
| `**refs.eventId`**                                               | **Cầu nối** tới bản ghi job trong `**decision_events_queue`** (consumer một cửa)                            | Một job → thường **nhiều** mốc live (ProcessingStart, DatachangedDone, HandlerDone…); cùng `eventId` = cùng job, khác `seq` / `e2eStepId`. |
| `**refs.eventType`**, `**eventSource`**, `**pipelineStage**`     | Mô tả **loại công việc** của envelope queue (G1 nghiệp vụ)                                                  | Đối chiếu bảng chi tiết Gx-Syy-Ezz; map máy trong `e2e_reference.go`.                                                                      |
| `**decisionCaseId`** (khi có)                                    | Nối `**decision_cases_runtime`** và các mốc engine / execute                                                | Cùng case có thể có **nhiều** `traceId` theo thời gian — ưu tiên case khi audit **quyết định**.                                            |
| `**entityId` / `entityType`**, `conversationId`, `customerId`, … | Neo **bản ghi nghiệp vụ** (mirror/canonical tuỳ miền)                                                       | Query ngược collection nguồn; giải thích «trên thực thể X đã chạy những mốc nào».                                                          |
| `**step.outputRef` / `inputRef`** (Phương án B)                  | Handoff: `enqueue`, `jobType`, `parentEventId`, id job con (khi builder điền)                               | Audit **từ mốc cha sang job con** mà không cần mở full payload.                                                                            |


**Hai mẫu đọc chuỗi (tư duy vận hành)**

1. **Cùng một job queue, nhiều mốc live:** `traceId` giữ nguyên, `refs.eventId` **giữ nguyên**, `seq` tăng dần, `e2eStepId` có thể **G2-S01** rồi nhiều lần **G2-S02** (cùng bước catalog, nhãn `e2eStepLabelVi` khác nhau từng mốc). Đây là chỗ user thấy **một việc** qua **nhiều mốc timeline**.
2. **Cùng một trace, nhiều job:** `traceId` giữ nguyên, `refs.eventId` **đổi** giữa các cụm mốc — job sau thường được enqueue từ handler / side-effect của job trước; tra `**outputRef`** / log enqueue để nối.

**Cầu nối queue ↔ org-live (audit ngược)**

1. Từ mốc consumer: lấy `refs.eventId`.
2. Tra `decision_events_queue` theo id đó → payload envelope, trạng thái lease, lỗi, retry.
3. Ngược lại: từ bản ghi queue có `traceId` → kéo toàn bộ mốc live cùng trace để xem **đã Publish những gì** trong lúc xử lý job.

---

#### Quy ước tối thiểu (copy + ref)

- **Summary:** một dòng hành động/kết quả, không tiền tố Gx.
- **ReasoningSummary:** một câu «vì sao có mốc này» / tác động bước sau.
- **Refs:** tối thiểu `**traceId`** và **một** neo nghiệp vụ: `eventId` (consumer) **hoặc** `decisionCaseId` (engine/case) **hoặc** cặp `entityId`+`entityType` / `conversationId` (ingress).
- **DetailBullets:** 1–3 dòng chính; tối đa ~5–6; dòng đầu sau Publish: **«Trong quy trình: Gx-Syy — …»** (`prependE2EPublishNarrative` trong `publish.go`); không lặp ý với `summary` / `reasoningSummary`.
- **DetailSections:** một accordion **«Thông tin thêm»** (queue) hoặc tương đương; không nhân đôi section cùng ý.
- **Lọc:** `outcomeAbnormal`, `outcomeKind`, kết hợp `severity` / `phase` khi cần.

---

#### Ma trận nhanh: nguồn mốc → neo trace điển hình


| Nguồn mốc                                            | Khối 2 (E2E)                   | Liên kết chính (khối 4)                                   | Ý nghiệp vụ (khối 3)                      |
| ---------------------------------------------------- | ------------------------------ | --------------------------------------------------------- | ----------------------------------------- |
| Queue consumer (`AID`)                               | G2-S01, G2-S02 (lặp mốc)       | `eventId` + `decision_events_queue`; `traceId`            | lease / side-effect / dispatch / handler  |
| Điều phối case + CIX + execute (`AID` + miền `cix`)  | G4–G6 (theo `phase` / refs)    | `decisionCaseId` + `decision_cases_runtime`; `traceId`    | case runtime, pipeline CIX, gate thực thi |
| Engine case / propose (`AID`)                        | G4 (theo `phase` → map `e2e*`) | `traceId` + `decisionCaseId`                              | rule / LLM / propose                      |
| Intel Meta/Ads & handoff miền (`INT` + `meta`/`ads`) | G3–G4                          | `traceId`; ref campaign/account trong `refs` / `inputRef` | intel miền / đề xuất                      |


---

#### Checklist trước khi coi mốc «đủ chuẩn trace/audit»

1. Có `**traceId`** (và org) — không có thì không có timeline hợp lệ.
2. Có **ít nhất một** neo tra cứu: `eventId` **hoặc** `decisionCaseId` **hoặc** `entityId`+`entityType` / `conversationId` (tuỳ loại mốc).
3. Có `**e2eStage` / `e2eStepId`** (hoặc map được từ `phase` qua `ResolveE2EForLivePhase`) — đối chiếu doc và `e2e_reference.go`.
4. Handoff sang job/worker khác: nên có dấu vết trong `**step.outputRef`** hoặc `refs` (vd. loại job đã enqueue) — tránh «mất cầu» giữa hai `eventId`.
5. **Không nhầm** `decisionCaseId` với `traceId` (ghi chú vận hành: một case có thể sống qua nhiều trace theo thời gian).
6. Copy người đọc: **tiếng Việt**; tên kỹ thuật giữ theo code trong `refs` / ref map.

**Code tham chiếu:** `decisionlive/publish.go` (`enrichPublishE2ERef`, `EnrichLiveOutcomeMetadata`), `decisionlive/outcome.go`, `decisionlive/process_trace.go`, `worker/queue_process_trace.go`, `worker/publish_queue_live.go`, `worker.aidecision.consumer.go` (`processEvent`), `worker.aidecision.consumer_dispatch.go` (`dispatchConsumerEvent`), `decisionlive/livecopy/queue.go` (`queueDetailSections`), `execute.go`, `ads.go`, `cix_orchestrate.go`, `livecopy/engine_llm.go`.

---

## 5. `resultSummary` — Đầu ra và tra cứu

`**resultSummary`:** **Kết quả** ở dạng catalog — **sáu pha chính** (trục vụ) neo **G1–G6** (máy), từng dòng `eventType` §5.3, và mức độ khớp code. **Thứ tự đọc theo trục vụ:** [§5.2](#52-bảng-giai-đoạn-lớn-đọc-trước) (bảng sáu pha → bảng G1–G6 → [gom bước](#gom-buoc-truc-vu)) → [§5.3](#bang-catalog-chi-tiet-e2e) (chi tiết + API). (Phân loại `outcome*` và hình dạng cây consumer nằm ở [§4.3](#43-phân-loại-kết-quả-outcome-và-cây-processtrace).) **Bản máy đọc (JSON)** cho UI: [§3.1](#31-api-catalog-e2e-json-cho-frontend) — GET `e2e-reference-catalog` (`stages` = sáu dòng G1–G6; `steps` khớp §5.3).

### 5.1. Mức độ khớp code (rà soát thực tế)


| Nội dung                                                                                                                                 | Đánh giá                                                                                                                                                                                                                                                  |
| ---------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Thứ tự G1→G6** (máy) và **sáu pha chính** (trục vụ: ghi thô = G1, merge = G2, intel = G3, ra quyết định = G4, thực thi = G5, học = G6) | **Khớp** kiến trúc trong `NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md` và `KHUNG_LUONG…`.                                                                                                                                                            |
| **Chuỗi `eventType`** trong bảng chi tiết                                                                                                | **Khớp** `eventtypes/names.go` (và các luồng handoff đã mô tả trong doc).                                                                                                                                                                                 |
| `**eventSource` + `pipelineStage` trên từng dòng**                                                                                       | **Phải tra đúng điểm emit** (`eventemit.EmitDecisionEvent`, `EmitEvent`, `EmitCixAnalysisRequested`, …). Bảng dưới đã **điều chỉnh** các dòng đã đối chiếu trực tiếp với file emit; chỗ còn ghi **“tuỳ đường emit”** là nơi có **nhiều đường** vào queue. |
| **Mốc consumer** (cùng **G1** / pha ghi thô với ingress+enqueue)                                                                         | **Không** phát sự kiện queue mới cho bước “apply side-effect” — cột `pipelineStage` ở các bước chỉ xử lý nội bộ để **—** (giá trị nằm trên **bản ghi đang xử lý**, từ lúc enqueue ở `G1-S04`).                                                            |


**Đối chiếu nhanh đã làm:** `crmqueue/crmqueue.go` (`crm.intelligence.compute_requested` → `after_l1_change`; `crm.intelligence.recompute_requested` sau merge → `crm_merge_queue` + `after_l2_merge`), `service.aidecision.emit_ads_recompute.go`, `service.aidecision.emit_cix.go`, `intelrecomputed/emit.go`, `hooks/datachanged.go`.

### 5.2. Bảng giai đoạn lớn (đọc trước)

**Mã lưu đồ (swimlane):** xem [§1.1](#11-lớp-kiến-trúc-swimlane--mã-cho-lưu-đồ-dòng-chảy). Cột «Nhóm trách nhiệm» ở bảng G1–G6 dưới khớp cột cùng tên ở [§5.3](#bang-catalog-chi-tiet-e2e).

#### Sáu pha chính (trục vụ — đọc trước)

**Đọc nhanh một vụ:** chỉ cần nhớ **sáu pha**; cột «Neo máy» trỏ tới **G** trong resolver / `e2eStage` / bảng dưới. Hai cột **Module giao việc** / **Module thực hiện** ghi **điển hình cấp pha** (xem ghi chú dưới bảng G1–G6).


| Pha chính             | Neo máy (G) | Module giao việc (nếu có) | Module thực hiện | Nội dung gọn                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| --------------------- | ----------- | ------------------------- | ---------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Pha ghi thô**       | **G1**      | `**aidecision**` (enqueue `decision_events_queue` sau `EmitDataChanged` + lọc `hooks/datachanged`) | **Ingress (nhóm B)** (ghi L1); `**aidecision**` (ghi job bus) | **CIO** nhận từ ngoài → xử lý & ghi L1 → `**EmitDataChanged`** → enqueue `decision_events_queue` (`eventSource` `**l1_datachanged`**; bản ghi cũ có thể còn `datachanged`). Chi tiết **G1-S01…S04** ở [§5.3](#g1-cio-l1-datachanged). **CIO không có bước debounce** trong catalog; gom tin nhắn (`message.batch_ready`) **không** còn bước catalog G4 — envelope neo **G2-S02** khi consumer xử lý job (xem **v34**). |
| **Pha merge**         | **G2**      | `**aidecision**` (consumer — xếp job merge, dispatch); **miền** (emit sau L2 — minh hoạ `**crm**`) | `**aidecision**` (`processEvent` G2-S01–S02); **worker miền** (merge L1→L2, vd. `**crm**`) | **Hai tầng:** (1) Một job `decision_events_queue` — lease → side-effect L1 (nếu có) → dispatch. (2) Worker miền — gộp L1→L2 canonical → có thể `crm.intelligence.recompute_requested`. Chi tiết [§5.3 — G2](#g2-consumer-va-merge-l2). |
| **Pha intel**         | **G3**      | `**aidecision**` (xếp `*_intel_compute`); nhánh **datachanged / enqueue trực tiếp từ miền** ([§1.4](#14-meta-trên-document-job-hàng-đợi-miền-đồng-bộ-bus--chuẩn-gộp-một-nguồn)) | `**aidecision**` (gom/gấp trên bus); `**crm**` / `**orderintel**` / `**conversationintel**`–`**cix**` / `**meta**` (worker `*_intel_compute`); miền emit `*_intel_recomputed` | **Ba bước:** (1) AID nhận job `decision_events_queue` — điển hình `**l2_datachanged`** sau G2-S05; (2) AID **gom/gấp** → xếp job `***_intel_compute`** từng miền; (3) miền xong → `***_intel_recomputed`** lại queue, AID xử lý tiếp (G4+). Chi tiết [§5.3 — G3](#g3-pha-intel-sau-l2-datachanged--gom-gấp--handoff-về-aid). |
| **Pha ra quyết định** | **G4**      | `**aidecision**` (`*.context_requested`); **miền** (`*.context_ready`) | `**aidecision**` (case, engine, orchestrate); **miền** (bổ sung ngữ cảnh); nhánh pipeline **CIX** trong code | **Bốn bước** (nhánh điển hình sau handoff intel): **(1)** **G4-S01** — AID xử lý `**<domain>_intel_recomputed`**; **(2)** rà ngữ cảnh, thiếu → `***.context_requested`** (**G4-S02**); **(3)** miền trả `***.context_ready`** (cùng **G4-S02**); **(4)** **G4-S03** — policy + engine + `**aidecision.execute_requested`** / `**executor.propose_requested`** (chi tiết catalog **G4-S03-E01…E03**). Gom tin `**message.batch_ready**` neo **G2-S02**. Chi tiết [§5.3 — G4](#g4-pha-ra-quyet-dinh-case-context-executor). Pipeline CIX / `phase` live: [§3](#3-inputsummary--đầu-vào-và-tham-chiếu-code). |
| **Pha thực thi**      | **G5**      | `**aidecision**` (lệnh execute / propose tới executor) | `**executor**` (+ `**delivery**` khi có) | Executor: duyệt, dispatch adapter, chạy action. |
| **Pha học**           | **G6**      | **—** (không có một module giao việc tập trung như G3; chuỗi sau outcome / feedback) | `**learning**`, `**ruleintel**` (đánh giá, gợi ý policy) | Outcome → `learning_cases` → đánh giá → gợi ý / feedback rule-policy (gồm nhánh `OUT` / `LRN` / `FBK` trên swimlane). |


#### Bảng G1–G6 (đồng bộ code, resolver, API `stages`)

**Sáu** dòng dưới đây khớp `**eventtypes/e2e_catalog.go`** (`E2EStageCatalog`) và trường `**stages`** của GET `e2e-reference-catalog`. **v7:** G1 chỉ CIO **S01–S04**; consumer queue + merge L2 gom **G2** (`E2ECatalogSchemaVersion` **7**).


| Giai đoạn | Mã swimlane | Tên giai đoạn | Module giao việc (nếu có) | Module thực hiện | Mô tả kỹ thuật (`summaryVi`) | Mô tả người dùng (`userSummaryVi`) | Pha chính (trục vụ) |
| --------- | --------------------- | --------------------------------------------------- | ---------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------- |
| **G1** | `ING`, `DOM` | Pha ghi thô: CIO → L1 → datachanged → enqueue | `**aidecision**` (enqueue bus sau lọc) | **Ingress (nhóm B)** (ghi L1); `**aidecision**` (job queue) | CIO/sync → L1 → `EmitDataChanged` → `decision_events_queue` (`eventSource` `l1_datachanged`) | Dữ liệu từ cửa hàng và kênh chat được thu về, lưu nhất quán; hệ thống ghi nhận thay đổi và chuẩn bị đưa vào hàng đợi xử lý của trợ lý. | **Pha ghi thô** |
| **G2** | `AID`, `DOM` | Pha merge: consumer queue + hợp nhất canonical (L2) | `**aidecision**` (consumer, xếp merge); **miền** (emit sau L2) | `**aidecision**`; **worker miền** (merge L2) | Trục vụ sau đổi L1: ưu tiên merge canonical (L2) với gom (debounce/cửa sổ); gấp chỉ bỏ hoặc rút ngắn gom, vẫn đủ luồng consumer. (1) `decision_events_queue` — G2-S01 lease (điển hình job `l1_datachanged` sau G1-S04) → G2-S02 `processEvent`: side-effect có thể xếp job merge cho miền (minh hoạ CRM: `crm_pending_merge` — AID không tự merge L2 trên consumer), rồi routing / `dispatchConsumerEvent`. (2) Worker miền G2-S03–S04 merge L1→L2; **G2-S05-E01** enqueue lại `decision_events_queue` — **cùng ý «báo thay đổi dữ liệu để AID xử lý tiếp» như G1-S04** nhưng sau **L2** (wire: `l2_datachanged`, `<prefix>.changed` từ collection nguồn, `after_l2_merge`; fallback `crm.intelligence.recompute_requested` nếu không map collection; bản ghi cũ có thể `crm_merge_queue` + `crm.intelligence.recompute_requested`). | Thay đổi nguồn được xếp hàng; trợ lý chuyển yêu cầu gộp sang worker miền; khi hồ sơ chung (L2) cập nhật, miền **báo lại** lên hàng đợi AID **giống cách báo đổi L1** ở G1-S04 — để các bước sau chạy tiếp. Có thể gom hoặc ưu tiên trước khi cấp bách. | **Pha merge** |
| **G3** | `INT` | Pha intel miền và bàn giao về AID | `**aidecision**` (xếp `*_intel_compute`); nhánh **miền** ([§1.4](#14-meta-trên-document-job-hàng-đợi-miền-đồng-bộ-bus--chuẩn-gộp-một-nguồn)) | `**aidecision**`; `**crm**` / `**orderintel**` / `**conversationintel**`–`**cix**` / `**meta**` | **(1)** Consumer AID **nhận** job trên `**decision_events_queue`** — điển hình sau merge L2: `**l2_datachanged`**, `**after_l2_merge`**, `**<prefix>.changed**` (G2-S05-E01; minh hoạ CRM `EmitAfterL2MergeForCrmIntel`); ngoài ra recompute/intel yêu cầu khác (prefix miền, bảng G3-S01). **(2)** AID gom (debounce/cửa sổ) và gấp (ưu tiên → xếp sớm) rồi tạo job `***_intel_compute`** ở **từng miền** liên quan. **(3)** Worker miền chạy xong → miền **phát** `**_intel_recomputed`** lên `**decision_events_queue`** (catalog G3-S06); AID nhận bản ghi đó ở **pha G4 — G4-S01** (`ResolveE2EForQueueEnvelope`). | **(1)** Trợ lý nhận tín hiệu trên hàng đợi — thường sau khi L2 cập nhật (`l2_datachanged`). **(2)** Trợ lý xếp việc tính lại intelligence cho từng miền, có **gom/gấp**. **(3)** Miền chạy xong thì **bắn kết quả phân tích** lên hàng đợi; trợ lý **mở/cập nhật vụ việc** (G4) để tiếp tục. | **Pha intel** |
| **G4** | `AID` | Pha ra quyết định: case, ngữ cảnh, điều phối | `**aidecision**` (`*.context_requested`); **miền** (`*.context_ready`) | `**aidecision**` (case, engine); **miền** (ngữ cảnh) | `**decision_cases_runtime`** — **(1) G4-S01:** AID bắt đầu xử lý job `**eventType` `<domain>_intel_recomputed`** trên `decision_events_queue` (sau G3-S06), ResolveOrCreate / cập nhật case. **(2)** AID rà ngữ cảnh; thiếu → phát `***.context_requested`**. **(3)** Miền trả `***.context_ready`** — cùng catalog **G4-S02**; có thể lặp (2)↔(3). **(4) G4-S03** — đủ ngữ cảnh + policy + engine (live `phase` → `ResolveE2EForLivePhase` neo **G4-S03**; `done`/`error` cùng bước) + phát execute/propose (**catalog G4-S03-E01…E03**). Gom `**message.batch_ready**` — envelope neo **G2-S02**. | **(1)** Nhận báo intel đã tính lại và cập nhật vụ. **(2)** Thiếu ngữ cảnh thì nhờ miền bổ sung. **(3)** Miền báo đã gửi ngữ cảnh. **(4)** Suy luận, chọn hành động và tạo lệnh thực thi hoặc đề xuất cho Executor. Gom tin nhắn (`message.batch_ready`) neo consumer **G2-S02**. | **Pha ra quyết định** |
| **G5** | `EXC` | Pha thực thi (Executor) | `**aidecision**` (execute / propose) | `**executor**` (+ `**delivery**`) | Duyệt, dispatch adapter, chạy action | Lệnh được duyệt (tự động hoặc có người) và gửi tới hệ thống thực thi để hoàn tất việc cần làm. | **Pha thực thi** |
| **G6** | `OUT` / `LRN` / `FBK` | Pha học: outcome, learning_cases, feedback | **—** | `**learning**`, `**ruleintel**` | Outcome → `learning_cases` → evaluation → gợi ý rule/policy | Kết quả thực tế được ghi nhận; hệ thống học từ phản hồi để gợi ý cải tiến quy tắc và trải nghiệm sau này. | **Pha học** |



**Ghi chú hai cột module (§5.2):** **Module giao việc** = lớp phát lệnh enqueue / handoff bước kế (tinh thần `enqueueSourceDomain` trên document job miền — [§1.4](#14-meta-trên-document-job-hàng-đợi-miền-đồng-bộ-bus--chuẩn-gộp-một-nguồn)). **Module thực hiện** = module/worker chạy bước chủ đạo (tinh thần `processorDomain` / `businessDomain` trên timeline — [§1.2](#12-miền-nghiệp-vụ-bounded-context--tên-gọi-thống-nhất)). Một pha có **nhiều** vai trò — bảng ghi **điển hình**; chi tiết từng bước xem §5.3. JSON `e2e-reference-catalog` (trường `stages`) **chưa** có hai cột này cho tới khi mở rộng schema catalog.

**Quy ước ID bước:** `Gx-Syy` (giai đoạn Gx, bước thứ yy). Sự kiện chi tiết: `Gx-Syy-Ezz` khi cần.

**API:** Cùng nội dung bảng G1–G6 (trường `stages`) — xem [§3.1](#31-api-catalog-e2e-json-cho-frontend).

**Gợi ý gom bước theo trục vụ:** Ưu tiên **sáu pha chính** ở đầu §5.2; **G1–G6** và §5.3 là **lớp máy** (`eventType`, resolver, `e2eStepId`).

- **Pha ghi thô (G1):** **G1-S01…G1-S03** (CIO nhận ngoài → xử lý & ghi L1 → `EmitDataChanged` + handler AID **lọc collection** trước queue); **G1-S04** — enqueue job báo **đổi L1** (`l1_datachanged`, `after_l1_change`; wire `*.changed`). Trên **trục vụ**, bước tiếp theo được kỳ vọng là **pha merge (G2)** — gộp L2, không phải “tính lại nặng” ngay. Debounce tin nhắn / recompute phía AID (`ai_decision_debounce`, `decision_recompute_debounce_queue` trong `global.vars`) — **không** gắn bước CIO trong bảng G1.
- **Pha merge (G2):** **Trục vụ:** sau đổi L1 → **merge canonical (L2)**; **gom** / **gấp** như trên §1. **G2-S01–S02** = một job `decision_events_queue` (`applyDatachangedSideEffects` → routing → `dispatchConsumerEvent` …). **Vòng bắn tay AID ↔ miền (minh hoạ CRM):** consumer AID trong **G2-S02** có thể **ghi `crm_pending_merge`** (yêu cầu merge — `crm/datachanged`, `datachangedsidefx`); **worker miền** (`CrmPendingMergeWorker`, **G2-S03–S04**) merge L1→L2; sau merge `**crm/datachanged/notify_after_crm_merge`** gọi `**aidecision/crmqueue.EmitAfterL2MergeForCrmIntel`** → lại `**decision_events_queue`** (**G2-S05-E01**): wire `**l2_datachanged`** + `**<prefix>.changed`** + `**after_l2_merge`** (đối chiếu G1-S04: `l1_datachanged` + `after_l1_change`); consumer AID nhánh `**IsPostL2MergeCrmIntelEnvelope`** → debounce/xếp `crm_intel_compute`, **không** `consumerreg.Lookup` theo `.changed` như L1. **G2-S03–S05** = worker miền + enqueue về AID; không chạy trong goroutine **G2-S01–S02**. Đọc trước: [§5.3 — G2](#g2-consumer-va-merge-l2).
- **Pha intel (G3) — ba bước trục vụ:** **(1)** **AID nhận** job trên `**decision_events_queue`** — điển hình `**l2_datachanged`** sau **G2-S05** (catalog **G3-S01**; còn recompute/L1 hoặc yêu cầu intel miền khác trong §5.3). **(2)** **AID tính toán và xếp job** tính lại intelligence ở **từng miền** (`*_intel_compute`) — catalog **G3-S02**; **gom** (debounce/cửa sổ, trailing…) và **gấp** (ưu tiên / policy → enqueue sớm). **(3)** **Worker miền** (G3-S03…S05) rồi **miền phát** (`*_intel_recomputed`, catalog **G3-S06**) **lên `decision_events_queue`** — **AID nhận** bản ghi đó ở **pha G4** (**G4-S01**, `ResolveE2EForQueueEnvelope`). §5.3 tách G3/G4 để tra máy; nối job: `**step.outputRef`**, `traceId`, `causalOrderingAtMs`. Đọc trước: [§5.3 — G3](#g3-pha-intel-sau-l2-datachanged--gom-gấp--handoff-về-aid).
- **Pha ra quyết định (G4) — bốn bước trục vụ** (điển hình sau `**<domain>_intel_recomputed`**; luồng case khác có thể rút gọn): **(1)** **G4-S01** — AID nhận / xử lý job trên `decision_events_queue`, ResolveOrCreate `**decision_cases_runtime`**. **(2)** Rà ngữ cảnh; thiếu → `***.context_requested`** (**G4-S02**). **(3)** Miền trả `***.context_ready`** (cùng **G4-S02**). **(4)** **G4-S03** — đủ context + policy + engine + phát execute/propose (**catalog G4-S03-E01…E03**; live `phase` `queued`/`propose`/`done`/`error` → resolver). `**message.batch_ready**` neo **G2-S02**. Chi tiết: [§5.3 — G4](#g4-pha-ra-quyet-dinh-case-context-executor).
- **Pha thực thi (G5) / pha học (G6):** §5.3 chi tiết từng bước catalog.

Không mọi vụ đi đủ sáu pha (ví dụ không merge / không intel); **pha merge → intel → ra quyết định** có thể lặp theo thời gian.

### 5.3. Bảng chi tiết (catalog đồng bộ code & API)

**Vai trò:** Liệt kê bước `Gx-Syy` (và `eventType` khi có) để tra resolver và GET `e2e-reference-catalog`; **G1-S04** gom một dòng — tra **emit** / `source_sync_registry` / `hooks/datachanged.go`. **Lớp máy + tra cứu sâu**; đọc nhanh trục vụ: [§5.2 + gom bước](#gom-buoc-truc-vu).

**Quy ước cột «Nhóm trách nhiệm»:** trùng tên với [§1.1](#11-lớp-kiến-trúc-swimlane--mã-cho-lưu-đồ-dòng-chảy) (swimlane). Khi vẽ lưu đồ theo **miền nghiệp vụ** (CRM, CIX, …), tra [§1.2](#12-miền-nghiệp-vụ-bounded-context--tên-gọi-thống-nhất).

**Hai cột mô tả (v10):** `**descriptionTechnicalVi`** (tra code, audit) và `**descriptionUserVi`** (UI/onboarding); bảng dưới khớp từng chữ với `E2EStepCatalog()` trong `eventtypes/e2e_catalog.go`. **G1-S04** trong catalog/API là **một dòng** (mô tả chung enqueue **báo thay đổi L1**): **wire** `eventType` = `**<prefix>.changed`** — `prefix` map từ **tên collection** (`source_sync_registry` / `eventtypes/source_collection_wire.go`); thao tác insert/update nằm ở payload `dataChangeOperation`. `**consumerreg.Lookup`** tương thích bản ghi cũ `*.inserted` / `*.updated`. `**l1_datachanged` là `eventSource`**. G2-S05-E01 — báo sau L2: `l2_datachanged`, `<prefix>.changed`, `after_l2_merge`; resolver/API vẫn neo G2-S05-E01 với bản ghi cũ `crm_merge_queue` + `crm.intelligence.recompute_requested`. G3 trong doc/API: ba bước trục vụ (nhận queue kể `l2_datachanged` → G3-S02 AID xếp `*_intel_compute` (gom/gấp) → worker miền rồi `*_intel_recomputed` về AID). Catalog v31: G3-S06 mô tả miền phát `<domain>_intel_recomputed`; bản ghi queue đó do AID xử lý — `ResolveE2EForQueueEnvelope` trả `e2eStepId` G4-S01 (nhận event — case), `LabelVi` theo `eventType`. G3-S01 nhận job intel đầu vào; G3-S02 AID xếp `*_intel_compute`. Chú ý: enqueue `<prefix>.changed` + `l2_datachanged` có thể neo G2-S05-E01. G4-S02 (v33+) — một dòng trong `steps` gom `*.context_requested` và `*.context_ready`; chi tiết miền xem cột `**eventType`** / `**eventSource`** / `**pipelineStage`**. **v34:** bỏ dòng catalog **G4-S03** (`message.batch_ready`); đánh lại **G4-S04→G4-S03**, **G4-S05→G4-S04**; `message.batch_ready` → resolver **G2-S02**. **v35:** bỏ bước catalog **G4-S04** — gộp execute/propose (**G4-S03-E01…E03**) vào **G4-S03**; resolver / livePhase theo **G4-S03** / **G4-S03-E\***.

**API:** Cùng nội dung bảng (trường `steps` trong JSON) — [§3.1](#31-api-catalog-e2e-json-cho-frontend); nguồn đồng bộ code: `eventtypes/e2e_catalog.go`.

**Hai cột module (bổ sung §5.3):** **Module giao việc (nếu có)** và **Module thực hiện** trong bảng markdown dưới đây là **mở rộng tra cứu** (cùng tinh thần §5.2 + [§1.2](#12-miền-nghiệp-vụ-bounded-context--tên-gọi-thống-nhất) / [§1.4](#14-meta-trên-document-job-hàng-đợi-miền-đồng-bộ-bus--chuẩn-gộp-một-nguồn)); JSON `e2e-reference-catalog` — trường `steps` **chưa** có hai cột này cho tới khi nâng `E2ECatalogSchemaVersion` và mở rộng schema.

#### Đoạn đầu G1 — CIO → ghi L1 → datachanged (đọc trước bảng)

Chuỗi nghiệp vụ **trước** khi consumer AID xử lý job (các bước **G1-S01**…**G1-S03** + enqueue **G1-S04**; **không** có bước debounce ở CIO trong catalog):

1. **G1-S01 — Tiếp nhận từ ngoài:** **CIO** (Channel / Ingress Orchestrator) và các đường vào tương đương (**webhook**, **job đồng bộ kênh**, HTTP callback…) **nhận** payload / sự kiện từ **nguồn bên ngoài** (Meta, Zalo, Pancake/POS, đối tác, …). Bước này gồm xác thực chữ ký hoặc token (nếu có), ghi nhận thời điểm và loại sự kiện — **chưa** ghi **L1**.
2. **G1-S02 — Xử lý và ghi L1:** **CIO / miền ingress** (và luồng domain đồng bộ) **xử lý** nội dung đã nhận: chuẩn hoá field, map sang schema nội bộ, áp rule từ chối/ghi nhận. Sau đó **ghi bản ghi mirror / L1-persist** bằng `DoSyncUpsert` hoặc **CRUD domain** tương ứng (collection mirror theo hợp đồng module — xem `KHUNG_LUONG_INGEST_MERGE_INTEL_CIO_AID_DOMAIN.md`).
3. **G1-S03 — Báo thay đổi nguồn (bus + lọc cho AID):** Khi **ghi L1 thành công**, hook sau CRUD/sync phát `**EmitDataChanged`** — bus nội bộ tới mọi subscriber. **Handler AID** (`hooks/datachanged`) **lọc** theo collection: map `source_sync` + `ShouldEmitDatachangedToDecisionQueue` — chỉ bản ghi thuộc phạm vi đăng ký mới đi tiếp tới **enqueue** (**G1-S04**). Bus **không** tự gắn `decision_events_queue`.
4. **G1-S04 — Vào bus AID:** Theo `NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION`, tác vụ sau datachanged dẫn tới **enqueue** `decision_events_queue` với `eventSource = l1_datachanged` (emit mới; bản ghi cũ có thể còn `datachanged`), `pipelineStage = after_l1_change` (bản ghi cũ có thể còn `after_source_persist` — `IsPipelineStageAfterL1Change`). `**eventType`** trên wire = `**<prefix>.changed`** (`prefix` từ tên collection qua `source_sync_registry`, `hooks/datachanged.go`); payload có `**sourceCollection`**, `**dataChangeOperation`**. Bản ghi queue cũ `*.inserted` / `*.updated` vẫn dispatch qua `**consumerreg.Lookup**`. Đọc gọn: mỗi job là thay đổi L1 theo một collection + nhãn kênh `**l1_datachanged**` trên `eventSource`. **Bảng catalog §5.3** một dòng G1-S04. **Gom cửa sổ tin nhắn** (debounce) khi cần là cơ chế phía **AID** (`message.batch_ready` — envelope **`ResolveE2EForQueueEnvelope` neo G2-S02**, không còn bước catalog G4), không phải bước CIO trong bảng G1.

#### G2 — Consumer một cửa và merge L2 (đọc trước bảng)

**Nguyên tắc trục vụ:** Sau **đổi L1**, chuỗi nghiệp vụ “đúng nghĩa gộp dữ liệu” là **merge sang canonical (L2)**. **Debounce** = trì hoãn có chủ đích (gom nhiều thay đổi trong cửa sổ). **Xử lý gấp** = **chỉ** bỏ qua hoặc rút ngắn các bước **gom** (ưu tiên xử lý ngay và trước) khi policy/urgency yêu cầu (payload queue + rule side-effect + `eventintake`) — **vẫn phải qua đủ** cùng luồng side-effect → routing → dispatch như trường hợp không gấp.

**Tách hai tầng** (dễ nhầm nếu gộp một cụm):

1. **Cùng một bản ghi `decision_events_queue` — `worker.aidecision.consumer` / `processEvent`:** **G2-S01** lease một job (trên trục merge điển hình **l1_datachanged**) → **G2-S02** (một bước catalog): nếu L1 datachanged — `applyDatachangedSideEffects` (**gom** cửa sổ debounce; **gấp** chỉ điều chỉnh gom, không cắt luồng) — có thể **xếp job merge** vào queue miền (minh hoạ: `**crm_pending_merge`**, không merge L2 tại consumer) …; sau đó routing → `dispatchConsumerEvent` / handler / `no_handler` / `routing_skipped` (xem `e2e_catalog.go` **G2-S02**).
2. **Queue merge + worker miền — tách goroutine:** **G2-S03–S04** đọc job (minh hoạ `**crm_pending_merge`**) và **gộp L1→L2**. **G2-S05-E01:** sau merge, miền **enqueue** `decision_events_queue` — **báo cập nhật L2** cho AID (song song ý **G1-S04** báo L1); minh hoạ `NotifyIntelRecomputeAfterCrmMergeIfNeeded` → `crmqueue.EmitAfterL2MergeForCrmIntel` — `l2_datachanged`, `<prefix>.changed` (hoặc fallback `crm.intelligence.recompute_requested`), `after_l2_merge`; consumer AID nhánh `IsPostL2MergeCrmIntelEnvelope` → debounce/xếp `crm_intel_compute` (pha **G3**). **Wire:** G1-S04 = `<prefix>.changed` / `l1_datachanged` / `after_l1_change`; G2-S05-E01 = `<prefix>.changed` / `l2_datachanged` / `after_l2_merge` (cột bảng §5.3).

**Code:** `worker.aidecision.consumer.go` (`processEvent`, `IsPostL2MergeCrmIntelEnvelope`), `worker.aidecision.datachanged_side_effects.go`, `crm/datachanged/sidefx_register.go` + `merge_from_datachanged.go`, `crm/service/service.crm.pending.merge.go`, worker CRM pending merge + `crm/datachanged/notify_after_crm_merge.go`, `aidecision/crmqueue/crmqueue.go`, `eventtypes/source_collection_wire.go` (tham chiếu `NGUYEN_TAC`).

#### G3 — Pha intel sau L2-datachanged — gom, gấp, handoff về AID {#g3-pha-intel-sau-l2-datachanged--gom-gấp--handoff-về-aid}

**Ba bước trục vụ (G3):**

1. **G3-S01 — AID nhận job** trên `**decision_events_queue`** — điển hình **báo đổi L2 (`l2_datachanged`)** sau **G2-S05**: `eventSource` `**l2_datachanged`**, `pipelineStage` `**after_l2_merge`**, `eventType` `**<prefix>.changed**` (minh hoạ: `notify_after_crm_merge` → `crmqueue.EmitAfterL2MergeForCrmIntel`). Ngoài ra vẫn có các job yêu cầu intel/recompute/analysis khác — **một dòng catalog G3-S01** gom prefix miền trên wire.
2. **G3-S02 — AID tính toán và xếp việc**: routing (nhánh post-L2, `consumerreg`…), **gom** (debounce/cửa sổ, trailing…), **gấp** (ưu tiên → enqueue sớm), rồi **tạo/enqueue** job `***_intel_compute`** cho **từng miền liên quan** (`crm_intel_compute`, `ads_intel_compute`, …).
3. **Worker miền (G3-S03…S05)** chạy pipeline + read model; sau đó **G3-S06** — miền **phát** `***_intel_recomputed`** lên `**decision_events_queue`**; **AID nhận** bản ghi đó ở **G4-S01** (ResolveOrCreate / case — `ResolveE2EForQueueEnvelope`), rồi **xử lý tiếp** (ngữ cảnh, orchestrate…).

**Bổ sung kỹ thuật:** Worker miền thực thi pipeline intel, ghi **read model**, rồi mới emit handoff về queue như bước 3.

**Vì sao catalog có G3-S01…S06 và G4-S01 (khác với «ba bước»)?** **Ba bước** là **trục vụ** (nhận queue → AID xếp job intel miền → miền chạy và bắn lại). **Lớp máy:** **G3-S01…S05** như trên; **G3-S06** — dòng catalog mô tả **emit** `<domain>_intel_recomputed` từ **DomainIntel**; **bản ghi queue** cùng `eventType` đó khi AID đọc được **resolver** neo **G4-S01** (pha case), `LabelVi` theo `eventType`. **Ánh xạ:** bước trục vụ 1 ≈ **G3-S01**; bước 2 ≈ **G3-S02**; bước 3 (worker + emit) ≈ **G3-S03…S06**; **nhận handoff** trên queue ≈ **G4-S01**.

**Minh hoạ CRM (chuỗi đầy đủ):** `IsPostL2MergeCrmIntelEnvelope` → `processCrmIntelligenceRecomputeRequested` → `ScheduleCrmIntelligenceRecomputeDebounce` (gom) hoặc `EnqueueCrmIntelComputeFromDecisionEvent` (gấp / hết cửa sổ) → worker `crm_intel_compute` → `crm_intel_recomputed` → lại `decision_events_queue`.

**Code (tham chiếu):** `worker.aidecision.consumer.go` (`processCrmIntelligenceRecomputeRequested`), `eventintake` (debounce/gấp intel), worker miền `*_intel_compute`, `intelrecomputed` / emit handoff về queue.

#### G4 — Pha ra quyết định: case, ngữ cảnh, Executor {#g4-pha-ra-quyet-dinh-case-context-executor}

**Bốn bước trục vụ** (sau khi miền đã phát `**<domain>_intel_recomputed`** — AID vào pha G4; các luồng vào case khác có thể không đi đủ bốn bước):

1. **G4-S01 — Worker AID xử lý** job có `**eventType` `<domain>_intel_recomputed`** trên `decision_events_queue` (`ResolveE2EForQueueEnvelope` neo **G4-S01**): **ResolveOrCreate**, cập nhật `**decision_cases_runtime`**, điều phối tiếp.
2. **Rà soát ngữ cảnh — thiếu thì gọi Domain:** Nếu cần snapshot / dữ liệu nền, AID phát job / `***.context_requested`** (vd. `customer.context_requested`, `ads.context_requested`) — catalog **G4-S02**.
3. **Domain báo đã gửi ngữ cảnh:** `***.context_ready`** — cùng **G4-S02**; có thể **lặp (2)↔(3)**.
4. **G4-S03 — Suy luận và chốt hành động:** `**HasAllRequiredContexts`**, policy, engine case (subgraph live → `ResolveE2EForLivePhase` neo **G4-S03**; `done`/`error` cùng bước) **và** phát `**aidecision.execute_requested`** / `**executor.propose_requested`** / `**ads.propose_requested` (legacy)** — chi tiết catalog **G4-S03-E01…E03**. **G5** nhận lệnh trong pipeline Executor.

**Ghi chú:** Gom tin nhắn **`message.batch_ready`** không còn bước catalog G4 — envelope trên queue do **`ResolveE2EForQueueEnvelope` neo G2-S02** (consumer **G2-S02** `processEvent`, v34+). **v35:** không còn bước catalog **G4-S04** — gộp vào **G4-S03**.


| Giai đoạn | Bước | Sự kiện | Mô tả kỹ thuật (`descriptionTechnicalVi`) | Mô tả người dùng (`descriptionUserVi`) | `eventType` | `eventSource` | `pipelineStage` | Module giao việc (nếu có) | Module thực hiện | Nhóm trách nhiệm |
| --------- | ------ | ---------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- | --------------------------------------------------------- | ------------------ | ------------------------- | ---------------- | ----------------
| G1 | G1-S01 | — | CIO / kênh ingress nhận dữ liệu từ nguồn bên ngoài (webhook, job sync kênh, callback…); xác thực & ghi nhận — chưa ghi L1 | Cửa hàng nhận tin từ khách qua các kênh (webhook, đồng bộ…); mới kiểm tra và ghi nhận, chưa lưu vào kho dữ liệu nội bộ. | — | — | — | **—** | **Ingress / CIO** | `CIO` |
| G1 | G1-S02 | — | CIO xử lý payload: chuẩn hoá, map schema; ghi mirror / L1-persist (DoSyncUpsert) hoặc CRUD domain | Dữ liệu nghiệp vụ được chuẩn hoá và lưu bản ghi nguồn (L1) cho các bước sau. | — | — | — | **—** | **Ingress / CIO** (ghi L1) | `CIO` |
| G1 | G1-S03 | — | Sau khi ghi L1 thành công: phát EmitDataChanged (bus nội bộ). Handler AID (hooks/datachanged) lọc collection: source_sync registry + ShouldEmitDatachangedToDecisionQueue — chỉ hợp lệ mới enqueue G1-S04 | Sau khi lưu xong, hệ thống báo nội bộ có thay đổi; chỉ những loại dữ liệu đã cấu hình mới được chuyển tiếp tới hàng đợi trợ lý. | — | — | — | **aidecision** (lọc `hooks/datachanged`) | **CIO** + **aidecision** | `CIO` |
| G1 | G1-S04 | — | Enqueue decision_events_queue — bước catalog «báo thay đổi L1» (mirror/L1-persist) để AID xử lý tiếp; tương tự ý enqueue sau cập nhật canonical ở G2-S05-E01 nhưng tầng L1. Payload: sourceCollection, dataChangeOperation. Wire: eventType `<prefix>.changed` (prefix từ collection, source_sync_registry). Lookup consumer tương thích `*.inserted`/`*.updated` cũ. eventSource = l1_datachanged. | Mỗi thay đổi nguồn đã đăng ký được đưa vào hàng đợi trợ lý; loại bản ghi biết qua collection. | `<prefix>.changed` — prefix từ collection | `l1_datachanged` | `after_l1_change` | **aidecision** (enqueue `decision_events_queue`) | **aidecision** | `CIO` |
| G2 | G2-S01 | — | AIDecisionConsumerWorker lease một bản ghi decision_events_queue — bắt đầu processEvent. Trên trục pha merge (L1→L2), điển hình là một job báo đổi L1: eventSource l1_datachanged, pipelineStage after_l1_change (sau G1-S04). Cùng bước lease áp dụng cho mọi job khác trên queue (l2_datachanged, *_intel_recomputed, …) — mốc timeline consumer vẫn neo G2-S01 mỗi lần lease. | Máy lấy một việc đã xếp hàng; trên luồng merge thường là một sự kiện báo đổi L1 (`l1_datachanged`). | — | — | — | **—** | **aidecision** (consumer — lease job) | `AID` |
| G2 | G2-S02 | — | Sau G2-S01, toàn bộ processEvent trên consumer. Với nguồn L1-datachanged: gom — cửa sổ debounce (ResolveDatachangedDeferWindowsViaRule), trì hoãn/xếp lại side-effect và debounce intel (trailing…); gấp — chỉ bỏ qua hoặc rút ngắn các bước gom (không chờ đủ cửa sổ, ưu tiên xử lý ngay và trước) qua ClassifyDatachangedBusinessUrgency, immediateSideEffects / urgentSideEffects / Realtime khi policy bật — không cắt luồng: vẫn applyDatachangedSideEffects đầy đủ (hydrate, Mongo, datachangedsidefx.Run — minh hoạ CRM: có thể ghi crm_pending_merge là yêu cầu merge cho worker miền, không merge L2 tại goroutine consumer; report…), rồi ShouldSkipDispatchForRoutingRule → noop hoặc dispatchConsumerEvent — consumerreg.Lookup → handler (orchestrate, intel, Ads…) hoặc no_handler. Mốc queue datachanged_done / handler_done / handler_error / routing_skipped / no_handler đều neo catalog G2-S02. Sau khi worker miền merge xong, minh hoạ: miền emit lại decision_events_queue (l2_datachanged + .changed + after_l2_merge; consumer nhánh IsPostL2MergeCrmIntelEnvelope → crm intel debounce, không Lookup .changed như L1) — bước catalog G2-S03–G2-S05-E01. **v34:** envelope **`message.batch_ready`** (debounce tin nhắn) — **`ResolveE2EForQueueEnvelope` neo G2-S02**, không còn dòng catalog G4. | Có thể gom nhiều cập nhật theo cửa sổ (debounce) để đỡ tải; khi gấp, bỏ phần gom và ưu tiên trước nhưng vẫn qua đủ đồng bộ — có thể gửi yêu cầu gộp sang worker miền; khi miền xong, việc quay lại hàng đợi trợ lý để chạy tiếp; gom tin nhắn batch neo **G2-S02** trên resolver. | — | — | — | **aidecision** (routing / side-effect) | **aidecision** (`processEvent`) | `AID` |
| G2 | G2-S03 | — | Worker miền lấy job gộp L1→L2 — tách khỏi consumer G2-S01–S02 (queue/worker theo từng miền dữ liệu). Minh hoạ CRM: đọc crm_pending_merge (CrmPendingMergeWorker), không chạy trong consumer AID. | Worker miền nhận việc gộp đã xếp từ bước trước (vd. hàng đợi merge CRM) và thực hiện gộp dữ liệu. | — | — | — | **aidecision** (đã xếp job merge) | **worker miền** (vd. **crm**) | `Domain` |
| G2 | G2-S04 | — | Áp merge: ghi canonical (uid, sourceIds, links). | Cập nhật hồ sơ chung để mọi kênh trỏ cùng một thực thể. | — | — | — | **—** | **worker miền** (merge L1→L2) | `Domain` |
| G2 | G2-S05 | G2-S05-E01 | Sau merge L2: enqueue decision_events_queue — bước catalog «báo thay đổi / cập nhật sau L2» (canonical đã gộp) để AID xử lý tiếp, cùng vai trò nghiệp vụ với G1-S04 (báo đổi L1) nhưng tầng L2. Minh hoạ CRM: notify_after_crm_merge → crmqueue.EmitAfterL2MergeForCrmIntel — eventSource l2_datachanged, pipelineStage after_l2_merge, eventType .changed (eventtypes/source_collection_wire.go; đồng bộ hooks/source_sync_registry); không map được collection → fallback crm.intelligence.recompute_requested. Consumer: IsPostL2MergeCrmIntelEnvelope → processCrmIntelligenceRecomputeRequested (debounce → crm_intel_compute). Bản ghi cũ có thể còn crm_merge_queue + crm.intelligence.recompute_requested (resolver vẫn G2-S05-E01). So với G1-S04: cùng .changed nhưng l2_datachanged + after_l2_merge (và không đi Lookup orchestrate L1). | Miền gộp xong hồ sơ chung (L2) rồi báo thay đổi lên hàng đợi AID như bước báo đổi L1 — để trợ lý xử lý tiếp. | `<prefix>.changed` — prefix từ collection | `l2_datachanged` | `after_l2_merge` | **miền** (emit sau L2) | **miền** (ghi canonical + bus) | `Domain` |
| G3 | G3-S01 | — | Trên trục pha intel (G3), consumer AID đã lease job decision_events_queue và đang xử lý envelope intel: điển hình báo đổi L2 — eventSource l2_datachanged, pipelineStage after_l2_merge, eventType `<prefix>.changed` (chuỗi sau G2-S05-E01; minh hoạ EmitAfterL2MergeForCrmIntel). Chú ý resolver envelope: bản ghi `<prefix>.changed` + l2 có thể neo G2-S05-E01 (bước emit sau merge L2), không phải G3-S01. G3-S01 là một dòng catalog gom mọi **loại job đầu vào** yêu cầu intel/recompute/analysis theo prefix miền (crm.*, ads.*, order.*, cix.*) — kể cả không từ l2_datachanged. Bước **tính toán và enqueue** job `*_intel_compute` nằm ở **G3-S02**. Sau khi miền phát handoff (catalog **G3-S06**), bản ghi `*_intel_recomputed` trên queue được **ResolveE2EForQueueEnvelope** neo **G4-S01**. Tra ResolveE2EForQueueEnvelope + consumerreg. | Trợ lý đang xử lý một việc trên hàng đợi liên quan làm mới phân tích — thường là báo đổi L2 sau khi hồ sơ chung cập nhật; cùng một bước catalog cho các tên yêu cầu intel khác trên wire. | `<domain>.*` — crm / ads / order / cix (vd. `crm.intelligence.*_requested`); điển hình sau L2: `<prefix>.changed` + `l2_datachanged` | điển hình `l2_datachanged`; tuỳ miền và emit (crm, meta_*, aidecision, …) | điển hình `after_l2_merge`; tuỳ emit (`after_l1_change`, `external_ingest`, …) | **bus / miền** (enqueue đầu vào intel; xem §1.4) | **aidecision** (consumer — envelope intel) | `AID` |
| G3 | G3-S02 | — | **AID** — sau G3-S01: routing + **gom/gấp** + enqueue job `*_intel_compute` cho **các miền liên quan** (crm_intel_compute, ads_intel_compute, …). Minh hoạ CRM: `IsPostL2MergeCrmIntelEnvelope` → `processCrmIntelligenceRecomputeRequested` → debounce / enqueue trực tiếp. Không chạy pipeline intel nặng ở bước này. | AID quyết định và tạo việc tính lại intelligence trên hàng đợi từng domain (có gom/gấp). | — | — | — | **aidecision** (xếp `*_intel_compute`) | **aidecision** | `AID` |
| G3 | G3-S03 | — | Worker miền lease/lấy job từ hàng đợi `*_intel_compute` (sau G3-S02). | Worker của từng domain lấy job tính toán lại intelligence | — | — | — | **aidecision** (job đã vào `*_intel_compute`) | **worker miền** | `Domain` |
| G3 | G3-S04 | — | Chạy pipeline phân tích (rule/LLM/snapshot). | Worker của từng domain chạy tính toán lại intelligence | — | — | — | **—** | **worker miền** (pipeline intel) | `Domain` |
| G3 | G3-S05 | — | Ghi bản ghi chạy intel + cập nhật read model. Mốc live `intel_domain_compute_done` neo **G3-S05**. | Lưu kết quả tính toán lại intelligence | — | — | — | **—** | **worker miền** (read model) | `Domain` |
| G3 | G3-S06 | — | Sau khi worker miền tính xong: **phía miền** phát `<domain>_intel_recomputed` lên `decision_events_queue` (wire cột `eventType` / `eventSource` / `pipelineStage`). Dòng catalog mô tả **emit** (DomainIntel); **không** dùng làm `e2eStepId` envelope khi AID consumer đọc job — **ResolveE2EForQueueEnvelope** neo **G4-S01**. | Miền bắn event đã tính toán xong intelligence lên hàng đợi AID để tiếp tục xử lý | `<domain>_intel_recomputed` | `cix_intel`, `crm_intel`, `order_intel`, `meta_ads_intel` | `domain_intel` | **miền** (emit handoff) | **worker miền** (DomainIntel) | `Domain` |
| G4 | G4-S01 | — | ResolveOrCreate và cập nhật decision_cases_runtime. **Điển hình:** AID nhận job `decision_events_queue` với **`<domain>_intel_recomputed`** (sau G3-S06) — vào pha quyết định; `ResolveE2EForQueueEnvelope` trả **G4-S01**, `LabelVi` theo `eventType`. Các luồng case khác vẫn dùng cùng bước catalog. | Trợ lý mở hoặc cập nhật vụ việc — thường khi nhận bàn giao phân tích từ miền (`*_intel_recomputed`) hoặc các luồng case khác. | — | — | — | **miền** (`*_intel_recomputed`) | **aidecision** (case runtime) | `AID` |
| G4 | G4-S02 | — | Điều phối ngữ cảnh bổ sung AID ↔ miền — **một dòng catalog** gom **.context_requested** và **.context_ready**; chi tiết theo `eventType` (vd. `customer.context_requested` / `customer.context_ready`, `ads.context_*`). **ResolveE2EForQueueEnvelope** neo **G4-S02** cho mọi wire loại này; **không** dùng `eventDetailId` tách E. | Trợ lý xin thêm hoặc nhận báo đủ dữ liệu nền — phân biệt requested / ready qua `eventType`. | `<domain>.context_requested` / `<domain>.context_ready` — vd. customer., ads. | `aidecision`; `crm`; `meta_ads_intel` (tuỳ event) | `aid_coordination` | **aidecision** (`*.context_requested`) / **miền** (`*.context_ready`) | **aidecision** + **miền** | `AID` |
| G4 | G4-S03 | — | Sau khi đủ ngữ cảnh: kiểm HasAllRequiredContexts, áp policy, engine case (parse / LLM / decision / policy / ads_evaluate trong live phase) — tính toán logic để chọn hướng đề xuất hành động; hoàn tất / lỗi pipeline live map cùng bước (không eventDetailId). Phát lệnh sang Executor: **G4-S03-E01…E03** (wire queue). `ResolveE2EForLivePhase` neo **G4-S03** cho các phase engine tương ứng. | Trợ lý xác nhận đủ thông tin, áp quy tắc, suy luận và chuẩn bị hoặc gửi hành động phù hợp. | — | — | — | **—** | **aidecision** (policy + engine) | `AID` |
| G4 | G4-S03 | G4-S03-E01 | Phát lệnh thực thi trực tiếp (execute) | Trợ lý chuẩn bị gửi hành động đi thực hiện (gửi tin, cập nhật hệ thống…). | `aidecision.execute_requested` | `aidecision` | `aid_coordination` | **aidecision** | **aidecision** | `AID` |
| G4 | G4-S03 | G4-S03-E02 | Tạo đề xuất hành động tại Executor (propose) | Trợ lý tạo đề xuất để bạn hoặc hệ thống duyệt trước khi chạy. | `executor.propose_requested` | `aidecision` | `aid_coordination` | **aidecision** | **aidecision** | `AID` |
| G4 | G4-S03 | G4-S03-E03 | Event cũ tương thích cho Ads | Luồng đề xuất quảng cáo kiểu cũ (tương thích hệ thống trước). | `ads.propose_requested` (legacy) | `aidecision` | `aid_coordination` | **aidecision** | **aidecision** | `AID` |
| G5 | G5-S01 | — | Executor nhận proposal/action từ AI Decision | Khối thực thi nhận lệnh hoặc đề xuất từ trợ lý. | — | — | — | **aidecision** | **executor** | `Executor` |
| G5 | G5-S02 | — | Duyệt theo policy (manual/auto/...) | Hành động được duyệt tự động hoặc chờ người xác nhận tùy cài đặt. | — | — | — | **aidecision** / policy | **executor** | `Executor` |
| G5 | G5-S03 | — | Dispatch adapter để thực thi | Lệnh được gửi đúng kênh kỹ thuật (API, tin nhắn…) để hoàn tất. | — | — | — | **executor** (adapter) | **executor** (+ **delivery** khi có) | `Executor` |
| G6 | G6-S01 | — | Ghi kết quả kỹ thuật (delivery/API response) | Hệ thống ghi nhận đã gửi thành công hay lỗi kỹ thuật. | — | — | — | **—** | **delivery** / outcome | `Outcome` |
| G6 | G6-S02 | — | Thu kết quả nghiệp vụ theo time window | Theo dõi kết quả thực tế sau một khoảng thời gian (theo cấu hình vụ việc). | — | — | — | **—** | **outcome** | `Outcome` |
| G6 | G6-S03 | — | Action chuyển trạng thái kết thúc (executed/rejected/failed) | Việc được đánh dấu hoàn thành, từ chối hoặc thất bại rõ ràng. | — | — | — | **—** | **learning** | `Learning` |
| G6 | G6-S04 | — | OnActionClosed → CreateLearningCaseFromAction → insert learning_cases | Từ kết thúc việc, hệ thống tạo bản ghi học để đánh giá sau. | — | — | — | **—** | **learning** | `Learning` |
| G6 | G6-S05 | — | Chạy RunEvaluationBatch / job đánh giá | Chạy đợt đánh giá chất lượng gợi ý và kết quả. | — | — | — | **—** | **learning** | `Learning` |
| G6 | G6-S06 | — | Ghi field evaluation (ví dụ outcome_class, attribution) | Gắn nhãn kết quả (tốt/xấu, nguyên nhân) phục vụ báo cáo. | — | — | — | **—** | **learning** | `Learning` |
| G6 | G6-S07 | — | Sinh param_suggestions, rule_candidate, insight | Sinh gợi ý chỉnh tham số hoặc quy tắc dựa trên dữ liệu thực tế. | — | — | — | **—** | **ruleintel** / feedback | `Feedback` |
| G6 | G6-S08 | — | Đẩy ngược cải tiến lên Rule/Policy/AID (có thể có bước duyệt người) | Gợi ý cải tiến được đưa lên bảng điều khiển quy tắc (có thể cần người duyệt). | — | — | — | **—** | **ruleintel** / **aidecision** (policy) | `Feedback` |

#### Bổ sung: `eventType` domain trên `decision_events_queue`

Nguồn hằng số: `api/internal/api/aidecision/eventtypes/names.go` (một nguồn cho dispatch / tier / emit). Bảng dưới **bổ sung** cho cột `eventType` ở bảng catalog — không thay thế từng dòng `steps` trong code.

| Nhóm quy ước | Ví dụ `eventType` | Ghi chú catalog / resolver (điển hình) |
| --- | --- | --- |
| Ngữ cảnh AID ↔ miền | `ads.context_requested`, `ads.context_ready`, `customer.context_requested`, `customer.context_ready` | **G4-S02**; `eventSource` tuỳ wire (aidecision, crm, meta_ads_intel, …) |
| Yêu cầu intel / phân tích | `crm.intelligence.compute_requested`, `crm.intelligence.recompute_requested`, `ads.intelligence.recompute_requested`, `ads.intelligence.recalculate_all_requested`, `order.recompute_requested`, `order.intelligence_requested` (legacy), `cix.analysis_requested` | **G3-S01** (job đầu vào); enqueue `*_intel_compute` ở **G3-S02** |
| Handoff sau worker intel | `crm_intel_recomputed`, `cix_intel_recomputed`, `order_intel_recomputed`, `campaign_intel_recomputed` | Emit miền — **G3-S06**; envelope queue khi AID đọc → **G4-S01** |
| Datachanged / L1–L2 | `<prefix>.changed` (map collection; legacy `*.inserted` / `*.updated`), `order.changed`, `conversation.changed`, `message.changed`, `meta_campaign.changed`, … | **G1-S04** (`l1_datachanged`); **G2-S05-E01** (`l2_datachanged`); resolver `<prefix>.changed`+L2 có thể neo **G2-S05-E01** |
| Khách đa kênh (tier) | `crm_customer.inserted` / `crm_customer.updated`, `pos_customer.inserted` / `pos_customer.updated`, `fb_customer.inserted` / `fb_customer.updated` | Datachanged L1 — **G1-S04** (nhãn tier trong `names.go`) |
| Ads snapshot | `ads.updated` | Cập nhật mirror Ads; routing intel / tier tùy cấu hình — thường gần nhóm **G3-S01** / datachanged |
| Legacy queue (tier / livecopy) | `order.inserted` / `order.updated`, `conversation.inserted` / `conversation.updated`, `message.inserted` / `message.updated`, `meta_campaign.inserted` / `meta_campaign.updated`, … | Tương thích `consumerreg` — thường cùng nhóm datachanged L1; ưu tiên wire `.changed` |
| POS / Meta chi tiết (livecopy) | `pos_shop.updated`, `pos_product.updated`, `meta_ad.updated`, `meta_ad_insight.updated`, … | Datachanged theo collection — map prefix như **G1-S04**; có thể hiển thị narrative livecopy |
| Hội thoại / batch | `message.batch_ready`, `conversation.message_inserted`, … | **`message.batch_ready`** → resolver **G2-S02** (v34+) |
| Thực thi / đề xuất | `aidecision.execute_requested`, `executor.propose_requested`, `ads.propose_requested` (legacy) | **G4-S03-E01…E03** |

Chi tiết từng bước `Gx-Syy` vẫn lấy từ bảng catalog phía trên và JSON `steps` của GET `e2e-reference-catalog`.


---

## 6. `nextStepHint` — Đọc tiếp và vận hành

`**nextStepHint`:** Sau khi tra catalog ở §5 — **pha cũ P01–P13**, **ghi chú vận hành**, **changelog** doc.

### 6.1. Mapping nhanh: giai đoạn lớn ↔ bản pha nhỏ cũ (P01–P13)


| Giai đoạn lớn | Pha nhỏ cũ (tham chiếu)               |
| ------------- | ------------------------------------- |
| G1            | P01 (ingress CIO → enqueue)           |
| G2            | P02–P04 (consumer một cửa + merge L2) |
| G3            | P05–P06                               |
| G4            | P07–P08                               |
| G5            | P09                                   |
| G6            | P10–P13                               |


### 6.2. Ghi chú vận hành

1. **Không mọi luồng đi đủ G1→G6:** có thể bỏ **G2** (không merge) hoặc **G3** (không intel). **CIO / ingress không có bước catalog riêng cho debounce**; gom tin nhắn **`message.batch_ready`** — **`ResolveE2EForQueueEnvelope` neo G2-S02** (v34+; không còn catalog G4).
2. **G1** (enqueue) và **G2** (consumer) thường nối tiếp rất nhanh trên cùng thay đổi nguồn — timeline consumer dùng `**e2eStage` = `G2`**, `**G2-S01`** / `**G2-S02**` (nhiều mốc có thể lặp **G2-S02**); envelope datachanged vẫn map **G1-S04**.
3. **G6** (học) có phần kết quả nghiệp vụ **trễ** so với **G5** (thực thi) — không nhất thiết tuyến tính theo thời gian tuyệt đối.
4. Trên `decision_events_queue`, phân biệt hai nhóm: luồng **L1 datachanged** (`l1_datachanged`, tương thích `datachanged` cũ; `pipelineStage` `**after_l1_change`**, tương thích `**after_source_persist`** cũ — `IsPipelineStageAfterL1Change`) và luồng bàn giao sau intel — xem `docs/05-development/NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md`.
5. Truy vết đầu-cuối: `traceId`, `correlationId`; không nhầm `decisionCaseId` với `traceId`.
6. **Tham chiếu E2E:** Lọc/query theo `e2eStage` / `e2eStepId` trên queue hoặc org-live persist; `eventType` chưa có trong map sẽ nhận nhãn “chưa map” trong resolver — cần bổ sung `e2e_reference.go` khi thêm loại sự kiện mới.
7. **Kết quả / bất thường:** Trên org-live persist (và JSON timeline), lọc theo `outcomeAbnormal`, `outcomeKind`, hoặc `severity` kết hợp `phase`; bản ghi cũ trước khi triển khai có thể thiếu `outcome*` (backfill khi đọc timeline sẽ suy luận nếu đủ `phase`/`severity`).

### 6.3. Changelog

- **2026-04-15:** Doc §5.3 — bảng catalog thêm **Module giao việc (nếu có)** / **Module thực hiện**; đồng bộ ô queue **G2-S01**, **G3-S01**, **G4-S01** với `eventtypes/e2e_catalog.go`; mục **Bổ sung: `eventType` domain** (tham chiếu `names.go`).
- **2026-04-15:** Doc §5.2 — bảng «Sáu pha chính» và «G1–G6» thêm cột **Module giao việc (nếu có)** / **Module thực hiện** + ghi chú liên kết §1.2 / §1.4.
- **2026-04-15:** Doc — **§1.4** meta trên document **job hàng đợi miền** (`eventType` / `eventSource` / `pipelineStage`, `ownerDomain`, `processorDomain`, `enqueueSourceDomain`, `e2eStage`, `e2eStepId`); bảng tham chiếu §3; §1.2 «Đọc thêm» trỏ `crmqueue/domain_queue_bus.go`; [co-cau-module-aid-va-domain-queue.md](../module-map/co-cau-module-aid-va-domain-queue.md) (mục «Meta job miền»).
- **2026-04-09:** Catalog + API + resolver + doc — **v35:** bỏ bước catalog **G4-S04** — gộp **execute/propose** (**G4-S03-E01…E03**) vào **G4-S03**; `queued` / `propose` / `done` / `error` và envelope execute → **G4-S03** / **G4-S03-E\***; `E2ECatalogSchemaVersion` **35**.
- **2026-04-09:** Catalog + API + resolver + doc — **v34:** bỏ dòng catalog **G4-S03** (`message.batch_ready`); đánh lại **G4-S04→G4-S03**, **G4-S05→G4-S04** (**E01–E03**); **`message.batch_ready`** → **`e2eStepId` G2-S02**; `stages` **G4** — **năm bước** trục vụ; `ResolveE2EForLivePhase` / execute-propose map theo ID mới; `E2ECatalogSchemaVersion` **34**.
- **2026-04-09:** Catalog + API + resolver + doc — **v33:** **G4-S02** — **một dòng** trong `steps` (gom `*.context_requested` + `*.context_ready`); **ResolveE2EForQueueEnvelope** trả `**e2eStepId` G4-S02** cho mọi event ngữ cảnh; `E2ECatalogSchemaVersion` **33**.
- **2026-04-09:** Catalog + API + doc — **v32:** `stages` **G4** — `summaryVi` / `userSummaryVi` mô tả **bốn bước** (G4-S01 → `*.context_requested` / `*.context_ready` → G4-S04 → G4-S05); doc §5.2–5.3 mục **G4**; `E2ECatalogSchemaVersion` **32**.
- **2026-04-09:** Catalog + API + resolver + doc — **v31:** envelope `**<domain>_intel_recomputed`** → `**e2eStepId` G4-S01** (AID nhận — case); **G3-S06** giữ trong `steps` là bước **miền phát** handoff; `E2ECatalogSchemaVersion` **31**.
- **2026-04-09:** Catalog + API + resolver + doc — **v30:** **G3-S02** — AID **xếp job** `*_intel_compute` (tách khỏi G3-S01 «nhận job»); handoff `*_intel_recomputed` catalog **G3-S06**; **livePhase** `intel_domain_compute_done` → **G3-S05** (persist); `E2ECatalogSchemaVersion` **30** (tới **v31** resolver handoff → **G4-S01**).
- **2026-04-09:** Catalog + API + resolver — **v29:** **G3-S05** gộp **một dòng** intel_recomputed trong `steps` (từ **v30** bước này là **G3-S06**); `e2eStepId` envelope = **G3-S05** (trước v30); `E2ECatalogSchemaVersion` **29**.
- **2026-04-09:** Catalog + API + doc — **v28:** **G3-S01** — **máy lấy một sự kiện báo đổi L2 (`l2_datachanged`)** điển hình; ghi chú resolver G2-S05-E01 vs một dòng gom intel; `steps` G3-S01 + mục §5.3 G3 + §3.1; `E2ECatalogSchemaVersion` **28**.
- **2026-04-09:** Catalog + API + doc + resolver — **v27:** **G2-S01** làm rõ **điển hình một job `l1_datachanged`** (trục merge); cùng lease mọi job queue; cập nhật `queueMilestones`, `ResolveE2EForQueueConsumerMilestone`, `ResolveE2EForLivePhase` (`queue_processing`); doc §1 mở đầu + §5.2–5.3 G2; `E2ECatalogSchemaVersion` **27**.
- **2026-04-09:** Catalog + API + doc — **v26:** **G3** — mô tả **ba bước trục vụ** (AID nhận `decision_events_queue` kể `**l2_datachanged`** → tạo job `***_intel_compute`** miền với **gom/gấp** → miền **bắn** `***_intel_recomputed`** lại queue cho AID/G4+); `stages` G3 + `steps` G3-S01 + mục §5.3 G3; `E2ECatalogSchemaVersion` **26**.
- **2026-04-09:** Catalog + API + resolver — **v25:** **G3-S01** — **một dòng** trong `steps` (prefix domain); `e2eStepId` envelope = **G3-S01** (nhãn `LabelVi` theo `eventType`); `E2ECatalogSchemaVersion` **25**.
- **2026-04-09:** Catalog + API + doc — **v24:** **G3** — mô tả **L2-datachanged → AID gom/gấp → hàng đợi *_intel_compute → _intel_recomputed* về `decision_events_queue`; mục §5.3 [G3](#g3-pha-intel-sau-l2-datachanged--gom-gấp--handoff-về-aid); `E2ECatalogSchemaVersion` **24**.
- **2026-04-09:** Catalog + API + code — **v23:** **G2-S05** — wire `**l2_datachanged`** + `**<prefix>.changed`** (`EmitAfterL2MergeForCrmIntel`, `IsPostL2MergeCrmIntelEnvelope`, `eventtypes/source_collection_wire.go`); tương thích bản ghi `**crm_merge_queue`** cũ; `E2ECatalogSchemaVersion` **23**.
- **2026-04-09:** Catalog + API — **v22:** **G2-S05** / **G1-S04** — cùng vai trò **enqueue báo thay đổi** (`decision_events_queue`): L1 vs L2; `E2ECatalogSchemaVersion` **22**.
- **2026-04-09:** Catalog + API — **v21:** **G2-S05-E01** — `descriptionUserVi` «hàng đợi AID»; phân biệt wire **G1-S04**; sửa bảng §5.3 cột `eventType`/`eventSource`; `E2ECatalogSchemaVersion` **21**.
- **2026-04-09:** Catalog + API — **v20:** **G2** — mô tả vòng **AID → `crm_pending_merge` → worker miền merge → emit lại `decision_events_queue`** (`crmqueue`, `crm_merge_queue`); `E2ECatalogSchemaVersion` **20**; doc §1, §3.1, §5.2–5.3 (mục G2 + bảng G2-S02…S05-E01).
- **2026-04-09:** Catalog + API — **v19:** **G2** — **gấp** chỉ bỏ/rút ngắn **gom**, vẫn đủ luồng consumer; `E2ECatalogSchemaVersion` **19**; doc §1, §3.1, §5.2–5.3.
- **2026-04-09:** Catalog + API — **v18:** **G2-S02** — mô tả làm rõ **gom** + **gấp** trong `processEvent`; `E2ECatalogSchemaVersion` **18**; doc §1, §3.1, §5.2–5.3 (bảng G2-S02 đồng bộ chữ với `e2e_catalog.go`).
- **2026-04-09:** Catalog + API — **v17:** G2 — **bỏ bước catalog G2-S03** (gộp dispatch/handler vào mô tả **G2-S02**); đánh lại worker merge **G2-S03–S05** (**G2-S05-E01** thay **G2-S06-E01**); resolver + doc §3.1, §4.3, §5.2–5.3.
- **2026-04-09:** Doc — mở đầu: **«Khung tham chiếu vs thực tế»** — G1–G6 là khung tiêu chuẩn toàn trình; event/job runtime có thể vào/ra bất kỳ điểm; neo E2E không có nghĩa mọi vụ đi hết catalog.
- **2026-04-09:** Catalog + API — **v15:** G2 — trục vụ sau L1: merge L2 + debounce + xử lý gấp; mô tả G2 `stages` + `G2-S02`/`G2-S03`; doc §1 chia pha, §5.2 gom bước, §5.3 G2 + bảng.
- **2026-04-09:** Code + catalog + API — **v14:** wire G1 datachanged = `<prefix>.changed`; `consumerreg` fallback `*.inserted`/`*.updated`; doc §5.3 + §3.1.
- **2026-04-09:** Catalog + API — **v13:** **G1-S04** — catalog gộp `eventType` `<prefix>.changed` (wire lúc đó vẫn `.inserted`/`.updated`); doc §5.3 + §3.1.
- **2026-04-09:** Catalog + API — **v12:** **G1-S04** — mô tả prefix + wire (`prefix`.inserted hoặc `prefix`.updated; `l1_datachanged` = `eventSource`); doc §5.3 + §3.1.
- **2026-04-09:** Catalog + API — **v11:** **G1-S04** gom **một dòng** trong `steps` (mô tả chung mọi miền enqueue L1; bỏ tách E01…E05 trong JSON). Doc §5.3 + §3.1 đồng bộ.
- **2026-04-09:** Catalog + doc — **Mô tả gộp theo nhóm miền:** G3-S01-E01…E05 (trước khi gộp v11), G3-S05-E01…E04 (trước khi gộp v29), G4-S02 requested/ready từng tách **G4-S02-E01…E04** (trước **v33** một dòng `steps`); G3 `userSummaryVi` giai đoạn rút gọn. (Trước v11, G1-S04 từng tách E01…E05.)
- **2026-04-09:** Doc + catalog — **G2 hai tầng:** consumer một job `decision_events_queue` (**G2-S01–S03**) tách khỏi worker merge L2 (**G2-S04–S06**); viết lại mô tả G2 trong `e2e_catalog.go`, §5.2, gom bước, thêm mục [§5.3 — G2](#g2-consumer-va-merge-l2).
- **2026-04-09:** Catalog + API doc — **v10:** `steps` tách `**descriptionTechnicalVi`** / `**descriptionUserVi`**; `stages` thêm `**userSummaryVi`**; `queueMilestones` thêm `**userLabelVi**`; bỏ `shortVi` trên `steps`. Doc **§5.3:** bảng chi tiết đổi từ một cột «Mô tả ngắn» sang hai cột mô tả (khớp JSON + `e2e_catalog.go`), đồng bộ cột sự kiện (vd. **G3-S01-E02**).
- **2026-04-09:** Catalog + doc — **G1-S03** làm rõ: bus `EmitDataChanged` + **lọc collection** trong handler AID trước **G1-S04**; `E2ECatalogSchemaVersion` **9**.
- **2026-04-09:** Code + doc — `**pipelineStage` `after_source_persist` → `after_l1_change`** (`PipelineStageAfterL1Change`); resolver đọc bản ghi cũ qua `IsPipelineStageAfterL1Change`; `E2ECatalogSchemaVersion` **8**.
- **2026-04-09:** Code + doc — **v7:** G1 chỉ CIO **S01–S04**; consumer timeline **G2-S01…S03**; merge L2 **G2-S04…S06** (**G2-S06-E01**); emit `eventSource` `**l1_datachanged`** (`IsL1DatachangedEventSource` chấp nhận `datachanged` cũ); `E2ECatalogSchemaVersion` **7**.
- **2026-04-09:** Code — `E2ECatalogSchemaVersion` **2** (bước G1-S02 trong JSON `steps` đổi nội dung / bỏ khóa ví dụ).
- **2026-04-09:** Doc + catalog — **G1-S02**: sửa ý nghĩa (debounce / bỏ qua debounce trong **cùng luồng**, không nhánh ingress riêng); bỏ ví dụ `cix.analysis_requested` gắn S02; `e2e_catalog.go` đồng bộ; §6.2 ghi chú vận hành; comment `e2e_reference.go`.
- **2026-04-09:** Doc — **Sáu pha chính** (ghi thô = G1+G2, merge, intel, ra quyết định, thực thi, học) ở đầu §5.2; bảng G1–G6 thêm cột «Pha chính»; §1 swimlane + «Cách chia pha»; §3.1 mô tả `stages` vs hiển thị 6 pha; comment `e2e_catalog.go`.
- **2026-04-09:** Doc — Bỏ lớp nhãn V1–V6; gom trục vụ trong **§5.2** (neo `gom-buoc-truc-vu`); §5.3 neo `bang-catalog-chi-tiet-e2e`.
- **2026-04-09:** Doc — **§1 «Trục vụ — mục đích của pha và bước»**: chốt pha/bước Gx-Syy là **trục theo dõi vụ** (CIO → learning/feedback), tách với **trục triển khai** (nhiều queue/worker); mục lục cột `label` nêu rõ «trục vụ».
- **2026-04-09:** Doc — mục **[§3.1](#31-api-catalog-e2e-json-cho-frontend)** mô tả đầy đủ GET `e2e-reference-catalog` (bảng field `data`); §5 mở đầu + §5.2/5.3 trỏ API; `docs/api/api-overview.md` và `docs/flows/README.md` cập nhật.
- **2026-04-09:** Doc + catalog — **v6:** xóa bước catalog **debounce CIO** (không tồn tại); đánh lại **G1-S02…S07** (S02 = xử lý+L1, S03 = `EmitDataChanged`, S04 = enqueue, S05…S07 = consumer); `E2ECatalogSchemaVersion` **6**.
- **2026-04-09:** Doc + catalog — **Chuỗi đầu G1** ([§5.3](#g1-cio-l1-datachanged)): mô tả CIO nhận từ ngoài → xử lý → ghi L1 → `EmitDataChanged` → enqueue; (sau đó v6 chỉnh số bước, bỏ debounce CIO).
- **2026-04-09:** Code — **G1–G6 (schema v4):** gộp stage kiến trúc cũ ingress+consumer thành **G1**; đánh lại G2…G6; v6: consumer milestone **G1-S05…S07**; **v7:** tách consumer sang **G2-S01…S03**; cập nhật `e2e_reference.go`, test, doc §5.2–5.3.
- **2026-04-09:** Code — API **GET `/v1/ai-decision/e2e-reference-catalog`** (read + org context như GET ai-decision khác): trả `stages` (§5.2), `steps` (§5.3), `queueMilestones` (consumer — **G2**), `livePhaseMap` (`DecisionLiveEvent.phase` → E2E); nguồn `eventtypes/e2e_catalog.go`, `decisionlive/e2e_live_phase_catalog.go`.
- **2026-04-09:** Code — field `**businessDomain`** trên `DecisionLiveEvent` + enrich `business_domain_enrich.go`, persist/ refs Mongo, index model; doc §1.2 đoạn Timeline/API + §2 persist.
- **2026-04-09:** Doc — thêm **§1.3 Owner domain của event**: quy ước `ownerDomain` vs `consumerDomain`, bảng map `eventType`/`eventSource` → miền nghiệp vụ, thứ tự ưu tiên suy luận owner để vẽ lưu đồ dòng chảy nghiệp vụ.
- **2026-04-09:** Doc — **§1.1–1.2** từ vựng thống nhất: mã swimlane (`ING`…`FBK`), bảng miền nghiệp vụ (tên lưu đồ + package Go); §5.2 thêm cột «Mã swimlane»; §5.3 chú thích khớp §1.1–1.2; ma trận §4.5 chỉnh nhãn nguồn mốc theo `AID`/`cix`/`INT`; trỏ [co-cau-module-aid-va-domain-queue.md](../module-map/co-cau-module-aid-va-domain-queue.md).
- **2026-04-09:** Doc — **Tái cấu trúc toàn bộ** theo khung sáu trường **cấp tài liệu** (mục lục đầu file): §1 `label`, §2 `purpose`, §3 `inputSummary`, §4 `logicSummary` (4.1–4.5), §5 `resultSummary` (5.1–5.3), §6 `nextStepHint` (6.1–6.3). «Khung sáu trường một bước» chỉ còn một nơi ở §4.1; Phương án B trỏ chéo §4.1; mục «Khung nội dung đầy đủ» đổi tên **§4.5**; bảng tra + khớp code gom **§5**; mapping P01–P13 + ghi chú + changelog gom **§6**.
- **2026-04-09:** Doc — Phương án B: thêm **«Khung sáu trường — một bước logic»** (`label` … `nextStepHint`, ví dụ copy, bảng ánh xạ `labelVi`/`detailVi`/`TraceStep`); checklist B.7 bổ sung kiểm sáu trường; khối 5 trong «Khung nội dung đầy đủ» trỏ chéo.
- **2026-04-09:** Doc — thay mục **«Khung nội dung timeline»** bằng **«Khung nội dung đầy đủ — timeline, trace, audit»**: sáu khối trên mỗi mốc; bảng liên kết (`traceId`, `correlationId`, W3C span, `eventId`↔queue, case, entity, handoff ref); hai mẫu đọc chuỗi; cầu nối queue↔org-live; ma trận nguồn mốc; checklist trace/audit; cập nhật trỏ chéo từ mục Publish.
- **2026-04-09:** Doc — thêm mục **«Publish live event — đọc nhanh»**: một câu vai trò, bảng queue vs Publish, bảng bước trong `publish.go`, sơ đồ mermaid, hai kênh WS, ba lớp nội dung cho người đọc; rút gọn khối quy tắc `DetailBullets`/`e2e`* thành đoạn bổ sung + trỏ mục khung A/B/C.
- **2026-04-08:** Code — `processTrace` consumer: nhãn và `detailVi` **ưu tiên người dùng cuối** (tiếng Việt, `QueueFriendlyEventLabel`, bỏ thuật ngữ registry/hàm nội bộ trên cây); cây con datachanged mô tả nghiệp vụ (gộp khách, báo cáo, quảng cáo, lịch xử lý).
- **2026-04-08:** Code — consumer queue: `**queue_trace_step.go`** điền `TraceStep.inputRef` / `outputRef` / `reasoning` trong `BuildQueueConsumerEvent` (Phương án B); test `queue_trace_step_test.go`.
- **2026-04-08:** Doc — **Phương án B** (chi tiết): lưu quá trình thật qua `**TraceStep`** (`inputRef` / `reasoning` / `outputRef`), phân công với `processTrace`, chuỗi mốc consumer, mở rộng `steps[]` vs `substeps`, checklist triển khai; bảng `processTrace` trỏ chéo.
- **2026-04-08:** Doc — mục `**processTrace`**: mô tả **chi tiết luồng runtime** consumer G2 (lease → `processEvent` → datachanged / routing / dispatch / handler), từng **mốc Publish** và **snapshot** cây, bảng `key`, giới hạn cắt cây, ngoại lệ `execute_requested`.
- **2026-04-08:** Doc — cập nhật mô tả **Publish**: dòng đầu `DetailBullets` dùng `**Trong quy trình:`** (thay tiền tố `E2E` trên UI); copy live thân thiện người dùng; accordion **«Thông tin thêm»**; thêm mục `**outcomeKind` / `outcomeAbnormal` / `outcomeLabelVi`** và `**processTrace`** (phân loại bất thường + cây bước queue); persist BSON và ghi chú vận hành.
- **2026-04-08:** Rà soát Publish: bỏ tiền tố `G2 —` trong `summary` mốc consumer (`livecopy/queue.go`); quy ước `summary`/`uiTitle` không lặp giai đoạn khi đã có dòng tham chiếu quy trình + `e2e`*.
- **2026-04-07:** Tạo bản P01–P13 có S/E; bảng một dòng.
- **2026-04-07:** Chuyển sang **giai đoạn lớn G1–G6**, gom pha nhỏ cũ vào mapping; giữ bảng chi tiết theo Gx-Syy.
- **2026-04-07:** Rà soát với code emit: sửa `pipelineStage`/`eventSource` một số dòng (CRM compute/recompute, Ads recompute, `crm_intel_recomputed`, G2 không emit mới); thêm mục “Mức độ khớp code”.
- **2026-04-07:** Thêm `**eventtypes/e2e_reference.go`**: trace queue + Publish live + persist org-live đều gắn tham chiếu G1–G6 theo bảng này.
- **2026-04-07:** Mở rộng mục **Tham chiếu E2E trong code** (bảng trường/hàm, hai lớp envelope vs consumer G2); sửa `eventSource` `order_intel_recomputed` → `order_intel`; `flows/README` trỏ đủ e2e.
- **2026-04-07:** Rà soát nội dung Publish: `enrichPublishE2ERef` chèn dòng tham chiếu vào `DetailBullets`; `ResolveE2EForLivePhase` chi tiết theo `phase`; văn bản queue consumer + `ads` livecopy neo **G2** / **G5**.
- **2026-04-07:** Đồng bộ **docs-shared**: `unified-data-contract.md` mục **§2.5e**; `ai-context/folkform/api-context.md` **Version 4.14** + mục lục changelog.
- **2026-04-07:** **api-context Version 4.15** — khung nội dung `DecisionLiveEvent` (A/B/C); cập nhật mô tả `enrichPublishE2ERef` / `detailBullets` trong [4.14](../../docs-shared/ai-context/folkform/api-context.md#version-414).
- **2026-04-07:** Thêm mục **Khung nội dung timeline (trace & audit)** (A/B/C, quy ước field, ma trận nguồn mốc, checklist); cập nhật mô tả `DetailBullets` Publish (sau đó chuyển sang `**Trong quy trình: Gx-Syy — …`**) và `DetailSections` queue.
- **2026-04-07:** Rút gọn copy live (`livecopy`): bỏ trùng dòng E2E; **một** `DetailSections` «Chi tiết kỹ thuật» cho queue; engine/execute/ads/orchestrate gọn bullets — đồng bộ doc + api-context 4.15.

