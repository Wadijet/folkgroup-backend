package notifsvc

import (
	"context"
	"fmt"

	notifmodels "meta_commerce/internal/api/notification/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NotificationChannelService là cấu trúc chứa các phương thức liên quan đến Notification Channel
type NotificationChannelService struct {
	*basesvc.BaseServiceMongoImpl[notifmodels.NotificationChannel]
}

// NewNotificationChannelService tạo mới NotificationChannelService
func NewNotificationChannelService() (*NotificationChannelService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.NotificationChannels)
	if !exist {
		return nil, fmt.Errorf("failed to get notification_channels collection: %v", common.ErrNotFound)
	}

	return &NotificationChannelService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[notifmodels.NotificationChannel](collection),
	}, nil
}

// FindByOrganizationID tìm tất cả channels của một organization, có thể filter theo channelTypes
func (s *NotificationChannelService) FindByOrganizationID(ctx context.Context, orgID primitive.ObjectID, channelTypes []string) ([]notifmodels.NotificationChannel, error) {
	filter := bson.M{
		"ownerOrganizationId": orgID,
		"isActive":             true,
	}

	if len(channelTypes) > 0 {
		filter["channelType"] = bson.M{"$in": channelTypes}
	}

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := s.Collection().Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var channels []notifmodels.NotificationChannel
	if err := cursor.All(ctx, &channels); err != nil {
		return nil, err
	}

	return channels, nil
}

// ValidateUniqueness validate uniqueness của notification channel (business logic validation)
func (s *NotificationChannelService) ValidateUniqueness(ctx context.Context, channel notifmodels.NotificationChannel) error {
	if channel.Name != "" && channel.ChannelType != "" && !channel.OwnerOrganizationID.IsZero() {
		filter := bson.M{
			"ownerOrganizationId": channel.OwnerOrganizationID,
			"channelType":         channel.ChannelType,
			"name":                channel.Name,
		}

		if !channel.ID.IsZero() {
			filter["_id"] = bson.M{"$ne": channel.ID}
		}

		existing, err := s.FindOne(ctx, filter, nil)
		if err == nil {
			return common.NewError(
				common.ErrCodeBusinessOperation,
				fmt.Sprintf("Đã tồn tại channel với tên '%s' và channelType '%s' trong organization này. Mỗi organization chỉ có thể có 1 channel với cùng tên và channelType", channel.Name, channel.ChannelType),
				common.StatusConflict,
				nil,
			)
		}
		if err != common.ErrNotFound {
			return fmt.Errorf("lỗi khi kiểm tra uniqueness name: %v", err)
		}
		_ = existing
	}

	if !channel.OwnerOrganizationID.IsZero() {
		if channel.ChannelType == "email" && len(channel.Recipients) > 0 {
			for _, recipient := range channel.Recipients {
				filter := bson.M{
					"ownerOrganizationId": channel.OwnerOrganizationID,
					"channelType":         "email",
					"recipients":          bson.M{"$in": []string{recipient}},
				}
				if !channel.ID.IsZero() {
					filter["_id"] = bson.M{"$ne": channel.ID}
				}
				existing, err := s.FindOne(ctx, filter, nil)
				if err == nil {
					return common.NewError(
						common.ErrCodeBusinessOperation,
						fmt.Sprintf("Đã tồn tại email channel với recipient '%s' trong organization này. Mỗi organization chỉ có thể có 1 channel cho mỗi recipient", recipient),
						common.StatusConflict,
						nil,
					)
				}
				if err != common.ErrNotFound {
					return fmt.Errorf("lỗi khi kiểm tra uniqueness recipient: %v", err)
				}
				_ = existing
			}
		}

		if channel.ChannelType == "telegram" && len(channel.ChatIDs) > 0 {
			for _, chatID := range channel.ChatIDs {
				filter := bson.M{
					"ownerOrganizationId": channel.OwnerOrganizationID,
					"channelType":         "telegram",
					"chatIds":             bson.M{"$in": []string{chatID}},
				}
				if !channel.ID.IsZero() {
					filter["_id"] = bson.M{"$ne": channel.ID}
				}
				existing, err := s.FindOne(ctx, filter, nil)
				if err == nil {
					return common.NewError(
						common.ErrCodeBusinessOperation,
						fmt.Sprintf("Đã tồn tại telegram channel với chatID '%s' trong organization này. Mỗi organization chỉ có thể có 1 channel cho mỗi chatID", chatID),
						common.StatusConflict,
						nil,
					)
				}
				if err != common.ErrNotFound {
					return fmt.Errorf("lỗi khi kiểm tra uniqueness chatID: %v", err)
				}
				_ = existing
			}
		}

		if channel.ChannelType == "webhook" && channel.WebhookURL != "" {
			filter := bson.M{
				"ownerOrganizationId": channel.OwnerOrganizationID,
				"channelType":         "webhook",
				"webhookUrl":          channel.WebhookURL,
			}
			if !channel.ID.IsZero() {
				filter["_id"] = bson.M{"$ne": channel.ID}
			}
			existing, err := s.FindOne(ctx, filter, nil)
			if err == nil {
				return common.NewError(
					common.ErrCodeBusinessOperation,
					fmt.Sprintf("Đã tồn tại webhook channel với webhookUrl '%s' trong organization này. Mỗi organization chỉ có thể có 1 channel cho mỗi webhookUrl", channel.WebhookURL),
					common.StatusConflict,
					nil,
				)
			}
			if err != common.ErrNotFound {
				return fmt.Errorf("lỗi khi kiểm tra uniqueness webhookUrl: %v", err)
			}
			_ = existing
		}
	}

	return nil
}

// InsertOne override để thêm business logic validation trước khi insert
func (s *NotificationChannelService) InsertOne(ctx context.Context, data notifmodels.NotificationChannel) (notifmodels.NotificationChannel, error) {
	if err := s.ValidateUniqueness(ctx, data); err != nil {
		return data, err
	}
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}
