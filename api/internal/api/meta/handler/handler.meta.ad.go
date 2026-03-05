// Package metahdl - Handler cho Meta Ad.
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

// HandleRecalculate tính lại currentMetrics cho Ad (raw + layer1 + layer2 + layer3).
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
		// Lấy adAccountId từ meta_ads
		ad, err := h.MetaAdService.FindOne(c.Context(), bson.M{"adId": input.AdId, "ownerOrganizationId": *orgID}, nil)
		if err != nil {
			c.Status(common.StatusNotFound).JSON(fiber.Map{
				"code": common.ErrCodeDatabaseQuery.Code, "message": "Không tìm thấy Ad", "status": "error",
			})
			return nil
		}
		if err := metasvc.RecalculateForEntity(c.Context(), "ad", ad.AdId, ad.AdAccountId, *orgID); err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeInternalServer.Code, "message": "Tính lại metrics thất bại: " + err.Error(), "status": "error",
			})
			return nil
		}
		// Lấy lại ad sau khi đã cập nhật currentMetrics
		updated, _ := h.MetaAdService.FindOne(c.Context(), bson.M{"adId": input.AdId, "ownerOrganizationId": *orgID}, nil)
		data := map[string]interface{}{}
		if updated.CurrentMetrics != nil {
			data = updated.CurrentMetrics
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã tính lại currentMetrics thành công", "data": data, "status": "success",
		})
		return nil
	})
}

// HandleRecalculateAllMetaAds xử lý POST /meta/ad/recalculate-all — tính toán lại currentMetrics cho toàn bộ Meta ads của org.
// Body: { "limit": 0 } — limit = 0 xử lý tất cả, limit > 0 giới hạn số Ad.
// Luồng: Ad (raw + layers) → AdSet roll-up → Campaign roll-up → AdAccount roll-up.
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
		result, err := metasvc.RecalculateAllMetaAds(c.Context(), *orgID, input.Limit)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeInternalServer.Code, "message": "Tính toán lại toàn bộ Meta ads thất bại: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK,
			"message": "Đã tính toán lại toàn bộ Meta ads thành công",
			"data": fiber.Map{
				"totalAdsProcessed":     result.TotalAdsProcessed,
				"totalAdsFailed":        result.TotalAdsFailed,
				"failedAdIds":           result.FailedAdIds,
				"totalAdSetsRolledUp":   result.TotalAdSetsRolledUp,
				"totalCampaignsRolledUp": result.TotalCampaignsRolledUp,
				"totalAccountsRolledUp": result.TotalAccountsRolledUp,
			},
			"status": "success",
		})
		return nil
	})
}
