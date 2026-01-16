package services

import (
	"context"
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// OrganizationService là cấu trúc chứa các phương thức liên quan đến tổ chức
type OrganizationService struct {
	*BaseServiceMongoImpl[models.Organization]
	roleService *RoleService
}

// NewOrganizationService tạo mới OrganizationService
func NewOrganizationService() (*OrganizationService, error) {
	organizationCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.Organizations)
	if !exist {
		return nil, fmt.Errorf("failed to get organizations collection: %v", common.ErrNotFound)
	}

	roleService, err := NewRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role service: %v", err)
	}

	return &OrganizationService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.Organization](organizationCollection),
		roleService:          roleService,
	}, nil
}

// GetChildrenIDs lấy tất cả ID của organization con (dùng cho Scope = 1)
func (s *OrganizationService) GetChildrenIDs(ctx context.Context, parentID primitive.ObjectID) ([]primitive.ObjectID, error) {
	// Lấy organization cha
	parent, err := s.FindOneById(ctx, parentID)
	if err != nil {
		return nil, err
	}

	// Query tất cả organization có Path bắt đầu với parent.Path
	filter := bson.M{
		"path":     bson.M{"$regex": "^" + parent.Path},
		"isActive": true,
	}

	orgs, err := s.Find(ctx, filter, nil)
	if err != nil {
		return nil, err
	}

	ids := make([]primitive.ObjectID, 0, len(orgs))
	for _, org := range orgs {
		ids = append(ids, org.ID)
	}

	return ids, nil
}

// GetParentIDs lấy tất cả ID của organization cha (dùng cho inverse lookup - xem dữ liệu cấp trên)
// Đi ngược lên cây organization để lấy tất cả parent IDs
func (s *OrganizationService) GetParentIDs(ctx context.Context, childID primitive.ObjectID) ([]primitive.ObjectID, error) {
	// Lấy organization con
	child, err := s.FindOneById(ctx, childID)
	if err != nil {
		return nil, err
	}

	// Nếu không có parent (root), trả về mảng rỗng
	if child.ParentID == nil {
		return []primitive.ObjectID{}, nil
	}

	parentIDs := make([]primitive.ObjectID, 0)
	currentID := *child.ParentID

	// Đi ngược lên cây để lấy tất cả parents
	for {
		parent, err := s.FindOneById(ctx, currentID)
		if err != nil {
			// Nếu không tìm thấy parent, dừng lại
			break
		}

		parentIDs = append(parentIDs, parent.ID)

		// Nếu không có parent nữa (đã đến root), dừng lại
		if parent.ParentID == nil {
			break
		}

		currentID = *parent.ParentID
	}

	return parentIDs, nil
}

// validateBeforeDelete kiểm tra các điều kiện trước khi xóa organization
// - Không cho phép xóa System organization
// - Không cho phép xóa nếu có tổ chức con
// - Không cho phép xóa nếu có role trực thuộc
func (s *OrganizationService) validateBeforeDelete(ctx context.Context, orgID primitive.ObjectID) error {
	// Lấy thông tin organization cần xóa
	org, err := s.FindOneById(ctx, orgID)
	if err != nil {
		return err
	}

	var modelOrg models.Organization
	bsonBytes, _ := bson.Marshal(org)
	err = bson.Unmarshal(bsonBytes, &modelOrg)
	if err != nil {
		return common.ErrInvalidFormat
	}

	// Kiểm tra 1: Nếu là System organization thì không cho phép xóa
	if modelOrg.Type == models.OrganizationTypeSystem && modelOrg.Code == "SYSTEM" && modelOrg.Level == -1 {
		return common.NewError(
			common.ErrCodeBusinessOperation,
			"Không thể xóa System organization. Đây là tổ chức cấp cao nhất chứa Administrator và không thể xóa.",
			common.StatusForbidden,
			nil,
		)
	}

	// Kiểm tra 2: Kiểm tra xem có tổ chức con không
	// Tìm các tổ chức có parentId = org.ID hoặc path bắt đầu với org.Path + "/"
	childrenFilter := bson.M{
		"$or": []bson.M{
			{"parentId": modelOrg.ID},
			{"path": bson.M{"$regex": "^" + modelOrg.Path + "/"}},
		},
	}
	children, err := s.Find(ctx, childrenFilter, nil)
	if err != nil && err != common.ErrNotFound {
		return err
	}
	if err == nil && len(children) > 0 {
		return common.NewError(
			common.ErrCodeBusinessOperation,
			fmt.Sprintf("Không thể xóa tổ chức '%s' vì có %d tổ chức con. Vui lòng xóa hoặc di chuyển các tổ chức con trước.", modelOrg.Name, len(children)),
			common.StatusConflict,
			nil,
		)
	}

	// Kiểm tra 3: Kiểm tra xem có role nào trực thuộc tổ chức này không
	rolesFilter := bson.M{
		"organizationId": modelOrg.ID,
	}
	roles, err := s.roleService.Find(ctx, rolesFilter, nil)
	if err != nil && err != common.ErrNotFound {
		return err
	}
	if err == nil && len(roles) > 0 {
		return common.NewError(
			common.ErrCodeBusinessOperation,
			fmt.Sprintf("Không thể xóa tổ chức '%s' vì có %d role trực thuộc. Vui lòng xóa hoặc di chuyển các role trước.", modelOrg.Name, len(roles)),
			common.StatusConflict,
			nil,
		)
	}

	return nil
}

