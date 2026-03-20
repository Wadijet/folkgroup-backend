# Rule Intelligence — Kiến Trúc Tham Chiếu

**Mục đích:** Tài liệu kiến trúc chính thức (canonical) cho module Rule Intelligence — đơn vị biến đổi cốt lõi của pipeline trí tuệ, nằm trước Learning Engine.

**Liên quan:** [Learning Engine](learning-engine.md), [Activity Framework](activity-framework.md), [Ads Metrics Pipeline](ads-metrics-pipeline.md)

### Trạng thái triển khai (2025-03-16)

| Domain | Trạng thái | Vỏ tối thiểu |
|--------|------------|---------------|
| **Ads** | Đã chuyển xong | `ComputeAlertFlags`, `computeLayer1/2/3ViaRuleEngine`, `computeSuggestedActions` — gọi Rule Engine |
| **CRM** | Phase 1 đã chuyển | `GetClassificationFromCustomer`, `ComputeClassificationFromMetricsOrRuleEngine` — gọi RULE_CRM_CLASSIFICATION |

**Nguyên tắc vỏ tối thiểu:** Giữ tên hàm cũ làm vỏ, bên trong gọi Rule Engine. Logic thực sự nằm trong Logic Script. Không fallback — khi Rule Engine trả nil thì trả map rỗng / empty.

**Tài liệu dùng chung:** [rule-intelligence-overview.md](../../../../docs/ai-context/folkform/design/rule-intelligence/rule-intelligence-overview.md) — tổng quan, trạng thái migration (trong `docs/ai-context/folkform/design/rule-intelligence/`).

---

### Tham Chiếu Tài Liệu Logic (Ads Domain)

Khi extract Logic Script từ code hoặc thiết kế rule mới cho domain Ads, **bắt buộc tham khảo**:

| Tài liệu | Đường dẫn | Nội dung |
|----------|-----------|----------|
| **FolkForm AI Agent Master Rules v4.1** | `docs-shared/ai-context/folkform/design/ads-intelligence/FolkForm AI Agent Master Rules v4.1.md` | 13 rules chi tiết (SL-A/B/C/D/E/F, CHS Kill, Mess Trap, Morning On, Noon Cut, Safety Net, Trim, Kill Off, Night Off, Increase, Decrease, Reset, Mess Trap Guard, Volume Push, Throttle); KPI Target; MQS; CHS; Adaptive Threshold; Per-Camp; Counterfactual; Predictive; Mode; Momentum; Decision Tree; Patches v4.1 (Onboarding, Pancake Heartbeat, Mess Trap Event Override, Dual-source). **Source of truth cho business logic.** |
| **FolkForm n8n Workflow Architecture v4.1** | `docs-shared/ai-context/folkform/design/ads-intelligence/FolkForm n8n Workflow Architecture v4.1.md` | WF-03 Kill Engine, WF-04 Budget Engine — luồng workflow, thứ tự trigger, timing. |

**Lưu ý:** Nếu n8n Workflow doc chưa có trong repo, code tham chiếu tại `worker.ads.auto_propose.go`. Logic Script phải trùng khớp điều kiện và ngưỡng trong Master Rules.

---

