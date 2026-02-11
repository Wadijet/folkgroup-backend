package agentsvc

import (
	"context"
	"fmt"
	"time"

	agentmodels "meta_commerce/internal/api/agent/models"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AgentActivityService xử lý logic cho agent activity logs
type AgentActivityService struct {
	*basesvc.BaseServiceMongoImpl[agentmodels.AgentActivityLog]
}

// NewAgentActivityService tạo mới AgentActivityService
func NewAgentActivityService() (*AgentActivityService, error) {
	collection, exist := global.RegistryCollections.Get("agent_activity_logs")
	if !exist {
		return nil, fmt.Errorf("failed to get agent_activity_logs collection")
	}
	return &AgentActivityService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[agentmodels.AgentActivityLog](collection),
	}, nil
}

// LogActivity log một activity
func (s *AgentActivityService) LogActivity(ctx context.Context, agentRegistryID primitive.ObjectID, activityType string, data map[string]interface{}, severity string) error {
	now := time.Now().Unix()
	activity := agentmodels.AgentActivityLog{
		ID:           primitive.NewObjectID(),
		AgentID:      agentRegistryID,
		ActivityType: activityType,
		Timestamp:    now,
		Data:         data,
		Severity:     severity,
	}
	_, err := s.BaseServiceMongoImpl.InsertOne(ctx, activity)
	return err
}
