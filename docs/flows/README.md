# Flows — Folkgroup Backend

**Mục đích:** Tài liệu về các luồng xử lý nghiệp vụ (approval, notification, CRM, v.v.).

---

## Tham Chiếu

- [Bảng giai đoạn — bước — sự kiện (E2E)](bang-pha-buoc-event-e2e.md) — G1–G6 (pha lớn), map `eventType` / `eventSource` / `pipelineStage`, **tham chiếu máy** `e2eStage` / `e2eStepId` (`eventtypes/e2e_reference.go`, queue + Publish + `decision_org_live_events`), **`outcomeKind` / `outcomeAbnormal` / `outcomeLabelVi`**, **`processTrace`** (consumer queue), copy timeline **«Trong quy trình: …»** + **«Thông tin thêm»**, và **khung nội dung timeline** (neo E2E / chứng cứ tra cứu / nghiệp vụ). **API JSON cho frontend:** GET `/v1/ai-decision/e2e-reference-catalog` — mục [§3.1](bang-pha-buoc-event-e2e.md#31-api-catalog-e2e-json-cho-frontend).
- [Approval workflow](../02-architecture/systems/) — Propose, approve, reject, execute
- [Notification processing](../02-architecture/systems/notification-processing-rules.md)
- [CRM logic](../02-architecture/business-logic/)
- [Endpoint workflow](../02-architecture/analysis/endpoint-workflow-general.md)
