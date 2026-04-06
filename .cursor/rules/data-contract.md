---
description: "Hợp đồng dữ liệu thống nhất (Unified Data Contract) — 4 lớp ID, event, action, decision, outcome"
alwaysApply: true
---

# Unified Data Contract — Tiêu Chuẩn Khi Code Backend

**Nguồn đầy đủ (bắt buộc đọc khi thêm/sửa model, DTO, event, queue, API liên module):** [unified-data-contract.md](../../docs-shared/architecture/data-contract/unified-data-contract.md)

## Khi nào phải mở tài liệu gốc

- Thêm hoặc đổi **định danh** (ID công khai, join key, mapping đa nguồn).
- Thiết kế **payload** API / message / `decision_events_queue` / hook cross-module.
- Thêm **event_type**, **action_type**, **Decision Event**, **Outcome** / learning input.
- Luồng **Rule Engine** (input ctx, output, trace log).
- Luồng **AI Decision queue** — envelope `traceId` / `correlationId`, debounce, case runtime, CIX `pipelineRuleTraceIds` (xem doc gốc **§2.5b** + backend **NGUYEN_TAC §9**).
- **W3C Trace Context** — `w3cTraceId` (32 hex) neo với `traceId`, span trên timeline, `traceparent` (xem doc gốc **§2.5c** + api-context **4.10**).
- **Case vs trace / audit / `DecisionLiveEvent.detailBullets`** — không 1-1 `decisionCaseId`↔`traceId`; list phiên nghiệp vụ; API read-only (xem **§2.5d** + api-context **4.11**).

**Đặt tên hệ thống (`uid`, `links`, module, collection, worker, route, env):** [uid-field-naming.md](../../docs-shared/architecture/data-contract/uid-field-naming.md) — prefix **khớp `utility/uid.go`**; collection **khớp `global.vars.go` / init** khi thêm mới.

## 1. Bốn lớp định danh / liên kết — trọng tâm contract (§1.5)

Mọi entity cần **phân tách đúng 4 lớp**, không gộp một field làm đủ vai trò:

| # | Lớp | Field (gợi ý contract) | Vai trò | Quy tắc khi code |
|---|-----|--------------------------|---------|-------------------|
| **(1)** | **Storage** | `_id` (MongoDB `ObjectID`) | Persistence, index, query nội bộ | **Cấm** đưa `_id` ra response API / event công khai / tài liệu tích hợp đối tác. |
| **(2)** | **Canonical** | `uid` hoặc ID có prefix chuẩn (`cust_`, `evt_`, `dec_`, …) | Một định danh công khai duy nhất trong hệ — join, tham chiếu cross-module | API và message **ưu tiên** lớp này; generate qua `utility.GenerateUID` / pattern đã khóa trong doc. |
| **(3)** | **External IDs** | `sourceIds` (map theo nguồn) | “Tôi là ai ở hệ ngoài” — merge, reconcile | Mọi ID từ POS/FB/Zalo/… **vào map**, resolve → `uid`; không dùng ID ngoài làm khóa chính thay `uid`. |
| **(4)** | **Relationship links** | `links` (hoặc struct tương đương: `entityRefs`, …) | “Tôi nối tới ai” — quan hệ giữa entity | Tham chiếu nên có **uid** rõ ràng; khi chỉ có ID ngoài thì dùng **`externalRefs`** (hoặc tương đương) để không link “mù”. |

**Một câu nhớ:** (1) chỉ DB nội bộ — (2) là mặt tiền — (3) là passport nguồn ngoài — (4) là đồ thị quan hệ.

**Entity đa nguồn (§1.6):** bắt buộc có **(2) + (3)**; tra cứu thống nhất qua resolve (`ResolveUnifiedId` / `ResolveByAnyIdentifier` — pattern CRM/Auth trong doc).

**Hai lớp persistence (§1.7):** **L1-persist (mirror)** = ingest theo nguồn; **L2-persist (canonical)** = đã merge (tương tác liên module). Cùng **bốn vai trò field** có thể dùng trên mirror và canonical nhưng kỳ vọng khác: **`uid` contract chính trên canonical**; mirror có thể có **`links` mirror→mirror** để merge suy ra **`links` canonical→canonical**. `_id` mirror ≠ `_id` canonical.

**Không nhầm:** Ký hiệu **L1/L2/L3** trong **pipeline rule CIX** hoặc trường BSON CRM **`layer1`/`layer2`** hoặc **Ads `computeLayer1/2/3`** là **khác hoàn toàn** với L1-persist/L2-persist ở đây — xem [KHUNG_KHUON_MODULE_INTELLIGENCE.md](../../docs/05-development/KHUNG_KHUON_MODULE_INTELLIGENCE.md) mục 0.

