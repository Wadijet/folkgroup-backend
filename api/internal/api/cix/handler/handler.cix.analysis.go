// Package handler — Handler cho module CIX (Contextual Conversation Intelligence).
package handler

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	cixdto "meta_commerce/internal/api/cix/dto"
	cixsvc "meta_commerce/internal/api/cix/service"
	"meta_commerce/internal/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CixAnalysisHandler handler phân tích hội thoại.
type CixAnalysisHandler struct {
	svc *cixsvc.CixAnalysisService
}

// NewCixAnalysisHandler tạo handler mới.
func NewCixAnalysisHandler() (*CixAnalysisHandler, error) {
	svc, err := cixsvc.NewCixAnalysisService()
	if err != nil {
		return nil, err
	}
	return &CixAnalysisHandler{svc: svc}, nil
}

// getActiveOrganizationID lấy active organization ID từ context.
func getActiveOrganizationID(c fiber.Ctx) *primitive.ObjectID {
	orgIDStr, ok := c.Locals("active_organization_id").(string)
	if !ok || orgIDStr == "" {
		return nil
	}
	orgID, err := primitive.ObjectIDFromHex(orgIDStr)
	if err != nil {
		return nil
	}
	return &orgID
}

// toResponseMap chuyển CixAnalysisResponse sang fiber.Map cho JSON.
func toResponseMap(r *cixdto.CixAnalysisResponse) fiber.Map {
	if r == nil {
		return nil
	}
	return fiber.Map{
		"id":                r.ID,
		"sessionUid":        r.SessionUid,
		"customerUid":       r.CustomerUid,
		"traceId":           r.TraceID,
		"layer1":            r.Layer1,
		"layer2":            r.Layer2,
		"layer3":            r.Layer3,
		"flags":             r.Flags,
		"actionSuggestions": r.ActionSuggestions,
		"createdAt":         r.CreatedAt,
	}
}

// HandleAnalyzeSession POST /cix/analyze — Phân tích session (sessionUid, customerUid).
func (h *CixAnalysisHandler) HandleAnalyzeSession(c fiber.Ctx) error {
	ctx := c.Context()
	orgID := getActiveOrganizationID(c)
	if orgID == nil || orgID.IsZero() {
		c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức", "status": "error",
		})
		return nil
	}
	var req cixdto.AnalyzeSessionRequest
	if err := c.Bind().JSON(&req); err != nil {
		c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code": common.ErrCodeValidationFormat.Code, "message": "Body JSON không hợp lệ", "status": "error",
		})
		return nil
	}
	if req.SessionUid == "" {
		c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code": common.ErrCodeValidationInput.Code, "message": "sessionUid bắt buộc", "status": "error",
		})
		return nil
	}
	result, err := h.svc.AnalyzeSession(ctx, req.SessionUid, req.CustomerUid, *orgID)
	if err != nil {
		errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Phân tích session thất bại")
		c.Status(statusCode).JSON(fiber.Map{"code": errCode, "message": msg, "status": "error"})
		return nil
	}
	resp := cixsvc.ToCixAnalysisResponse(result)
	c.Status(common.StatusOK).JSON(fiber.Map{
		"code": "0", "message": "OK", "data": toResponseMap(resp), "status": "success",
	})
	return nil
}

// HandleGetAnalysisBySession GET /cix/analysis/:sessionUid — Lấy kết quả phân tích theo session.
func (h *CixAnalysisHandler) HandleGetAnalysisBySession(c fiber.Ctx) error {
	ctx := c.Context()
	sessionUid := c.Params("sessionUid")
	if sessionUid == "" {
		c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code": common.ErrCodeValidationInput.Code, "message": "sessionUid bắt buộc", "status": "error",
		})
		return nil
	}
	orgID := getActiveOrganizationID(c)
	if orgID == nil || orgID.IsZero() {
		c.Status(common.StatusBadRequest).JSON(fiber.Map{
			"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức", "status": "error",
		})
		return nil
	}
	result, err := h.svc.FindBySessionUid(ctx, sessionUid, *orgID)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			c.Status(common.StatusOK).JSON(fiber.Map{
				"code": "0", "message": "Chưa có kết quả phân tích", "data": nil, "status": "success",
			})
			return nil
		}
		errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Lấy kết quả phân tích thất bại")
		c.Status(statusCode).JSON(fiber.Map{"code": errCode, "message": msg, "status": "error"})
		return nil
	}
	resp := cixsvc.ToCixAnalysisResponse(result)
	c.Status(common.StatusOK).JSON(fiber.Map{
		"code": "0", "message": "OK", "data": toResponseMap(resp), "status": "success",
	})
	return nil
}
