package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// OrganizationShareHandler xử lý các request liên quan đến Organization Share
// Đã dùng CRUD chuẩn - logic nghiệp vụ (duplicate check, validation) đã được đưa vào service.InsertOne override
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
