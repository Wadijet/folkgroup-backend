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

// OrganizationShareHandler xử lý các request liên quan đến Organization Share
type OrganizationShareHandler struct {
	BaseHandler[models.OrganizationShare, dto.OrganizationShareCreateInput, dto.OrganizationShareUpdateInput]
	OrganizationShareService *services.OrganizationShareService
}

// NewOrganizationShareHandler tạo mới OrganizationShareHandler
func NewOrganizationShareHandler() (*OrganizationShareHandler, error) {
	shareService, err := services.NewOrganizationShareService()
	if err != nil {
		return nil, fmt.Errorf("failed to create organization share service: %v", err)
	}

	baseHandler := NewBaseHandler[models.OrganizationShare, dto.OrganizationShareCreateInput, dto.OrganizationShareUpdateInput](shareService)
	handler := &OrganizationShareHandler{
		BaseHandler:              *baseHandler,
		OrganizationShareService: shareService,
	}

	return handler, nil
}

// CreateShare tạo sharing giữa 2 organizations
// POST /api/v1/organization-shares
func (h *OrganizationShareHandler) CreateShare(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var input dto.OrganizationShareCreateInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Validate ObjectIDs
		ownerOrgID, err := primitive.ObjectIDFromHex(input.OwnerOrganizationID)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("ownerOrganizationId không hợp lệ: %v", err),
				common.StatusBadRequest,
				err,
			))
			return nil
		}

		// Parse ToOrgIDs từ mảng string sang mảng ObjectID
		var toOrgIDs []primitive.ObjectID
		if len(input.ToOrgIDs) > 0 {
			toOrgIDs = make([]primitive.ObjectID, 0, len(input.ToOrgIDs))
			for _, toOrgIDStr := range input.ToOrgIDs {
				toOrgID, err := primitive.ObjectIDFromHex(toOrgIDStr)
				if err != nil {
					h.HandleResponse(c, nil, common.NewError(
						common.ErrCodeValidationFormat,
						fmt.Sprintf("toOrgIds chứa ID không hợp lệ: %v", err),
						common.StatusBadRequest,
						err,
					))
					return nil
				}
				// Validate: ownerOrgID không được có trong ToOrgIDs
				if toOrgID == ownerOrgID {
					h.HandleResponse(c, nil, common.NewError(
						common.ErrCodeValidationInput,
						"ownerOrganizationId không được có trong toOrgIds",
						common.StatusBadRequest,
						nil,
					))
					return nil
				}
				toOrgIDs = append(toOrgIDs, toOrgID)
			}
		}
		// Nếu ToOrgIDs rỗng hoặc null → share với tất cả (để toOrgIDs = nil hoặc [])

		// Validate: user có quyền share data của ownerOrg
		userIDStr, ok := c.Locals("user_id").(string)
		if !ok {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeAuth,
				"Không tìm thấy user ID",
				common.StatusUnauthorized,
				nil,
			))
			return nil
		}
		userID, _ := primitive.ObjectIDFromHex(userIDStr)

		// Kiểm tra user có quyền truy cập ownerOrg không
		allowedOrgIDs, err := services.GetUserAllowedOrganizationIDs(c.Context(), userID, "")
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		hasAccess := false
		for _, orgID := range allowedOrgIDs {
			if orgID == ownerOrgID {
				hasAccess = true
				break
			}
		}

		if !hasAccess {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeAuth,
				"Bạn không có quyền share data của organization này",
				common.StatusForbidden,
				nil,
			))
			return nil
		}

		// Kiểm tra share đã tồn tại chưa
		// Query tất cả shares có cùng ownerOrgID và so sánh thủ công
		existingShares, err := h.OrganizationShareService.Find(c.Context(), bson.M{
			"ownerOrganizationId": ownerOrgID,
		}, nil)
		if err != nil && err != common.ErrNotFound {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// So sánh với shares hiện có
		for _, existingShare := range existingShares {
			// So sánh ToOrgIDs (không quan tâm thứ tự)
			if len(toOrgIDs) == 0 {
				// Share với tất cả: kiểm tra xem share hiện có cũng share với tất cả không
				if len(existingShare.ToOrgIDs) == 0 {
					// Cả 2 đều share với tất cả, kiểm tra PermissionNames
					if comparePermissionNames(input.PermissionNames, existingShare.PermissionNames) {
						h.HandleResponse(c, nil, common.NewError(
							common.ErrCodeBusinessOperation,
							"Share với tất cả organizations đã tồn tại cho organization này với cùng permissions",
							common.StatusConflict,
							nil,
						))
						return nil
					}
				}
			} else {
				// Share với orgs cụ thể: kiểm tra xem ToOrgIDs có giống nhau không (không quan tâm thứ tự)
				if len(existingShare.ToOrgIDs) == len(toOrgIDs) {
					// Tạo map để so sánh nhanh
					existingMap := make(map[primitive.ObjectID]bool)
					for _, id := range existingShare.ToOrgIDs {
						existingMap[id] = true
					}
					allMatch := true
					for _, id := range toOrgIDs {
						if !existingMap[id] {
							allMatch = false
							break
						}
					}
					if allMatch {
						// ToOrgIDs giống nhau, kiểm tra PermissionNames
						if comparePermissionNames(input.PermissionNames, existingShare.PermissionNames) {
							h.HandleResponse(c, nil, common.NewError(
								common.ErrCodeBusinessOperation,
								"Share với các organizations này đã tồn tại với cùng permissions",
								common.StatusConflict,
								nil,
							))
							return nil
						}
					}
				}
			}
		}

		// Tạo share record
		share := models.OrganizationShare{
			OwnerOrganizationID: ownerOrgID,
			ToOrgIDs:            toOrgIDs, // Mảng ObjectIDs (rỗng = share với tất cả)
			PermissionNames:     input.PermissionNames,
			Description:         input.Description, // Mô tả về lệnh share
			CreatedAt:           utility.CurrentTimeInMilli(),
			CreatedBy:           userID,
		}

		data, err := h.BaseService.InsertOne(c.Context(), share)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// DeleteShare xóa sharing
// DELETE /api/v1/organization-shares/:id
func (h *OrganizationShareHandler) DeleteShare(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		id := c.Params("id")
		if !primitive.IsValidObjectID(id) {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationFormat,
				fmt.Sprintf("ID không hợp lệ: %s", id),
				common.StatusBadRequest,
				nil,
			))
			return nil
		}
		shareID := utility.String2ObjectID(id)

		// Lấy share để kiểm tra quyền
		share, err := h.OrganizationShareService.FindOneById(c.Context(), shareID)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		// Validate: user có quyền xóa share này (phải là người tạo hoặc có quyền với fromOrg)
		userIDStr, ok := c.Locals("user_id").(string)
		if !ok {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeAuth,
				"Không tìm thấy user ID",
				common.StatusUnauthorized,
				nil,
			))
			return nil
		}
		userID, _ := primitive.ObjectIDFromHex(userIDStr)

		// Kiểm tra user có phải người tạo không
		if share.CreatedBy != userID {
			// Kiểm tra user có quyền với ownerOrg không
			allowedOrgIDs, err := services.GetUserAllowedOrganizationIDs(c.Context(), userID, "")
			if err != nil {
				h.HandleResponse(c, nil, err)
				return nil
			}

			hasAccess := false
			for _, orgID := range allowedOrgIDs {
				if orgID == share.OwnerOrganizationID {
					hasAccess = true
					break
				}
			}

			if !hasAccess {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeAuth,
					"Bạn không có quyền xóa share này",
					common.StatusForbidden,
					nil,
				))
				return nil
			}
		}

		// Xóa share
		err = h.BaseService.DeleteById(c.Context(), shareID)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		h.HandleResponse(c, map[string]interface{}{
			"message": "Xóa share thành công",
		}, nil)
		return nil
	})
}

