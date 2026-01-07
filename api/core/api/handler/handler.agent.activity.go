package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// AgentActivityLogHandler xử lý các route CRUD cho agent activity log
// Kế thừa từ BaseHandler để có sẵn các method CRUD
type AgentActivityLogHandler struct {
	*BaseHandler[models.AgentActivityLog, dto.AgentActivityLogCreateInput, dto.AgentActivityLogUpdateInput]
}

// NewAgentActivityLogHandler tạo mới AgentActivityLogHandler
// Returns:
//   - *AgentActivityLogHandler: Instance mới của AgentActivityLogHandler
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAgentActivityLogHandler() (*AgentActivityLogHandler, error) {
	activityService, err := services.NewAgentActivityService()
	if err != nil {
		return nil, fmt.Errorf("failed to create agent activity service: %w", err)
	}

	return &AgentActivityLogHandler{
		BaseHandler: NewBaseHandler[models.AgentActivityLog, dto.AgentActivityLogCreateInput, dto.AgentActivityLogUpdateInput](activityService.BaseServiceMongoImpl),
	}, nil
}
