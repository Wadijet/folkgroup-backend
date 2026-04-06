// Package adshdl — Handler cấu hình approval (approvalConfig) và Meta Ads (meta config).
package adshdl

import (
	"github.com/gofiber/fiber/v3"

	adsconfig "meta_commerce/internal/api/ads_meta/config"
	adsmodels "meta_commerce/internal/api/ads_meta/models"
	adssvc "meta_commerce/internal/api/ads_meta/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"
)

// HandleGetApprovalConfig lấy approvalConfig của ad account.
// GET /ads/config/approval?adAccountId=act_xxx
func HandleGetApprovalConfig(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		adAccountId := c.Query("adAccountId")
		if adAccountId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "adAccountId không được để trống", "status": "error",
			})
			return nil
		}
		config, err := adssvc.GetApprovalConfig(c.Context(), adAccountId, *orgID)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Không tìm thấy cấu hình approval")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": config, "status": "success",
		})
		return nil
	})
}

// HandleUpdateApprovalConfig cập nhật approvalConfig.
// PUT /ads/config/approval
func HandleUpdateApprovalConfig(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var body struct {
			AdAccountId   string                 `json:"adAccountId"`
			ApprovalConfig map[string]interface{} `json:"approvalConfig"`
		}
		if err := c.Bind().JSON(&body); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Dữ liệu gửi lên không đúng định dạng JSON", "status": "error",
			})
			return nil
		}
		if body.AdAccountId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "adAccountId không được để trống", "status": "error",
			})
			return nil
		}
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		err := adssvc.UpdateApprovalConfig(c.Context(), body.AdAccountId, *orgID, body.ApprovalConfig)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Cập nhật approvalConfig thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã cập nhật approvalConfig", "data": nil, "status": "success",
		})
		return nil
	})
}

// HandleGetMetaConfig lấy cấu hình Meta Ads đầy đủ (account, campaign, adSet, ad).
// GET /ads/config/meta?adAccountId=act_xxx
func HandleGetMetaConfig(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		adAccountId := c.Query("adAccountId")
		if adAccountId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "adAccountId không được để trống", "status": "error",
			})
			return nil
		}
		config, err := adssvc.GetAdsMetaConfig(c.Context(), adAccountId, *orgID)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Lấy cấu hình Meta Ads thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		metadata := adsconfig.GetConfigMetadata()
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": config, "metadata": metadata, "status": "success",
		})
		return nil
	})
}

// HandleUpdateMetaConfig cập nhật cấu hình Meta Ads. Body có thể chứa account, campaign, adSet, ad — merge từng phần.
// PUT /ads/config/meta
func HandleUpdateMetaConfig(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var body struct {
			AdAccountId string                      `json:"adAccountId"`
			Account     *adsmodels.AccountConfig    `json:"account"`
			Campaign    *adsmodels.CampaignConfig   `json:"campaign"`
			AdSet       *adsmodels.AdSetConfig     `json:"adSet"`
			Ad          *adsmodels.AdConfig        `json:"ad"`
		}
		if err := c.Bind().JSON(&body); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Dữ liệu gửi lên không đúng định dạng JSON", "status": "error",
			})
			return nil
		}
		if body.AdAccountId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "adAccountId không được để trống", "status": "error",
			})
			return nil
		}
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		existing, err := adssvc.GetAdsMetaConfig(c.Context(), body.AdAccountId, *orgID)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Lấy cấu hình Meta Ads thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		if body.Account != nil {
			existing.Account = *body.Account
		}
		if body.Campaign != nil {
			existing.Campaign = *body.Campaign
		}
		if body.AdSet != nil {
			existing.AdSet = *body.AdSet
		}
		if body.Ad != nil {
			existing.Ad = *body.Ad
		}
		err = adssvc.UpdateAdsMetaConfig(c.Context(), body.AdAccountId, *orgID, existing)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Cập nhật cấu hình Meta Ads thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã cập nhật cấu hình Meta Ads", "data": nil, "status": "success",
		})
		return nil
	})
}

// HandleGetMetricDefinitions lấy danh sách metric definitions từ DB (FolkForm v4.1).
// GET /ads/config/metric-definitions
// Trả về metrics theo window (7d, 2h, 1h, 30p) — dùng cho Momentum Tracker, CB-4, ...
func HandleGetMetricDefinitions(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		defs, err := adssvc.GetMetricDefinitions(c.Context())
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Lấy danh sách metric definitions thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		// Rỗng = chưa seed — frontend có thể fallback
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": defs, "status": "success",
		})
		return nil
	})
}
