# Module Ads — Sử dụng cơ chế duyệt

Module ads **dùng** package `approval` (cơ chế duyệt tách riêng). Không chứa logic queue/approve/reject.

## Phạm vi

- **Propose:** Wrapper gọi `approval.Propose(domain="ads", ...)` với payload ads-specific
- **Config:** `approvalConfig` trong `meta_ad_accounts`
- **Executor:** Đăng ký `approval.RegisterExecutor("ads", ...)` — thực thi khi approve (TODO: Meta API)

## API

| Route | Mô tả |
|-------|-------|
| POST /ads/actions/propose | Thêm đề xuất ads (gọi approval) |
| POST /ads/actions/approve | Duyệt (delegate approval) |
| POST /ads/actions/reject | Từ chối (delegate approval) |
| GET /ads/actions/pending | Danh sách pending domain=ads |
| GET/PUT /ads/config/approval | approvalConfig |

## Cơ chế duyệt

Logic queue, approve, reject nằm ở **`internal/approval`** — package độc lập, dùng chung cho ads, content, ...
