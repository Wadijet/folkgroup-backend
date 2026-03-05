// Package approvalhdl — Handler cho cơ chế duyệt (generic).
package approvalhdl

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

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

// ProposeInput body cho propose (generic).
type ProposeInput struct {
	Domain             string                 `json:"domain"`
	ActionType         string                 `json:"actionType"`
	Reason             string                 `json:"reason"`
	Payload            map[string]interface{} `json:"payload"`
	EventTypePending   string                 `json:"eventTypePending,omitempty"`
	ApprovePath        string                 `json:"approvePath,omitempty"`
	RejectPath         string                 `json:"rejectPath,omitempty"`
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

// HandlePropose POST /approval/actions/propose
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
		result, err := approval.Propose(c.Context(), input.Domain, approval.ProposeInput{
			ActionType:       input.ActionType,
			Reason:           input.Reason,
			Payload:          input.Payload,
			EventTypePending: input.EventTypePending,
			ApprovePath:      input.ApprovePath,
			RejectPath:       input.RejectPath,
		}, *orgID, baseURL)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeInternalServer.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusCreated).JSON(fiber.Map{
			"code": common.StatusCreated, "message": "Đã thêm đề xuất vào queue", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleApprove POST /approval/actions/approve
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

// HandleReject POST /approval/actions/reject
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
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã từ chối đề xuất", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleListPending GET /approval/actions/pending?domain=ads&limit=50
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
