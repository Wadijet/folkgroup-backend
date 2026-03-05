// Package metahdl - Handler cho Meta Ad Set.
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

// MetaAdSetHandler xử lý request Meta Ad Set.
type MetaAdSetHandler struct {
	*basehdl.BaseHandler[metamodels.MetaAdSet, metadto.MetaAdSetCreateInput, metadto.MetaAdSetUpdateInput]
	MetaAdSetService *metasvc.MetaAdSetService
}

// NewMetaAdSetHandler tạo MetaAdSetHandler.
func NewMetaAdSetHandler() (*MetaAdSetHandler, error) {
	svc, err := metasvc.NewMetaAdSetService()
	if err != nil {
		return nil, fmt.Errorf("tạo MetaAdSetService: %w", err)
	}
	return &MetaAdSetHandler{
		BaseHandler:      basehdl.NewBaseHandler[metamodels.MetaAdSet, metadto.MetaAdSetCreateInput, metadto.MetaAdSetUpdateInput](svc),
		MetaAdSetService: svc,
	}, nil
}

// HandleSyncUpsertOne xử lý sync-upsert: nhận metaData từ Meta API, upsert vào meta_adsets.
func (h *MetaAdSetHandler) HandleSyncUpsertOne(c fiber.Ctx) error {
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
		doc := metamodels.MetaAdSet{
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
		if doc.AdSetId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "metaData phải có id (adSetId)", "status": "error",
			})
			return nil
		}
		result, err := h.MetaAdSetService.Upsert(c.Context(), bson.M{"adSetId": doc.AdSetId}, &doc)
		h.HandleResponse(c, result, err)
		return nil
	})
}
