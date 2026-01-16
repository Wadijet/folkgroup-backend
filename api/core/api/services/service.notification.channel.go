package services

import (
	"context"
	"fmt"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NotificationChannelService là cấu trúc chứa các phương thức liên quan đến Notification Channel
type NotificationChannelService struct {
	*BaseServiceMongoImpl[models.NotificationChannel]
}

// NewNotificationChannelService tạo mới NotificationChannelService
func NewNotificationChannelService() (*NotificationChannelService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.NotificationChannels)
	if !exist {
		return nil, fmt.Errorf("failed to get notification_channels collection: %v", common.ErrNotFound)
	}

	return &NotificationChannelService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.NotificationChannel](collection),
	}, nil
}

// FindByOrganizationID tìm tất cả channels của một organization, có thể filter theo channelTypes
func (s *NotificationChannelService) FindByOrganizationID(ctx context.Context, orgID primitive.ObjectID, channelTypes []string) ([]models.NotificationChannel, error) {
	filter := bson.M{
		"ownerOrganizationId": orgID, // Phân quyền dữ liệu
		"isActive":           true,
	}

	// Filter theo channelTypes nếu có
	if len(channelTypes) > 0 {
		filter["channelType"] = bson.M{"$in": channelTypes}
	}

	fmt.Printf("🔔 [NOTIFICATION] Querying channels with filter: orgID=%s, channelTypes=%v\n", orgID.Hex(), channelTypes)

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := s.BaseServiceMongoImpl.collection.Find(ctx, filter, opts)
	if err != nil {
		fmt.Printf("🔔 [NOTIFICATION] Error querying channels: %v\n", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	var channels []models.NotificationChannel
	if err := cursor.All(ctx, &channels); err != nil {
		fmt.Printf("🔔 [NOTIFICATION] Error reading channels: %v\n", err)
		return nil, err
	}

	fmt.Printf("🔔 [NOTIFICATION] Found %d channels for orgID %s\n", len(channels), orgID.Hex())
	return channels, nil
}

// ValidateUniqueness validate uniqueness của notification channel (business logic validation)
//
// LÝ DO PHẢI TẠO METHOD NÀY (không dùng CRUD base):
// 1. Business rules - Uniqueness constraints phức tạp:
//    a) Name + ChannelType + OwnerOrganizationID: Mỗi organization chỉ có thể có 1 channel với cùng tên và channelType
//    b) Email channels: Mỗi recipient trong mảng Recipients phải unique trong organization
//       - Check duplicate bằng MongoDB $in operator: recipients: {$in: [recipient]}
//       - Phải check tất cả channels (cả active và inactive) để tránh duplicate
//    c) Telegram channels: Mỗi chatID trong mảng ChatIDs phải unique trong organization
//       - Check duplicate bằng MongoDB $in operator: chatIds: {$in: [chatID]}
//       - Phải check tất cả channels (cả active và inactive) để tránh duplicate
//    d) Webhook channels: WebhookURL phải unique trong organization
//       - Check duplicate webhookUrl + ownerOrganizationId + channelType
//       - Phải check tất cả channels (cả active và inactive) để tránh duplicate
//
// Tham số:
//   - ctx: Context
//   - channel: Notification channel cần validate
//
// Trả về:
//   - error: Lỗi nếu validation thất bại (duplicate channel), nil nếu hợp lệ
func (s *NotificationChannelService) ValidateUniqueness(ctx context.Context, channel models.NotificationChannel) error {
	// 1. Validate Name + ChannelType + OwnerOrganizationID uniqueness
	if channel.Name != "" && channel.ChannelType != "" && !channel.OwnerOrganizationID.IsZero() {
		filter := bson.M{
			"ownerOrganizationId": channel.OwnerOrganizationID,
			"channelType":         channel.ChannelType,
			"name":                channel.Name,
			// Bỏ filter isActive - check tất cả channels (cả active và inactive) để tránh duplicate
		}
		
		// Nếu đang update, exclude chính document đó
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
		_ = existing // Tránh unused variable warning
	}

	// 2. Validate duplicate recipients/webhookUrl/chatIDs dựa trên channelType
	if !channel.OwnerOrganizationID.IsZero() {
		// Check duplicate recipients cho email
		if channel.ChannelType == "email" && len(channel.Recipients) > 0 {
			for _, recipient := range channel.Recipients {
				// Check trong array recipients (MongoDB $in operator)
				filter := bson.M{
					"ownerOrganizationId": channel.OwnerOrganizationID,
					"channelType":         "email",
					"recipients":          bson.M{"$in": []string{recipient}},
					// Bỏ filter isActive - check tất cả channels (cả active và inactive) để tránh duplicate
				}
				
				// Nếu đang update, exclude chính document đó
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
				_ = existing // Tránh unused variable warning
			}
		}

		// Check duplicate chatIDs cho telegram
		if channel.ChannelType == "telegram" && len(channel.ChatIDs) > 0 {
			for _, chatID := range channel.ChatIDs {
				// Check trong array chatIds (MongoDB $in operator)
				filter := bson.M{
					"ownerOrganizationId": channel.OwnerOrganizationID,
					"channelType":         "telegram",
					"chatIds":             bson.M{"$in": []string{chatID}},
					// Bỏ filter isActive - check tất cả channels (cả active và inactive) để tránh duplicate
				}
				
				// Nếu đang update, exclude chính document đó
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
				_ = existing // Tránh unused variable warning
			}
		}

		// Check duplicate webhookUrl cho webhook
		if channel.ChannelType == "webhook" && channel.WebhookURL != "" {
			filter := bson.M{
				"ownerOrganizationId": channel.OwnerOrganizationID,
				"channelType":         "webhook",
				"webhookUrl":          channel.WebhookURL,
				// Bỏ filter isActive - check tất cả channels (cả active và inactive) để tránh duplicate
			}
			
			// Nếu đang update, exclude chính document đó
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
			_ = existing // Tránh unused variable warning
		}
	}

	return nil
}

// InsertOne override để thêm business logic validation trước khi insert
//
// LÝ DO PHẢI OVERRIDE (không dùng BaseServiceMongoImpl.InsertOne trực tiếp):
// 1. Business logic validation:
//    - Validate uniqueness (Name + ChannelType + OwnerOrganizationID)
//    - Validate uniqueness recipients (email), chatIDs (telegram), webhookUrl (webhook)
//    - Đảm bảo không có duplicate channels trong cùng organization
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Validate uniqueness bằng ValidateUniqueness()
// ✅ Gọi BaseServiceMongoImpl.InsertOne để đảm bảo:
//   - Set timestamps (CreatedAt, UpdatedAt)
//   - Generate ID nếu chưa có
//   - Insert vào MongoDB
func (s *NotificationChannelService) InsertOne(ctx context.Context, data models.NotificationChannel) (models.NotificationChannel, error) {
	// Validate uniqueness (business logic validation)
	if err := s.ValidateUniqueness(ctx, data); err != nil {
		return data, err
	}

	// Gọi InsertOne của base service
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}

// ✅ Các method DeleteById, UpdateById đã được xử lý bởi BaseServiceMongoImpl
// với cơ chế bảo vệ dữ liệu hệ thống chung (IsSystem)

