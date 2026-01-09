package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"
	"meta_commerce/core/utility"

	"github.com/sirupsen/logrus"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// UserService là cấu trúc chứa các phương thức liên quan đến người dùng
type UserService struct {
	*BaseServiceMongoImpl[models.User]
	userRoleService *BaseServiceMongoImpl[models.UserRole]
	collection      *mongo.Collection // Lưu reference để insert trực tiếp với bson.M
}

// NewUserService tạo mới UserService
func NewUserService() (*UserService, error) {
	// Lấy collections từ registry mới
	userCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.Users)
	if !exist {
		return nil, fmt.Errorf("failed to get users collection: %v", common.ErrNotFound)
	}

	userRoleCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.UserRoles)
	if !exist {
		return nil, fmt.Errorf("failed to get user_roles collection: %v", common.ErrNotFound)
	}

	return &UserService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.User](userCollection),
		userRoleService:      NewBaseServiceMongo[models.UserRole](userRoleCollection),
		collection:           userCollection,
	}, nil
}

// Logout đăng xuất người dùng
func (s *UserService) Logout(ctx context.Context, userID primitive.ObjectID, input *dto.UserLogoutInput) error {
	// Tìm user
	user, err := s.BaseServiceMongoImpl.FindOneById(ctx, userID)
	if err != nil {
		return err
	}

	// Xóa token của hwid
	newTokens := make([]models.Token, 0)
	for _, t := range user.Tokens {
		if t.Hwid != input.Hwid {
			newTokens = append(newTokens, t)
		}
	}
	user.Tokens = newTokens
	user.Token = "" // Xóa token hiện tại
	user.UpdatedAt = time.Now().Unix()

	// Cập nhật user
	_, err = s.BaseServiceMongoImpl.UpdateById(ctx, userID, user)
	return err
}

