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

// HandleCreateCommand tạo lệnh chờ duyệt — user có MetaAdAccount.Read có thể gọi.
// POST /ads/commands
func HandleCreateCommand(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error { return handleProposeLogic(c) })
}

// HandlePropose thêm đề xuất vào queue, trigger notification.
// POST /ads/actions/propose (yêu cầu MetaAdAccount.Update)
func HandlePropose(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error { return handleProposeLogic(c) })
}

// handleProposeLogic logic chung cho tạo lệnh/propose.
func handleProposeLogic(c fiber.Ctx) error {
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
		if input.Reason == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Lý do (reason) không được để trống", "status": "error",
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
		eventID, err := adssvc.Propose(c.Context(), &adssvc.ProposeInput{
			ActionType:   input.ActionType,
			AdAccountId:  input.AdAccountId,
			CampaignId:   input.CampaignId,
			CampaignName: input.CampaignName,
			AdSetId:      input.AdSetId,
			AdId:         input.AdId,
			Value:        input.Value,
			Reason:       input.Reason,
			RuleCode:     input.RuleCode,
			TraceID:      input.TraceID,
			Payload:      input.Payload,
		}, *orgID, baseURL)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Thêm đề xuất thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		c.Status(common.StatusAccepted).JSON(fiber.Map{
			"code": common.StatusAccepted, "message": "Đã nhận đề xuất, đang xử lý. Proposal sẽ xuất hiện trong vài giây.", "data": fiber.Map{"eventId": eventID}, "status": "success",
		})
		return nil
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
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Duyệt đề xuất thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã duyệt đề xuất", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleExecute thực thi thủ công đề xuất đã duyệt. Dùng cho test — thay vì chờ worker.
// POST /ads/actions/execute
func HandleExecute(c fiber.Ctx) error {
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
		result, err := approval.Execute(c.Context(), input.ActionId, *orgID)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Thực thi thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã thực thi thành công", "data": result, "status": "success",
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
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Từ chối đề xuất thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		// Learning: OnActionClosed (Reject) tạo learning case
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã từ chối đề xuất", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleResumeAds bật lại campaign sau Circuit Breaker. Tương đương /resume_ads.
// POST /ads/commands/resume-ads
func HandleResumeAds(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input struct {
			AdAccountId string `json:"adAccountId"`
		}
		if err := c.Bind().JSON(&input); err != nil {
			input.AdAccountId = c.Query("adAccountId")
		}
		if input.AdAccountId == "" {
			input.AdAccountId = c.Query("adAccountId")
		}
		if input.AdAccountId == "" {
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
		resumed, err := adssvc.ResumeAds(c.Context(), input.AdAccountId, *orgID)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Bật lại campaign thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã bật lại campaign", "data": fiber.Map{"resumed": resumed}, "status": "success",
		})
		return nil
	})
}

// HandlePancakeOk gỡ pancakeDownOverride — xác nhận Pancake đã hoạt động. /pancake_ok
// POST /ads/commands/pancake-ok
func HandlePancakeOk(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		cleared, err := adssvc.PancakeOk(c.Context())
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Gỡ Pancake Down override thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã gỡ Pancake Down override", "data": fiber.Map{"cleared": cleared}, "status": "success",
		})
		return nil
	})
}

// HandleFindById xem chi tiết một đề xuất theo id — phục vụ frontend.
// GET /ads/actions/find-by-id/:id
func HandleFindById(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		id := c.Params("id")
		if id == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "id không được để trống", "status": "error",
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
		result, err := approval.FindById(c.Context(), id, *orgID)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Không tìm thấy đề xuất")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleFind danh sách đề xuất với filter — phục vụ frontend. Mặc định domain=ads khi gọi từ /ads/actions/find.
// GET /ads/actions/find?status=pending&limit=50&sortField=proposedAt&sortOrder=-1
func HandleFind(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		domain := c.Query("domain")
		if domain == "" {
			domain = "ads"
		}
		filter := approval.FindFilter{
			Domain:    domain,
			Status:    c.Query("status"),
			Limit:     50,
			SortField: "proposedAt",
			SortOrder: -1,
		}
		if s := c.Query("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				filter.Limit = n
			}
		}
		if s := c.Query("sortField"); s != "" {
			filter.SortField = s
		}
		if s := c.Query("sortOrder"); s != "" {
			if n, err := strconv.Atoi(s); err == nil {
				filter.SortOrder = n
			}
		}
		if s := c.Query("from"); s != "" {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
				filter.FromProposedAt = n
			}
		}
		if s := c.Query("to"); s != "" {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
				filter.ToProposedAt = n
			}
		}
		list, err := approval.Find(c.Context(), *orgID, filter)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Lấy danh sách thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": list, "status": "success",
		})
		return nil
	})
}

// HandleFindWithPagination danh sách có phân trang — mặc định domain=ads.
// GET /ads/actions/find-with-pagination?page=1&limit=20&status=pending
func HandleFindWithPagination(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		domain := c.Query("domain")
		if domain == "" {
			domain = "ads"
		}
		filter := approval.FindWithPaginationFilter{
			FindFilter: approval.FindFilter{
				Domain:    domain,
				Status:    c.Query("status"),
				Limit:     50,
				SortField: "proposedAt",
				SortOrder: -1,
			},
			Page: 1,
		}
		if s := c.Query("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				filter.Limit = n
			}
		}
		if s := c.Query("page"); s != "" {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
				filter.Page = n
			}
		}
		if s := c.Query("sortField"); s != "" {
			filter.SortField = s
		}
		if s := c.Query("sortOrder"); s != "" {
			if n, err := strconv.Atoi(s); err == nil {
				filter.SortOrder = n
			}
		}
		if s := c.Query("from"); s != "" {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
				filter.FromProposedAt = n
			}
		}
		if s := c.Query("to"); s != "" {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
				filter.ToProposedAt = n
			}
		}
		items, total, err := approval.FindWithPagination(c.Context(), *orgID, filter)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Lấy danh sách thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		totalPage := int64(0)
		if filter.Limit > 0 && total > 0 {
			totalPage = (total + int64(filter.Limit) - 1) / int64(filter.Limit)
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": fiber.Map{
				"items":     items,
				"page":      filter.Page,
				"limit":     int64(filter.Limit),
				"itemCount": int64(len(items)),
				"total":     total,
				"totalPage": totalPage,
			}, "status": "success",
		})
		return nil
	})
}

// HandleCount đếm theo filter — mặc định domain=ads.
// GET /ads/actions/count?status=pending
func HandleCount(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		domain := c.Query("domain")
		if domain == "" {
			domain = "ads"
		}
		status := c.Query("status")
		var fromProposedAt, toProposedAt int64
		if s := c.Query("from"); s != "" {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
				fromProposedAt = n
			}
		}
		if s := c.Query("to"); s != "" {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
				toProposedAt = n
			}
		}
		count, err := approval.Count(c.Context(), *orgID, domain, status, fromProposedAt, toProposedAt)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Đếm thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": fiber.Map{"count": count}, "status": "success",
		})
		return nil
	})
}

// HandleCancel hủy đề xuất pending.
// POST /ads/actions/cancel
func HandleCancel(c fiber.Ctx) error {
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
		result, err := approval.Cancel(c.Context(), input.ActionId, *orgID)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Hủy đề xuất thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã hủy đề xuất", "data": result, "status": "success",
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
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Lấy danh sách đề xuất thất bại")
			c.Status(statusCode).JSON(fiber.Map{
				"code": errCode, "message": msg, "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": list, "status": "success",
		})
		return nil
	})
}
