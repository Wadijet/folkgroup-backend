// Package service — CixQueueService enqueue phân tích từ CIO event.
package service

import (
	"context"
	"fmt"
	"time"

	cixmodels "meta_commerce/internal/api/cix/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	basesvc "meta_commerce/internal/api/base/service"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CixQueueService service hàng đợi phân tích CIX.
type CixQueueService struct {
	*basesvc.BaseServiceMongoImpl[cixmodels.CixPendingAnalysis]
}

// NewCixQueueService tạo service mới.
func NewCixQueueService() (*CixQueueService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CixPendingAnalysis)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.CixPendingAnalysis, common.ErrNotFound)
	}
	return &CixQueueService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[cixmodels.CixPendingAnalysis](coll),
	}, nil
}

// EnqueueAnalysisInput input để enqueue job phân tích.
type EnqueueAnalysisInput struct {
	ConversationID      string
	CustomerID           string
	Channel              string
	CioEventUid          string
	OwnerOrganizationID  primitive.ObjectID
}

// EnqueueAnalysis thêm job vào cix_pending_analysis (upsert theo conversationId).
// Gọi từ CIO ingestion sau khi InsertOne cio_event (conversation_updated, message_updated).
func (s *CixQueueService) EnqueueAnalysis(ctx context.Context, input EnqueueAnalysisInput) error {
	if input.ConversationID == "" {
		return nil
	}
	now := time.Now().UnixMilli()
	job := cixmodels.CixPendingAnalysis{
		ConversationID:      input.ConversationID,
		CustomerID:          input.CustomerID,
		Channel:             input.Channel,
		CioEventUid:         input.CioEventUid,
		OwnerOrganizationID: input.OwnerOrganizationID,
		ProcessedAt:         nil,
		RetryCount:          0,
		CreatedAt:           now,
	}
	filter := bson.M{
		"conversationId":       job.ConversationID,
		"ownerOrganizationId": job.OwnerOrganizationID,
	}
	update := bson.M{
		"$set": bson.M{
			"customerId":    job.CustomerID,
			"channel":       job.Channel,
			"cioEventUid":   job.CioEventUid,
			"processedAt":  nil,
			"processError":  "",
			"retryCount":   job.RetryCount,
		},
		"$setOnInsert": bson.M{
			"createdAt": now,
		},
	}
	_, err := s.Collection().UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}
