// Package executorhdl — Handler cho Executor (Approval Gate + Execution).
package executorhdl

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	approval "meta_commerce/internal/approval"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

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

// ProposeInput body cho propose.
type ProposeInput struct {
	Domain           string                 `json:"domain"`
	ActionType       string                 `json:"actionType"`
	Reason           string                 `json:"reason"`
	Payload          map[string]interface{} `json:"payload"`
	EventTypePending string                 `json:"eventTypePending,omitempty"`
	ApprovePath      string                 `json:"approvePath,omitempty"`
	RejectPath       string                 `json:"rejectPath,omitempty"`
}

// ApproveInput body cho approve.
type ApproveInput struct {
	ActionId string `json:"actionId"`
}

// RejectInput body cho reject.
type RejectInput struct {
	ActionId     string `json:"actionId"`
	DecisionNote string `json:"decisionNote"`
}

// ExecuteInput body cho execute (thực thi thủ công).
type ExecuteInput struct {
	ActionId string `json:"actionId"`
}

// HandlePropose POST /executor/actions/propose
// Vision 08 Phase 0: API này chủ yếu dùng nội bộ (AI Decision, test). External nên dùng domain-specific API:
// - Ads: POST /ads/actions/propose
// - CIO: không còn domain propose riêng (đã gỡ session/plan/touchpoint).
func HandlePropose(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input ProposeInput
		if err := c.Bind().JSON(&input); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Dữ liệu gửi lên không đúng định dạng JSON", "status": "error",
			})
			return nil
		}
		if input.Domain == "" || input.ActionType == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "domain và actionType không được để trống", "status": "error",
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
		approvePath := input.ApprovePath
		if approvePath == "" {
			approvePath = "/api/v1/executor/actions/approve"
		}
		rejectPath := input.RejectPath
		if rejectPath == "" {
			rejectPath = "/api/v1/executor/actions/reject"
		}
		proposeInput := approval.ProposeInput{
			ActionType:       input.ActionType,
			Reason:           input.Reason,
			Payload:          input.Payload,
			EventTypePending: input.EventTypePending,
			ApprovePath:      approvePath,
			RejectPath:       rejectPath,
		}
		// Vision 08 event-driven: Ads/CIO emit event; domain khác gọi approval.Propose
		var err error
		switch input.Domain {
		case "ads":
			var eventID string
			eventID, err = aidecisionsvc.EmitAdsProposeRequest(c.Context(), proposeInput, *orgID, baseURL)
			if err == nil {
				c.Status(common.StatusAccepted).JSON(fiber.Map{
					"code": common.StatusAccepted, "message": "Đã nhận đề xuất, đang xử lý", "data": fiber.Map{"eventId": eventID}, "status": "success",
				})
				return nil
			}
		default:
			// Vision: mọi action cần decisionId, contextSnapshot — EnrichProposeInputWithTrace trước khi Propose
			aidecisionsvc.EnrichProposeInputWithTrace(input.Domain, &proposeInput)
			result, proposeErr := approval.Propose(c.Context(), input.Domain, proposeInput, *orgID, baseURL)
			if proposeErr != nil {
				err = proposeErr
				break
			}
			c.Status(common.StatusCreated).JSON(fiber.Map{
				"code": common.StatusCreated, "message": "Đã thêm đề xuất vào queue", "data": result, "status": "success",
			})
			return nil
		}
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeInternalServer.Code, "message": err.Error(), "status": "error",
			})
		}
		return nil
	})
}

// HandleApprove POST /executor/actions/approve
func HandleApprove(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input ApproveInput
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

// HandleExecute POST /executor/actions/execute
func HandleExecute(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input ExecuteInput
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
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã thực thi thành công", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleReject POST /executor/actions/reject
func HandleReject(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input RejectInput
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
		result, err := approval.Reject(c.Context(), input.ActionId, *orgID, input.DecisionNote, "")
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": err.Error(), "status": "error",
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

// HandleFindById GET /executor/actions/find-by-id/:id
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

// HandleFind GET /executor/actions/find
func HandleFind(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		filter := approval.FindFilter{
			Domain:    c.Query("domain"),
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

// HandleFindWithPagination GET /executor/actions/find-with-pagination
func HandleFindWithPagination(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		filter := approval.FindWithPaginationFilter{
			FindFilter: approval.FindFilter{
				Domain:    c.Query("domain"),
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
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeInternalServer.Code, "message": err.Error(), "status": "error",
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

// HandleCount GET /executor/actions/count
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
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeInternalServer.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": fiber.Map{"count": count}, "status": "success",
		})
		return nil
	})
}

// HandleCancel POST /executor/actions/cancel
func HandleCancel(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input ApproveInput
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
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã hủy đề xuất", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleListPending GET /executor/actions/pending
func HandleListPending(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		domain := c.Query("domain")
		limit := 50
		if s := c.Query("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				limit = n
			}
		}
		list, err := approval.ListPending(c.Context(), *orgID, domain, limit)
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
