// Package adshdl — Handler Counterfactual Kill Tracker (FolkForm v4.1 Section 2.3).
// B4–B5: Kill Accuracy Rate, đề xuất điều chỉnh threshold.
package adshdl

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	adssvc "meta_commerce/internal/api/ads_meta/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"
)

// HandleGetKillAccuracy B4: Lấy Kill Accuracy Rate.
// GET /ads/counterfactual/accuracy?adAccountId=act_xxx&windowDays=14
func HandleGetKillAccuracy(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		adAccountId := c.Query("adAccountId")
		windowDays := 14
		if s := c.Query("windowDays"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 90 {
				windowDays = n
			}
		}
		result, err := adssvc.ComputeKillAccuracy(c.Context(), *orgID, adAccountId, windowDays)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Lỗi tính Kill Accuracy")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		if result == nil {
			result = &adssvc.KillAccuracyResult{}
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleGetThresholdSuggestion B5: Đề xuất điều chỉnh khi Kill_Accuracy < 70% liên tục 2 tuần.
// GET /ads/counterfactual/suggestion?adAccountId=act_xxx
func HandleGetThresholdSuggestion(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		adAccountId := c.Query("adAccountId")
		shouldAdjust, result, err := adssvc.ShouldSuggestThresholdAdjustment(c.Context(), *orgID, adAccountId)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Lỗi kiểm tra đề xuất")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		payload := fiber.Map{
			"shouldAdjust": shouldAdjust,
			"accuracy":     result,
			"message":      "",
		}
		if result == nil {
			payload["accuracy"] = &adssvc.KillAccuracyResult{}
		}
		if shouldAdjust {
			payload["message"] = "Kill Accuracy < 70% trong 2 tuần gần nhất. Cân nhắc nới threshold (CPA_Mess, Conv_Rate) cho các rule kill."
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": payload, "status": "success",
		})
		return nil
	})
}
