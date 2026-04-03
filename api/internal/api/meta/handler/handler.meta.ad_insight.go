// Package metahdl - Handler cho Meta Ad Insight.
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

// MetaAdInsightHandler xử lý request Meta Ad Insight.
type MetaAdInsightHandler struct {
	*basehdl.BaseHandler[metamodels.MetaAdInsight, metadto.MetaAdInsightCreateInput, metadto.MetaAdInsightUpdateInput]
	MetaAdInsightService *metasvc.MetaAdInsightService
}

// NewMetaAdInsightHandler tạo MetaAdInsightHandler.
func NewMetaAdInsightHandler() (*MetaAdInsightHandler, error) {
	svc, err := metasvc.NewMetaAdInsightService()
	if err != nil {
		return nil, fmt.Errorf("tạo MetaAdInsightService: %w", err)
	}
	return &MetaAdInsightHandler{
		BaseHandler:          basehdl.NewBaseHandler[metamodels.MetaAdInsight, metadto.MetaAdInsightCreateInput, metadto.MetaAdInsightUpdateInput](svc),
		MetaAdInsightService: svc,
	}, nil
}

// HandleSyncUpsertOne xử lý sync-upsert: nhận metaData từ Meta API, upsert vào meta_ad_insights.
// Cần adAccountId, objectId, objectType (phụ thuộc level: account/campaign/adset/ad).
func (h *MetaAdInsightHandler) HandleSyncUpsertOne(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input metadto.MetaAdInsightSyncUpsertInput
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
		if input.AdAccountId == "" || input.ObjectId == "" || input.ObjectType == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "adAccountId, objectId, objectType không được để trống", "status": "error",
			})
			return nil
		}
		orgID := resolveOwnerOrgIDFromCtx(c, input.OwnerOrganizationID)
		now := time.Now().UnixMilli()
		doc := metamodels.MetaAdInsight{
			ObjectId:            input.ObjectId,
			ObjectType:          input.ObjectType,
			AdAccountId:         input.AdAccountId,
			MetaData:            input.MetaData,
			OwnerOrganizationID: orgID,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if err := utility.ExtractDataIfExists(&doc); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Dữ liệu metaData không hợp lệ: " + err.Error(), "status": "error",
			})
			return nil
		}
		if doc.DateStart == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "metaData phải có date_start", "status": "error",
			})
			return nil
		}
		filter := bson.M{
			"adAccountId": input.AdAccountId,
			"objectId":    input.ObjectId,
			"dateStart":   doc.DateStart,
			"objectType":  input.ObjectType,
		}
		result, err := h.MetaAdInsightService.Upsert(c.Context(), filter, &doc)
		if err == nil {
			_ = metasvc.SaveDailySnapshot(c.Context(), &result)
		}
		h.HandleResponse(c, result, err)
		return nil
	})
}
