package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/utility"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrganizationHandler xử lý các request liên quan đến Organization
type OrganizationHandler struct {
	BaseHandler[models.Organization, dto.OrganizationCreateInput, dto.OrganizationUpdateInput]
	OrganizationService *services.OrganizationService
}

// NewOrganizationHandler tạo mới OrganizationHandler
func NewOrganizationHandler() (*OrganizationHandler, error) {
	organizationService, err := services.NewOrganizationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create organization service: %v", err)
	}

	handler := &OrganizationHandler{
		OrganizationService: organizationService,
	}
	handler.BaseService = handler.OrganizationService

	// Khởi tạo filterOptions với giá trị mặc định
	handler.filterOptions = FilterOptions{
		DeniedFields: []string{
			"password",
			"token",
			"secret",
			"key",
			"hash",
		},
		AllowedOperators: []string{
			"$eq",
			"$gt",
			"$gte",
			"$lt",
			"$lte",
			"$in",
			"$nin",
			"$exists",
		},
		MaxFields: 10,
	}

	return handler, nil
}

// InsertOne override method InsertOne để chuyển đổi từ DTO sang Model và tính toán Path, Level
//
// LÝ DO PHẢI OVERRIDE (không thể dùng CRUD chuẩn):
// 1. Tính toán Path và Level dựa trên parent (logic nghiệp vụ phức tạp):
//    - Nếu có ParentID:
//      + Query parent organization từ database để lấy Path và Level
//      + Tính Path mới: parent.Path + "/" + code
//      + Tính Level mới: dựa trên Type và parent.Level (sử dụng calculateLevel)
//    - Nếu không có ParentID:
//      + Chỉ có thể là "system" (Level = -1, Path = "/" + code) hoặc "group" (Level = 0, Path = "/" + code)
//      + Validate: các Type khác phải có parent
// 2. Validation nghiệp vụ đặc biệt:
//    - Validate ParentID tồn tại trong database (nếu có)
//    - Validate Type: chỉ "system" và "group" mới có thể không có parent
//    - Validate Type khác phải có parent
// 3. Logic tính toán Level phức tạp:
//    - System: Level = -1
//    - Group: Level = 0
//    - Company: Level = 1
//    - Department: Level = 2
//    - Division: Level = 3
//    - Team: Level = parentLevel + 1 (có thể là 4+)
//    - Các Type khác: Level = parentLevel + 1
// 4. Query database để lấy parent:
//    - Cần query parent organization để lấy Path và Level
//    - Validate parent tồn tại trước khi tính toán
//
// KẾT LUẬN: Cần giữ override vì logic nghiệp vụ phức tạp (tính toán Path/Level dựa trên parent,
//           query database để lấy parent, validate Type và parent relationship)
func (h *OrganizationHandler) InsertOne(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		// Parse request body thành DTO
		var input dto.OrganizationCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON hoặc không khớp với cấu trúc yêu cầu. Chi tiết: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Chuyển đổi DTO sang Model
		orgModel := models.Organization{
			Name:     input.Name,
			Code:     input.Code,
			Type:     input.Type,
			IsActive: input.IsActive,
		}

		// Xử lý ParentID nếu có
		if input.ParentID != "" {
			if !primitive.IsValidObjectID(input.ParentID) {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("ParentID '%s' không đúng định dạng MongoDB ObjectID", input.ParentID),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
			parentID := utility.String2ObjectID(input.ParentID)
			orgModel.ParentID = &parentID

			// Lấy thông tin parent để tính Path và Level
			parent, err := h.OrganizationService.FindOneById(c.Context(), parentID)
			if err != nil {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeBusinessOperation,
					fmt.Sprintf("Không tìm thấy tổ chức cha với ID: %s", input.ParentID),
					common.StatusBadRequest,
					err,
				))
				return nil
			}

			var modelParent models.Organization
			bsonBytes, _ := bson.Marshal(parent)
			if err := bson.Unmarshal(bsonBytes, &modelParent); err != nil {
				h.HandleResponse(c, nil, common.ErrInvalidFormat)
				return nil
			}

			// Tính Path: parent.Path + "/" + code
			orgModel.Path = modelParent.Path + "/" + input.Code

			// Tính Level dựa trên Type
			orgModel.Level = h.calculateLevel(input.Type, modelParent.Level)
		} else {
			// Không có parent - chỉ có thể là system hoặc group
			if input.Type == models.OrganizationTypeSystem {
				orgModel.Path = "/" + input.Code
				orgModel.Level = -1
			} else if input.Type == models.OrganizationTypeGroup {
				orgModel.Path = "/" + input.Code
				orgModel.Level = 0
			} else {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeBusinessOperation,
					fmt.Sprintf("Loại tổ chức '%s' phải có parent. Chỉ 'system' và 'group' mới có thể không có parent.", input.Type),
					common.StatusBadRequest,
					nil,
				))
				return nil
			}
		}

		// Thực hiện insert
		data, err := h.BaseService.InsertOne(c.Context(), orgModel)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// calculateLevel tính toán Level dựa trên Type và Level của parent
func (h *OrganizationHandler) calculateLevel(orgType string, parentLevel int) int {
	switch orgType {
	case models.OrganizationTypeSystem:
		return -1
	case models.OrganizationTypeGroup:
		return 0
	case models.OrganizationTypeCompany:
		return 1
	case models.OrganizationTypeDepartment:
		return 2
	case models.OrganizationTypeDivision:
		return 3
	case models.OrganizationTypeTeam:
		// Team có thể là Level 4+ tùy thuộc vào parent
		return parentLevel + 1
	default:
		// Mặc định tăng level lên 1 so với parent
		return parentLevel + 1
	}
}
