// Package learninghdl — Handler cho Learning engine API.
package learninghdl

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/api/learning/dto"
	"meta_commerce/internal/api/learning/models"
	learningsvc "meta_commerce/internal/api/learning/service"
	"meta_commerce/internal/common"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var learningSvc *learningsvc.LearningCaseService

func init() {
	learningSvc = learningsvc.NewLearningCaseService()
}

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

// HandleFindLearningCaseById GET /learning/cases/:id
func HandleFindLearningCaseById(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		idStr := c.Params("id")
		if idStr == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "id không được để trống", "status": "error",
			})
			return nil
		}
		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "id không hợp lệ", "status": "error",
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
		dc, err := learningSvc.FindLearningCaseById(c.Context(), oid, *orgID)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Không tìm thấy learning case")
			c.Status(statusCode).JSON(fiber.Map{"code": errCode, "message": msg, "status": "error"})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": dc, "status": "success",
		})
		return nil
	})
}

// HandleListLearningCases GET /learning/cases
func HandleListLearningCases(c fiber.Ctx) error {
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
		if caseType := c.Query("caseType"); caseType != "" {
			filter["caseType"] = caseType
		}
		if caseCategory := c.Query("caseCategory"); caseCategory != "" {
			filter["caseCategory"] = caseCategory
		}
		if goalCode := c.Query("goalCode"); goalCode != "" {
			filter["goalCode"] = goalCode
		}
		if result := c.Query("result"); result != "" {
			filter["result"] = result
		}
		if targetType := c.Query("targetType"); targetType != "" {
			filter["targetType"] = targetType
		}
		if targetId := c.Query("targetId"); targetId != "" {
			filter["targetId"] = targetId
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

		list, total, err := learningSvc.ListLearningCases(c.Context(), *orgID, filter, limit, skip, sortField, sortOrder)
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

// HandleCreateLearningCase POST /learning/cases
func HandleCreateLearningCase(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input dto.LearningCaseCreateInput
		if err := c.Bind().JSON(&input); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Dữ liệu gửi lên không đúng định dạng JSON", "status": "error",
			})
			return nil
		}
		if input.CaseId == "" || input.GoalCode == "" || input.Result == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "caseId, goalCode, result không được để trống", "status": "error",
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

		dc := inputToModel(&input, *orgID)
		created, err := learningSvc.CreateLearningCase(c.Context(), dc)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeInternalServer.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusCreated).JSON(fiber.Map{
			"code": common.StatusCreated, "message": "Đã tạo learning case", "data": created, "status": "success",
		})
		return nil
	})
}

func inputToModel(input *dto.LearningCaseCreateInput, ownerOrgID primitive.ObjectID) *models.LearningCase {
	now := int64(0)
	if input.SourceClosedAt > 0 {
		now = input.SourceClosedAt
	}
	if now == 0 {
		now = int64(primitive.NewObjectID().Timestamp().Unix()) * 1000
	}
	return &models.LearningCase{
		CaseId:              input.CaseId,
		CaseType:            input.CaseType,
		CaseCategory:        input.CaseCategory,
		Domain:              input.Domain,
		TargetType:          input.TargetType,
		TargetId:            input.TargetId,
		SourceRefType:       input.SourceRefType,
		SourceRefID:         input.SourceRefId,
		GoalCode:            input.GoalCode,
		ActionType:          input.GoalCode,
		Result:              input.Result,
		OwnerOrganizationID: ownerOrgID,
		ClosedAt:            now,
		Evaluation: models.LearningEvaluation{
			PrimaryMetric: input.SummaryPrimaryMetric,
			BaselineValue: input.SummaryBaselineValue,
			FinalValue:    input.SummaryFinalValue,
			Delta:         input.SummaryDelta,
		},
	}
}
