// Package router đăng ký các route thuộc domain Agent: Check-in, Registry, Config, Command, Activity.
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	agenthdl "meta_commerce/internal/api/agent/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký tất cả route agent management lên v1.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	agentManagementHandler, err := agenthdl.NewAgentManagementHandler()
	if err != nil {
		return fmt.Errorf("create agent management handler: %w", err)
	}
	checkInMiddleware := middleware.AuthMiddleware("AgentManagement.CheckIn")
	apirouter.RegisterRouteWithMiddleware(v1, "/agent-management", "POST", "/check-in", []fiber.Handler{checkInMiddleware}, agentManagementHandler.HandleEnhancedCheckIn)

	agentRegistryHandler, err := agenthdl.NewAgentRegistryHandler()
	if err != nil {
		return fmt.Errorf("create agent registry handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/agent-management/registry", agentRegistryHandler, apirouter.ReadWriteConfig, "AgentRegistry")

	agentConfigHandler, err := agenthdl.NewAgentConfigHandler()
	if err != nil {
		return fmt.Errorf("create agent config handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/agent-management/config", agentConfigHandler, apirouter.ReadWriteConfig, "AgentConfig")
	configUpdateMiddleware := middleware.AuthMiddleware("AgentConfig.Update")
	apirouter.RegisterRouteWithMiddleware(v1, "/agent-management/config", "PUT", "/:agentId/update-data", []fiber.Handler{configUpdateMiddleware}, agentConfigHandler.HandleUpdateConfigData)

	agentCommandHandler, err := agenthdl.NewAgentCommandHandler()
	if err != nil {
		return fmt.Errorf("create agent command handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/agent-management/command", agentCommandHandler, apirouter.ReadWriteConfig, "AgentCommand")
	claimAgentCommandsMiddleware := middleware.AuthMiddleware("AgentCommand.Update")
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	apirouter.RegisterRouteWithMiddleware(v1, "/agent-management/command", "POST", "/claim-pending", []fiber.Handler{claimAgentCommandsMiddleware, orgContextMiddleware}, agentCommandHandler.ClaimPendingCommands)
	updateAgentHeartbeatMiddleware := middleware.AuthMiddleware("AgentCommand.Update")
	apirouter.RegisterRouteWithMiddleware(v1, "/agent-management/command", "POST", "/update-heartbeat", []fiber.Handler{updateAgentHeartbeatMiddleware, orgContextMiddleware}, agentCommandHandler.UpdateHeartbeat)
	apirouter.RegisterRouteWithMiddleware(v1, "/agent-management/command", "POST", "/update-heartbeat/:commandId", []fiber.Handler{updateAgentHeartbeatMiddleware, orgContextMiddleware}, agentCommandHandler.UpdateHeartbeat)
	releaseStuckAgentCommandsMiddleware := middleware.AuthMiddleware("AgentCommand.Update")
	apirouter.RegisterRouteWithMiddleware(v1, "/agent-management/command", "POST", "/release-stuck", []fiber.Handler{releaseStuckAgentCommandsMiddleware, orgContextMiddleware}, agentCommandHandler.ReleaseStuckCommands)

	agentActivityHandler, err := agenthdl.NewAgentActivityLogHandler()
	if err != nil {
		return fmt.Errorf("create agent activity handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/agent-management/activity", agentActivityHandler, apirouter.ReadOnlyConfig, "AgentActivityLog")

	return nil
}
