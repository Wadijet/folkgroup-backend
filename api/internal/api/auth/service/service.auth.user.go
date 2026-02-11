// Package authsvc - service người dùng (User).
package authsvc

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	authdto "meta_commerce/internal/api/auth/dto"
	models "meta_commerce/internal/api/auth/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"

	"github.com/sirupsen/logrus"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UserService là cấu trúc chứa các phương thức liên quan đến người dùng
type UserService struct {
	*basesvc.BaseServiceMongoImpl[models.User]
	userRoleService *basesvc.BaseServiceMongoImpl[models.UserRole]
}

// NewUserService tạo mới UserService
func NewUserService() (*UserService, error) {
	userCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.Users)
	if !exist {
		return nil, fmt.Errorf("failed to get users collection: %v", common.ErrNotFound)
	}
	userRoleCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.UserRoles)
	if !exist {
		return nil, fmt.Errorf("failed to get user_roles collection: %v", common.ErrNotFound)
	}

	return &UserService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[models.User](userCollection),
		userRoleService:      basesvc.NewBaseServiceMongo[models.UserRole](userRoleCollection),
	}, nil
}

// Logout đăng xuất người dùng (xóa token theo hwid)
func (s *UserService) Logout(ctx context.Context, userID primitive.ObjectID, input *authdto.UserLogoutInput) error {
	user, err := s.BaseServiceMongoImpl.FindOneById(ctx, userID)
	if err != nil {
		return err
	}
	newTokens := make([]models.Token, 0)
	for _, t := range user.Tokens {
		if t.Hwid != input.Hwid {
			newTokens = append(newTokens, t)
		}
	}
	updateData := &basesvc.UpdateData{
		Set: map[string]interface{}{
			"tokens": newTokens,
			"token":  "",
		},
	}
	_, err = s.BaseServiceMongoImpl.UpdateById(ctx, userID, updateData)
	return err
}

