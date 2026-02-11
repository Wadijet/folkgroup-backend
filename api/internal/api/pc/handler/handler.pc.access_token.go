package pchdl

import (
	"fmt"
	basehdl "meta_commerce/internal/api/base/handler"
	pcmodels "meta_commerce/internal/api/pc/models"
	pcdto "meta_commerce/internal/api/pc/dto"
	pcsvc "meta_commerce/internal/api/pc/service"
)

// AccessTokenHandler xử lý các route liên quan đến Access Token
type AccessTokenHandler struct {
	*basehdl.BaseHandler[pcmodels.AccessToken, pcdto.AccessTokenCreateInput, pcdto.AccessTokenUpdateInput]
	AccessTokenService *pcsvc.AccessTokenService
}

// NewAccessTokenHandler tạo AccessTokenHandler mới
func NewAccessTokenHandler() (*AccessTokenHandler, error) {
	service, err := pcsvc.NewAccessTokenService()
	if err != nil {
		return nil, fmt.Errorf("failed to create access token service: %v", err)
	}
	hdl := &AccessTokenHandler{AccessTokenService: service}
	hdl.BaseHandler = basehdl.NewBaseHandler[pcmodels.AccessToken, pcdto.AccessTokenCreateInput, pcdto.AccessTokenUpdateInput](service)
	return hdl, nil
}
