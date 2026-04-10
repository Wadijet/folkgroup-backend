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
