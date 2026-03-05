// Package adshdl — Handler cho module Ads (cơ chế duyệt).
package adshdl

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	adsdto "meta_commerce/internal/api/ads/dto"
	adssvc "meta_commerce/internal/api/ads/service"
	"meta_commerce/internal/approval"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// getActiveOrgID lấy organization ID từ context (active_organization_id).
func getActiveOrgID(c fiber.Ctx) *primitive.ObjectID {
	orgIDStr, ok := c.Locals("active_organization_id").(string)
	if !ok || orgIDStr == "" {
		return nil
	}
	oid, err := primitive.ObjectIDFromHex(orgIDStr)
	if err != nil {
		return nil
	}
	return &oid
}

// getRejectedBy lấy user ID hoặc email từ auth context (user_id, user).
func getRejectedBy(c fiber.Ctx) string {
	if userIDStr, ok := c.Locals("user_id").(string); ok && userIDStr != "" {
		return "user:" + userIDStr
	}
	return ""
}

// HandlePropose thêm đề xuất vào queue, trigger notification.
// POST /ads/actions/propose
func HandlePropose(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input adsdto.ProposeInput
		if err := c.Bind().JSON(&input); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Dữ liệu gửi lên không đúng định dạng JSON", "status": "error",
			})
			return nil
		}
		if input.ActionType == "" || input.AdAccountId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "actionType và adAccountId không được để trống", "status": "error",
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
		baseURL := c.Protocol() + "://" + c.Host()
		result, err := adssvc.Propose(c.Context(), &adssvc.ProposeInput{
			ActionType:   input.ActionType,
			AdAccountId:  input.AdAccountId,
			CampaignId:   input.CampaignId,
			CampaignName: input.CampaignName,
			AdSetId:      input.AdSetId,
			AdId:         input.AdId,
			Value:        input.Value,
			Reason:       input.Reason,
			Payload:      input.Payload,
		}, *orgID, baseURL)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeInternalServer.Code, "message": "Thêm đề xuất thất bại: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusCreated).JSON(fiber.Map{
			"code": common.StatusCreated, "message": "Đã thêm đề xuất vào queue", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleApprove duyệt đề xuất.
// POST /ads/actions/approve
func HandleApprove(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input adsdto.ApproveInput
		if err := c.Bind().JSON(&input); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Dữ liệu gửi lên không đúng định dạng JSON", "status": "error",
			})
			return nil
		}
		if input.ActionId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "actionId không được để trống", "status": "error",
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
		result, err := approval.Approve(c.Context(), input.ActionId, *orgID)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã duyệt đề xuất", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleReject từ chối đề xuất.
// POST /ads/actions/reject
func HandleReject(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input adsdto.RejectInput
		if err := c.Bind().JSON(&input); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Dữ liệu gửi lên không đúng định dạng JSON", "status": "error",
			})
			return nil
		}
		if input.ActionId == "" || input.DecisionNote == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "actionId và decisionNote không được để trống", "status": "error",
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
		rejectedBy := getRejectedBy(c)
		result, err := approval.Reject(c.Context(), input.ActionId, *orgID, input.DecisionNote, rejectedBy)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã từ chối đề xuất", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleListPending danh sách đề xuất chờ duyệt.
// GET /ads/actions/pending
func HandleListPending(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		limit := 50
		if s := c.Query("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				limit = n
			}
		}
		list, err := approval.ListPending(c.Context(), *orgID, "ads", limit)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeInternalServer.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": list, "status": "success",
		})
		return nil
	})
}
