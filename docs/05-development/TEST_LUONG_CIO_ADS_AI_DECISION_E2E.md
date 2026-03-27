# Luồng đầy đủ: CIO → AI Decision → Ads Intelligence → Action → Executor → Learning

**Mục đích:** Mô tả pipeline event-driven khi **bot đồng bộ campaign qua CIO**, không dùng shortcut `POST /ads/actions/propose` trực tiếp.

---

## Sơ đồ (code hiện tại)

```
Agent / Bot
  → POST /api/v1/cio/ingest  (domain: meta_campaign, data.metaData = payload Meta)
  → MetaCampaign Upsert → meta_campaigns
  → OnDataChanged → decision_events_queue  (eventType: meta_campaign.inserted | .updated)

Worker AIDecisionConsumer
  → ProcessMetaCampaignDataChanged → case ads_optimization → emit ads.context_requested (lane batch)

Worker AIDecisionConsumer (cùng process)
  → ads.context_requested
  → BuildAdsIntelligenceContextPayloadFromDB (snapshot meta_campaigns.currentMetrics — raw, L1–L3, alertFlags)
  → emit ads.context_ready

Worker AIDecisionConsumer
  → ads.context_ready → UpdateCaseWithAdsContext
  → adsautop.RunAdsProposeFromContextReady (metasvc.ComputeFinalActionsFromCurrentMetrics — ACTION_RULE)
  → khi có action → emit executor.propose_requested (domain=ads; tương thích queue cũ ads.propose_requested)
  → processExecutorProposeRequested → ProposeForAds → action_pending (domain ads)

Executor
  → approve / reject / execute

Learning Engine
  → OnActionClosed → learning_case
```

---

## Worker bắt buộc

- **`ai_decision_consumer`** — xử lý toàn bộ: `meta_campaign.*`, `ads.context_requested`, `ads.context_ready`, `executor.propose_requested` (và alias `ads.propose_requested`), Ads recompute, CRM/CIX, v.v.

Không còn worker `ads_context` riêng.

---

## Script kiểm thử

```powershell
cd api-tests\scripts
.\test-cio-ads-ai-decision-e2e.ps1
```

Script gửi campaign qua `POST /cio/ingest`, đợi consumer, rồi kiểm tra `GET /ads/actions/pending`.

---

## Learning

- **Executor** xử lý duyệt/thực thi như luồng Ads hiện có.
- **Learning:** `OnActionClosed` → learning case khi action đóng.

---

## File code liên quan

- `api/internal/api/aidecision/hooks/datachanged.go` — emit `meta_campaign.*`
- `api/internal/api/aidecision/service/service.aidecision.ads_pipeline.go` — `ProcessMetaCampaignDataChanged`
- `api/internal/api/aidecision/service/service.aidecision.ads_context_payload.go` — snapshot Intelligence từ DB
- `api/internal/api/aidecision/adsautop/context_ready.go` — `RunAdsProposeFromContextReady`
- `api/internal/api/aidecision/worker/worker.aidecision.consumer.go` — dispatch event
- `api/internal/api/cio/handler/handler.cio.ingest.go` — `domain: meta_campaign`

---

**Ranh giới:** Rollup Intelligence (meta) chỉ ghi **alertFlags** + layers. **ACTION_RULE** (đề xuất PAUSE/DECREASE/…) do **metasvc.ComputeFinalActionsFromCurrentMetrics**, được gọi từ **adsautop** / consumer AID, không tính trong pipeline rollup.
