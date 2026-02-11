package authhdl

import (
	"fmt"
	authdto "meta_commerce/internal/api/auth/dto"
	authsvc "meta_commerce/internal/api/auth/service"
	basehdl "meta_commerce/internal/api/base/handler"
	models "meta_commerce/internal/api/auth/models"
)

// OrganizationHandler xử lý các request liên quan đến Organization
type OrganizationHandler struct {
	*basehdl.BaseHandler[models.Organization, authdto.OrganizationCreateInput, authdto.OrganizationUpdateInput]
	OrganizationService *authsvc.OrganizationService
}

// NewOrganizationHandler tạo mới OrganizationHandler
func NewOrganizationHandler() (*OrganizationHandler, error) {
	organizationService, err := authsvc.NewOrganizationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create organization service: %v", err)
	}
	base := basehdl.NewBaseHandler[models.Organization, authdto.OrganizationCreateInput, authdto.OrganizationUpdateInput](organizationService)
	h := &OrganizationHandler{
		BaseHandler:         base,
		OrganizationService: organizationService,
	}
	h.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields: []string{"password", "token", "secret", "key", "hash"},
	})
	return h, nil
}
