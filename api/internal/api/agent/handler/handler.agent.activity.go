package agenthdl

import (
	"fmt"
	agentdto "meta_commerce/internal/api/agent/dto"
	agentmodels "meta_commerce/internal/api/agent/models"
	agentsvc "meta_commerce/internal/api/agent/service"
	basehdl "meta_commerce/internal/api/base/handler"
)

// AgentActivityLogHandler xử lý các route CRUD cho agent activity log
type AgentActivityLogHandler struct {
	*basehdl.BaseHandler[agentmodels.AgentActivityLog, agentdto.AgentActivityLogCreateInput, agentdto.AgentActivityLogUpdateInput]
}

// NewAgentActivityLogHandler tạo mới AgentActivityLogHandler
func NewAgentActivityLogHandler() (*AgentActivityLogHandler, error) {
	activityService, err := agentsvc.NewAgentActivityService()
	if err != nil {
		return nil, fmt.Errorf("failed to create agent activity service: %w", err)
	}
	return &AgentActivityLogHandler{
		BaseHandler: basehdl.NewBaseHandler[agentmodels.AgentActivityLog, agentdto.AgentActivityLogCreateInput, agentdto.AgentActivityLogUpdateInput](activityService.BaseServiceMongoImpl),
	}, nil
}
