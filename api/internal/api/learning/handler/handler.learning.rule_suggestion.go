// Package learninghdl — Handler cho Rule Suggestions API (Phase 3).
package learninghdl

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	basehdl "meta_commerce/internal/api/base/handler"
	learningsvc "meta_commerce/internal/api/learning/service"
	"meta_commerce/internal/common"

	"go.mongodb.org/mongo-driver/bson"
)

// HandleListRuleSuggestions GET /learning/rule-suggestions
func HandleListRuleSuggestions(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		filter := bson.M{}
		if domain := c.Query("domain"); domain != "" {
			filter["domain"] = domain
		}
		if goalCode := c.Query("goalCode"); goalCode != "" {
			filter["goalCode"] = goalCode
		}
		if status := c.Query("status"); status != "" {
			filter["status"] = status
		}

		limit := 50
		if s := c.Query("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				limit = n
			}
		}
		page := 1
		if s := c.Query("page"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				page = n
			}
		}
		sortField := "createdAt"
		if s := c.Query("sortField"); s != "" {
			sortField = s
		}
		sortOrder := -1
		if s := c.Query("sortOrder"); s != "" {
			if n, err := strconv.Atoi(s); err == nil {
				sortOrder = n
			}
		}
		skip := (page - 1) * limit

		list, total, err := learningsvc.ListRuleSuggestions(c.Context(), *orgID, filter, limit, skip, sortField, sortOrder)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeInternalServer.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		totalPage := int64(0)
		if limit > 0 && total > 0 {
			totalPage = (total + int64(limit) - 1) / int64(limit)
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": fiber.Map{
				"items":     list,
				"page":      page,
				"limit":     limit,
				"itemCount": len(list),
				"total":     total,
				"totalPage": totalPage,
			}, "status": "success",
		})
		return nil
	})
}

// HandlePatchRuleSuggestion PATCH /learning/rule-suggestions/:id — cập nhật status (reviewed, applied, dismissed).
func HandlePatchRuleSuggestion(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		idStr := c.Params("id")
		if idStr == "" {
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
		var input struct {
			Status     string `json:"status"`
			ReviewedBy string `json:"reviewedBy"`
		}
		if err := c.Bind().JSON(&input); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Dữ liệu gửi lên không đúng định dạng JSON", "status": "error",
			})
			return nil
		}
		if input.Status == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "status không được để trống", "status": "error",
			})
			return nil
		}
		err := learningsvc.UpdateRuleSuggestionStatus(c.Context(), idStr, *orgID, input.Status, input.ReviewedBy)
		if err != nil {
			if err == common.ErrNotFound {
				c.Status(common.StatusNotFound).JSON(fiber.Map{
					"code": common.ErrCodeDatabaseQuery.Code, "message": "Không tìm thấy rule suggestion", "status": "error",
				})
				return nil
			}
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeInternalServer.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã cập nhật", "status": "success",
		})
		return nil
	})
}
