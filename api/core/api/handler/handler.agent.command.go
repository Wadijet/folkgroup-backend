package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// AgentCommandHandler xử lý các route CRUD cho agent command
// Kế thừa từ BaseHandler để có sẵn các method CRUD
type AgentCommandHandler struct {
	*BaseHandler[models.AgentCommand, dto.AgentCommandCreateInput, dto.AgentCommandUpdateInput]
}

// NewAgentCommandHandler tạo mới AgentCommandHandler
// Returns:
//   - *AgentCommandHandler: Instance mới của AgentCommandHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAgentCommandHandler() (*AgentCommandHandler, error) {
	commandService, err := services.NewAgentCommandService()
	if err != nil {
		return nil, fmt.Errorf("failed to create agent command service: %w", err)
	}

	return &AgentCommandHandler{
		BaseHandler: NewBaseHandler[models.AgentCommand, dto.AgentCommandCreateInput, dto.AgentCommandUpdateInput](commandService.BaseServiceMongoImpl),
	}, nil
}
