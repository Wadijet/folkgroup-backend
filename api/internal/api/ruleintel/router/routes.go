// Package router — Route cho Rule Intelligence.
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	ruleintelhdl "meta_commerce/internal/api/ruleintel/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký route Rule Intelligence lên v1.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	actionMiddleware := middleware.AuthMiddleware("MetaAdAccount.Update")
	readMiddleware := middleware.AuthMiddleware("MetaAdAccount.Read")

	// Run rule — tạo handler trong Register (sau InitRegistry), cùng luồng với CRUD handlers
	runHandler, err := ruleintelhdl.NewRunRuleHandler()
	if err != nil {
		return fmt.Errorf("tạo RunRuleHandler: %w", err)
	}
	apirouter.RegisterRouteWithMiddleware(v1, "/rule-intelligence/run", "POST", "", []fiber.Handler{actionMiddleware, orgContextMiddleware}, runHandler)

	// Xem rule execution log theo trace_id — link từ proposal "Xem log tạo đề xuất"
	logHandler, err := ruleintelhdl.NewGetTraceLogHandler()
	if err != nil {
		return fmt.Errorf("tạo GetTraceLogHandler: %w", err)
	}
	apirouter.RegisterRouteWithMiddleware(v1, "/rule-intelligence/logs", "GET", "/:traceId", []fiber.Handler{readMiddleware, orgContextMiddleware}, logHandler)

	// CRUD
	defHandler, err := ruleintelhdl.NewRuleDefinitionHandler()
	if err != nil {
		return fmt.Errorf("tạo RuleDefinitionHandler: %w", err)
	}
	logicHandler, err := ruleintelhdl.NewLogicScriptHandler()
	if err != nil {
		return fmt.Errorf("tạo LogicScriptHandler: %w", err)
	}
	paramHandler, err := ruleintelhdl.NewParamSetHandler()
	if err != nil {
		return fmt.Errorf("tạo ParamSetHandler: %w", err)
	}
	outputHandler, err := ruleintelhdl.NewOutputContractHandler()
	if err != nil {
		return fmt.Errorf("tạo OutputContractHandler: %w", err)
	}

	r.RegisterCRUDRoutes(v1, "/rule-intelligence/definition", defHandler, apirouter.ReadWriteConfig, "RuleDefinition")
	r.RegisterCRUDRoutes(v1, "/rule-intelligence/logic", logicHandler, apirouter.ReadWriteConfig, "LogicScript")
	r.RegisterCRUDRoutes(v1, "/rule-intelligence/param-set", paramHandler, apirouter.ReadWriteConfig, "ParamSet")
	r.RegisterCRUDRoutes(v1, "/rule-intelligence/output-contract", outputHandler, apirouter.ReadWriteConfig, "OutputContract")

	return nil
}
