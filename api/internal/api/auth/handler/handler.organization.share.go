package authhdl

import (
	"fmt"
	authdto "meta_commerce/internal/api/auth/dto"
	authsvc "meta_commerce/internal/api/auth/service"
	basehdl "meta_commerce/internal/api/base/handler"
	models "meta_commerce/internal/api/auth/models"
)

// OrganizationShareHandler xử lý các request liên quan đến Organization Share
type OrganizationShareHandler struct {
	*basehdl.BaseHandler[models.OrganizationShare, authdto.OrganizationShareCreateInput, authdto.OrganizationShareUpdateInput]
	OrganizationShareService *authsvc.OrganizationShareService
}

// NewOrganizationShareHandler tạo mới OrganizationShareHandler
func NewOrganizationShareHandler() (*OrganizationShareHandler, error) {
	shareService, err := authsvc.NewOrganizationShareService()
	if err != nil {
		return nil, fmt.Errorf("failed to create organization share service: %v", err)
	}
	base := basehdl.NewBaseHandler[models.OrganizationShare, authdto.OrganizationShareCreateInput, authdto.OrganizationShareUpdateInput](shareService)
	return &OrganizationShareHandler{
		BaseHandler:              base,
		OrganizationShareService: shareService,
	}, nil
}
