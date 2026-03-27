# Quy Tắc Backend Cho AI — 09-ai-context

Thư mục chứa **quy tắc thiết kế backend** cho AI khi làm việc với folkgroup-backend.

**Khác với `docs-shared/ai-context/`** (API contract, design cross-repo). 09-ai-context = conventions, patterns, bảng quy tắc nội bộ backend.

## 📌 Nội dung

- **Quy tắc đầy đủ:** [`.cursor/rules/folkgroup-backend.mdc`](../../.cursor/rules/folkgroup-backend.mdc) — Cursor tự áp dụng
- **[BANG_QUY_TAC_THIET_KE_HE_THONG.md](./BANG_QUY_TAC_THIET_KE_HE_THONG.md)** — Bảng "khi cần gì đọc tài liệu nào"
- [Handler pattern CRUD vs custom](./handler-pattern-crud-vs-custom.md)

## 📍 API Contract (Shared)

API contract và design cross-repo: [docs-shared/ai-context/folkform/api-context.md](../../docs-shared/ai-context/folkform/api-context.md)

- **Trace W3C (`w3cTraceId`, span, timeline):** [api-context — Version 4.10](../../docs-shared/ai-context/folkform/api-context.md#version-410); [unified-data-contract — §2.5c](../../docs-shared/architecture/data-contract/unified-data-contract.md#contract-25c-w3c-trace).
- **Case vs trace / API audit (list case, queue, org-live persist):** [api-context — Version 4.11](../../docs-shared/ai-context/folkform/api-context.md#version-411); [unified-data-contract — §2.5d](../../docs-shared/architecture/data-contract/unified-data-contract.md#contract-25d-case-trace-audit).

---

**Cập nhật:** 2026-03-26