// DeleteOne override method DeleteOne để kiểm tra trước khi xóa
func (s *OrganizationService) DeleteOne(ctx context.Context, filter interface{}) error {
	// Lấy thông tin organization cần xóa
	org, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
	if err != nil {
		return err
	}

	var modelOrg models.Organization
	bsonBytes, _ := bson.Marshal(org)
	err = bson.Unmarshal(bsonBytes, &modelOrg)
	if err != nil {
		return common.ErrInvalidFormat
	}

	// Kiểm tra trước khi xóa
	if err := s.validateBeforeDelete(ctx, modelOrg.ID); err != nil {
		return err
	}

	// Thực hiện xóa nếu không có ràng buộc
	return s.BaseServiceMongoImpl.DeleteOne(ctx, filter)
}

// DeleteById override method DeleteById để kiểm tra trước khi xóa
func (s *OrganizationService) DeleteById(ctx context.Context, id primitive.ObjectID) error {
	// Kiểm tra trước khi xóa
	if err := s.validateBeforeDelete(ctx, id); err != nil {
		return err
	}

	// Thực hiện xóa nếu không có ràng buộc
	return s.BaseServiceMongoImpl.DeleteById(ctx, id)
}

// DeleteMany override method DeleteMany để kiểm tra trước khi xóa
func (s *OrganizationService) DeleteMany(ctx context.Context, filter interface{}) (int64, error) {
	// Lấy danh sách organizations sẽ bị xóa
	orgs, err := s.BaseServiceMongoImpl.Find(ctx, filter, nil)
	if err != nil && err != common.ErrNotFound {
		return 0, err
	}

	// Kiểm tra từng organization trước khi xóa
	for _, org := range orgs {
		var modelOrg models.Organization
		bsonBytes, _ := bson.Marshal(org)
		if err := bson.Unmarshal(bsonBytes, &modelOrg); err != nil {
			continue
		}

		// Kiểm tra trước khi xóa
		if err := s.validateBeforeDelete(ctx, modelOrg.ID); err != nil {
			return 0, err
		}
	}

	// Thực hiện xóa nếu không có ràng buộc
	return s.BaseServiceMongoImpl.DeleteMany(ctx, filter)
}

// FindOneAndDelete override method FindOneAndDelete để kiểm tra trước khi xóa
func (s *OrganizationService) FindOneAndDelete(ctx context.Context, filter interface{}, opts *mongoopts.FindOneAndDeleteOptions) (models.Organization, error) {
	var zero models.Organization

	// Lấy thông tin organization sẽ bị xóa
	org, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
	if err != nil {
		return zero, err
	}

	var modelOrg models.Organization
	bsonBytes, _ := bson.Marshal(org)
	err = bson.Unmarshal(bsonBytes, &modelOrg)
	if err != nil {
		return zero, common.ErrInvalidFormat
	}

	// Kiểm tra trước khi xóa
	if err := s.validateBeforeDelete(ctx, modelOrg.ID); err != nil {
		return zero, err
	}

	// Thực hiện xóa nếu không có ràng buộc
	return s.BaseServiceMongoImpl.FindOneAndDelete(ctx, filter, opts)
}