// LoginWithFirebase đăng nhập bằng Firebase ID token
func (s *UserService) LoginWithFirebase(ctx context.Context, input *dto.FirebaseLoginInput) (*models.User, error) {
	// Đã tắt debug log để giảm log

	// 1. Verify Firebase ID token
	token, err := utility.VerifyIDToken(ctx, input.IDToken)
	if err != nil {
		logrus.WithError(err).Error("LoginWithFirebase: Lỗi verify Firebase ID token")
		return nil, common.NewError(
			common.ErrCodeAuthCredentials,
			"Token không hợp lệ",
			common.StatusUnauthorized,
			err,
		)
	}

	// 2. Lấy thông tin user từ Firebase
	firebaseUser, err := utility.GetUserByUID(ctx, token.UID)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"firebase_uid": token.UID,
			"error":        err.Error(),
		}).Error("LoginWithFirebase: Lỗi lấy thông tin user từ Firebase")
		return nil, err
	}

	// Đã tắt debug log để giảm log

	// 3. Kiểm tra conflict với email/phone trước khi upsert
	// (để tránh tạo user mới khi đã có user khác dùng email/phone này)
	var existingUser *models.User
	var foundBy string

	// Kiểm tra theo email nếu có
	if firebaseUser.Email != "" {
		emailFilter := bson.M{"email": firebaseUser.Email}
		// Đã tắt debug log để giảm log
		if emailUser, emailErr := s.BaseServiceMongoImpl.FindOne(ctx, emailFilter, nil); emailErr == nil {
			existingUser = &emailUser
			foundBy = "email"
		} else if !errors.Is(emailErr, common.ErrNotFound) {
			logrus.WithError(emailErr).Error("LoginWithFirebase: Lỗi khi tìm user theo email")
			return nil, emailErr
		}
	}

	// Kiểm tra theo phone nếu có và chưa tìm thấy user
	if existingUser == nil && firebaseUser.PhoneNumber != "" {
		phoneFilter := bson.M{"phone": firebaseUser.PhoneNumber}
		// Đã tắt debug log để giảm log
		if phoneUser, phoneErr := s.BaseServiceMongoImpl.FindOne(ctx, phoneFilter, nil); phoneErr == nil {
			existingUser = &phoneUser
			foundBy = "phone"
		} else if !errors.Is(phoneErr, common.ErrNotFound) {
			logrus.WithError(phoneErr).Error("LoginWithFirebase: Lỗi khi tìm user theo phone")
			return nil, phoneErr
		}
	}

	// 4. Nếu tìm thấy user đã tồn tại với email/phone, kiểm tra conflict
	if existingUser != nil {
		// Kiểm tra xem user này đã có firebaseUid chưa
		if existingUser.FirebaseUID != "" && existingUser.FirebaseUID != token.UID {
			// User này đã có firebaseUid khác - conflict
			var conflictField string
			if foundBy == "email" {
				conflictField = fmt.Sprintf("Email '%s'", firebaseUser.Email)
			} else {
				conflictField = fmt.Sprintf("Số điện thoại '%s'", firebaseUser.PhoneNumber)
			}
			logrus.WithFields(logrus.Fields{
				"existing_firebase_uid": existingUser.FirebaseUID,
				"new_firebase_uid":      token.UID,
				"found_by":              foundBy,
			}).Warn("LoginWithFirebase: Conflict - email/phone đã được sử dụng bởi tài khoản khác")
			return nil, common.NewError(
				common.ErrCodeAuthCredentials,
				fmt.Sprintf("%s đã được sử dụng bởi tài khoản khác. Vui lòng sử dụng %s khác hoặc đăng nhập bằng tài khoản cũ.", conflictField, foundBy),
				common.StatusConflict,
				nil,
			)
		}
		// User này chưa có firebaseUid hoặc firebaseUid trùng, sẽ update bằng upsert
		// Đã tắt debug log để giảm log
	}

	// 5. Chuẩn bị dữ liệu để upsert
	// Tạo update data với chỉ các field có dữ liệu (không set email/phone nếu rỗng)
	updateData := &UpdateData{
		Set: make(map[string]interface{}),
	}

	// Luôn set firebaseUid và các field bắt buộc
	updateData.Set["firebaseUid"] = token.UID
	updateData.Set["emailVerified"] = firebaseUser.EmailVerified
	updateData.Set["phoneVerified"] = firebaseUser.PhoneNumber != ""
	updateData.Set["isBlock"] = false
	updateData.Set["tokens"] = []models.Token{}
	updateData.Set["token"] = "" // Set token rỗng ban đầu, sẽ được cập nhật sau

	// Chỉ set các field không rỗng
	if firebaseUser.DisplayName != "" {
		updateData.Set["name"] = firebaseUser.DisplayName
	}
	if firebaseUser.PhotoURL != "" {
		updateData.Set["avatarUrl"] = firebaseUser.PhotoURL
	}
	// Chỉ set email nếu không rỗng (quan trọng cho sparse unique index)
	if firebaseUser.Email != "" {
		updateData.Set["email"] = firebaseUser.Email
	}
	// Chỉ set phone nếu không rỗng (quan trọng cho sparse unique index)
	if firebaseUser.PhoneNumber != "" {
		updateData.Set["phone"] = firebaseUser.PhoneNumber
	}

	// 6. Upsert với filter firebaseUid (hoặc _id nếu đã tìm thấy user)
	var filter bson.M
	var user models.User
	if existingUser != nil {
		// Nếu đã tìm thấy user, upsert với _id để update user đó
		filter = bson.M{"_id": existingUser.ID}
	} else {
		// Nếu chưa tìm thấy, upsert với firebaseUid để tạo mới hoặc update
		filter = bson.M{"firebaseUid": token.UID}
	}
	// Đã tắt debug log để giảm log

	user, err = s.BaseServiceMongoImpl.Upsert(ctx, filter, updateData)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"filter": filter,
			"error":  err.Error(),
		}).Error("LoginWithFirebase: Lỗi khi gọi Upsert")
		// Nếu bị lỗi duplicate (có thể do race condition), thử tìm lại user
		if errors.Is(err, common.ErrMongoDuplicate) {
			logrus.Warn("LoginWithFirebase: Lỗi duplicate, thử tìm lại user theo firebaseUid")
			// Thử tìm lại user theo firebaseUid
			firebaseFilter := bson.M{"firebaseUid": token.UID}
			if found, findErr := s.BaseServiceMongoImpl.FindOne(ctx, firebaseFilter, nil); findErr == nil {
				user = found
				// Đã tắt debug log để giảm log
			} else {
				logrus.WithError(findErr).Error("LoginWithFirebase: Không tìm thấy user sau lỗi duplicate")
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	// Đã tắt debug log để giảm log

	// 6. Kiểm tra user bị block
	if user.IsBlock {
		return nil, common.NewError(
			common.ErrCodeAuth,
			"Tài khoản đã bị khóa",
			common.StatusForbidden,
			nil,
		)
	}

	// 7. Tạo JWT token cho backend
	rdNumber := rand.Intn(100)
	currentTime := time.Now().Unix()

	tokenMap, err := utility.CreateToken(
		global.MongoDB_ServerConfig.JwtSecret,
		user.ID.Hex(),
		strconv.FormatInt(currentTime, 16),
		strconv.Itoa(rdNumber),
	)
	if err != nil {
		return nil, err
	}

	// 8. Cập nhật token vào user
	user.Token = tokenMap["token"]

	// Cập nhật hoặc thêm token vào tokens array (theo hwid)
	var idTokenExist int = -1
	for i, _token := range user.Tokens {
		if _token.Hwid == input.Hwid {
			idTokenExist = i
			break
		}
	}

	if idTokenExist == -1 {
		user.Tokens = append(user.Tokens, models.Token{
			Hwid:     input.Hwid,
			JwtToken: tokenMap["token"],
		})
	} else {
		user.Tokens[idTokenExist].JwtToken = tokenMap["token"]
	}
	// Đã tắt debug log để giảm log

	// 9. Lưu user - Sử dụng UpdateData để đảm bảo update đúng các field

	// Sử dụng UpdateData để update chỉ các field cần thiết
	tokenUpdateData := &UpdateData{
		Set: map[string]interface{}{
			"token":  user.Token,
			"tokens": user.Tokens,
		},
	}
	
	// Đã tắt debug log và force log để giảm log
	updatedUser, err := s.BaseServiceMongoImpl.UpdateById(ctx, user.ID, tokenUpdateData)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id": user.ID.Hex(),
			"error":   err.Error(),
		}).Error("LoginWithFirebase: Lỗi khi cập nhật token vào user")
		return nil, err
	}
	// Đã tắt debug log để giảm log

	// 10. Nếu chưa có admin nào, tự động set user đầu tiên làm admin
	// Đây là phương án phổ biến: "First user becomes admin"
	initService, err := NewInitService()
	if err == nil {
		hasAdmin, err := initService.HasAnyAdministrator()
		if err == nil && !hasAdmin {
			// Chưa có admin, tự động set user này làm admin
			logrus.WithFields(logrus.Fields{
				"user_id": updatedUser.ID.Hex(),
			}).Info("LoginWithFirebase: Tự động set user đầu tiên làm admin")
			_, err = initService.SetAdministrator(updatedUser.ID)
			if err != nil && err != common.ErrUserAlreadyAdmin {
				logrus.WithError(err).Warn("LoginWithFirebase: Lỗi khi set admin, nhưng không fail login")
				// Log warning nhưng không fail login
				// User vẫn có thể login, chỉ là chưa có quyền admin
				// Có thể set admin sau bằng cách khác
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"user_id": updatedUser.ID.Hex(),
		"email":   updatedUser.Email,
	}).Info("LoginWithFirebase: Đăng nhập thành công")

	return &updatedUser, nil
}

// getUpdateDataKeys lấy danh sách keys từ UpdateData
func getUpdateDataKeys(updateData *UpdateData) []string {
	if updateData == nil || updateData.Set == nil {
		return []string{}
	}
	keys := make([]string, 0, len(updateData.Set))
	for k := range updateData.Set {
		keys = append(keys, k)
	}
	return keys
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
