package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// AgentRegistryHandler xử lý các route CRUD cho agent registry
// Kế thừa từ BaseHandler để có sẵn các method CRUD
type AgentRegistryHandler struct {
	*BaseHandler[models.AgentRegistry, dto.AgentRegistryCreateInput, dto.AgentRegistryUpdateInput]
}

// NewAgentRegistryHandler tạo mới AgentRegistryHandler
// Returns:
//   - *AgentRegistryHandler: Instance mới của AgentRegistryHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAgentRegistryHandler() (*AgentRegistryHandler, error) {
	registryService, err := services.NewAgentRegistryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create agent registry service: %w", err)
	}

	return &AgentRegistryHandler{
		BaseHandler: NewBaseHandler[models.AgentRegistry, dto.AgentRegistryCreateInput, dto.AgentRegistryUpdateInput](registryService.BaseServiceMongoImpl),
	}, nil
}