**Lưu ý backend thực tế:** `OwnerOrganizationID` (`ObjectID`) thuộc **lớp (1)** cho tenant; nếu contract yêu cầu `org_id` **chuỗi công khai** (`org_*`), đó là **lớp (2)** — không tự động coi `.Hex()` của ObjectID là đủ thay `org_id` contract nếu doc quy định prefix.

## 2. Định danh toàn cục (áp cho lớp Canonical & ID có prefix)

- Format: `{prefix}_{unique_part}` — prefix 3–5 ký tự lowercase; phần unique 12–24 ký tự alphanumeric.
- Bảng prefix chuẩn (rút gọn): `org_`, `cust_`, `sess_`, `evt_`, `ord_`, `trace_`, `corr_`, `dec_`, `act_`, `exe_`; rule có thể dùng dạng `RULE_*` theo doc.
- **Base context** (khi có thể): `org_id`, `trace_id`, `timestamp` (ISO 8601).

## 3. Join keys chuẩn

Dùng đúng cặp field theo bảng doc (session ↔ customer qua `customer_id`, event ↔ session qua `session_id`, order ↔ customer, execution ↔ action/decision, v.v.). **Không** đặt tên field tùy ý thay thế các key đã khóa nếu đang trong phạm vi contract.

## 4. Event

- **Event base** (mọi event): `event_id`, `event_type`, `org_id`, `timestamp`, `trace_id`, khi cần `correlation_id`, `source`.
- **event_type** chuẩn: dùng các giá trị đã liệt kê trong doc (`interaction`, `delivery_result`, `decision_made`, …) — không phát minh trùng ý nghĩa với tên khác nếu đã có chuẩn.
- **Decision Event** (AI Decision): `eventType` dạng `domain.event_name`, `source`, `entityRefs`, `payload`, `timestamp`. Flags / intelligence → emit như Decision Event khi thuộc phạm vi quyết định.

## 5. Decision Events Queue (`decision_events_queue`)

- Domain collection là **source of truth**; queue chỉ mang **tín hiệu quyết định** + metadata correlate (`event_id`, `event_type`, `event_source`, `entity_*`, `payload` tối giản, `trace_id` / `correlation_id`).
- Tuân [NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md](../../docs/05-development/NGUYEN_TAC_LUONG_CRUD_DATACHANGED_AI_DECISION.md) — không tách side-effect khỏi luồng đã quy định.

## 6. Action & Execution

- **Một** schema Action duy nhất — không tách proposed/final như mô hình cũ đã bỏ trong doc.
- **action_type** chỉ dùng enum chuẩn (`SEND_MESSAGE`, `UPDATE_AD`, …) hoặc mở rộng có cập nhật doc + backward compatibility có chủ đích.
- **Execution result**: `execution_id`, `action_id`, `org_id`, `status` (`SUCCESS` | `FAILED` | `RETRYING`), trace fields; bám schema doc.

## 7. Rule Engine

- Input: `domain`, `rule_id`, `entity_ref`, `layers`, `params`, `trace_id`, …
- Output bắt buộc có **`output`** và **`report.log`**; mỗi lần chạy ghi trace/log theo contract (xem §4 doc).

## 8. Decision packet & trace

- Decision output: `decision_id`, `decision_mode` (`rule` | `llm` | `hybrid`), `selected_strategy`, `selected_actions`, `trace_id`, `correlation_id`, … theo §5 doc.
- Xâu chuỗi audit qua **`trace_id`**; **`correlation_id`** cho lifecycle entity.

## 9. Outcome & Learning

- Outcome record: `outcome_id`, `org_id`, `entity_type` / `entity_id`, liên kết `decision_id` / `trace_id` / `correlation_id`, `outcome`, `outcome_recorded_at`, `source` — theo §6 doc.
- Learning input: tổng hợp `decision_snapshot` + `outcome_snapshot` + lifecycle end — không làm mất các trường bắt buộc đã khóa.

## Checklist nhanh trước khi merge

- [ ] **4 lớp:** Đã tách rõ storage vs canonical vs sourceIds vs links/entityRefs — không dùng một field làm hai vai trò.
- [ ] **(1)** `_id` / `ObjectID` không xuất hiện trên contract công khai.
- [ ] **(2)** ID hiển thị / gửi đi có prefix chuẩn hoặc `uid` đúng nghĩa canonical.
- [ ] **(3)** ID nguồn ngoài nằm trong `sourceIds` (hoặc map tương đương), có đường resolve.
- [ ] **(4)** Quan hệ cross-entity có uid hoặc externalRefs rõ ràng.
- [ ] Payload có `org_id` và trace khi luồng yêu cầu audit (theo base context / event envelope).
- [ ] Event / queue field naming và semantics khớp doc (hoặc doc đã được cập nhật đồng bộ).
- [ ] Action không phân tán thành nhiều entity conflict với §3 doc.
- [ ] Rule output có `output` + `report.log`.
