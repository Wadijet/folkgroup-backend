// Package metahdl - Handler cho Meta Campaign.
package metahdl

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"

	basehdl "meta_commerce/internal/api/base/handler"
	metadto "meta_commerce/internal/api/meta/dto"
	metamodels "meta_commerce/internal/api/meta/models"
	metasvc "meta_commerce/internal/api/meta/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/utility"
)

// MetaCampaignHandler xử lý request Meta Campaign.
type MetaCampaignHandler struct {
	*basehdl.BaseHandler[metamodels.MetaCampaign, metadto.MetaCampaignCreateInput, metadto.MetaCampaignUpdateInput]
	MetaCampaignService *metasvc.MetaCampaignService
}

// NewMetaCampaignHandler tạo MetaCampaignHandler.
func NewMetaCampaignHandler() (*MetaCampaignHandler, error) {
	svc, err := metasvc.NewMetaCampaignService()
	if err != nil {
		return nil, fmt.Errorf("tạo MetaCampaignService: %w", err)
	}
	return &MetaCampaignHandler{
		BaseHandler:         basehdl.NewBaseHandler[metamodels.MetaCampaign, metadto.MetaCampaignCreateInput, metadto.MetaCampaignUpdateInput](svc),
		MetaCampaignService: svc,
	}, nil
}

// HandleSyncUpsertOne xử lý sync-upsert: nhận metaData từ Meta API, upsert vào meta_campaigns.
func (h *MetaCampaignHandler) HandleSyncUpsertOne(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input metadto.MetaSyncUpsertInput
		if err := c.Bind().JSON(&input); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Dữ liệu gửi lên không đúng định dạng JSON", "status": "error",
			})
			return nil
		}
		if input.MetaData == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "metaData không được để trống", "status": "error",
			})
			return nil
		}
		orgID := resolveOwnerOrgIDFromCtx(c, input.OwnerOrganizationID)
		now := time.Now().UnixMilli()
		doc := metamodels.MetaCampaign{
			MetaData:            input.MetaData,
			OwnerOrganizationID: orgID,
			CreatedAt:           now,
			UpdatedAt:           now,
			LastSyncedAt:        now,
		}
		if err := utility.ExtractDataIfExists(&doc); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Dữ liệu metaData không hợp lệ: " + err.Error(), "status": "error",
			})
			return nil
		}
		if doc.CampaignId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "metaData phải có id (campaignId)", "status": "error",
			})
			return nil
		}
		result, err := h.MetaCampaignService.Upsert(c.Context(), bson.M{"campaignId": doc.CampaignId}, &doc)
		h.HandleResponse(c, result, err)
		return nil
	})
}