// ListShares liệt kê các shares của organization
// GET /api/v1/organization-shares?ownerOrganizationId=xxx hoặc ?toOrgId=xxx
func (h *OrganizationShareHandler) ListShares(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		ownerOrgIDStr := c.Query("ownerOrganizationId")
		toOrgIDStr := c.Query("toOrgId")

		filter := bson.M{}

		// Filter theo ownerOrganizationID
		if ownerOrgIDStr != "" {
			ownerOrgID, err := primitive.ObjectIDFromHex(ownerOrgIDStr)
			if err != nil {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("ownerOrganizationId không hợp lệ: %v", err),
					common.StatusBadRequest,
					err,
				))
				return nil
			}

			// Validate: user có quyền xem shares của ownerOrg này
			userIDStr, ok := c.Locals("user_id").(string)
			if ok {
				userID, _ := primitive.ObjectIDFromHex(userIDStr)
				allowedOrgIDs, err := services.GetUserAllowedOrganizationIDs(c.Context(), userID, "")
				if err == nil {
					hasAccess := false
					for _, orgID := range allowedOrgIDs {
						if orgID == ownerOrgID {
							hasAccess = true
							break
						}
					}
					if !hasAccess {
						h.HandleResponse(c, nil, common.NewError(
							common.ErrCodeAuth,
							"Bạn không có quyền xem shares của organization này",
							common.StatusForbidden,
							nil,
						))
						return nil
					}
				}
			}

			filter["ownerOrganizationId"] = ownerOrgID
		}

		// Filter theo toOrgId (tìm shares có toOrgId này trong mảng ToOrgIDs)
		if toOrgIDStr != "" {
			toOrgID, err := primitive.ObjectIDFromHex(toOrgIDStr)
			if err != nil {
				h.HandleResponse(c, nil, common.NewError(
					common.ErrCodeValidationFormat,
					fmt.Sprintf("toOrgId không hợp lệ: %v", err),
					common.StatusBadRequest,
					err,
				))
				return nil
			}
			// Tìm shares có toOrgID trong mảng ToOrgIDs hoặc share với tất cả (ToOrgIDs rỗng)
			filter["$or"] = []bson.M{
				{"toOrgIds": toOrgID}, // ToOrgIDs chứa toOrgID
				{"$or": []bson.M{ // Share với tất cả
					{"toOrgIds": bson.M{"$exists": false}},
					{"toOrgIds": bson.M{"$size": 0}},
					{"toOrgIds": nil},
				}},
			}
		}

		// Nếu không có filter nào, trả về lỗi
		if len(filter) == 0 {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeValidationInput,
				"Cần cung cấp ít nhất một trong các tham số: ownerOrganizationId hoặc toOrgId",
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Query shares
		data, err := h.BaseService.Find(c.Context(), filter, nil)
		h.HandleResponse(c, data, err)
		return nil
	})
}

// comparePermissionNames so sánh 2 mảng PermissionNames (không quan tâm thứ tự)
// Trả về true nếu 2 mảng giống nhau (cùng elements, không quan tâm thứ tự)
func comparePermissionNames(perms1, perms2 []string) bool {
	if len(perms1) != len(perms2) {
		return false
	}
	if len(perms1) == 0 {
		return true // Cả 2 đều rỗng = giống nhau
	}
	// Tạo map để so sánh
	perms1Map := make(map[string]bool)
	for _, p := range perms1 {
		perms1Map[p] = true
	}
	for _, p := range perms2 {
		if !perms1Map[p] {
			return false
		}
	}
	return true
}
