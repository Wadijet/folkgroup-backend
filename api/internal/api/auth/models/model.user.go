// Package models - model người dùng (User) thuộc domain auth.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User định nghĩa mô hình người dùng
// Token chứa token xác thực mới nhất của người dùng
// Tokens chứa danh sách các token, mỗi thiết bị khác nhau sẽ có một token riêng để xác thực (bằng hwid)
type User struct {
	_Relationships struct{}          `relationship:"collection:user_roles,field:userId,message:Không thể xóa user vì có %d role đang được gán cho user này. Vui lòng gỡ các role trước."`
	ID             primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name           string             `json:"name" bson:"name"`
	Email          string             `json:"email,omitempty" bson:"email,omitempty" index:"unique,sparse"`
	Password       string             `json:"-" bson:"password,omitempty"`
	Salt           string             `json:"-" bson:"salt,omitempty"`
	Phone          string             `json:"phone,omitempty" bson:"phone,omitempty" index:"unique,sparse"`
	FirebaseUID    string             `json:"firebaseUid" bson:"firebaseUid" index:"unique"`
	EmailVerified  bool               `json:"emailVerified" bson:"emailVerified"`
	PhoneVerified  bool               `json:"phoneVerified" bson:"phoneVerified"`
	AvatarURL      string             `json:"avatarUrl" bson:"avatarUrl"`
	Token          string             `json:"token" bson:"token"`
	Tokens         []Token            `json:"-" bson:"tokens"`
	IsBlock        bool               `json:"-" bson:"isBlock"`
	BlockNote      string             `json:"-" bson:"blockNote"`
	CreatedAt      int64              `json:"createdAt" bson:"createdAt"`
	UpdatedAt      int64              `json:"updatedAt" bson:"updatedAt"`
}

// UserPaginateResult đại diện cho kết quả phân trang User
type UserPaginateResult struct {
	Page      int64  `json:"page" bson:"page"`
	Limit     int64  `json:"limit" bson:"limit"`
	ItemCount int64  `json:"itemCount" bson:"itemCount"`
	Items     []User `json:"items" bson:"items"`
}
