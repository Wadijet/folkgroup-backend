package services

import (
	"context"
	"fmt"
	"time"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AgentActivityService xử lý logic cho agent activity logs
type AgentActivityService struct {
	*BaseServiceMongoImpl[models.AgentActivityLog]
}

// NewAgentActivityService tạo mới AgentActivityService
func NewAgentActivityService() (*AgentActivityService, error) {
	collection, exist := global.RegistryCollections.Get("agent_activity_logs")
	if !exist {
		return nil, fmt.Errorf("failed to get agent_activity_logs collection")
	}

	return &AgentActivityService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AgentActivityLog](collection),
	}, nil
}

// LogActivity log một activity
func (s *AgentActivityService) LogActivity(ctx context.Context, agentRegistryID primitive.ObjectID, activityType string, data map[string]interface{}, severity string) error {
	now := time.Now().Unix()

	activity := models.AgentActivityLog{
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
