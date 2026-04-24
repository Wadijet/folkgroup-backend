package pchdl

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	pcdto "meta_commerce/internal/api/pc/dto"
	pcmodels "meta_commerce/internal/api/pc/models"
	pcsvc "meta_commerce/internal/api/pc/service"
	"meta_commerce/internal/common"

	"github.com/gofiber/fiber/v3"
)

// ManualPosOrderHandler CRUD + CIO sync cho order_src_manual_orders.
type ManualPosOrderHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosOrder, pcdto.PcPosOrderCreateInput, pcdto.PcPosOrderCreateInput]
	ManualPosOrderService *pcsvc.ManualPosOrderService
}

func NewManualPosOrderHandler() (*ManualPosOrderHandler, error) {
	svc, err := pcsvc.NewManualPosOrderService()
	if err != nil {
		return nil, fmt.Errorf("manual pos order service: %w", err)
	}
	h := &ManualPosOrderHandler{ManualPosOrderService: svc}
	h.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosOrder, pcdto.PcPosOrderCreateInput, pcdto.PcPosOrderCreateInput](svc)
	return h, nil
}

func (h *ManualPosOrderHandler) SyncUpsertOneFromParts(c fiber.Ctx, filter map[string]interface{}, body []byte) error {
	result, skipped, err := h.ManualPosOrderService.RunSyncUpsertOneFromJSON(c.Context(), filter, body, h.GetActiveOrganizationID(c))
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	if skipped {
		return c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Bỏ qua (dữ liệu không thay đổi)", "data": nil, "skipped": true, "status": "success",
		})
	}
	h.HandleResponse(c, result, nil)
	return nil
}

// ManualPosProductHandler — order_src_manual_products.
type ManualPosProductHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosProduct, pcdto.PcPosProductCreateInput, pcdto.PcPosProductCreateInput]
	ManualPosProductService *pcsvc.ManualPosProductService
}

func NewManualPosProductHandler() (*ManualPosProductHandler, error) {
	svc, err := pcsvc.NewManualPosProductService()
	if err != nil {
		return nil, err
	}
	h := &ManualPosProductHandler{ManualPosProductService: svc}
	h.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosProduct, pcdto.PcPosProductCreateInput, pcdto.PcPosProductCreateInput](svc.BaseServiceMongoImpl)
	return h, nil
}

func (h *ManualPosProductHandler) SyncUpsertOneFromParts(c fiber.Ctx, filter map[string]interface{}, body []byte) error {
	result, skipped, err := h.ManualPosProductService.RunSyncUpsertOneFromJSON(c.Context(), filter, body, h.GetActiveOrganizationID(c))
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	if skipped {
		return c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Bỏ qua (dữ liệu không thay đổi)", "data": nil, "skipped": true, "status": "success",
		})
	}
	h.HandleResponse(c, result, nil)
	return nil
}

// ManualPosVariationHandler — order_src_manual_variations.
type ManualPosVariationHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosVariation, pcdto.PcPosVariationCreateInput, pcdto.PcPosVariationCreateInput]
	ManualPosVariationService *pcsvc.ManualPosVariationService
}

func NewManualPosVariationHandler() (*ManualPosVariationHandler, error) {
	svc, err := pcsvc.NewManualPosVariationService()
	if err != nil {
		return nil, err
	}
	h := &ManualPosVariationHandler{ManualPosVariationService: svc}
	h.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosVariation, pcdto.PcPosVariationCreateInput, pcdto.PcPosVariationCreateInput](svc.BaseServiceMongoImpl)
	return h, nil
}

func (h *ManualPosVariationHandler) SyncUpsertOneFromParts(c fiber.Ctx, filter map[string]interface{}, body []byte) error {
	result, skipped, err := h.ManualPosVariationService.RunSyncUpsertOneFromJSON(c.Context(), filter, body, h.GetActiveOrganizationID(c))
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	if skipped {
		return c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Bỏ qua (dữ liệu không thay đổi)", "data": nil, "skipped": true, "status": "success",
		})
	}
	h.HandleResponse(c, result, nil)
	return nil
}

// ManualPosCategoryHandler — order_src_manual_categories.
type ManualPosCategoryHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosCategory, pcdto.PcPosCategoryCreateInput, pcdto.PcPosCategoryCreateInput]
	ManualPosCategoryService *pcsvc.ManualPosCategoryService
}

func NewManualPosCategoryHandler() (*ManualPosCategoryHandler, error) {
	svc, err := pcsvc.NewManualPosCategoryService()
	if err != nil {
		return nil, err
	}
	h := &ManualPosCategoryHandler{ManualPosCategoryService: svc}
	h.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosCategory, pcdto.PcPosCategoryCreateInput, pcdto.PcPosCategoryCreateInput](svc.BaseServiceMongoImpl)
	return h, nil
}

