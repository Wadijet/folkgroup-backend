// Package router đăng ký các route thuộc domain PC (Pancake): AccessToken, Order, POS Customer/Shop/Warehouse/Product/Variation/Category/Order.
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	pchdl "meta_commerce/internal/api/pc/handler"
	apirouter "meta_commerce/internal/api/router"
)

// Register đăng ký tất cả route PC (Pancake) lên v1.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	accessTokenHandler, err := pchdl.NewAccessTokenHandler()
	if err != nil {
		return fmt.Errorf("create access token handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/access-token", accessTokenHandler, apirouter.ReadWriteConfig, "AccessToken")

	pcOrderHandler, err := pchdl.NewPcOrderHandler()
	if err != nil {
		return fmt.Errorf("create pancake order handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/pancake/order", pcOrderHandler, apirouter.ReadWriteConfig, "PcOrder")

	pcPosCustomerHandler, err := pchdl.NewPcPosCustomerHandler()
	if err != nil {
		return fmt.Errorf("create pc pos customer handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/pc-pos-customer", pcPosCustomerHandler, apirouter.ReadWriteConfig, "PcPosCustomer")

	pcPosShopHandler, err := pchdl.NewPcPosShopHandler()
	if err != nil {
		return fmt.Errorf("create pancake pos shop handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/pancake-pos/shop", pcPosShopHandler, apirouter.ReadWriteConfig, "PcPosShop")

	pcPosWarehouseHandler, err := pchdl.NewPcPosWarehouseHandler()
	if err != nil {
		return fmt.Errorf("create pancake pos warehouse handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/pancake-pos/warehouse", pcPosWarehouseHandler, apirouter.ReadWriteConfig, "PcPosWarehouse")

	pcPosProductHandler, err := pchdl.NewPcPosProductHandler()
	if err != nil {
		return fmt.Errorf("create pancake pos product handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/pancake-pos/product", pcPosProductHandler, apirouter.ReadWriteConfig, "PcPosProduct")

	pcPosVariationHandler, err := pchdl.NewPcPosVariationHandler()
	if err != nil {
		return fmt.Errorf("create pancake pos variation handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/pancake-pos/variation", pcPosVariationHandler, apirouter.ReadWriteConfig, "PcPosVariation")

	pcPosCategoryHandler, err := pchdl.NewPcPosCategoryHandler()
	if err != nil {
		return fmt.Errorf("create pancake pos category handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/pancake-pos/category", pcPosCategoryHandler, apirouter.ReadWriteConfig, "PcPosCategory")

	pcPosOrderHandler, err := pchdl.NewPcPosOrderHandler()
	if err != nil {
		return fmt.Errorf("create pancake pos order handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/pancake-pos/order", pcPosOrderHandler, apirouter.ReadWriteConfig, "PcPosOrder")

	return nil
}