**Mục lục:** [1. Rà Soát Logic](#1-rà-soát-logic-hiện-tại) · [2. Loại Rule](#2-định-nghĩa-loại-rule) · [3. Rule Definition](#3-rule-definition) · [4. Logic Script](#4-logic-script) · [5. Parameter Sets](#5-parameter-sets) · [6. Output Contract](#6-output-contract) · [7. Rule Engine](#7-rule-engine-design) · [8. Rule Registry](#8-rule-registry-architecture) · [9. Versioning](#9-chiến-lược-versioning) · [10. Trace](#10-hệ-thống-execution-trace) · [11. Theo Dõi](#11-theo-dõi-và-thống-kê) · [12. Learning Engine](#12-tích-hợp-với-decision-brain) · [13. Migration](#13-chiến-lược-migration) · [14. Developer](#14-hướng-dẫn-cho-developer) · [15. Triển Khai](#15-đề-xuất-triển-khai)

---

## Kiến Trúc Logic: Script-Only

Hệ thống áp dụng **Script-Only Logic Architecture**. Không có logic dựa trên expression (CEL, expr). Toàn bộ logic rule phải triển khai dưới dạng **Logic Script** có version.

**Lý do:**
- Kiến trúc thống nhất
- Hỗ trợ logic phức tạp, nhiều bước
- Dễ migrate từ logic service hiện có
- Traceability đầy đủ cho Learning Engine học

---

## Bốn Lớp Độc Lập

Rule execution tách thành **bốn lớp**, mỗi lớp version độc lập:

| Lớp | Trách nhiệm | Version |
|-----|-------------|---------|
| **Rule Definition** | Định nghĩa khi nào, ở đâu logic chạy — không chứa business logic | rule_version |
| **Logic Script** | Artifact reasoning — toàn bộ logic trong script. Mỗi rule 1 script, logic độc lập | logic_version |
| **Parameter Set** | Giá trị cấu hình có thể tune (ngưỡng, window, ...). Tách riêng để đánh giá param | param_version |
| **Output Contract** | Schema output, validation | output_version |

**Mục tiêu tách Logic vs Param:** Cho phép đánh giá từng rule theo 2 lớp — (1) Logic có đúng spec? (2) Param có tối ưu? Learning Engine / A/B test có thể phân tích riêng từng lớp.

---

## Tổng Quan Pipeline

```
Raw Data
  → Rule Transform
  → Metric Layer 1
  → Rule Transform
  → Metric Layer 2
  → Rule Transform
  → Metric Layer 3
  → Rule Transform
  → Signal / Flag / State
  → Rule Transform
  → Action / Recommendation
  → Outcome
  → Learning Engine (học từ outcome)
```

**Mỗi bước chuyển lớp đều do Rule thực hiện.** Rule không chỉ là alert hay automation — chúng là đơn vị biến đổi cốt lõi.

---

## Nguyên Tắc Thiết Kế: Mở & Mở Rộng

Rule Intelligence được thiết kế để **áp dụng cho nhiều module** (Ads, CRM, Content, Operational, ...) trong tương lai. Đầu vào và đầu ra phải **mở, không khóa cứng** theo logic hiện có.

| Nguyên tắc | Áp dụng |
|------------|---------|
| **Input/Output generic** | Schema dùng JSON Schema hoặc schema_ref — module tự định nghĩa, Rule Engine chỉ validate theo schema đã đăng ký |
| **Layers mở** | `from_layer`, `to_layer` là string — không enum cố định. VD: raw, layer1, flag, action (Ads) hoặc crm_profile, engagement_score, flow_trigger (CRM) |
| **Domain mở** | `domain` là string — ads, crm, content, operational, ... Module mới thêm domain mới |
| **Output type mở** | `output_type` là string — metric, flag, action, alert, recommendation, ... Module có thể đăng ký type mới |
| **Contract-based** | Module đăng ký input schema + output schema. Rule Engine giao tiếp qua contract, không biết chi tiết module |
| **Rule chỉ trả về output** | Rule Engine không thực thi, không gọi approval/notification. Module đối tác nhận output rồi tự tạo action, gửi alert, trigger flow, ... |
| **Logic = Logic Script** | 1 rule = 1 script. Không có expression-based logic. Script đọc input, params → trả output theo Output Contract. report = object (các field + log text) — căn cứ cho module |

### Phân Tách Trách Nhiệm: Rule Chỉ Trả Về, Module Thực Thi

| Thành phần | Trách nhiệm |
|------------|-------------|
| **Rule Intelligence** | Chạy Logic Script → **trả về output** (kết quả + metadata). Không tạo action, không gọi approval, không gửi notification. Thuần khiết: input → output. |
| **Module đối tác** (Ads, CRM, Content, ...) | Gọi Rule Engine với context → nhận output → **tự quyết định thực thi**: tạo action pending, gửi alert, trigger flow, cập nhật DB, ... |

**Ví dụ:** Rule tạo action chỉ trả về 1 object action cùng thông tin liên quan (actionType, value, reason, ruleCode). Việc tạo action đó (Propose), theo dõi (approval workflow), quản lý (ExecuteAdsAction) — **tất cả ở module Ads**. Rule không biết approval, Meta API, notifytrigger.

---

## 1. Rà Soát Logic Hiện Tại

### 1.1 Các Pattern Logic Đã Có Trong Hệ Thống

| Pattern | Vị trí hiện tại | Mô tả | Trạng thái |
|---------|-----------------|-------|------------|
| **Metric calculations** | `service.meta.evaluation` — `computeLayer1/2/3ViaRuleEngine` | CTR, ConvRate, ROAS, CHS, MQS, lifecycle, portfolioCell | ✅ Đã chuyển — RULE_ADS_LAYER1/2/3 |
| **Threshold detection** | `meta/service.ComputeAlertFlags` | 30 flags qua Logic Script | ✅ Đã chuyển — RULE_ADS_FLAG_* |
| **Anomaly detection** | Window Shopping Pattern | Pattern bất thường (mess cao, orders thấp) | ✅ Đã chuyển — RULE_ADS_FLAG_WINDOW_SHOPPING |
| **Scoring models** | Trong script Layer2 | Chuyển metric → điểm 0–100 | ✅ Đã chuyển — trong LOGIC_ADS_LAYER2 |
| **Classification rules** | Trong script Layer3 | Lifecycle × performanceTier → cell; CHS → healthState | ✅ Đã chuyển — trong LOGIC_ADS_LAYER3 |
| **CRM Classification** | `GetClassificationFromCustomer`, `ComputeClassificationFromMetricsOrRuleEngine` | valueTier, lifecycleStage, journeyStage, channel, loyaltyStage, momentumStage | ✅ Đã chuyển — RULE_CRM_CLASSIFICATION |
| **Automation triggers** | `computeSuggestedActions` | Flag → action (PAUSE, DECREASE, INCREASE) | ✅ Đã chuyển — Rule Engine |
| **Governance conditions** | Trong Logic Script | isLifecycleNew, noon cut, dual-source | ✅ Trong script |

### 1.2 Chuyển Đổi Logic Hiện Tại Thành Rule Cấu Trúc

| Logic hiện tại | Rule type | Logic Script | from_layer | to_layer |
|----------------|-----------|--------------|------------|----------|
| `convRate = orders / mess` | derivation | LOGIC_ADS_CONV_RATE | raw | layer1 |
| `roas = revenue / spend` | derivation | LOGIC_ADS_ROAS | raw | layer1 |
| `chs = (eff + demand + auction + sat + mom) / 5` | derivation | LOGIC_ADS_CHS | layer2 | layer3 |
| `spendPct > 0.20 AND runtimeMinutes > 90` | interpretation | LOGIC_ADS_SL_A | layer1 | flag |
| `healthState = critical` → flag `chs_critical` | interpretation | LOGIC_ADS_CHS_CRITICAL | layer3 | flag |
| `sl_a` → PAUSE | execution | LOGIC_ADS_KILL_POLICY | flag | action |
| `lifecycle == "NEW"` → skip propose | governance | LOGIC_ADS_LIFECYCLE_PRECONDITION | layer1 | precondition |

### 1.3 Lợi Ích Chuyển Đổi

- **Logic không hardcode** trong service — dễ thay đổi, AI hỗ trợ sinh rule
- **Traceability** — mỗi output có rule_id, logic_version, param_version
- **Auditability** — trace đầy đủ input, params, output, explanation
- **Tuning độc lập** — thay đổi ngưỡng không cần sửa logic

---

## 2. Định Nghĩa Loại Rule

### 2.1 Derivation Rules

**Chức năng:** Biến đổi raw data thành metrics. Logic trong Logic Script.

| Ví dụ | Input | Output | Logic Script |
|-------|-------|--------|--------------|
| CTR | impressions, clicks | ctr | LOGIC_ADS_CTR |
| ConvRate_7d | orders, mess | convRate_7d | LOGIC_ADS_CONV_RATE |
| MQS | mess, convRate, timeFactor | mqs_7d | LOGIC_ADS_MQS |
| Lifecycle | metaCreatedAt | lifecycle | LOGIC_ADS_LIFECYCLE |
| CHS | efficiency, demandQuality, ... | chs | LOGIC_ADS_CHS |

### 2.2 Interpretation Rules

**Chức năng:** Diễn giải metrics thành signal/flag.

| Ví dụ | Input | Output | Mô tả |
|-------|-------|--------|-------|
| sl_a | spendPct, runtimeMinutes, cpaMess, mess, mqs | flag `sl_a` | Stop Loss A — CH cao, runtime đủ |
| chs_critical | healthState | flag `chs_critical` | CHS critical |
| repeat_gap_risk | lastOrderAt, now | flag `repeat_gap_risk` | CRM: khoảng cách mua lặp bất thường |
| window_shopping_pattern | orders_2h, mess_2h, timeWindow | flag `window_shopping_pattern` | Pattern bất thường (Mess Trap) |

### 2.3 Execution Rules

**Chức năng:** Biến đổi signal/flag thành action/recommendation.

| Ví dụ | Input | Output | Mô tả |
|-------|-------|--------|-------|
| sl_a → PAUSE | flag `sl_a` | action PAUSE | Kill rule |
| chs_warning → DECREASE 15% | flags `chs_warning`, `cpa_mess_high` | action DECREASE | Decrease rule |
| mo_eligible → INCREASE 20% | flag `mo_eligible` | action INCREASE | Increase rule |
| trigger_follow_up | flag `repeat_gap_risk` | recommendation | CRM: trigger flow re-engagement |

### 2.4 Governance Rules

**Chức năng:** Ràng buộc hệ thống, điều kiện an toàn.

| Ví dụ | Mô tả |
|-------|-------|
| Min sample before evaluation | Campaign < 7 ngày → không đề xuất |
| Noon cut | INCREASE không chạy 12:00–14:30 |
| Dual-source confirm | Kill rule cần xác nhận Pancake + FB |

---

## 3. Rule Definition

**Rule Definition** định nghĩa **khi nào và ở đâu** logic chạy — **không chứa business logic**. Rule tham chiếu Logic Script, Parameter Set, Output Contract.

### 3.1 Schema Canonical

```json
{
  "rule_id": "RULE_ADS_KILL_CANDIDATE",
  "rule_version": 5,
  "rule_code": "sl_a",
  "domain": "ads",
  "from_layer": "flag",
  "to_layer": "action",
  "input_ref": {
    "schema_ref": "schema_ads_layer1",
    "required_fields": ["spendPct_7d", "runtimeMinutes", "cpaMess_7d", "mess", "mqs_7d", "lifecycle"]
  },
  "logic_ref": {
    "logic_id": "LOGIC_ADS_KILL_POLICY",
    "logic_version": 2
  },
  "param_ref": {
    "param_set_id": "PARAM_ADS_KILL_DEFAULT",
    "param_version": 3
  },
  "output_ref": {
    "output_id": "OUT_ACTION_CANDIDATE",
    "output_version": 1
  },
  "priority": 10,
  "status": "active",
  "metadata": {
    "label": "SL-A",
    "description": "Stop Loss A — CH cao, runtime đủ",
    "docReference": "FolkForm v4.1 Section 2.1"
  }
}
```

### 3.2 Giải Thích Từng Trường

| Trường | Mục đích |
|--------|----------|
| `rule_id` | ID duy nhất (UUID hoặc business ID) |
| `rule_version` | Version rule — tăng khi bind logic/param/output khác |
| `rule_code` | Mã nghiệp vụ (sl_a, chs_critical, ...) — dùng trong output, trace |
| `domain` | **Mở** — ads, crm, content, operational, ... |
| `from_layer` | **Mở** — Lớp đầu vào (string). VD: raw, layer1, flag (Ads); crm_profile (CRM) |
| `to_layer` | **Mở** — Lớp đầu ra (string). VD: layer1, flag, action (Ads); flow_trigger (CRM) |
| `input_ref` | Tham chiếu input schema — schema_ref, required_fields. Module đăng ký |
| `logic_ref` | Tham chiếu Logic Script (logic_id + logic_version) |
| `param_ref` | Tham chiếu Parameter Set (param_set_id + param_version) |
| `output_ref` | Tham chiếu Output Contract (output_id + output_version) |
| `priority` | Thứ tự ưu tiên (nhỏ = chạy trước) |
| `status` | active \| draft \| deprecated |
| `metadata` | Label, description, docReference |

**Lưu ý:** Điều kiện tiên quyết (preconditions) như lifecycle, noon cut — **đưa vào Logic Script**, không để trong Rule Definition.

### 3.3 Input/Output Contract — Giao Tiếp Với Module

Rule Engine **không hardcode** schema theo domain. Thay vào đó:

1. **Schema Registry:** Lưu schema do module đăng ký (JSON Schema hoặc schema_ref).
2. **Input contract:** Module cung cấp context (entity_ref, layers, params_override). Rule Engine validate theo `input_ref.schema_ref` nếu có.
3. **Output contract:** Rule Engine **trả về** output (result, entity_ref, rule_id, trace_id). Module nhận output rồi **tự thực thi** — Rule Engine không gọi handler, không tạo action, không gửi notification.
4. **Module thực thi:** Module Ads nhận output action → Propose(), approval, ExecuteAdsAction. Module Ads nhận output alert → SendAdsAlert. Module CRM nhận output recommendation → trigger flow. Mỗi module tự quyết định.

---

## 4. Logic Script

**Logic Script** là artifact reasoning có version. Toàn bộ logic rule triển khai trong script. Script không trigger action thực tế — chỉ trả output theo Output Contract.

### 4.1 Schema Logic Script

```json
{
  "logic_id": "LOGIC_ADS_KILL_POLICY",
  "logic_version": 2,
  "logic_type": "script",
  "runtime": "goja",
  "entry_function": "evaluate",
  "source_hash": "sha256:abc123...",
  "change_reason": "Thêm điều kiện noon cut",
  "status": "active",
  "script": "
    function evaluate(ctx) {
      var input = ctx.layers.layer1 || {};
      var params = ctx.params || {};
      var report = { input: input, params: params, log: '' };
      
      if (input.lifecycle === 'NEW') {
        report.result = 'filtered';
        report.log = '1. Lifecycle: filtered — Campaign NEW (< 7 ngày)';
        return { output: null, report: report };
      }
      report.log = '1. Lifecycle: passed (' + input.lifecycle + ')';
      
      var cond = input.spendPct_7d > params.th_spendPctBase && input.runtimeMinutes > params.th_runtimeMin &&
                 input.cpaMess_7d > params.th_cpaMessKill && (input.mess || 0) < params.th_messMax && (input.mqs_7d || 0) < params.th_mqsMin;
      if (!cond) {
        report.result = 'no_match';
        report.log += '\\n2. Điều kiện sl_a: no match';
        return { output: null, report: report };
      }
      report.result = 'match';
      report.log += '\\n2. Điều kiện sl_a: match\\n3. Kết quả: PAUSE sl_a';
      
      var action = { action_code: 'PAUSE', ruleCode: 'sl_a', reason: 'Hệ thống đề xuất [SL-A]: CPA mess cao...', value: null };
      return { output: action, report: report };
    }
  ",
  "metadata": {
    "metricsUsed": ["spendPct_7d", "runtimeMinutes", "cpaMess_7d", "mess", "mqs_7d"],
    "paramKeys": ["th_spendPctBase", "th_runtimeMin", "th_cpaMessKill", "th_messMax", "th_mqsMin"]
  }
}
```

### 4.2 Giải Thích Trường Logic Script

| Trường | Mục đích |
|--------|----------|
| `logic_id` | ID logic (có thể có nhiều version) |
| `logic_version` | Version — tăng mỗi lần script thay đổi |
| `logic_type` | Luôn `"script"` — không có expression-based |
| `runtime` | `goja` hoặc tương đương — JavaScript sandbox |
| `entry_function` | Tên hàm entry point (vd: `evaluate`) |
| `source_hash` | Hash source — phục vụ audit, cache invalidation |
| `change_reason` | Lý do thay đổi — phục vụ traceability |
| `status` | active \| draft \| deprecated |
| `metadata` | metricsUsed, paramKeys — validation, docs |

### 4.3 Quy Ước Script

| Quy ước | Mô tả |
|---------|-------|
| **Chỉ đọc** | Script chỉ đọc `ctx` (input) và `ctx.params` (params). Không ghi DB, không gọi API, không I/O |
| **Input** | `ctx.layers` — layers từ context; `ctx.entity_ref` — entity_ref |
| **Params** | `ctx.params` — parameters từ param_ref |
| **Return** | `{ output: ... }` hoặc `{ output: null }`. Output phải theo Output Contract |
| **Report** | Script **bắt buộc** trả về `report` — object (các field + log text). Xem 4.4 |

### 4.4 Báo Cáo (report) — Bắt Buộc

Script **phải** trả về `report` — **object** gồm:
- Các field lưu lại toàn bộ thành phần của rule (input snapshot, params dùng, kết quả từng bước, ...) — structure do script tự định nghĩa
- Field `log` — chuỗi text, script tự append trong code (kiểu báo cáo công việc). Cần thêm gì thì script thêm

**Cấu trúc gợi ý:**
```json
{
  "input": { "spendPct_7d": 0.25, "runtimeMinutes": 120, "lifecycle": "CALIBRATED", ... },
  "params": { "th_spendPctBase": 0.20, "th_runtimeMin": 90, ... },
  "result": "match",
  "log": "1. Lifecycle: passed (CALIBRATED)\n2. Điều kiện sl_a: match (spendPct=0.25 > 0.20)\n3. Noon cut: bỏ qua\n4. Dual-source: passed\n5. Kết quả: PAUSE sl_a"
}
```

- Các field khác (input, params, result, ...) — script tự quyết định, lưu gì cần cho audit
- `log` — **bắt buộc**, string. Script tự append trong code: `report.log += "..."` hoặc build array rồi `join("\n")`

**Khi filtered:**
```json
{
  "input": { "lifecycle": "NEW", ... },
  "result": "filtered",
  "log": "1. Lifecycle: filtered — Campaign NEW (< 7 ngày), không đề xuất"
}
```

Script return: `{ output: null, report: {...} }` hoặc `{ output: {...}, report: {...} }`

### 4.5 Sandbox & Bảo Mật

- **Runtime:** goja (Go) hoặc tương đương — JavaScript subset, sandbox
- **Timeout:** 100ms mặc định
- **Memory limit:** 10MB
- **Whitelist:** Chỉ `ctx`, `ctx.layers`, `ctx.params`, `ctx.entity_ref`. Không `fetch`, `require`, `eval`, `Function`, `global`
- **Không I/O:** Script không được gọi API, đọc file, ghi DB

### 4.6 Nguyên Tắc Thiết Kế Logic Script

| Nguyên tắc | Mô tả |
|------------|-------|
| **Deterministic** | Cùng input + params → cùng output. Không dùng random, Date.now() cho logic quyết định |
| **No side effects** | Script không sửa state bên ngoài — không ghi DB, gọi API, emit event |
| **Context-aware** | Script có thể đánh giá nhiều signal, flag, layer — logic phức tạp nằm trong script |
| **Structured reasoning** | Script **bắt buộc** trả `report` object — input snapshot, params, result, log text |
| **Versioned evolution** | Mỗi thay đổi logic → tạo `logic_version` mới. Không sửa script tại chỗ |

---

## 5. Parameter Sets

### 5.1 Cấu Trúc Parameter Set

Parameter Set định nghĩa giá trị cấu hình có thể tune — version độc lập với Logic Script.

```json
{
  "param_set_id": "PARAM_ADS_KILL_DEFAULT",
  "param_version": 3,
  "parameters": {
    "th_spendPctBase": 0.20,
    "th_runtimeMin": 90,
    "th_cpaMessKill": 180000,
    "th_messMax": 3,
    "th_mqsMin": 1
  },
  "domain": "ads",
  "segment": "default",
  "metadata": {
    "label": "Stop Loss — Default",
    "updatedAt": 1700000000000,
    "updatedBy": "system"
  }
}
```

### 5.2 Giải Thích

| Trường | Mục đích |
|--------|----------|
| `param_set_id` | ID bộ tham số |
| `param_version` | Version — tăng khi giá trị thay đổi |
| `parameters` | Map key → value (số, chuỗi, boolean). VD: thresholds, cooldown windows, min sample size, sensitivity multipliers |
| `domain` | ads, crm, ... |
| `segment` | default, org_xxx, campaign_xxx (Per-Camp Adaptive) |
| `metadata` | Label, updatedAt, updatedBy |

### 5.3 Tuning Không Sửa Logic

- Thay đổi `parameters` → tăng `param_version`
- Logic Script giữ nguyên → `logic_version` không đổi
- Rule bind `param_ref` → khi chạy dùng param_version theo cấu hình

---

## 6. Output Contract

**Output Contract** định nghĩa schema output mà Logic Script phải tuân thủ. Output phải theo đúng schema đã đăng ký.

### 6.1 Nguyên Tắc: Output Mở

Output Contract **không khóa cứng** theo domain. Mỗi module đăng ký output schema cho `(domain, output_type)`. **Rule Engine chỉ validate theo schema và trả về output** — không gọi handler, không thực thi. Module đối tác nhận output rồi tự xử lý.

### 6.2 Schema Output Contract

```json
{
  "output_id": "OUT_ACTION_CANDIDATE",
  "output_version": 1,
  "output_type": "action",
  "schema_definition": {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "properties": {
      "action_code": { "type": "string", "enum": ["PAUSE", "DECREASE", "INCREASE", "RESUME", "ARCHIVE"] },
      "severity": { "type": "string", "enum": ["critical", "warning", "info"] },
      "eligibility": { "type": "boolean" },
      "reason": { "type": "string" },
      "confidence": { "type": "number", "minimum": 0, "maximum": 1 },
      "recommendation": { "type": "string" }
    }
  },
  "required_fields": ["action_code", "reason"],
  "validation_rules": [
    "output != null => action_code in allowed_values"
  ]
}
```

### 6.3 Trường Output Thường Dùng

| Trường | Mô tả |
|--------|-------|
| `action_code` | Mã hành động (PAUSE, DECREASE, INCREASE, ...) |
| `severity` | Mức độ (critical, warning, info) |
| `eligibility` | Có đủ điều kiện thực thi không |
| `reason` | Lý do đề xuất |
| `confidence` | Độ tin cậy 0–1 |
| `recommendation` | Gợi ý bổ sung |

Script **bắt buộc** tuân thủ schema đã định nghĩa. Rule Engine validate trước khi trả về.

### 6.4 Các Loại Output — Ví Dụ (không giới hạn)

| output_type | Mô tả | Nguồn code | output_schema |
|-------------|-------|------------|---------------|
| **metric** | Object metrics (layer1, layer2, layer3) | `computeLayer1/2/3` | `{ type: "object", properties: {...} }` |
| **flag** | Mã cờ (sl_a, chs_critical, ...) | `alertFlags`, `EvaluateFlags` | `{ type: "string", allowed_values: [...] }` |
| **state** | Trạng thái (healthState, portfolioCell, lifecycle) | `computeLayer3`, `derivePortfolioCell` | `{ type: "string", enum: [...] }` |
| **signal** | Điểm số, risk (0–100) | scoring, risk_score | `{ type: "number", min: 0, max: 100 }` |
| **action** | Hành động cần thực thi (qua approval) | Ads: EvaluateAlertFlags, EvaluateForDecrease | Xem 6.6 |
| **alert** | Cảnh báo — chỉ gửi notification, không execute | Ads: CB-3, CB-4, Predictive Trend | Xem 6.7 |
| **recommendation** | Đề xuất flow (CRM, content) | trigger_follow_up (tương lai) | `{ type: "object", flowId, payload }` |

### 6.5 Input Contract — Module Cung Cấp Context

Rule Engine nhận **context** từ module gọi. Cấu trúc **mở** — module định nghĩa layers theo domain:

```json
{
  "entity_ref": {
    "domain": "ads",
    "objectType": "campaign",
    "objectId": "123",
    "ownerOrganizationId": "..."
  },
  "layers": {
    "raw": { ... },
    "layer1": { "spendPct_7d": 0.25, "runtimeMinutes": 120, ... },
    "layer2": { ... },
    "layer3": { "chs": 45, "healthState": "critical", ... },
    "flag": ["sl_a"]
  },
  "params_override": {}
}
```

- `layers`: **Mở** — module định nghĩa key. Ads: raw, layer1, layer2, layer3, flag. CRM: crm_profile, engagement_score, last_order_at, ... Rule Engine đọc `layers[from_layer]`.
- `entity_ref`: Bắt buộc — trace và để module đối tác biết target khi nhận output.

### 6.6 Action Output — Ví Dụ (Ads)

**Nguồn:** `dto.ads.action`, `service.ads.executor`, `ads/rules/engine` — **chỉ là ví dụ**. Module khác định nghĩa action schema riêng.

| actionType | value | Mô tả | Executor |
|------------|-------|-------|----------|
| KILL, PAUSE | — | Tạm dừng (status=PAUSED) | Meta API Post status |
| RESUME | — | Bật lại (status=ACTIVE) | Meta API Post status |
| ARCHIVE | — | Lưu trữ (status=ARCHIVED) | Meta API Post status |
| DELETE | — | Xóa (status=DELETED) | Meta API Post status |
| INCREASE | % (vd: 30) | Tăng budget theo % | Lấy budget hiện tại × (1 + %/100) |
| DECREASE | % (vd: 20) | Giảm budget theo % | Lấy budget hiện tại × (1 - %/100) |
| SET_BUDGET | cent | Đặt daily_budget tuyệt đối | Meta API Post daily_budget |
| SET_LIFETIME_BUDGET | cent | Đặt lifetime_budget | Meta API Post lifetime_budget |
| SET_NAME | string | Đổi tên entity | Meta API Post name |

**Schema action output (tương thích action_code):**
```json
{
  "action_code": "PAUSE",
  "actionType": "PAUSE",
  "value": 20,
  "reason": "Hệ thống đề xuất [SL-A]: CPA mess cao...",
  "ruleCode": "sl_a",
  "label": "SL-A: CPA Mess + MQS",
  "result_check": {
    "afterHours": 4,
    "source": "siblings",
    "fields": ["cr", "orders"]
  }
}
```

**result_check (tùy chọn):** Cấu hình check kết quả sau khi thực thi — định nghĩa trong Rule (Param Set). Module Ads copy vào ActionPending.Payload; Counterfactual đọc để biết sau bao lâu check, lấy field nào.

| Trường | Mô tả |
|--------|-------|
| `afterHours` | Số giờ sau khi thực thi mới đánh giá (vd: 4) |
| `source` | Nguồn data: `siblings` (campaign anh em), `self`, ... |
| `fields` | Các field cần lấy: `cr`, `orders`, ... |

### 6.7 Alert Output — Ví Dụ (Ads)

**Nguồn:** `service.ads.circuit_breaker`, `service.ads.predictive_trend` — **chỉ là ví dụ**. Module CRM/Content có thể có alert type riêng.

| eventType | Mô tả | Có PAUSE? |
|-----------|-------|-----------|
| ads_circuit_breaker_alert | CB-1, CB-2: PAUSE + Alert; CB-3, CB-4: Alert only | CB-1/2: có; CB-3/4: không |
| ads_predictive_trend_alert | Dự báo freq, CPM, CPA, CR decay | Không |
| ads_pancake_down | Pancake không có order 2h | Không |
| ads_pancake_suspect | FB Mess cao, Pancake 0 đơn | Không |
| ads_momentum_alert | Momentum thay đổi | Không |
| ads_chs_kill | CHS Kill đã execute | Không |

**Schema alert output:**
```json
{
  "eventType": "ads_circuit_breaker_alert",
  "payload": {
    "code": "CB-3",
    "message": "Zero delivery 30p...",
    "adAccountId": "...",
    "ownerOrganizationId": "..."
  }
}
```

**Module Ads xử lý output (Rule không làm):**
- **action** → Module Ads nhận output → Propose() → approval workflow → ExecuteAdsAction → Meta API
- **alert** → Module Ads nhận output → SendAdsAlert → notifytrigger → delivery

### 6.8 Refactoring Rule Ads: RULE_ADS_KILL_CANDIDATE

Ví dụ đầy đủ cách bốn lớp tương tác — Rule Definition, Logic Script, Parameter Set, Output Contract.

| Thành phần | ID | Mô tả |
|------------|-----|-------|
| **Rule** | RULE_ADS_KILL_CANDIDATE | Khi flag sl_a → đề xuất PAUSE. Chỉ tham chiếu, không chứa logic |
| **Logic Script** | LOGIC_ADS_KILL_POLICY | Script đánh giá lifecycle, điều kiện sl_a, noon cut, dual-source → trả action hoặc null |
| **Parameter Set** | PARAM_ADS_KILL_DEFAULT | Ngưỡng: th_spendPctBase, th_runtimeMin, th_cpaMessKill, th_messMax, th_mqsMin |
| **Output Contract** | OUT_ACTION_CANDIDATE | Schema: action_code, reason, severity, ... |

**Luồng tương tác:**
1. Rule Engine load RULE_ADS_KILL_CANDIDATE → resolve logic_ref → LOGIC_ADS_KILL_POLICY v2
2. Resolve param_ref → PARAM_ADS_KILL_DEFAULT v3 → bind vào ctx.params
3. Resolve output_ref → OUT_ACTION_CANDIDATE v1 → validate output
4. Chạy script với ctx → script return { output, report }
5. Validate output theo OUT_ACTION_CANDIDATE → ghi trace → trả về module Ads

**Logic hiện tại (service.ads):** `EvaluateAlertFlagsWithConfig`, `ShouldAutoPropose` → **chuyển vào** LOGIC_ADS_KILL_POLICY. Ngưỡng hardcode → **chuyển vào** PARAM_ADS_KILL_DEFAULT.

### 6.9 Định Dạng Output Rule Engine Trả Về

Rule Engine **chỉ trả về** object sau. Module đối tác nhận rồi tự xử lý. **Output bắt buộc đi kèm report (object)** — căn cứ và tài liệu cho module.

```json
{
  "output_type": "action",
  "result": {
    "action_code": "PAUSE",
    "value": null,
    "reason": "Hệ thống đề xuất [SL-A]: CPA mess cao...",
    "ruleCode": "sl_a",
    "label": "SL-A: CPA Mess + MQS"
  },
  "report": {
    "input": { "spendPct_7d": 0.25, "runtimeMinutes": 120, "lifecycle": "CALIBRATED", ... },
    "params": { "th_spendPctBase": 0.20, "th_runtimeMin": 90, ... },
    "result": "match",
    "log": "1. Lifecycle: passed (CALIBRATED)\n2. Điều kiện sl_a: match (spendPct=0.25 > 0.20)\n3. Noon cut: bỏ qua\n4. Dual-source: passed\n5. Kết quả: PAUSE sl_a"
  },
  "entity_ref": {
    "domain": "ads",
    "objectType": "campaign",
    "objectId": "123",
    "ownerOrganizationId": "..."
  },
  "rule_id": "RULE_ADS_KILL_CANDIDATE",
  "rule_code": "sl_a",
  "trace_id": "trace_xxx",
  "logic_id": "LOGIC_ADS_KILL_POLICY",
  "logic_version": 2,
  "param_set_id": "PARAM_ADS_KILL_DEFAULT",
  "param_version": 3
}
```

| Trường | Mục đích |
|--------|----------|
| `result` | Kết quả rule (theo output_schema). Có thể null nếu rule không trigger. |
| `report` | **Bắt buộc** — Object: các field lưu thành phần rule (input, params, result, ...) + `log` (text). Script tự định nghĩa structure, tự append vào log. |
| `entity_ref` | Copy từ context — module biết target. |
| `trace_id` | Để audit, debug, liên kết với Learning Engine. |

**Module đối tác dùng `report` để:**
- `report.log` — hiển thị "Tại sao đề xuất này?" trong UI
- `report.input`, `report.params` — audit, debug
- Lưu toàn bộ report vào `actionDebugReport` khi ghi activity

**Link từ đề xuất đến rule log:**
- Module Ads truyền `trace_id` từ Rule Engine vào action → `ProposeInput.TraceID` → `action_pending_approval.payload.traceId`
- UI hiển thị link "Xem log tạo đề xuất" → gọi `GET /rule-intelligence/logs/:traceId` để xem chi tiết (input_snapshot, explanation.log, output_object)

---

## 7. Rule Engine Design

### 7.1 Luồng Thực Thi

```
1. Rule Loading
   - Load rule theo domain, from_layer, status=active
   - Resolve logic_ref, param_ref, output_ref

2. Dependency Resolution
   - Xác định thứ tự rule (from_layer → to_layer)
   - Derivation: raw→L1 → L1→L2 → L2→L3
   - Interpretation: L3→flag
   - Execution: flag→action

3. Input Resolution
   - Lấy input từ context (layers, entity_ref)
   - Validate theo input_ref
   - Bind parameters từ param_ref vào ctx.params

4. Script Execution
   - Tạo ctx = { layers, params, entity_ref }
   - Chạy script JavaScript (goja) với ctx
   - Timeout 100ms, memory limit
   - Script return { output, report }

5. Output Validation
   - Kiểm tra script có trả về report (bắt buộc)
   - Validate output theo Output Contract (output_ref) nếu output != null
   - Reject nếu invalid

6. Trace Logging
   - Ghi rule_execution_logs với input_snapshot, params_snapshot, output, report, timestamp

7. Return
   - Trả về { result, report, entity_ref, rule_id, trace_id, logic_version, param_version } cho module gọi
   - Module đối tác nhận output + report → tự thực thi (tạo action, gửi alert, ...); dùng report làm căn cứ/tài liệu
```

### 7.2 Sơ Đồ Luồng

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│ Rule        │     │ Input        │     │ Param       │
│ Registry    │────▶│ Resolver     │────▶│ Binder       │
└─────────────┘     └──────────────┘     └─────────────┘
                           │                     │
                           ▼                     ▼
                    ┌──────────────────────────────┐
                    │ Script Executor (JavaScript)  │
                    │ ctx = { layers, params }     │
                    │ return { output, report }     │
                    └──────────────────────────────┘
                                    │
                                    ▼
                    ┌──────────────────────────────┐
                    │ Output Validator (schema)     │
                    │ + report (bắt buộc)           │
                    └──────────────────────────────┘
                                    │
                                    ▼
                    ┌──────────────────────────────┐
                    │ Trace Logger                 │
                    └──────────────────────────────┘
                                    │
                                    ▼
                    ┌──────────────────────────────┐
                    │ Return output → Module gọi    │
                    │ (Module tự thực thi)          │
                    └──────────────────────────────┘
```

---

## 8. Rule Registry Architecture

### 8.1 Các Thành Phần Nội Bộ

| Thành phần | Trách nhiệm | Storage |
|------------|-------------|---------|
| **Rule Registry** | Lưu rule definitions, bind logic/param/output | `rule_definitions` |
| **Logic Registry** | Lưu Logic Script definitions (script, version) | `rule_logic_definitions` |
| **Parameter Registry** | Lưu param sets (domain, segment, version) | `rule_param_sets` |
| **Output Registry** | Lưu output definitions (schema, mapping) | `rule_output_definitions` |
| **Schema Registry** | **Mở** — Lưu input/output schema do module đăng ký (JSON Schema) | `rule_schemas` |
| **Rule Execution Logs** | Trace mỗi lần chạy rule (execution_time, execution_status, error_message, explanation) | `rule_execution_logs` |
| **Rule Execution Stats** | Thống kê aggregate (total_runs, error_count, avg_duration_ms, ...) — optional | `rule_execution_stats` |
| **Rule Change Logs** | Lịch sử thay đổi rule/logic/param | `rule_change_logs` |

### 8.2 Sơ Đồ Phụ Thuộc

```
Rule Registry
  ├── logic_ref → Logic Registry
  ├── param_ref → Parameter Registry
  └── output_ref → Output Registry
        └── schema_definition → Schema Registry

Rule Engine
  ├── Read: Rule Registry, Logic, Param, Output, Schema Registry
  ├── Return: output (result, entity_ref, trace_id, ...)
  └── Write: Rule Execution Logs

Module (Ads, CRM, Content, ...)
  ├── Đăng ký: Schema (input/output)
  ├── Gọi: Rule Engine với context
  └── Nhận output → tự thực thi (Propose, SendAlert, trigger flow, ...)
```

---

## 9. Chiến Lược Versioning

### 9.1 Bốn Loại Version

| Version | Khi tăng | Ví dụ |
|---------|----------|-------|
| `logic_version` | script thay đổi | Sửa logic sl_a |
| `param_version` | Ngưỡng, multiplier, time window | th_spendPctBase 0.20 → 0.25 |
| `output_version` | output_schema, mapping, severity | Thêm field explanation |
| `rule_version` | Rule bind logic/param khác, scope, metadata | Rule dùng logic v2 |

### 9.2 Hỗ Trợ Debugging & Rollback

- **Debugging:** Trace chứa logic_version, param_version → biết chính xác logic/param đã dùng
- **Rollback:** Có thể revert param_version hoặc rule_version mà không cần deploy code
- **Root cause:** So sánh trace giữa 2 thời điểm khi outcome khác nhau

### 9.3 Rule Evolution

- Logic mới → tạo logic_id mới hoặc tăng logic_version
- Param mới → tạo param_set mới hoặc tăng param_version
- Rule mới → tạo rule mới, status=draft → test → active

---

## 10. Hệ Thống Execution Trace

Hệ thống ghi **full rule execution trace** cho mỗi lần chạy — phục vụ debugging, audit, observability, Learning Engine learning.

### 10.1 Cấu Trúc Trace

```json
{
  "trace_id": "trace_xxx",
  "rule_id": "RULE_ADS_KILL_CANDIDATE",
  "rule_version": 5,
  "logic_id": "LOGIC_ADS_KILL_POLICY",
  "logic_version": 2,
  "param_set_id": "PARAM_ADS_KILL_DEFAULT",
  "param_version": 3,
  "input_snapshot": {
    "spendPct_7d": 0.25,
    "runtimeMinutes": 120,
    "lifecycle": "CALIBRATED"
  },
  "parameters_snapshot": {
    "th_spendPctBase": 0.20,
    "th_runtimeMin": 90
  },
  "output_object": { "action_code": "PAUSE", "ruleCode": "sl_a", ... },
  "execution_status": "success",
  "explanation": {
    "input": {...},
    "params": {...},
    "result": "match",
    "log": "1. Lifecycle: passed\n2. Điều kiện sl_a: match\n3. Kết quả: PAUSE sl_a"
  },
  "execution_time": 12,
  "timestamp": 1700000000000,
  "entity_ref": {
    "domain": "ads",
    "objectType": "campaign",
    "objectId": "123",
    "ownerOrganizationId": "..."
  }
}
```

| Trường | Mục đích |
|--------|----------|
| `rule_id`, `rule_version` | Rule đã chạy |
| `logic_id`, `logic_version` | Logic Script đã dùng |
| `param_set_id`, `param_version` | Parameter Set đã dùng |
| `input_snapshot` | Snapshot input tại thời điểm chạy |
| `parameters_snapshot` | Snapshot params đã bind |
| `output_object` | Output trả về (null nếu không trigger) |
| `execution_status` | success \| error \| timeout |
| `error_message` | Khi lỗi — message (script crash, timeout, ...) |
| `explanation` | Report từ script — reasoning, log |
| `execution_time` | Thời gian chạy (ms) |
| `timestamp` | Thời điểm chạy |

### 10.2 Hỗ Trợ Auditing, Debugging, Learning

| Mục đích | Dữ liệu dùng |
|----------|--------------|
| **Auditing** | rule_id, rule_version, input_snapshot, output_object, explanation, timestamp |
| **Debugging** | parameters_snapshot, explanation |
| **Learning** | Trace + Outcome (từ Learning Engine) → phân tích rule có hiệu quả không |
| **Decision analysis** | So sánh trace với decision case (Context, Choice, Outcome) |

### 10.3 API Xem Log Theo trace_id

| Method | Path | Mô tả |
|--------|------|-------|
| GET | `/rule-intelligence/logs/:traceId` | Lấy rule execution log theo trace_id — dùng cho link "Xem log tạo đề xuất" từ proposal |

**Auth:** `MetaAdAccount.Read` + OrganizationContext. Chỉ trả log khi `entity_ref.ownerOrganizationId` trùng org hiện tại.

**Response:** Document từ `rule_execution_logs` (input_snapshot, parameters_snapshot, output_object, explanation, execution_status, ...).

---

## 11. Theo Dõi và Thống Kê

Hệ thống theo dõi và thống kê việc chạy từng rule để quản lý, phát hiện vấn đề, tối ưu hiệu năng.

### 11.1 Nguồn Dữ Liệu

- **rule_execution_logs** — mỗi lần chạy rule ghi 1 document
- **Aggregation** — aggregate theo rule_id, domain, khoảng thời gian

### 11.2 Các Chỉ Số Theo Dõi (per rule, per domain, per org)

| Chỉ số | Mô tả | Cách tính |
|--------|-------|-----------|
| **Số lần chạy** | Tổng số lần rule được gọi | count(trace_id) |
| **Số lần thành công** | execution_status = success | count where status=success |
| **Số lần lỗi** | execution_status = error | count where status=error |
| **Tỷ lệ lỗi** | % lỗi | errors / total × 100 |
| **Thời gian chạy** | execution_time mỗi lần (ms) | Rule Engine ghi vào trace (section 10.1) |
| **Thời gian TB** | avg duration_ms | avg(execution_time) từ trace |
| **Thời gian P95/P99** | Phân vị 95, 99 | percentile(execution_time, 95/99) |
| **Số lần timeout** | Script vượt timeout | count where status=timeout |
| **Số lần có output** | output != null (rule trigger) | count where output_object != null |
| **Tỷ lệ trigger** | % lần có output | trigger_count / total × 100 |

### 11.3 Cấu Trúc Trace Bổ Sung (cho monitoring)

Trace (section 10.1) đã có `execution_time`, `execution_status`. Cần ghi thêm khi lỗi:

```json
{
  "execution_time": 12,
  "execution_status": "success|error|timeout",
  "error_message": "..."
}
```

- `execution_time`: Thời gian chạy script (ms). Tương đương duration_ms trong stats.
- `execution_status`: success | error | timeout
- `error_message`: Khi error — message lỗi (script crash, timeout, ...)

### 11.4 Collection / View Thống Kê

**Option A: Aggregate on-demand** — Query rule_execution_logs với aggregation pipeline theo time range.

**Option B: rule_execution_stats** — Collection lưu thống kê theo window (vd: 1h, 1d). Worker hoặc trigger sau mỗi batch cập nhật.

```json
{
  "rule_id": "rule_001",
  "domain": "ads",
  "owner_organization_id": "...",
  "window": "1d",
  "window_start": "2025-03-13T00:00:00Z",
  "total_runs": 1523,
  "success_count": 1500,
  "error_count": 20,
  "timeout_count": 3,
  "trigger_count": 45,
  "avg_duration_ms": 8.5,
  "p95_duration_ms": 15,
  "p99_duration_ms": 45,
  "last_run_at": 1700000000000,
  "last_error_at": 1699999000000,
  "last_error_message": "script timeout"
}
```

### 11.5 Dashboard / API Quản Lý

- **API:** GET `/rule-intelligence/stats?rule_id=&domain=&from=&to=&window=1d`
- **Dashboard:** Bảng rule, cột: total_runs, error_rate, avg_duration_ms, trigger_rate, last_error
- **Alert:** Khi error_rate > X% hoặc avg_duration_ms > Y — gửi notification

### 11.6 Các Thông Tin Quan Trọng Khác

| Thông tin | Mục đích |
|-----------|----------|
| **last_run_at** | Rule có còn được gọi không |
| **last_error_at, last_error_message** | Debug lỗi gần nhất |
| **trigger_count / total** | Rule có quá nhạy (trigger nhiều) hay quá chặt (ít trigger) |
| **duration trend** | Script có chậm dần (memory leak, logic phức tạp thêm) |
| **error by domain/org** | Lỗi tập trung ở org nào — config, data đặc thù |

---

## 12. Tích Hợp Với Learning Engine

### 12.1 Phân Công Trách Nhiệm

| Module | Trách nhiệm | Thời điểm |
|--------|-------------|-----------|
| **Rule Intelligence** | Eval rule, trả về output (metric, flag, action, ...). Không thực thi | Khi module gọi |
| **Module đối tác** (Ads, CRM, ...) | Nhận output, thực thi (Propose, SendAlert, trigger flow, ...) | Sau khi nhận output |
| **Learning Engine** | Phân tích outcome sau khi entity đóng | Sau khi lifecycle closed |

### 12.2 Luồng Tích Hợp

```
Rule Intelligence (chỉ trả về output)
  - Metrics → Flags → Actions (output object)
  - Return output cho module gọi — không propose, không execute

Module Ads (nhận output, tự thực thi)
  - Nhận output action → Propose() → approval workflow → ExecuteAdsAction
  - Entity (ActionPending) đóng: executed / rejected / failed

Learning Engine
  - BuildDecisionCaseFromAction(doc)
  - Lưu: Context, Choice, Goal, Outcome, Lesson
  - AI retrieval, clustering, learning
```

### 12.3 Learning Engine Đề Xuất Cải Thiện Rule

- Learning Engine phân tích: rule X trigger → outcome success/partial/failed
- Có thể đề xuất: "Rule sl_a có tỷ lệ false positive cao → cân nhắc tăng th_spendPctBase"
- Hoặc: "Rule chs_critical khi CHS yesterday healthy → có thể data anomaly, giữ exception"
- Các đề xuất này có thể feed vào Rule Intelligence (param tuning, logic evolution)

---

## 13. Chiến Lược Migration

Logic hiện tại trong service sẽ migrate sang Script-Only Architecture.

**Yêu cầu bắt buộc:** Sau khi triển khai module Rule Intelligence, phải **rà soát toàn bộ codebase** để tìm và thay thế các chỗ gọi logic cũ (service, flag_evaluator, EvaluateAlertFlagsWithConfig, computeLayer1/2/3, ...) bằng Rule Engine. Không để logic song song — mọi biến đổi lớp (raw→metric→flag→action) phải đi qua Rule Engine.

### 13.0 Phương Án Tổng Thể — Logic Hiện Có → Logic Script

**Nguyên tắc:** Script phải **replicate chính xác** logic trong code. Không tạo script mới — extract logic hiện có.

#### 13.0.1 Ánh Xạ Logic Hiện Tại

| Logic hiện tại | Vị trí code | Input | Output | Script cần tạo |
|----------------|-------------|-------|--------|----------------|
| **Metrics → Flags** | `ads/rules/flag_evaluator.EvaluateFlags` | raw, layer1, layer2, layer3, FactsContext | flags[] | LOGIC_ADS_FLAG_EVALUATOR |
| **Flags → Kill (PAUSE)** | `ads/rules/engine.EvaluateAlertFlagsWithConfig` | flags, EvalOptions, cfg | RuleResult (PAUSE) | LOGIC_ADS_KILL_ENGINE |
| **Flags → Decrease** | `ads/rules/engine.EvaluateForDecreaseWithConfig` | flags, cfg | RuleResult (DECREASE) | LOGIC_ADS_DECREASE_ENGINE |
| **Flags → Increase** | `ads/rules/engine.EvaluateForIncrease` | flags, cfg | RuleResult (INCREASE) | LOGIC_ADS_INCREASE_ENGINE |
| **Flags → Resume** | `ads/rules/engine.EvaluateForResume` | flags, cfg | RuleResult (RESUME) | LOGIC_ADS_RESUME_ENGINE |
| **raw → layer1** | `meta/service.computeLayer1` | raw | layer1 | LOGIC_ADS_LAYER1 |
| **layer1 → layer2** | `meta/service.computeLayer2` | raw, layer1 | layer2 | LOGIC_ADS_LAYER2 |
| **layer2 → layer3** | `meta/service.computeLayer3` | layer1, layer2 | layer3 | LOGIC_ADS_LAYER3 |

#### 13.0.2 Per-Rule Logic Độc Lập (Khuyến Nghị)

**Nguyên tắc:** Mỗi rule = 1 Logic Script riêng, logic **tương đối độc lập** theo logic hiện có trong code. Mục tiêu: **đánh giá từng rule theo 2 lớp — logic và param**.

| Lớp | Nội dung | Đánh giá |
|-----|----------|----------|
| **Logic** | Điều kiện, flow, business rule (vd: sl_a = spendPct > X AND runtimeMinutes > Y AND ...) | Logic có đúng không? Có match spec không? |
| **Param** | Ngưỡng, window, multiplier (th_spendPctBase, th_runtimeMin, th_cpaMessKill, ...) | Ngưỡng có tối ưu không? Tuning hiệu quả? |

**Ví dụ:** RULE_ADS_KILL_SL_A → LOGIC_ADS_KILL_SL_A (logic sl_a) + PARAM_ADS_KILL_SL_A (thresholds). Đánh giá: (1) Logic sl_a có đúng spec? (2) Param sl_a có tối ưu cho outcome?

- **LOGIC_ADS_KILL_SL_A:** Logic sl_a — spendPct > th_spendPctBase, runtimeMinutes > th_runtimeMin, cpaMess > th_cpaMessKill, mess < th_messMax, mqs < th_mqsMin. Đọc thresholds từ `params`.
- **LOGIC_ADS_KILL_SL_B, LOGIC_ADS_KILL_CHS_CRITICAL, LOGIC_ADS_KILL_MESS_TRAP_SUSPECT, ...** — mỗi rule 1 script, logic độc lập.
- **LOGIC_ADS_FLAG_SL_A, LOGIC_ADS_FLAG_SL_B, ...** — metrics → flag, mỗi flag 1 script nếu logic khác nhau.

**Lợi ích:** Tách bạch logic vs param → Learning Engine / A/B test có thể đánh giá: "Rule sl_a logic đúng nhưng param chưa tối ưu" hoặc "Rule sl_b logic cần sửa".

#### 13.0.3 Ràng Buộc Quan Trọng

| Ràng buộc | Giải pháp |
|-----------|-----------|
| **Script không gọi DB** | `GetAdaptiveThreshold` (Per-Camp) — caller pre-compute, truyền thresholds vào `params`. `DetectWindowShoppingPattern` — caller gọi trước, truyền `window_shopping_pattern` vào layers.flag nếu có. |
| **Script không dùng timezone phức tạp** | `inTrimWindow`, `IsBefore1400Vietnam` — caller tính trước, truyền vào layers hoặc params. |
| **Config từ ads_meta_config** | Module Ads sync config → Parameter Set khi có thay đổi. Hoặc Rule Engine resolve param_ref → load từ collection rule_param_sets (đã sync từ ads_meta_config). |

#### 13.0.4 Thứ Tự Tạo Script (Per-Rule)

Mỗi rule = 1 script. Extract logic tương ứng từ code:

1. **Kill rules:** LOGIC_ADS_KILL_SL_A, LOGIC_ADS_KILL_SL_B, LOGIC_ADS_KILL_CHS_CRITICAL, LOGIC_ADS_KILL_MESS_TRAP_SUSPECT, ... — từ `EvaluateAlertFlagsWithConfig` + KillRules config.
2. **Decrease rules:** LOGIC_ADS_DECREASE_xxx — từ `EvaluateForDecreaseWithConfig`.
3. **Increase rules:** LOGIC_ADS_INCREASE_xxx — từ `EvaluateForIncrease`.
4. **Resume:** LOGIC_ADS_RESUME_MORNING_ON — từ `EvaluateForResume`.
5. **Flag rules:** LOGIC_ADS_FLAG_SL_A, LOGIC_ADS_FLAG_SL_B, ... — từ `EvaluateFlags` + FlagDefinitions (mỗi flag có logic riêng → 1 script).
6. **Derivation:** LOGIC_ADS_LAYER1/2/3 — từ `computeLayer1/2/3` (nếu migrate).

#### 13.0.5 Luồng Caller (Module Ads) Khi Gọi Rule Engine

```
1. Load campaign + currentMetrics + ads_meta_config
2. Pre-compute: adaptive thresholds (nếu có), window_shopping_pattern, inTrimWindow
3. Build context: layers = { raw, layer1, layer2, layer3, flag: alertFlags }
4. Sync config → Parameter Set (killRules, flagDefinitions, thresholds) — hoặc dùng param_set_id đã sync
5. Gọi Rule Engine: Run(rule_id, domain, entity_ref, layers, params_override)
6. Nhận output → Propose action
```

#### 13.0.6 Lộ Trình Thực Tế

| Giai đoạn | Việc làm |
|-----------|----------|
| **Phase 1** | LOGIC_ADS_KILL_SL_A + PARAM_ADS_KILL_SL_A — validate pipeline. Seed + tích hợp computeSuggestedActions. |
| **Phase 2** | Các kill rule còn lại (sl_b, chs_critical, mess_trap_suspect, ...). Decrease, Increase, Resume rules. |
| **Phase 3** | Flag rules (LOGIC_ADS_FLAG_SL_A, ...). Cập nhật computeAlertFlags. |
| **Phase 4** | LOGIC_ADS_LAYER1/2/3 (optional). |

### 13.1 Nguyên Tắc Migration

| Bước | Mô tả |
|------|-------|
| **Extract logic** | Logic trong `service.meta.evaluation`, `ads/rules/flag_evaluator`, `EvaluateAlertFlagsWithConfig` → extract thành Logic Script |
| **Rule Definition** | Tạo Rule Definition tham chiếu logic_ref, param_ref, output_ref — không chứa business logic |
| **Parameter Set** | Ngưỡng, window, multiplier hiện hardcode → chuyển vào Parameter Set versioned |
| **Output Contract** | Output hiện trả về ad-hoc → đăng ký Output Contract với schema_definition, required_fields |

### 13.2 Thứ Tự Migration

1. **Phase 1:** 1–2 rule Ads (vd: sl_a) → Logic Script + Rule Definition + Param Set + Output Contract
2. **Phase 2:** Rà soát codebase → FlagDefinitions → Interpretation Rules; ActionRules → Execution Rules. Cập nhật call site: thay gọi trực tiếp service bằng gọi Rule Engine.
3. **Phase 3:** computeLayer1/2/3 → Derivation Rules (hoặc wrap qua Rule Engine)
4. **Phase 4:** Governance rules (min sample, noon cut, dual-source)

### 13.3 Rủi Ro và Giảm Thiểu

- **Logic khác biệt:** So sánh output Rule Engine vs service cũ trên cùng input — regression test
- **Performance:** Script timeout 100ms — logic phức tạp cần tối ưu hoặc tách rule

---

## 14. Hướng Dẫn Cho Developer

Khi thêm rule mới, developer phải tuân thủ quy trình sau.

### 14.1 Quy Trình Thêm Rule

| Bước | Hành động |
|------|-----------|
| 1 | **Viết Logic Script** — logic trong script, không trong Rule Definition |
| 2 | **Định nghĩa Parameter Set** — tách riêng thresholds, window, multiplier |
| 3 | **Đăng ký Output Contract** — schema_definition, required_fields, validation_rules |
| 4 | **Tạo Rule Definition** — tham chiếu logic_ref, param_ref, output_ref |
| 5 | **Trả report** — script bắt buộc return { output, report } với report.log |
| 6 | **Tăng version** — mỗi thay đổi logic → logic_version mới; thay đổi params → param_version mới |

### 14.2 Khởi tạo Handler

- **Tạo handler trong router `Register()`**, không dùng `init()`. RuleEngineService, RuleDefinitionService, ... dùng `RegistryCollections` — collection chỉ đăng ký sau `InitRegistry()` trong `main()`. Handler được import trước `main()` nên `init()` chạy quá sớm → panic. Xem [api-structure.md](../../.cursor/rules/api-structure.md).

### 14.3 Checklist

- [ ] Script deterministic, không side effects
- [ ] Output theo đúng Output Contract
- [ ] Report có log text
- [ ] Parameters trong Parameter Set, không hardcode trong script
- [ ] Rule Definition chỉ tham chiếu, không chứa logic

---

## 15. Đề Xuất Triển Khai

### 15.1 Phase 1 — Foundation

1. **Collections:** rule_definitions, rule_logic_definitions, rule_param_sets, rule_output_definitions, rule_execution_logs (execution_time, execution_status, error_message, explanation)
2. **Rule Engine core:** Load rule, resolve logic_ref/param_ref/output_ref, chạy Logic Script (goja), validate output, trace (bao gồm explanation)
3. **Migration:** Chuyển 1–2 rule Ads (vd: sl_a) sang Rule Intelligence schema, validate

### 15.2 Phase 2 — Migration

1. **Rà soát codebase** — tìm call site gọi logic cũ (`EvaluateAlertFlagsWithConfig`, `EvaluateFlags`, `computeLayer1/2/3`, ...)
2. Chuyển FlagDefinitions → Interpretation Rules (Logic Script); ActionRules → Execution Rules
3. **Cập nhật call site** — thay gọi trực tiếp service bằng gọi Rule Engine
4. Chuyển computeLayer1/2/3 → Derivation Rules (hoặc wrap qua Rule Engine)

### 15.3 Phase 3 — Mở Rộng

1. CRM domain: repeat_gap_risk, trigger_follow_up
2. Governance rules: min sample, noon cut, dual-source
3. AI-assisted rule generation
4. **Theo dõi & thống kê:** rule_execution_stats, API stats, dashboard, alert khi error_rate cao hoặc duration tăng

---

## Changelog

- 2025-03-17: **Link đề xuất → rule log** — trace_id truyền từ Rule Engine qua action → ProposeInput → action_pending_approval.payload.traceId; thêm API GET `/rule-intelligence/logs/:traceId` để xem log; section 10.3 API Xem Log
- 2025-03-16: **Bỏ toàn bộ fallback** — Layer1/2/3: dùng empty map khi Rule Engine nil; computeSuggestedActions: không fallback adsrules; window_shopping: bỏ DetectWindowShoppingPattern fallback; formatFlagsDetail: đơn giản hóa (LogicText từ FlagDefinitions); diagnose_auto_propose: dùng ComputeActionsFromMetrics (Rule Engine).
- 2025-03-16: **Migration Derivation Rule Layer1, Layer2** — RULE_ADS_LAYER1 (raw→layer1), RULE_ADS_LAYER2 (raw+layer1→layer2). Logic script trong seed_rule_ads_system.go.
- 2025-03-16: **Migration Derivation Rule Layer3** — RULE_ADS_LAYER3 (layer1+layer2 → chs, healthState, portfolioCell).
- 2025-03-16: **Migration Interpretation Rules** — 30 flags chuyển sang Rule Engine (RULE_ADS_FLAG_*). Scheduler dùng metasvc.ComputeAlertFlags thay EvaluateFlags. Export ComputeAlertFlags cho scheduler.
- 2025-03-15: Refactor — xóa FindOne dư thừa trong RuleDefinitionService, LogicScriptService, ParamSetService, OutputContractService (dùng BaseServiceMongoImpl.FindOne kế thừa)
- 2025-03-13: Rà soát tài liệu — sửa input_ref, đánh số section 4, thêm rule_code, error_message vào trace, align Phase 2 migration, thêm mục lục
- 2025-03-13: **Script-Only Logic Architecture** — Kiến trúc canonical: bốn lớp (Rule Definition, Logic Script, Parameter Set, Output Contract); loại bỏ expression-based; thêm Script Design Guidelines, Migration Strategy, Developer Guidelines; ví dụ RULE_ADS_KILL_CANDIDATE
- 2025-03-13: Thêm section 11 — Theo dõi và thống kê (duration, error, trigger rate, ...)
- 2025-03-13: report = object (các field + log text). 1 rule = 1 script, logic phức tạp trong code
- 2025-03-13: Quyết định triển khai JavaScript script; output bắt buộc đi kèm report
- 2025-03-13: Tạo đề xuất kiến trúc Rule Intelligence ban đầu
