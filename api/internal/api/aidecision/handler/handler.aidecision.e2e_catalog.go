package aidecisionhdl

import (
	"github.com/gofiber/fiber/v3"

	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/eventtypes"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"
)

// HandleE2EReferenceCatalog GET /ai-decision/e2e-reference-catalog — Bảng pha/bước G1–G6 + milestone consumer + map phase live (cho UI swimlane / tooltip).
// Mỗi bước có descriptionTechnicalVi (kỹ thuật) và descriptionUserVi (end-user); stages có userSummaryVi; queueMilestones có userLabelVi.
// G1-S04 trong steps là một dòng gom mọi miền enqueue L1 (schemaVersion ≥ 11); wire datachanged = <prefix>.changed (v14+); consumer Lookup tương thích *.inserted/*.updated cũ.
// stages G2 (v17+): G2-S01–S02 consumer; G2-S03–S05 worker miền merge + emit lại decision_events_queue (vd. crm_pending_merge → crmqueue); schemaVersion ≥20 mô tả vòng AID↔miền.
// schemaVersion ≥35: bỏ bước catalog G4-S04 — execute/propose gộp G4-S03-E01…E03; livePhase queued/propose/done/error → G4-S03 / G4-S03-E*; ≥34: bỏ catalog G4-S03 (message.batch_ready); G4-S04→G4-S03, G4-S05→G4-S04 (E01–E03); envelope message.batch_ready → resolver G2-S02; stages G4 — bốn bước (sau v35); ≥33: steps — G4-S02 một dòng (*.context_requested + *.context_ready); resolver e2eStepId G4-S02; ≥32: stages G4 — bốn bước (trước v34); ≥31: envelope <domain>_intel_recomputed → G4-S01 (AID nhận — case); G3-S06 catalog = phát từ miền; ≥30: G3-S02 xếp *_intel_compute; livePhase intel_domain_compute_done → G3-S05; ≥29: gộp một dòng intel_recomputed (resolver đổi theo v31); ≥28: G3-S01 l2_datachanged; ≥27: G2-S01 l1_datachanged; ≥26: G3 ba bước; ≥25: G3-S01 prefix; ≥24: G3 L2→intel; ≥23: G2-S05 l2_datachanged.
//
// Không phụ thuộc org; vẫn qua middleware đọc như các GET ai-decision khác. Nguồn: docs/flows/bang-pha-buoc-event-e2e.md §5.2–5.3 + eventtypes.ResolveE2EForLivePhase.
func HandleE2EReferenceCatalog(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "OK", "status": "success",
			"data": fiber.Map{
				"schemaVersion":   eventtypes.E2ECatalogSchemaVersion,
				"docRef":          "docs/flows/bang-pha-buoc-event-e2e.md §5.2–5.3",
				"stages":          eventtypes.E2EStageCatalog(),
				"steps":           eventtypes.E2EStepCatalog(),
				"queueMilestones": eventtypes.E2EQueueMilestoneCatalog(),
				"livePhaseMap":    decisionlive.E2ELivePhaseCatalog(),
			},
		})
		return nil
	})
}
