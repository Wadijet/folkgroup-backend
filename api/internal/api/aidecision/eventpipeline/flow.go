// Package eventpipeline: một nơi nạp module side-effect sau L1 + log contributor; không nhân bản bảng pha/bước E2E.
package eventpipeline

// Tham chiếu bảng chi tiết pha/bước (G1–G6, Gx-Syy) — nguồn chân lý trong code + doc:
//   - eventtypes.E2EStageCatalog, eventtypes.E2EStepCatalog
//   - GET /ai-decision/e2e-reference-catalog (handler.aidecision.e2e_catalog)
//   - docs/flows/bang-pha-buoc-event-e2e.md §5.2–5.3
//
// Package này chỉ bổ sung: gom _import các gói datachangedsidefx.Register (ensure.go) và log Snapshot() — tầng «sau khi job L1 vào consumer».

const (
	// DatachangedE2ECatalogPointer — dùng log / comment; không import eventtypes ở đây để tránh vòng phụ thuộc không cần thiết.
	DatachangedE2ECatalogPointer = "eventtypes.E2EStageCatalog + E2EStepCatalog; API GET /ai-decision/e2e-reference-catalog"
	DatachangedE2EDocPointer     = "docs/flows/bang-pha-buoc-event-e2e.md §5.2–5.3"
	// DatachangedSideEffectModuleLine — giữ trùng các dòng _ import trong ensure.go khi thêm miền.
	DatachangedSideEffectModuleLine = "crm, report, meta_ads, conversationintel (CIX), orderintel, aidecision/datachangedsidefx (CRM refresh defer)"
)
