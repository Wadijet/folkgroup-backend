// Package router đăng ký các route thuộc domain AI: Workflows, Steps, PromptTemplates, ProviderProfiles, Runs, Commands.
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	aihdl "meta_commerce/internal/api/ai/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký tất cả route AI lên v1.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	aiWorkflowHandler, err := aihdl.NewAIWorkflowHandler()
	if err != nil {
		return fmt.Errorf("create AI workflow handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/ai/workflows", aiWorkflowHandler, apirouter.ReadWriteConfig, "AIWorkflows")

	aiStepHandler, err := aihdl.NewAIStepHandler()
	if err != nil {
		return fmt.Errorf("create AI step handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/ai/steps", aiStepHandler, apirouter.ReadWriteConfig, "AISteps")
	authMiddleware := middleware.AuthMiddleware("AISteps.Read")
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	apirouter.RegisterRouteWithMiddleware(v1, "/api/v2", "POST", "/ai/steps/:id/render-prompt", []fiber.Handler{authMiddleware, orgContextMiddleware}, aiStepHandler.RenderPrompt)

	aiPromptTemplateHandler, err := aihdl.NewAIPromptTemplateHandler()
	if err != nil {
		return fmt.Errorf("create AI prompt template handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/ai/prompt-templates", aiPromptTemplateHandler, apirouter.ReadWriteConfig, "AIPromptTemplates")

	aiProviderProfileHandler, err := aihdl.NewAIProviderProfileHandler()
	if err != nil {
		return fmt.Errorf("create AI provider profile handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/ai/provider-profiles", aiProviderProfileHandler, apirouter.ReadWriteConfig, "AIProviderProfiles")

	aiWorkflowRunHandler, err := aihdl.NewAIWorkflowRunHandler()
	if err != nil {
		return fmt.Errorf("create AI workflow run handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/ai/workflow-runs", aiWorkflowRunHandler, apirouter.ReadWriteConfig, "AIWorkflowRuns")

	aiStepRunHandler, err := aihdl.NewAIStepRunHandler()
	if err != nil {
		return fmt.Errorf("create AI step run handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/ai/step-runs", aiStepRunHandler, apirouter.ReadWriteConfig, "AIStepRuns")

	aiGenerationBatchHandler, err := aihdl.NewAIGenerationBatchHandler()
	if err != nil {
		return fmt.Errorf("create AI generation batch handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/ai/generation-batches", aiGenerationBatchHandler, apirouter.ReadWriteConfig, "AIGenerationBatches")

	aiCandidateHandler, err := aihdl.NewAICandidateHandler()
	if err != nil {
		return fmt.Errorf("create AI candidate handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/ai/candidates", aiCandidateHandler, apirouter.ReadWriteConfig, "AICandidates")

	aiRunHandler, err := aihdl.NewAIRunHandler()
	if err != nil {
		return fmt.Errorf("create AI run handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/ai/ai-runs", aiRunHandler, apirouter.ReadWriteConfig, "AIRuns")

	aiWorkflowCommandHandler, err := aihdl.NewAIWorkflowCommandHandler()
	if err != nil {
		return fmt.Errorf("create AI workflow command handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/ai/workflow-commands", aiWorkflowCommandHandler, apirouter.ReadWriteConfig, "AIWorkflowCommands")

	claimCommandsMiddleware := middleware.AuthMiddleware("AIWorkflowCommands.Update")
	apirouter.RegisterRouteWithMiddleware(v1, "/ai/workflow-commands", "POST", "/claim-pending", []fiber.Handler{claimCommandsMiddleware, orgContextMiddleware}, aiWorkflowCommandHandler.ClaimPendingCommands)
	updateHeartbeatMiddleware := middleware.AuthMiddleware("AIWorkflowCommands.Update")
	apirouter.RegisterRouteWithMiddleware(v1, "/ai/workflow-commands", "POST", "/update-heartbeat", []fiber.Handler{updateHeartbeatMiddleware, orgContextMiddleware}, aiWorkflowCommandHandler.UpdateHeartbeat)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai/workflow-commands", "POST", "/update-heartbeat/:commandId", []fiber.Handler{updateHeartbeatMiddleware, orgContextMiddleware}, aiWorkflowCommandHandler.UpdateHeartbeat)
	releaseStuckMiddleware := middleware.AuthMiddleware("AIWorkflowCommands.Update")
	apirouter.RegisterRouteWithMiddleware(v1, "/ai/workflow-commands", "POST", "/release-stuck", []fiber.Handler{releaseStuckMiddleware, orgContextMiddleware}, aiWorkflowCommandHandler.ReleaseStuckCommands)

	return nil
}
