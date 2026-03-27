// Package metahdl - Handler cho Meta Ad.
package metahdl

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"

	basehdl "meta_commerce/internal/api/base/handler"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	metadto "meta_commerce/internal/api/meta/dto"
	metamodels "meta_commerce/internal/api/meta/models"
	metasvc "meta_commerce/internal/api/meta/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/utility"
)

// MetaAdHandler xử lý request Meta Ad.
type MetaAdHandler struct {
	*basehdl.BaseHandler[metamodels.MetaAd, metadto.MetaAdCreateInput, metadto.MetaAdUpdateInput]
	MetaAdService *metasvc.MetaAdService
}

// NewMetaAdHandler tạo MetaAdHandler.
func NewMetaAdHandler() (*MetaAdHandler, error) {
	svc, err := metasvc.NewMetaAdService()
	if err != nil {
		return nil, fmt.Errorf("tạo MetaAdService: %w", err)
	}
	return &MetaAdHandler{
		BaseHandler:   basehdl.NewBaseHandler[metamodels.MetaAd, metadto.MetaAdCreateInput, metadto.MetaAdUpdateInput](svc),
		MetaAdService: svc,
	}, nil
}

// HandleSyncUpsertOne xử lý sync-upsert: nhận metaData từ Meta API, upsert vào meta_ads.
func (h *MetaAdHandler) HandleSyncUpsertOne(c fiber.Ctx) error {
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
		doc := metamodels.MetaAd{
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
		if doc.AdId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "metaData phải có id (adId)", "status": "error",
			})
			return nil
		}
		result, err := h.MetaAdService.Upsert(c.Context(), bson.M{"adId": doc.AdId}, &doc)
		h.HandleResponse(c, result, err)
		return nil
	})
}

// RecalculateInput body cho endpoint recalculate.
type RecalculateInput struct {
	AdId string `json:"adId"`
}

// RecalculateAllInput body cho endpoint recalculate-all.
type RecalculateAllInput struct {
	Limit int `json:"limit"` // Giới hạn số Ad xử lý (0 = tất cả)
}

// HandleRecalculate đưa yêu cầu tính lại currentMetrics (full) vào queue AI Decision — HTTP 202, worker gọi RecalculateForEntity.
// POST /meta/ad/recalculate với body { "adId": "xxx" }. OwnerOrgID lấy từ context.
func (h *MetaAdHandler) HandleRecalculate(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input RecalculateInput
		if err := c.Bind().JSON(&input); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Dữ liệu gửi lên không đúng định dạng JSON", "status": "error",
			})
			return nil
		}
		if input.AdId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "adId không được để trống", "status": "error",
			})
			return nil
		}
		orgID := h.GetActiveOrganizationID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		ad, err := h.MetaAdService.FindOne(c.Context(), bson.M{"adId": input.AdId, "ownerOrganizationId": *orgID}, nil)
		if err != nil {
			c.Status(common.StatusNotFound).JSON(fiber.Map{
				"code": common.ErrCodeDatabaseQuery.Code, "message": "Không tìm thấy Ad", "status": "error",
			})
			return nil
		}
		eventID, err := aidecisionsvc.EmitAdsIntelligenceRecomputeRequested(c.Context(), "ad", ad.AdId, ad.AdAccountId, *orgID, "meta", metasvc.RecomputeModeFull)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeInternalServer.Code, "message": "Không thể đưa yêu cầu vào queue AI Decision: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusAccepted).JSON(fiber.Map{
			"code": common.StatusAccepted, "message": "Đã đưa yêu cầu tính lại metrics vào queue AI Decision (worker xử lý bất đồng bộ)",
			"data": fiber.Map{"eventId": eventID, "status": "queued"},
			"status": "success",
		})
		return nil
	})
}

// HandleRecalculateAllMetaAds đưa batch RecalculateAllMetaAds vào queue AI Decision (lane batch) — HTTP 202.
// Body: { "limit": 0 } — limit = 0 xử lý tất cả, limit > 0 giới hạn số Ad.
func (h *MetaAdHandler) HandleRecalculateAllMetaAds(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input RecalculateAllInput
		_ = c.Bind().Body(&input)
		orgID := h.GetActiveOrganizationID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		eventID, err := aidecisionsvc.EmitAdsIntelligenceRecalculateAllRequested(c.Context(), *orgID, input.Limit)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeInternalServer.Code, "message": "Không thể đưa batch vào queue AI Decision: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusAccepted).JSON(fiber.Map{
			"code":    common.StatusAccepted,
			"message": "Đã đưa batch tính lại Meta ads vào queue AI Decision (worker xử lý bất đồng bộ)",
			"data": fiber.Map{
				"eventId": eventID,
				"limit":   input.Limit,
				"status":  "queued",
			},
			"status": "success",
		})
		return nil
	})
}
