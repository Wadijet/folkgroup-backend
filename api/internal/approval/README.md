# Package approval — Bridge kết nối pkg/approval với app

Package **internal/approval** là bridge: delegate sang `pkg/approval` engine, inject Storage (MongoDB) và Notifier (notifytrigger).

## Cấu trúc

- **pkg/approval** — Package độc lập: interfaces Storage, Notifier, Executor; logic Propose/Approve/Reject/ListPending
- **internal/approval** — Bridge: init engine, implement Storage qua `bridge.MongoStorage`, Notifier qua `bridge.NotifytriggerNotifier`
- **internal/approval/bridge** — Storage + Notifier impl

## Phạm vi

- **Queue:** `action_pending_approval` (collection chung)
- **Domain:** Mỗi domain (ads, content, ...) đăng ký Executor và gọi Propose với domain tương ứng
- **API:** `/approval/actions/*` — propose, approve, reject, pending
- **Notification:** EventType theo domain (ví dụ ads → `ads_action_pending_approval`)

## Sử dụng

```go
// Init() gọi trong main sau InitRegistry
approval.Init()

// Domain ads đăng ký executor khi init (ads/executor.go)
approval.RegisterExecutor("ads", pkgapproval.ExecutorFunc(executeAdsAction))
approval.RegisterEventTypes("ads", map[string]string{"executed": "ads_action_executed", "rejected": "ads_action_rejected"})

// Khi có đề xuất
approval.Propose(ctx, "ads", approval.ProposeInput{...}, ownerOrgID, baseURL)
```

## Tách độc lập

- **pkg/approval** — Package thuần, không import meta, ads, global, notifytrigger. Có thể extract thành repo riêng.
- **internal/approval** — Bridge inject implementation vào app.
