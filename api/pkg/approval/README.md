# pkg/approval — Cơ chế duyệt độc lập

Package **hoàn toàn độc lập** với meta_ads và các domain khác. Có thể extract thành repo riêng.

## Phụ thuộc

- Chỉ `context`, `go.mongodb.org/mongo-driver/bson/primitive` (ObjectID)
- **Không** import: meta, ads, global, notifytrigger, logger

## Interfaces

- **Storage**: Insert, Update, FindById, FindPending
- **Notifier**: Notify(eventType, payload)
- **Executor**: Execute(doc) — mỗi domain đăng ký

## Sử dụng

App inject Storage + Notifier, gọi `approval.NewEngine(storage, notifier)`.
Domain đăng ký: `engine.RegisterExecutor(domain, ex)`, `engine.RegisterEventTypes(domain, types)`.
