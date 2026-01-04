package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CTATracking - CTA Click Tracking
// Lưu trữ lịch sử click vào CTA buttons
type CTATracking struct {
	ID                 primitive.ObjectID  `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID  `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu (required) - lấy từ NotificationHistory
	HistoryID          primitive.ObjectID  `json:"historyId" bson:"historyId" index:"single:1"`                        // ID của NotificationHistory
	CTAIndex           int                 `json:"ctaIndex" bson:"ctaIndex"`                                            // Index của CTA trong mảng CTAs (0-based)
	CTACode            string              `json:"ctaCode" bson:"ctaCode" index:"single:1"`                            // Mã CTA (từ CTALibrary.Code)
	ClickedAt          int64               `json:"clickedAt" bson:"clickedAt" index:"single:1"`                      // Thời gian click
	IPAddress          string              `json:"ipAddress,omitempty" bson:"ipAddress,omitempty"`                    // IP address của user
	UserAgent          string              `json:"userAgent,omitempty" bson:"userAgent,omitempty"`                   // User agent
	CreatedAt          int64               `json:"createdAt" bson:"createdAt"`
}
