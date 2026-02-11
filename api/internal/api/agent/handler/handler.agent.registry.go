package agenthdl

import (
	"fmt"
	agentdto "meta_commerce/internal/api/agent/dto"
	agentmodels "meta_commerce/internal/api/agent/models"
	agentsvc "meta_commerce/internal/api/agent/service"
	basehdl "meta_commerce/internal/api/base/handler"
)

// AgentRegistryHandler xử lý các route CRUD cho agent registry
type AgentRegistryHandler struct {
	*basehdl.BaseHandler[agentmodels.AgentRegistry, agentdto.AgentRegistryCreateInput, agentdto.AgentRegistryUpdateInput]
}

// NewAgentRegistryHandler tạo mới AgentRegistryHandler
func NewAgentRegistryHandler() (*AgentRegistryHandler, error) {
	registryService, err := agentsvc.NewAgentRegistryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create agent registry service: %w", err)
	}
	return &AgentRegistryHandler{
		BaseHandler: basehdl.NewBaseHandler[agentmodels.AgentRegistry, agentdto.AgentRegistryCreateInput, agentdto.AgentRegistryUpdateInput](registryService.BaseServiceMongoImpl),
	}, nil
}
