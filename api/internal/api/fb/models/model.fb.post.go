package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// FbPost đại diện cho một bài viết Facebook từ Pancake
type FbPost struct {
	ID          primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`                                                                          // ID của bài viết
	PageId      string                 `json:"pageId" bson:"pageId" index:"text" extract:"PanCakeData\\.page_id"`                                          // ID của trang (extract từ PanCakeData["page_id"])
	PostId      string                 `json:"postId" bson:"postId" index:"unique;text" extract:"PanCakeData\\.id"`                                        // ID của bài viết (extract từ PanCakeData["id"])
	InsertedAt  int64                  `json:"insertedAt" bson:"insertedAt" extract:"PanCakeData\\.inserted_at,converter=time,format=2006-01-02T15:04:05"` // Thời gian insert bài viết (extract từ PanCakeData["inserted_at"])
	PanCakeData map[string]interface{} `json:"panCakeData" bson:"panCakeData"`                                                                             // Dữ liệu API

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo bài viết
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật bài viết
}
