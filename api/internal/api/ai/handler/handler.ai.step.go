package aihdl

import (
	"fmt"
	aidto "meta_commerce/internal/api/ai/dto"
	aimodels "meta_commerce/internal/api/ai/models"
	basehdl "meta_commerce/internal/api/base/handler"
	aisvc "meta_commerce/internal/api/ai/service"
	"meta_commerce/internal/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIStepHandler xử lý các request liên quan (Module 2)
type AIStepHandler struct {
	*basehdl.BaseHandler[aimodels.AIStep, aidto.AIStepCreateInput, aidto.AIStepUpdateInput]
	AIStepService *aisvc.AIStepService
}

// NewAIStepHandler tạo mới AIStepHandler
func NewAIStepHandler() (*AIStepHandler, error) {
	aiStepService, err := aisvc.NewAIStepService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI step service: %v", err)
	}

	hdl := &AIStepHandler{
		AIStepService: aiStepService,
	}
	hdl.BaseHandler = basehdl.NewBaseHandler[aimodels.AIStep, aidto.AIStepCreateInput, aidto.AIStepUpdateInput](aiStepService)

	return hdl, nil
}

// RenderPrompt render prompt template cho step với variables từ step input
// POST /api/v2/ai/steps/:id/render-prompt
func (h *AIStepHandler) RenderPrompt(c fiber.Ctx) error {
	return h.SafeHandler(c, func() error {
		var params aidto.AIStepRenderPromptParams
		if err := h.ParseRequestParams(c, &params); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
		stepID, _ := primitive.ObjectIDFromHex(params.ID)

		var input aidto.AIStepRenderPromptInput
		if err := h.ParseRequestBody(c, &input); err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}

		renderedPrompt, providerProfileID, provider, model, temperature, maxTokens, err := h.AIStepService.RenderPromptForStep(
			c.Context(),
			stepID,
			input.Variables,
		)
		if err != nil {
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeInternalServer,
				fmt.Sprintf("Lỗi khi render prompt: %v", err),
				common.StatusInternalServerError,
				err,
			))
			return nil
		}

		output := aidto.AIStepRenderPromptOutput{
			RenderedPrompt:    renderedPrompt,
			ProviderProfileID: providerProfileID,
			Provider:          provider,
			Model:             model,
			Temperature:       temperature,
			MaxTokens:         maxTokens,
			Variables:         input.Variables,
		}

		h.HandleResponse(c, output, nil)
		return nil
	})
}