func (h *ManualPosCategoryHandler) SyncUpsertOneFromParts(c fiber.Ctx, filter map[string]interface{}, body []byte) error {
	result, skipped, err := h.ManualPosCategoryService.RunSyncUpsertOneFromJSON(c.Context(), filter, body, h.GetActiveOrganizationID(c))
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	if skipped {
		return c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Bỏ qua (dữ liệu không thay đổi)", "data": nil, "skipped": true, "status": "success",
		})
	}
	h.HandleResponse(c, result, nil)
	return nil
}

// ManualPosCustomerHandler — order_src_manual_customers.
type ManualPosCustomerHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosCustomer, pcdto.PcPosCustomerCreateInput, pcdto.PcPosCustomerCreateInput]
	ManualPosCustomerService *pcsvc.ManualPosCustomerService
}

func NewManualPosCustomerHandler() (*ManualPosCustomerHandler, error) {
	svc, err := pcsvc.NewManualPosCustomerService()
	if err != nil {
		return nil, err
	}
	h := &ManualPosCustomerHandler{ManualPosCustomerService: svc}
	h.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosCustomer, pcdto.PcPosCustomerCreateInput, pcdto.PcPosCustomerCreateInput](svc.BaseServiceMongoImpl)
	return h, nil
}

func (h *ManualPosCustomerHandler) SyncUpsertOneFromParts(c fiber.Ctx, filter map[string]interface{}, body []byte) error {
	result, skipped, err := h.ManualPosCustomerService.RunSyncUpsertOneFromJSON(c.Context(), filter, body, h.GetActiveOrganizationID(c))
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	if skipped {
		return c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Bỏ qua (dữ liệu không thay đổi)", "data": nil, "skipped": true, "status": "success",
		})
	}
	h.HandleResponse(c, result, nil)
	return nil
}

// ManualPosShopHandler — order_src_manual_shops.
type ManualPosShopHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosShop, pcdto.PcPosShopCreateInput, pcdto.PcPosShopCreateInput]
	ManualPosShopService *pcsvc.ManualPosShopService
}

func NewManualPosShopHandler() (*ManualPosShopHandler, error) {
	svc, err := pcsvc.NewManualPosShopService()
	if err != nil {
		return nil, err
	}
	h := &ManualPosShopHandler{ManualPosShopService: svc}
	h.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosShop, pcdto.PcPosShopCreateInput, pcdto.PcPosShopCreateInput](svc.BaseServiceMongoImpl)
	return h, nil
}

func (h *ManualPosShopHandler) SyncUpsertOneFromParts(c fiber.Ctx, filter map[string]interface{}, body []byte) error {
	result, skipped, err := h.ManualPosShopService.RunSyncUpsertOneFromJSON(c.Context(), filter, body, h.GetActiveOrganizationID(c))
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	if skipped {
		return c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Bỏ qua (dữ liệu không thay đổi)", "data": nil, "skipped": true, "status": "success",
		})
	}
	h.HandleResponse(c, result, nil)
	return nil
}

// ManualPosWarehouseHandler — order_src_manual_warehouses.
type ManualPosWarehouseHandler struct {
	*basehdl.BaseHandler[pcmodels.PcPosWarehouse, pcdto.PcPosWarehouseCreateInput, pcdto.PcPosWarehouseCreateInput]
	ManualPosWarehouseService *pcsvc.ManualPosWarehouseService
}

func NewManualPosWarehouseHandler() (*ManualPosWarehouseHandler, error) {
	svc, err := pcsvc.NewManualPosWarehouseService()
	if err != nil {
		return nil, err
	}
	h := &ManualPosWarehouseHandler{ManualPosWarehouseService: svc}
	h.BaseHandler = basehdl.NewBaseHandler[pcmodels.PcPosWarehouse, pcdto.PcPosWarehouseCreateInput, pcdto.PcPosWarehouseCreateInput](svc.BaseServiceMongoImpl)
	return h, nil
}

func (h *ManualPosWarehouseHandler) SyncUpsertOneFromParts(c fiber.Ctx, filter map[string]interface{}, body []byte) error {
	result, skipped, err := h.ManualPosWarehouseService.RunSyncUpsertOneFromJSON(c.Context(), filter, body, h.GetActiveOrganizationID(c))
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}
	if skipped {
		return c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Bỏ qua (dữ liệu không thay đổi)", "data": nil, "skipped": true, "status": "success",
		})
	}
	h.HandleResponse(c, result, nil)
	return nil
}