// CalculatePathAndLevel tính toán Path và Level cho organization dựa trên parent (business logic)
//
// LÝ DO PHẢI TẠO METHOD NÀY (không dùng CRUD base):
// 1. Business rules - Tính toán Path và Level:
//    - Nếu có ParentID: Query parent organization từ database để lấy Path và Level
//      + Tính Path mới: parent.Path + "/" + code
//      + Tính Level mới: dựa trên Type và parent.Level (sử dụng calculateLevel)
//    - Nếu không có ParentID:
//      + Chỉ có thể là "system" (Level = -1, Path = "/" + code) hoặc "group" (Level = 0, Path = "/" + code)
//      + Validate: các Type khác phải có parent
//
// 2. Business rules - Level calculation:
//    - System: Level = -1
//    - Group: Level = 0
//    - Company: Level = 1
//    - Department: Level = 2
//    - Division: Level = 3
//    - Team: Level = parentLevel + 1 (có thể là 4+)
//    - Các Type khác: Level = parentLevel + 1
//
// Tham số:
//   - ctx: Context
//   - org: Organization cần tính toán Path và Level
//
// Trả về:
//   - path: Path đã được tính toán
//   - level: Level đã được tính toán
//   - error: Lỗi nếu validation thất bại, nil nếu hợp lệ
func (s *OrganizationService) CalculatePathAndLevel(ctx context.Context, org models.Organization) (string, int, error) {
	// Nếu có ParentID, query parent để lấy Path và Level
	if org.ParentID != nil && !org.ParentID.IsZero() {
		parent, err := s.FindOneById(ctx, *org.ParentID)
		if err != nil {
			return "", 0, common.NewError(
				common.ErrCodeBusinessOperation,
				fmt.Sprintf("Không tìm thấy tổ chức cha với ID: %s", org.ParentID.Hex()),
				common.StatusBadRequest,
				err,
			)
		}

		var modelParent models.Organization
		bsonBytes, _ := bson.Marshal(parent)
		if err := bson.Unmarshal(bsonBytes, &modelParent); err != nil {
			return "", 0, common.ErrInvalidFormat
		}

		// Tính Path: parent.Path + "/" + code
		path := modelParent.Path + "/" + org.Code

		// Tính Level dựa trên Type
		level := s.calculateLevel(org.Type, modelParent.Level)

		return path, level, nil
	}

	// Không có parent - chỉ có thể là system hoặc group
	if org.Type == models.OrganizationTypeSystem {
		return "/" + org.Code, -1, nil
	} else if org.Type == models.OrganizationTypeGroup {
		return "/" + org.Code, 0, nil
	} else {
		return "", 0, common.NewError(
			common.ErrCodeBusinessOperation,
			fmt.Sprintf("Loại tổ chức '%s' phải có parent. Chỉ 'system' và 'group' mới có thể không có parent.", org.Type),
			common.StatusBadRequest,
			nil,
		)
	}
}

// calculateLevel tính toán Level dựa trên Type và Level của parent (helper method)
func (s *OrganizationService) calculateLevel(orgType string, parentLevel int) int {
	switch orgType {
	case models.OrganizationTypeSystem:
		return -1
	case models.OrganizationTypeGroup:
		return 0
	case models.OrganizationTypeCompany:
		return 1
	case models.OrganizationTypeDepartment:
		return 2
	case models.OrganizationTypeDivision:
		return 3
	case models.OrganizationTypeTeam:
		// Team có thể là Level 4+ tùy thuộc vào parent
		return parentLevel + 1
	default:
		// Mặc định tăng level lên 1 so với parent
		return parentLevel + 1
	}
}

// InsertOne override để thêm business logic validation và tính toán Path/Level trước khi insert
//
// LÝ DO PHẢI OVERRIDE (không dùng BaseServiceMongoImpl.InsertOne trực tiếp):
// 1. Business logic:
//    - Tính toán Path và Level dựa trên parent (business logic phức tạp)
//    - Validate ParentID tồn tại trong database (nếu có)
//    - Validate Type: chỉ "system" và "group" mới có thể không có parent
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Tính toán Path và Level bằng CalculatePathAndLevel()
// ✅ Gọi BaseServiceMongoImpl.InsertOne để đảm bảo:
//   - Set timestamps (CreatedAt, UpdatedAt)
//   - Generate ID nếu chưa có
//   - Insert vào MongoDB
func (s *OrganizationService) InsertOne(ctx context.Context, data models.Organization) (models.Organization, error) {
	// Tính toán Path và Level (business logic)
	path, level, err := s.CalculatePathAndLevel(ctx, data)
	if err != nil {
		return data, err
	}

	// Set Path và Level vào data
	data.Path = path
	data.Level = level

	// Gọi InsertOne của base service
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}
