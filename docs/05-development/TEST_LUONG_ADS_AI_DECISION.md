# Test luồng Ads → AI Decision (tối giản — POST /ads/actions/propose)

**Mục đích:** Khép nhanh qua **`POST /ads/actions/propose`** (không qua CIO).

**Luồng đầy đủ CIO → Ads Intelligence → …:** xem [TEST_LUONG_CIO_ADS_AI_DECISION_E2E.md](./TEST_LUONG_CIO_ADS_AI_DECISION_E2E.md) và script `test-cio-ads-ai-decision-e2e.ps1`.

---

**Mục đích (bản tối giản):** Khép một vòng **đơn giản nhất** trên domain **Ads** — không cần CIX, không cần conversation.

## Luồng logic

```
POST /api/v1/ads/actions/propose  (action PAUSE + campaignId + adAccountId)
    → adssvc.Propose → EmitAdsProposeRequest → EmitExecutorProposeRequest (domain=ads)
    → decision_events_queue  (eventType = executor.propose_requested, lane = fast)
    → Worker AIDecisionConsumer  (processExecutorProposeRequested)
    → ProposeForAds → approval.Propose (domain = ads)
    → Đề xuất chờ duyệt (pending)
```

**Xác minh:** `GET /api/v1/ads/actions/pending` — thấy bản ghi **PAUSE** tương ứng `campaignId` (sau vài giây).

## Điều kiện chạy

| Điều kiện | Ghi chú |
|-----------|---------|
| API + MongoDB | Server đang chạy |
| Worker **`aidecision_consumer`** | Phải **active** — nếu tắt, event nằm trong queue, không có pending |
| Dữ liệu Meta | Ít nhất **một** campaign trong `meta_campaigns` (đúng org) — lấy `campaignId`, `adAccountId` |
| Quyền JWT | `MetaAdAccount.Update` (propose), `MetaCampaign.Read` (nếu script tự lấy campaign), `MetaAdAccount.Read` (pending) |

## Chạy script (PowerShell)

```powershell
cd api-tests\scripts
.\test-ads-aidecision-flow.ps1
```

Tùy chọn:

```powershell
$env:TEST_API_BASE_URL = "http://localhost:8080/api/v1"
$env:TEST_ADS_CAMPAIGN_ID = "<campaign_id_meta>"
$env:TEST_ADS_ACCOUNT_ID = "act_xxx"
$env:TEST_ADMIN_TOKEN = "<jwt>"
.\test-ads-aidecision-flow.ps1
```

Nếu không set `TEST_ADS_*`, script thử `GET /meta/campaign/find-with-pagination?limit=1` với filter rỗng.

## Worker bị tắt?

`GET /api/v1/system/worker-config` — tìm **`aidecision_consumer`** / **`WorkerAIDecisionConsumer`** và bật (hoặc chỉnh env tương ứng theo tài liệu worker).

## Tại sao chọn PAUSE?

- `actionType` hỗ trợ sẵn, **không** cần `value` như SET_BUDGET.
- Cần ít nhất một trong `campaignId` / `adSetId` / `adId` — với campaign là đủ.

---

**Tham chiếu code:** `api/internal/api/ads/service/service.ads.propose.go` (`Propose`), `api/internal/api/aidecision/service/service.aidecision.emit_propose.go` (`EmitExecutorProposeRequest`), `api/internal/api/aidecision/worker/worker.aidecision.consumer.go` (`processExecutorProposeRequested`).