// LoginWithFirebase đăng nhập bằng Firebase ID token
func (s *UserService) LoginWithFirebase(ctx context.Context, input *authdto.FirebaseLoginInput) (*models.User, error) {
	token, err := utility.VerifyIDToken(ctx, input.IDToken)
	if err != nil {
		logrus.WithError(err).Error("LoginWithFirebase: Lỗi verify Firebase ID token")
		return nil, common.NewError(common.ErrCodeAuthCredentials, "Token không hợp lệ", common.StatusUnauthorized, err)
	}

	firebaseUser, err := utility.GetUserByUID(ctx, token.UID)
	if err != nil {
		logrus.WithFields(logrus.Fields{"firebase_uid": token.UID, "error": err.Error()}).Error("LoginWithFirebase: Lỗi lấy thông tin user từ Firebase")
		return nil, err
	}

	var existingUser *models.User
	var foundBy string
	if firebaseUser.Email != "" {
		emailFilter := bson.M{"email": firebaseUser.Email}
		if emailUser, emailErr := s.BaseServiceMongoImpl.FindOne(ctx, emailFilter, nil); emailErr == nil {
			existingUser = &emailUser
			foundBy = "email"
		} else if !errors.Is(emailErr, common.ErrNotFound) {
			logrus.WithError(emailErr).Error("LoginWithFirebase: Lỗi khi tìm user theo email")
			return nil, emailErr
		}
	}
	if existingUser == nil && firebaseUser.PhoneNumber != "" {
		phoneFilter := bson.M{"phone": firebaseUser.PhoneNumber}
		if phoneUser, phoneErr := s.BaseServiceMongoImpl.FindOne(ctx, phoneFilter, nil); phoneErr == nil {
			existingUser = &phoneUser
			foundBy = "phone"
		} else if !errors.Is(phoneErr, common.ErrNotFound) {
			logrus.WithError(phoneErr).Error("LoginWithFirebase: Lỗi khi tìm user theo phone")
			return nil, phoneErr
		}
	}

	if existingUser != nil {
		if existingUser.FirebaseUID != "" && existingUser.FirebaseUID != token.UID {
			var conflictField string
			if foundBy == "email" {
				conflictField = fmt.Sprintf("Email '%s'", firebaseUser.Email)
			} else {
				conflictField = fmt.Sprintf("Số điện thoại '%s'", firebaseUser.PhoneNumber)
			}
			logrus.WithFields(logrus.Fields{"existing_firebase_uid": existingUser.FirebaseUID, "new_firebase_uid": token.UID, "found_by": foundBy}).Warn("LoginWithFirebase: Conflict")
			return nil, common.NewError(common.ErrCodeAuthCredentials, conflictField+" đã được sử dụng bởi tài khoản khác. Vui lòng sử dụng "+foundBy+" khác hoặc đăng nhập bằng tài khoản cũ.", common.StatusConflict, nil)
		}
	}

	updateData := &basesvc.UpdateData{Set: make(map[string]interface{})}
	updateData.Set["firebaseUid"] = token.UID
	updateData.Set["emailVerified"] = firebaseUser.EmailVerified
	updateData.Set["phoneVerified"] = firebaseUser.PhoneNumber != ""
	updateData.Set["isBlock"] = false
	updateData.Set["tokens"] = []models.Token{}
	updateData.Set["token"] = ""

	if firebaseUser.DisplayName != "" {
		updateData.Set["name"] = firebaseUser.DisplayName
	}
	if firebaseUser.PhotoURL != "" {
		updateData.Set["avatarUrl"] = firebaseUser.PhotoURL
	}
	if firebaseUser.Email != "" {
		updateData.Set["email"] = firebaseUser.Email
	}
	if firebaseUser.PhoneNumber != "" {
		updateData.Set["phone"] = firebaseUser.PhoneNumber
	}

	var filter bson.M
	var user models.User
	if existingUser != nil {
		filter = bson.M{"_id": existingUser.ID}
	} else {
		filter = bson.M{"firebaseUid": token.UID}
	}

	user, err = s.BaseServiceMongoImpl.Upsert(ctx, filter, updateData)
	if err != nil {
		logrus.WithFields(logrus.Fields{"filter": filter, "error": err.Error()}).Error("LoginWithFirebase: Lỗi khi gọi Upsert")
		if errors.Is(err, common.ErrMongoDuplicate) {
			logrus.Warn("LoginWithFirebase: Lỗi duplicate, thử tìm lại user theo firebaseUid")
			firebaseFilter := bson.M{"firebaseUid": token.UID}
			if found, findErr := s.BaseServiceMongoImpl.FindOne(ctx, firebaseFilter, nil); findErr == nil {
				user = found
			} else {
				logrus.WithError(findErr).Error("LoginWithFirebase: Không tìm thấy user sau lỗi duplicate")
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	if user.IsBlock {
		return nil, common.NewError(common.ErrCodeAuth, "Tài khoản đã bị khóa", common.StatusForbidden, nil)
	}

	rdNumber := rand.Intn(100)
	currentTime := time.Now().Unix()
	tokenMap, err := utility.CreateToken(global.MongoDB_ServerConfig.JwtSecret, user.ID.Hex(), strconv.FormatInt(currentTime, 16), strconv.Itoa(rdNumber))
	if err != nil {
		return nil, err
	}

	user.Token = tokenMap["token"]
	var idTokenExist int = -1
	for i, _token := range user.Tokens {
		if _token.Hwid == input.Hwid {
			idTokenExist = i
			break
		}
	}
	if idTokenExist == -1 {
		user.Tokens = append(user.Tokens, models.Token{Hwid: input.Hwid, JwtToken: tokenMap["token"]})
	} else {
		user.Tokens[idTokenExist].JwtToken = tokenMap["token"]
	}

	tokenUpdateData := &basesvc.UpdateData{
		Set: map[string]interface{}{
			"token":  user.Token,
			"tokens": user.Tokens,
		},
	}
	updatedUser, err := s.BaseServiceMongoImpl.UpdateById(ctx, user.ID, tokenUpdateData)
	if err != nil {
		logrus.WithFields(logrus.Fields{"user_id": user.ID.Hex(), "error": err.Error()}).Error("LoginWithFirebase: Lỗi khi cập nhật token vào user")
		return nil, err
	}

	// Ghi chú: Logic "first user becomes admin" được xử lý ở auth handler (tránh import cycle authsvc -> services)

	logrus.WithFields(logrus.Fields{"user_id": updatedUser.ID.Hex(), "email": updatedUser.Email}).Info("LoginWithFirebase: Đăng nhập thành công")
	return &updatedUser, nil
}
