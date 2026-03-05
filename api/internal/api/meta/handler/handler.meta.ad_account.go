// Package metahdl - Handler cho Meta Ad Account.
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

// MetaAdAccountHandler xử lý request Meta Ad Account.
type MetaAdAccountHandler struct {
	*basehdl.BaseHandler[metamodels.MetaAdAccount, metadto.MetaAdAccountCreateInput, metadto.MetaAdAccountUpdateInput]
	MetaAdAccountService *metasvc.MetaAdAccountService
}

// NewMetaAdAccountHandler tạo MetaAdAccountHandler.
func NewMetaAdAccountHandler() (*MetaAdAccountHandler, error) {
	svc, err := metasvc.NewMetaAdAccountService()
	if err != nil {
		return nil, fmt.Errorf("tạo MetaAdAccountService: %w", err)
	}
	return &MetaAdAccountHandler{
		BaseHandler:         basehdl.NewBaseHandler[metamodels.MetaAdAccount, metadto.MetaAdAccountCreateInput, metadto.MetaAdAccountUpdateInput](svc),
		MetaAdAccountService: svc,
	}, nil
}

// HandleSyncUpsertOne xử lý sync-upsert: nhận metaData từ Meta API, upsert vào meta_ad_accounts.
func (h *MetaAdAccountHandler) HandleSyncUpsertOne(c fiber.Ctx) error {
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
		doc := metamodels.MetaAdAccount{
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
		if doc.AdAccountId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "metaData phải có id (adAccountId)", "status": "error",
			})
			return nil
		}
		result, err := h.MetaAdAccountService.Upsert(c.Context(), bson.M{"adAccountId": doc.AdAccountId}, &doc)
		h.HandleResponse(c, result, err)
		return nil
	})
}

