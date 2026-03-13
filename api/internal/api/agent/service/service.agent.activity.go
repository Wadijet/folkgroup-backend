package agentsvc

import (
	"context"
	"fmt"
	"time"

	agentmodels "meta_commerce/internal/api/agent/models"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AgentActivityService xử lý logic cho agent activity logs
type AgentActivityService struct {
	*basesvc.BaseServiceMongoImpl[agentmodels.AgentActivityLog]
}

// NewAgentActivityService tạo mới AgentActivityService
func NewAgentActivityService() (*AgentActivityService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AgentActivityLogs)
	if !exist {
		return nil, fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.AgentActivityLogs)
	}
	return &AgentActivityService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[agentmodels.AgentActivityLog](collection),
	}, nil
}

// LogActivity log một activity
func (s *AgentActivityService) LogActivity(ctx context.Context, agentRegistryID primitive.ObjectID, activityType string, data map[string]interface{}, severity string) error {
	now := time.Now().UnixMilli()
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

// DeleteOlderThan xóa các activity log có timestamp cũ hơn cutoff (Unix ms).
// Dùng raw DeleteMany để tránh load toàn bộ documents vào memory.
// Trả về số bản ghi đã xóa.
func (s *AgentActivityService) DeleteOlderThan(ctx context.Context, cutoffUnix int64) (int64, error) {
	filter := bson.M{"timestamp": bson.M{"$lt": cutoffUnix}}
	result, err := s.Collection().DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}
