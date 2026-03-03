// Package initsvc chứa InitService dùng để khởi tạo dữ liệu ban đầu (permissions, roles, org, notification, AI, ...).
// Tách ra package riêng để tránh import cycle giữa auth/service và services.
package initsvc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	aimodels "meta_commerce/internal/api/ai/models"
	aisvc "meta_commerce/internal/api/ai/service"
	authmodels "meta_commerce/internal/api/auth/models"
	authsvc "meta_commerce/internal/api/auth/service"
	basesvc "meta_commerce/internal/api/base/service"
	ctamodels "meta_commerce/internal/api/cta/models"
	notifmodels "meta_commerce/internal/api/notification/models"
	notifsvc "meta_commerce/internal/api/notification/service"
	reportmodels "meta_commerce/internal/api/report/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/utility"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CTALibraryIniter interface dùng cho init CTA Library (inject từ package cta để tránh import cycle)
type CTALibraryIniter interface {
	FindOne(ctx context.Context, filter interface{}, opts *options.FindOneOptions) (ctamodels.CTALibrary, error)
	InsertOne(ctx context.Context, data ctamodels.CTALibrary) (ctamodels.CTALibrary, error)
	UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts *options.UpdateOptions) (ctamodels.CTALibrary, error)
}

// InitService là cấu trúc chứa các phương thức khởi tạo dữ liệu ban đầu cho hệ thống
// Bao gồm khởi tạo người dùng, vai trò, quyền và các quan hệ giữa chúng
type InitService struct {
	userService                 *authsvc.UserService                 // Service xử lý người dùng
	roleService                 *authsvc.RoleService                 // Service xử lý vai trò
	permissionService           *authsvc.PermissionService           // Service xử lý quyền
	rolePermissionService       *authsvc.RolePermissionService       // Service xử lý quan hệ vai trò-quyền
	userRoleService             *authsvc.UserRoleService             // Service xử lý quan hệ người dùng-vai trò
	organizationService         *authsvc.OrganizationService         // Service xử lý tổ chức
	organizationShareService    *authsvc.OrganizationShareService    // Service xử lý organization share
	notificationSenderService   *notifsvc.NotificationSenderService   // Service xử lý notification sender
	notificationTemplateService *notifsvc.NotificationTemplateService // Service xử lý notification template
	notificationChannelService  *notifsvc.NotificationChannelService  // Service xử lý notification channel
	notificationRoutingService  *notifsvc.NotificationRoutingService  // Service xử lý notification routing
	ctaLibraryService           CTALibraryIniter                     // Service xử lý CTA Library (inject từ bên ngoài)
	aiProviderProfileService    *aisvc.AIProviderProfileService    // Service xử lý AI provider profiles
	aiPromptTemplateService     *aisvc.AIPromptTemplateService     // Service xử lý AI prompt templates
	aiStepService               *aisvc.AIStepService               // Service xử lý AI steps
	aiWorkflowService           *aisvc.AIWorkflowService           // Service xử lý AI workflows
	aiWorkflowCommandService    *aisvc.AIWorkflowCommandService    // Service xử lý AI workflow commands
}

// NewInitService tạo mới một đối tượng InitService
// Khởi tạo các service con cần thiết để xử lý các tác vụ liên quan
// Returns:
//   - *InitService: Instance mới của InitService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewInitService() (*InitService, error) {
	// Khởi tạo các auth services (từ domain auth)
	userService, err := authsvc.NewUserService()
	if err != nil {
		return nil, fmt.Errorf("failed to create user service: %v", err)
	}

	roleService, err := authsvc.NewRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role service: %v", err)
	}

	permissionService, err := authsvc.NewPermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create permission service: %v", err)
	}

	rolePermissionService, err := authsvc.NewRolePermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role permission service: %v", err)
	}

	userRoleService, err := authsvc.NewUserRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create user role service: %v", err)
	}

	organizationService, err := authsvc.NewOrganizationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create organization service: %v", err)
	}

	notificationSenderService, err := notifsvc.NewNotificationSenderService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification sender service: %v", err)
	}

	notificationTemplateService, err := notifsvc.NewNotificationTemplateService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification template service: %v", err)
	}

	notificationChannelService, err := notifsvc.NewNotificationChannelService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification channel service: %v", err)
	}

	notificationRoutingService, err := notifsvc.NewNotificationRoutingService()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification routing service: %v", err)
	}

	organizationShareService, err := authsvc.NewOrganizationShareService()
	if err != nil {
		return nil, fmt.Errorf("failed to create organization share service: %v", err)
	}

	// ctaLibraryService được inject từ bên ngoài (handler init) để tránh import cycle với package cta

	aiProviderProfileService, err := aisvc.NewAIProviderProfileService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI provider profile service: %v", err)
	}

	aiPromptTemplateService, err := aisvc.NewAIPromptTemplateService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI prompt template service: %v", err)
	}

	aiStepService, err := aisvc.NewAIStepService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI step service: %v", err)
	}

	aiWorkflowService, err := aisvc.NewAIWorkflowService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI workflow service: %v", err)
	}

	aiWorkflowCommandService, err := aisvc.NewAIWorkflowCommandService()
	if err != nil {
		return nil, fmt.Errorf("failed to create AI workflow command service: %v", err)
	}

	// Đăng ký callback kiểm tra admin cho base service (tránh import cycle services -> auth)
	basesvc.SetIsAdminFromContextFunc(authsvc.IsUserAdministratorFromContext)

	return &InitService{
		userService:                 userService,
		roleService:                 roleService,
		permissionService:           permissionService,
		rolePermissionService:       rolePermissionService,
		userRoleService:             userRoleService,
		organizationService:         organizationService,
		organizationShareService:    organizationShareService,
		notificationSenderService:   notificationSenderService,
		notificationTemplateService: notificationTemplateService,
		notificationChannelService:  notificationChannelService,
		notificationRoutingService:  notificationRoutingService,
		ctaLibraryService:           nil, // Inject qua SetCTALibraryService từ handler init
		aiProviderProfileService:    aiProviderProfileService,
		aiPromptTemplateService:     aiPromptTemplateService,
		aiStepService:               aiStepService,
		aiWorkflowService:           aiWorkflowService,
		aiWorkflowCommandService:    aiWorkflowCommandService,
	}, nil
}

// InitDefaultNotificationTeam khởi tạo team mặc định cho hệ thống notification
// Tạo team "Tech Team" thuộc System Organization và channel mặc định
// Returns:
//   - *authmodels.Organization: Team mặc định đã tạo
//   - error: Lỗi nếu có trong quá trình khởi tạo
func (h *InitService) InitDefaultNotificationTeam() (*authmodels.Organization, error) {
	// Sử dụng context cho phép insert system data trong quá trình init
	// Context cho phép insert system data trong init (từ package services)
	ctx := basesvc.WithSystemDataInsertAllowed(context.TODO())
	currentTime := time.Now().Unix()

	// Lấy System Organization
	systemOrg, err := h.GetRootOrganization()
	if err != nil {
		return nil, fmt.Errorf("failed to get system organization: %v", err)
	}

	// Kiểm tra team mặc định đã tồn tại chưa
	teamFilter := bson.M{
		"code":     "TECH_TEAM",
		"parentId": systemOrg.ID,
	}
	existingTeam, err := h.organizationService.BaseServiceMongoImpl.FindOne(ctx, teamFilter, nil)
	if err != nil && err != common.ErrNotFound {
		return nil, fmt.Errorf("failed to check existing tech team: %v", err)
	}

	var techTeam *authmodels.Organization
	if err == common.ErrNotFound {
		// Tạo mới Tech Team
		techTeamModel := authmodels.Organization{
			Name:      "Tech Team",
			Code:      "TECH_TEAM",
			Type:      authmodels.OrganizationTypeTeam,
			ParentID:  &systemOrg.ID,
			Path:      systemOrg.Path + "/TECH_TEAM",
			Level:     systemOrg.Level + 1, // Level = 0 (vì System là -1)
			IsActive:  true,
			IsSystem:  true, // Đánh dấu là dữ liệu hệ thống
			CreatedAt: currentTime,
			UpdatedAt: currentTime,
		}

		createdTeam, err := h.organizationService.BaseServiceMongoImpl.InsertOne(ctx, techTeamModel)
		if err != nil {
			return nil, fmt.Errorf("failed to create tech team: %v", err)
		}

		var modelTeam authmodels.Organization
		bsonBytes, _ := bson.Marshal(createdTeam)
		if err := bson.Unmarshal(bsonBytes, &modelTeam); err != nil {
			return nil, fmt.Errorf("failed to decode tech team: %v", err)
		}
		techTeam = &modelTeam
	} else {
		// Team đã tồn tại
		var modelTeam authmodels.Organization
		bsonBytes, _ := bson.Marshal(existingTeam)
		if err := bson.Unmarshal(bsonBytes, &modelTeam); err != nil {
			return nil, fmt.Errorf("failed to decode existing tech team: %v", err)
		}
		techTeam = &modelTeam
	}

	return techTeam, nil
}

// InitialPermissions định nghĩa danh sách các quyền mặc định của hệ thống
// Được chia thành các module: Auth (Xác thực) và Pancake (Quản lý trang Facebook)
var InitialPermissions = []authmodels.Permission{
	// ====================================  AUTH MODULE =============================================
	// Quản lý người dùng: Thêm, xem, sửa, xóa, khóa và phân quyền
	{Name: "User.Insert", Describe: "Quyền tạo người dùng", Group: "Auth", Category: "User"},
	{Name: "User.Read", Describe: "Quyền xem danh sách người dùng", Group: "Auth", Category: "User"},
	{Name: "User.Update", Describe: "Quyền cập nhật thông tin người dùng", Group: "Auth", Category: "User"},
	{Name: "User.Delete", Describe: "Quyền xóa người dùng", Group: "Auth", Category: "User"},
	{Name: "User.Block", Describe: "Quyền khóa/mở khóa người dùng", Group: "Auth", Category: "User"},
	{Name: "User.SetRole", Describe: "Quyền phân quyền cho người dùng", Group: "Auth", Category: "User"},

	// Quản lý tổ chức: Thêm, xem, sửa, xóa
	{Name: "Organization.Insert", Describe: "Quyền tạo tổ chức", Group: "Auth", Category: "Organization"},
	{Name: "Organization.Read", Describe: "Quyền xem danh sách tổ chức", Group: "Auth", Category: "Organization"},
	{Name: "Organization.Update", Describe: "Quyền cập nhật tổ chức", Group: "Auth", Category: "Organization"},
	{Name: "Organization.Delete", Describe: "Quyền xóa tổ chức", Group: "Auth", Category: "Organization"},

	// Quản lý cấu hình tổ chức: xem (raw/resolved), cập nhật, xóa config theo tổ chức
	{Name: "OrganizationConfig.Read", Describe: "Quyền xem cấu hình tổ chức (raw và resolved)", Group: "Auth", Category: "OrganizationConfig"},
	{Name: "OrganizationConfig.Update", Describe: "Quyền cập nhật cấu hình tổ chức", Group: "Auth", Category: "OrganizationConfig"},
	{Name: "OrganizationConfig.Delete", Describe: "Quyền xóa cấu hình tổ chức (không áp dụng cho config hệ thống)", Group: "Auth", Category: "OrganizationConfig"},

	// Quản lý chia sẻ dữ liệu giữa các tổ chức: Thêm, xem, sửa, xóa
	{Name: "OrganizationShare.Insert", Describe: "Quyền tạo chia sẻ dữ liệu giữa các tổ chức (CRUD)", Group: "Auth", Category: "OrganizationShare"},
	{Name: "OrganizationShare.Read", Describe: "Quyền xem danh sách chia sẻ dữ liệu giữa các tổ chức", Group: "Auth", Category: "OrganizationShare"},
	{Name: "OrganizationShare.Update", Describe: "Quyền cập nhật chia sẻ dữ liệu giữa các tổ chức", Group: "Auth", Category: "OrganizationShare"},
	{Name: "OrganizationShare.Delete", Describe: "Quyền xóa chia sẻ dữ liệu giữa các tổ chức", Group: "Auth", Category: "OrganizationShare"},
	// Quyền đặc biệt cho route CreateShare (có validation riêng về quyền với fromOrg)
	{Name: "OrganizationShare.Create", Describe: "Quyền tạo chia sẻ dữ liệu giữa các tổ chức (route đặc biệt)", Group: "Auth", Category: "OrganizationShare"},

	// Quản lý vai trò: Thêm, xem, sửa, xóa vai trò
	{Name: "Role.Insert", Describe: "Quyền tạo vai trò", Group: "Auth", Category: "Role"},
	{Name: "Role.Read", Describe: "Quyền xem danh sách vai trò", Group: "Auth", Category: "Role"},
	{Name: "Role.Update", Describe: "Quyền cập nhật vai trò", Group: "Auth", Category: "Role"},
	{Name: "Role.Delete", Describe: "Quyền xóa vai trò", Group: "Auth", Category: "Role"},

	// Quản lý quyền: Thêm, xem, sửa, xóa quyền
	{Name: "Permission.Insert", Describe: "Quyền tạo quyền", Group: "Auth", Category: "Permission"},
	{Name: "Permission.Read", Describe: "Quyền xem danh sách quyền", Group: "Auth", Category: "Permission"},
	{Name: "Permission.Update", Describe: "Quyền cập nhật quyền", Group: "Auth", Category: "Permission"},
	{Name: "Permission.Delete", Describe: "Quyền xóa quyền", Group: "Auth", Category: "Permission"},

	// Quản lý phân quyền cho vai trò: Thêm, xem, sửa, xóa phân quyền
	{Name: "RolePermission.Insert", Describe: "Quyền tạo phân quyền cho vai trò", Group: "Auth", Category: "RolePermission"},
	{Name: "RolePermission.Read", Describe: "Quyền xem phân quyền của vai trò", Group: "Auth", Category: "RolePermission"},
	{Name: "RolePermission.Update", Describe: "Quyền cập nhật phân quyền của vai trò", Group: "Auth", Category: "RolePermission"},
	{Name: "RolePermission.Delete", Describe: "Quyền xóa phân quyền của vai trò", Group: "Auth", Category: "RolePermission"},

	// Quản lý phân vai trò cho người dùng: Thêm, xem, sửa, xóa phân vai trò
	{Name: "UserRole.Insert", Describe: "Quyền phân công vai trò cho người dùng", Group: "Auth", Category: "UserRole"},
	{Name: "UserRole.Read", Describe: "Quyền xem vai trò của người dùng", Group: "Auth", Category: "UserRole"},
	{Name: "UserRole.Update", Describe: "Quyền cập nhật vai trò của người dùng", Group: "Auth", Category: "UserRole"},
	{Name: "UserRole.Delete", Describe: "Quyền xóa vai trò của người dùng", Group: "Auth", Category: "UserRole"},

	// Quản lý đại lý: Thêm, xem, sửa, xóa và kiểm tra trạng thái
	{Name: "Agent.Insert", Describe: "Quyền tạo đại lý", Group: "Auth", Category: "Agent"},
	{Name: "Agent.Read", Describe: "Quyền xem danh sách đại lý", Group: "Auth", Category: "Agent"},
	{Name: "Agent.Update", Describe: "Quyền cập nhật thông tin đại lý", Group: "Auth", Category: "Agent"},
	{Name: "Agent.Delete", Describe: "Quyền xóa đại lý", Group: "Auth", Category: "Agent"},
	{Name: "Agent.CheckIn", Describe: "Quyền kiểm tra trạng thái đại lý", Group: "Auth", Category: "Agent"},
	{Name: "Agent.CheckOut", Describe: "Quyền kiểm tra trạng thái đại lý", Group: "Auth", Category: "Agent"},

	// Quản lý khởi tạo hệ thống: Thiết lập administrator và đồng bộ quyền
	{Name: "Init.SetAdmin", Describe: "Quyền thiết lập administrator và đồng bộ quyền cho Administrator", Group: "Auth", Category: "Init"},

	// Quản lý MongoDB: Xem danh sách collections, xóa toàn bộ, export (chỉ dành cho admin hệ thống)
	{Name: "MongoDB.Manage", Describe: "Quyền quản lý MongoDB: xem collections, xóa toàn bộ documents, tải export", Group: "Auth", Category: "MongoDB"},

	// ==================================== PANCAKE MODULE ===========================================
	// Quản lý token truy cập: Thêm, xem, sửa, xóa token
	{Name: "AccessToken.Insert", Describe: "Quyền tạo token", Group: "Pancake", Category: "AccessToken"},
	{Name: "AccessToken.Read", Describe: "Quyền xem danh sách token", Group: "Pancake", Category: "AccessToken"},
	{Name: "AccessToken.Update", Describe: "Quyền cập nhật token", Group: "Pancake", Category: "AccessToken"},
	{Name: "AccessToken.Delete", Describe: "Quyền xóa token", Group: "Pancake", Category: "AccessToken"},

	// Quản lý trang Facebook: Thêm, xem, sửa, xóa và cập nhật token
	{Name: "FbPage.Insert", Describe: "Quyền tạo trang Facebook", Group: "Pancake", Category: "FbPage"},
	{Name: "FbPage.Read", Describe: "Quyền xem danh sách trang Facebook", Group: "Pancake", Category: "FbPage"},
	{Name: "FbPage.Update", Describe: "Quyền cập nhật thông tin trang Facebook", Group: "Pancake", Category: "FbPage"},
	{Name: "FbPage.Delete", Describe: "Quyền xóa trang Facebook", Group: "Pancake", Category: "FbPage"},
	{Name: "FbPage.UpdateToken", Describe: "Quyền cập nhật token trang Facebook", Group: "Pancake", Category: "FbPage"},

	// Quản lý cuộc trò chuyện Facebook: Thêm, xem, sửa, xóa
	{Name: "FbConversation.Insert", Describe: "Quyền tạo cuộc trò chuyện", Group: "Pancake", Category: "FbConversation"},
	{Name: "FbConversation.Read", Describe: "Quyền xem danh sách cuộc trò chuyện", Group: "Pancake", Category: "FbConversation"},
	{Name: "FbConversation.Update", Describe: "Quyền cập nhật cuộc trò chuyện", Group: "Pancake", Category: "FbConversation"},
	{Name: "FbConversation.Delete", Describe: "Quyền xóa cuộc trò chuyện", Group: "Pancake", Category: "FbConversation"},

	// Quản lý tin nhắn Facebook: Thêm, xem, sửa, xóa
	{Name: "FbMessage.Insert", Describe: "Quyền tạo tin nhắn", Group: "Pancake", Category: "FbMessage"},
	{Name: "FbMessage.Read", Describe: "Quyền xem danh sách tin nhắn", Group: "Pancake", Category: "FbMessage"},
	{Name: "FbMessage.Update", Describe: "Quyền cập nhật tin nhắn", Group: "Pancake", Category: "FbMessage"},
	{Name: "FbMessage.Delete", Describe: "Quyền xóa tin nhắn", Group: "Pancake", Category: "FbMessage"},

	// Quản lý bài viết Facebook: Thêm, xem, sửa, xóa
	{Name: "FbPost.Insert", Describe: "Quyền tạo bài viết", Group: "Pancake", Category: "FbPost"},
	{Name: "FbPost.Read", Describe: "Quyền xem danh sách bài viết", Group: "Pancake", Category: "FbPost"},
	{Name: "FbPost.Update", Describe: "Quyền cập nhật bài viết", Group: "Pancake", Category: "FbPost"},
	{Name: "FbPost.Delete", Describe: "Quyền xóa bài viết", Group: "Pancake", Category: "FbPost"},

	// Quản lý đơn hàng Pancake: Thêm, xem, sửa, xóa
	{Name: "PcOrder.Insert", Describe: "Quyền tạo đơn hàng", Group: "Pancake", Category: "PcOrder"},
	{Name: "PcOrder.Read", Describe: "Quyền xem danh sách đơn hàng", Group: "Pancake", Category: "PcOrder"},
	{Name: "PcOrder.Update", Describe: "Quyền cập nhật đơn hàng", Group: "Pancake", Category: "PcOrder"},
	{Name: "PcOrder.Delete", Describe: "Quyền xóa đơn hàng", Group: "Pancake", Category: "PcOrder"},

	// Quản lý tin nhắn Facebook Item: Thêm, xem, sửa, xóa
	{Name: "FbMessageItem.Insert", Describe: "Quyền tạo tin nhắn item", Group: "Pancake", Category: "FbMessageItem"},
	{Name: "FbMessageItem.Read", Describe: "Quyền xem danh sách tin nhắn item", Group: "Pancake", Category: "FbMessageItem"},
	{Name: "FbMessageItem.Update", Describe: "Quyền cập nhật tin nhắn item", Group: "Pancake", Category: "FbMessageItem"},
	{Name: "FbMessageItem.Delete", Describe: "Quyền xóa tin nhắn item", Group: "Pancake", Category: "FbMessageItem"},

	// Quản lý khách hàng: Thêm, xem, sửa, xóa
	{Name: "Customer.Insert", Describe: "Quyền tạo khách hàng", Group: "Pancake", Category: "Customer"},
	{Name: "Customer.Read", Describe: "Quyền xem danh sách khách hàng", Group: "Pancake", Category: "Customer"},
	{Name: "Customer.Update", Describe: "Quyền cập nhật thông tin khách hàng", Group: "Pancake", Category: "Customer"},
	{Name: "Customer.Delete", Describe: "Quyền xóa khách hàng", Group: "Pancake", Category: "Customer"},

	// Quản lý khách hàng Facebook: Thêm, xem, sửa, xóa
	{Name: "FbCustomer.Insert", Describe: "Quyền tạo khách hàng Facebook", Group: "Pancake", Category: "FbCustomer"},
	{Name: "FbCustomer.Read", Describe: "Quyền xem danh sách khách hàng Facebook", Group: "Pancake", Category: "FbCustomer"},
	{Name: "FbCustomer.Update", Describe: "Quyền cập nhật thông tin khách hàng Facebook", Group: "Pancake", Category: "FbCustomer"},
	{Name: "FbCustomer.Delete", Describe: "Quyền xóa khách hàng Facebook", Group: "Pancake", Category: "FbCustomer"},

	// Quản lý khách hàng POS: Thêm, xem, sửa, xóa
	{Name: "PcPosCustomer.Insert", Describe: "Quyền tạo khách hàng POS", Group: "Pancake", Category: "PcPosCustomer"},
	{Name: "PcPosCustomer.Read", Describe: "Quyền xem danh sách khách hàng POS", Group: "Pancake", Category: "PcPosCustomer"},
	{Name: "PcPosCustomer.Update", Describe: "Quyền cập nhật thông tin khách hàng POS", Group: "Pancake", Category: "PcPosCustomer"},
	{Name: "PcPosCustomer.Delete", Describe: "Quyền xóa khách hàng POS", Group: "Pancake", Category: "PcPosCustomer"},

	// Quản lý cửa hàng Pancake POS: Thêm, xem, sửa, xóa
	{Name: "PcPosShop.Insert", Describe: "Quyền tạo cửa hàng từ Pancake POS", Group: "Pancake", Category: "PcPosShop"},
	{Name: "PcPosShop.Read", Describe: "Quyền xem danh sách cửa hàng từ Pancake POS", Group: "Pancake", Category: "PcPosShop"},
	{Name: "PcPosShop.Update", Describe: "Quyền cập nhật thông tin cửa hàng từ Pancake POS", Group: "Pancake", Category: "PcPosShop"},
	{Name: "PcPosShop.Delete", Describe: "Quyền xóa cửa hàng từ Pancake POS", Group: "Pancake", Category: "PcPosShop"},

	// Quản lý kho hàng Pancake POS: Thêm, xem, sửa, xóa
	{Name: "PcPosWarehouse.Insert", Describe: "Quyền tạo kho hàng từ Pancake POS", Group: "Pancake", Category: "PcPosWarehouse"},
	{Name: "PcPosWarehouse.Read", Describe: "Quyền xem danh sách kho hàng từ Pancake POS", Group: "Pancake", Category: "PcPosWarehouse"},
	{Name: "PcPosWarehouse.Update", Describe: "Quyền cập nhật thông tin kho hàng từ Pancake POS", Group: "Pancake", Category: "PcPosWarehouse"},
	{Name: "PcPosWarehouse.Delete", Describe: "Quyền xóa kho hàng từ Pancake POS", Group: "Pancake", Category: "PcPosWarehouse"},

	// Quản lý sản phẩm Pancake POS: Thêm, xem, sửa, xóa
	{Name: "PcPosProduct.Insert", Describe: "Quyền tạo sản phẩm từ Pancake POS", Group: "Pancake", Category: "PcPosProduct"},
	{Name: "PcPosProduct.Read", Describe: "Quyền xem danh sách sản phẩm từ Pancake POS", Group: "Pancake", Category: "PcPosProduct"},
	{Name: "PcPosProduct.Update", Describe: "Quyền cập nhật thông tin sản phẩm từ Pancake POS", Group: "Pancake", Category: "PcPosProduct"},
	{Name: "PcPosProduct.Delete", Describe: "Quyền xóa sản phẩm từ Pancake POS", Group: "Pancake", Category: "PcPosProduct"},

	// Quản lý biến thể sản phẩm Pancake POS: Thêm, xem, sửa, xóa
	{Name: "PcPosVariation.Insert", Describe: "Quyền tạo biến thể sản phẩm từ Pancake POS", Group: "Pancake", Category: "PcPosVariation"},
	{Name: "PcPosVariation.Read", Describe: "Quyền xem danh sách biến thể sản phẩm từ Pancake POS", Group: "Pancake", Category: "PcPosVariation"},
	{Name: "PcPosVariation.Update", Describe: "Quyền cập nhật thông tin biến thể sản phẩm từ Pancake POS", Group: "Pancake", Category: "PcPosVariation"},
	{Name: "PcPosVariation.Delete", Describe: "Quyền xóa biến thể sản phẩm từ Pancake POS", Group: "Pancake", Category: "PcPosVariation"},

	// Quản lý danh mục sản phẩm Pancake POS: Thêm, xem, sửa, xóa
	{Name: "PcPosCategory.Insert", Describe: "Quyền tạo danh mục sản phẩm từ Pancake POS", Group: "Pancake", Category: "PcPosCategory"},
	{Name: "PcPosCategory.Read", Describe: "Quyền xem danh sách danh mục sản phẩm từ Pancake POS", Group: "Pancake", Category: "PcPosCategory"},
	{Name: "PcPosCategory.Update", Describe: "Quyền cập nhật thông tin danh mục sản phẩm từ Pancake POS", Group: "Pancake", Category: "PcPosCategory"},
	{Name: "PcPosCategory.Delete", Describe: "Quyền xóa danh mục sản phẩm từ Pancake POS", Group: "Pancake", Category: "PcPosCategory"},

	// Quản lý đơn hàng Pancake POS: Thêm, xem, sửa, xóa
	{Name: "PcPosOrder.Insert", Describe: "Quyền tạo đơn hàng từ Pancake POS", Group: "Pancake", Category: "PcPosOrder"},
	{Name: "PcPosOrder.Read", Describe: "Quyền xem danh sách đơn hàng từ Pancake POS", Group: "Pancake", Category: "PcPosOrder"},
	{Name: "PcPosOrder.Update", Describe: "Quyền cập nhật thông tin đơn hàng từ Pancake POS", Group: "Pancake", Category: "PcPosOrder"},
	{Name: "PcPosOrder.Delete", Describe: "Quyền xóa đơn hàng từ Pancake POS", Group: "Pancake", Category: "PcPosOrder"},

	// Báo cáo theo chu kỳ (Phase 1)
	{Name: "Report.Read", Describe: "Quyền xem báo cáo trend", Group: "Report", Category: "Report"},
	{Name: "Report.Recompute", Describe: "Quyền chạy lại tính toán báo cáo", Group: "Report", Category: "Report"},

	// ==================================== NOTIFICATION MODULE ===========================================
	// Quản lý Notification Sender: Thêm, xem, sửa, xóa
	{Name: "NotificationSender.Insert", Describe: "Quyền tạo cấu hình sender thông báo", Group: "Notification", Category: "NotificationSender"},
	{Name: "NotificationSender.Read", Describe: "Quyền xem danh sách cấu hình sender thông báo", Group: "Notification", Category: "NotificationSender"},
	{Name: "NotificationSender.Update", Describe: "Quyền cập nhật cấu hình sender thông báo", Group: "Notification", Category: "NotificationSender"},
	{Name: "NotificationSender.Delete", Describe: "Quyền xóa cấu hình sender thông báo", Group: "Notification", Category: "NotificationSender"},

	// Quản lý Notification Channel: Thêm, xem, sửa, xóa
	{Name: "NotificationChannel.Insert", Describe: "Quyền tạo kênh thông báo cho team", Group: "Notification", Category: "NotificationChannel"},
	{Name: "NotificationChannel.Read", Describe: "Quyền xem danh sách kênh thông báo", Group: "Notification", Category: "NotificationChannel"},
	{Name: "NotificationChannel.Update", Describe: "Quyền cập nhật kênh thông báo", Group: "Notification", Category: "NotificationChannel"},
	{Name: "NotificationChannel.Delete", Describe: "Quyền xóa kênh thông báo", Group: "Notification", Category: "NotificationChannel"},

	// Quản lý Notification Template: Thêm, xem, sửa, xóa
	{Name: "NotificationTemplate.Insert", Describe: "Quyền tạo template thông báo", Group: "Notification", Category: "NotificationTemplate"},
	{Name: "NotificationTemplate.Read", Describe: "Quyền xem danh sách template thông báo", Group: "Notification", Category: "NotificationTemplate"},
	{Name: "NotificationTemplate.Update", Describe: "Quyền cập nhật template thông báo", Group: "Notification", Category: "NotificationTemplate"},
	{Name: "NotificationTemplate.Delete", Describe: "Quyền xóa template thông báo", Group: "Notification", Category: "NotificationTemplate"},

	// Quản lý Notification Routing Rule: Thêm, xem, sửa, xóa
	{Name: "NotificationRouting.Insert", Describe: "Quyền tạo routing rule thông báo", Group: "Notification", Category: "NotificationRouting"},
	{Name: "NotificationRouting.Read", Describe: "Quyền xem danh sách routing rule thông báo", Group: "Notification", Category: "NotificationRouting"},
	{Name: "NotificationRouting.Update", Describe: "Quyền cập nhật routing rule thông báo", Group: "Notification", Category: "NotificationRouting"},
	{Name: "NotificationRouting.Delete", Describe: "Quyền xóa routing rule thông báo", Group: "Notification", Category: "NotificationRouting"},

	// Quản lý Delivery History: Chỉ xem (thuộc Delivery System)
	{Name: "DeliveryHistory.Read", Describe: "Quyền xem lịch sử delivery", Group: "Delivery", Category: "DeliveryHistory"},

	// Trigger Notification: Gửi thông báo
	{Name: "Notification.Trigger", Describe: "Quyền trigger/gửi thông báo", Group: "Notification", Category: "Notification"},

	// ==================================== CTA MODULE ===========================================
	// Quản lý CTA Library: Thêm, xem, sửa, xóa
	{Name: "CTALibrary.Insert", Describe: "Quyền tạo CTA Library", Group: "CTA", Category: "CTALibrary"},
	{Name: "CTALibrary.Read", Describe: "Quyền xem danh sách CTA Library", Group: "CTA", Category: "CTALibrary"},
	{Name: "CTALibrary.Update", Describe: "Quyền cập nhật CTA Library", Group: "CTA", Category: "CTALibrary"},
	{Name: "CTALibrary.Delete", Describe: "Quyền xóa CTA Library", Group: "CTA", Category: "CTALibrary"},

	// ==================================== DELIVERY MODULE ===========================================
	// Delivery Send: Gửi notification trực tiếp
	{Name: "Delivery.Send", Describe: "Quyền gửi notification trực tiếp qua Delivery Service", Group: "Delivery", Category: "Delivery"},

	// Quản lý Delivery Sender: Thêm, xem, sửa, xóa (tương tự NotificationSender nhưng trong delivery namespace)
	{Name: "DeliverySender.Insert", Describe: "Quyền tạo cấu hình sender delivery", Group: "Delivery", Category: "DeliverySender"},
	{Name: "DeliverySender.Read", Describe: "Quyền xem danh sách cấu hình sender delivery", Group: "Delivery", Category: "DeliverySender"},
	{Name: "DeliverySender.Update", Describe: "Quyền cập nhật cấu hình sender delivery", Group: "Delivery", Category: "DeliverySender"},
	{Name: "DeliverySender.Delete", Describe: "Quyền xóa cấu hình sender delivery", Group: "Delivery", Category: "DeliverySender"},

	// Quản lý Delivery History: Chỉ xem
	{Name: "DeliveryHistory.Read", Describe: "Quyền xem lịch sử delivery", Group: "Delivery", Category: "DeliveryHistory"},

	// ==================================== AGENT MANAGEMENT MODULE ===========================================
	// Quản lý Agent Registry (Bot Registry): Thêm, xem, sửa, xóa
	{Name: "AgentRegistry.Insert", Describe: "Quyền tạo bot registry", Group: "AgentManagement", Category: "AgentRegistry"},
	{Name: "AgentRegistry.Read", Describe: "Quyền xem danh sách bot registry", Group: "AgentManagement", Category: "AgentRegistry"},
	{Name: "AgentRegistry.Update", Describe: "Quyền cập nhật bot registry", Group: "AgentManagement", Category: "AgentRegistry"},
	{Name: "AgentRegistry.Delete", Describe: "Quyền xóa bot registry", Group: "AgentManagement", Category: "AgentRegistry"},

	// Quản lý Agent Config: Thêm, xem, sửa, xóa
	{Name: "AgentConfig.Insert", Describe: "Quyền tạo bot config", Group: "AgentManagement", Category: "AgentConfig"},
	{Name: "AgentConfig.Read", Describe: "Quyền xem danh sách bot config", Group: "AgentManagement", Category: "AgentConfig"},
	{Name: "AgentConfig.Update", Describe: "Quyền cập nhật bot config", Group: "AgentManagement", Category: "AgentConfig"},
	{Name: "AgentConfig.Delete", Describe: "Quyền xóa bot config", Group: "AgentManagement", Category: "AgentConfig"},

	// Quản lý Agent Command: Thêm, xem, sửa, xóa
	{Name: "AgentCommand.Insert", Describe: "Quyền tạo bot command", Group: "AgentManagement", Category: "AgentCommand"},
	{Name: "AgentCommand.Read", Describe: "Quyền xem danh sách bot command", Group: "AgentManagement", Category: "AgentCommand"},
	{Name: "AgentCommand.Update", Describe: "Quyền cập nhật bot command", Group: "AgentManagement", Category: "AgentCommand"},
	{Name: "AgentCommand.Delete", Describe: "Quyền xóa bot command", Group: "AgentManagement", Category: "AgentCommand"},

	// Lưu ý: Agent Status đã được ghép vào Agent Registry, không cần permission riêng nữa
	// Status có thể được xem/update qua Agent Registry permissions

	// Quản lý Agent Activity Log: Chỉ xem (bot tự log)
	{Name: "AgentActivityLog.Read", Describe: "Quyền xem bot activity log", Group: "AgentManagement", Category: "AgentActivityLog"},

	// Quyền đặc biệt cho check-in endpoint
	{Name: "AgentManagement.CheckIn", Describe: "Quyền check-in từ bot", Group: "AgentManagement", Category: "AgentManagement"},

	// ==================================== CONTENT MODULE (MODULE 1 - CONTENT STORAGE) ===========================================
	// Quản lý Content Nodes (collection: content_nodes): Thêm, xem, sửa, xóa
	{Name: "ContentNodes.Insert", Describe: "Quyền tạo content node", Group: "Content", Category: "ContentNodes"},
	{Name: "ContentNodes.Read", Describe: "Quyền xem danh sách content nodes", Group: "Content", Category: "ContentNodes"},
	{Name: "ContentNodes.Update", Describe: "Quyền cập nhật content node", Group: "Content", Category: "ContentNodes"},
	{Name: "ContentNodes.Delete", Describe: "Quyền xóa content node", Group: "Content", Category: "ContentNodes"},

	// Quản lý Videos (collection: content_videos): Thêm, xem, sửa, xóa
	{Name: "ContentVideos.Insert", Describe: "Quyền tạo video", Group: "Content", Category: "ContentVideos"},
	{Name: "ContentVideos.Read", Describe: "Quyền xem danh sách videos", Group: "Content", Category: "ContentVideos"},
	{Name: "ContentVideos.Update", Describe: "Quyền cập nhật video", Group: "Content", Category: "ContentVideos"},
	{Name: "ContentVideos.Delete", Describe: "Quyền xóa video", Group: "Content", Category: "ContentVideos"},

	// Quản lý Publications (collection: content_publications): Thêm, xem, sửa, xóa
	{Name: "ContentPublications.Insert", Describe: "Quyền tạo publication", Group: "Content", Category: "ContentPublications"},
	{Name: "ContentPublications.Read", Describe: "Quyền xem danh sách publications", Group: "Content", Category: "ContentPublications"},
	{Name: "ContentPublications.Update", Describe: "Quyền cập nhật publication", Group: "Content", Category: "ContentPublications"},
	{Name: "ContentPublications.Delete", Describe: "Quyền xóa publication", Group: "Content", Category: "ContentPublications"},

	// Quản lý Draft Content Nodes (collection: content_draft_nodes): Thêm, xem, sửa, xóa
	{Name: "ContentDraftNodes.Insert", Describe: "Quyền tạo draft content node", Group: "Content", Category: "ContentDraftNodes"},
	{Name: "ContentDraftNodes.Read", Describe: "Quyền xem danh sách draft content nodes", Group: "Content", Category: "ContentDraftNodes"},
	{Name: "ContentDraftNodes.Update", Describe: "Quyền cập nhật draft content node", Group: "Content", Category: "ContentDraftNodes"},
	{Name: "ContentDraftNodes.Delete", Describe: "Quyền xóa draft content node", Group: "Content", Category: "ContentDraftNodes"},

	// Quản lý Draft Videos (collection: content_draft_videos): Thêm, xem, sửa, xóa
	{Name: "ContentDraftVideos.Insert", Describe: "Quyền tạo draft video", Group: "Content", Category: "ContentDraftVideos"},
	{Name: "ContentDraftVideos.Read", Describe: "Quyền xem danh sách draft videos", Group: "Content", Category: "ContentDraftVideos"},
	{Name: "ContentDraftVideos.Update", Describe: "Quyền cập nhật draft video", Group: "Content", Category: "ContentDraftVideos"},
	{Name: "ContentDraftVideos.Delete", Describe: "Quyền xóa draft video", Group: "Content", Category: "ContentDraftVideos"},

	// Quản lý Draft Publications (collection: content_draft_publications): Thêm, xem, sửa, xóa
	{Name: "ContentDraftPublications.Insert", Describe: "Quyền tạo draft publication", Group: "Content", Category: "ContentDraftPublications"},
	{Name: "ContentDraftPublications.Read", Describe: "Quyền xem danh sách draft publications", Group: "Content", Category: "ContentDraftPublications"},
	{Name: "ContentDraftPublications.Update", Describe: "Quyền cập nhật draft publication", Group: "Content", Category: "ContentDraftPublications"},
	{Name: "ContentDraftPublications.Delete", Describe: "Quyền xóa draft publication", Group: "Content", Category: "ContentDraftPublications"},

	// Quyền phê duyệt/từ chối từng draft node (route: POST /content/drafts/nodes/:id/approve, POST /content/drafts/nodes/:id/reject)
	{Name: "ContentDraftNodes.Approve", Describe: "Quyền phê duyệt draft content node", Group: "Content", Category: "ContentDraftNodes"},
	{Name: "ContentDraftNodes.Reject", Describe: "Quyền từ chối draft content node", Group: "Content", Category: "ContentDraftNodes"},

	// Quyền đặc biệt cho commit draft content node (commit draft → production)
	{Name: "ContentDraftNodes.Commit", Describe: "Quyền commit draft content node sang production", Group: "Content", Category: "ContentDraftNodes"},

	// ==================================== AI SERVICE MODULE (MODULE 2 - AI SERVICE) ===========================================
	// Quản lý AI Workflows (collection: ai_workflows): Thêm, xem, sửa, xóa
	{Name: "AIWorkflows.Insert", Describe: "Quyền tạo AI workflow", Group: "AI", Category: "AIWorkflows"},
	{Name: "AIWorkflows.Read", Describe: "Quyền xem danh sách AI workflows", Group: "AI", Category: "AIWorkflows"},
	{Name: "AIWorkflows.Update", Describe: "Quyền cập nhật AI workflow", Group: "AI", Category: "AIWorkflows"},
	{Name: "AIWorkflows.Delete", Describe: "Quyền xóa AI workflow", Group: "AI", Category: "AIWorkflows"},

	// Quản lý AI Steps (collection: ai_steps): Thêm, xem, sửa, xóa
	{Name: "AISteps.Insert", Describe: "Quyền tạo AI step", Group: "AI", Category: "AISteps"},
	{Name: "AISteps.Read", Describe: "Quyền xem danh sách AI steps", Group: "AI", Category: "AISteps"},
	{Name: "AISteps.Update", Describe: "Quyền cập nhật AI step", Group: "AI", Category: "AISteps"},
	{Name: "AISteps.Delete", Describe: "Quyền xóa AI step", Group: "AI", Category: "AISteps"},

	// Quản lý AI Prompt Templates (collection: ai_prompt_templates): Thêm, xem, sửa, xóa
	{Name: "AIPromptTemplates.Insert", Describe: "Quyền tạo AI prompt template", Group: "AI", Category: "AIPromptTemplates"},
	{Name: "AIPromptTemplates.Read", Describe: "Quyền xem danh sách AI prompt templates", Group: "AI", Category: "AIPromptTemplates"},
	{Name: "AIPromptTemplates.Update", Describe: "Quyền cập nhật AI prompt template", Group: "AI", Category: "AIPromptTemplates"},
	{Name: "AIPromptTemplates.Delete", Describe: "Quyền xóa AI prompt template", Group: "AI", Category: "AIPromptTemplates"},

	// Quản lý AI Provider Profiles (collection: ai_provider_profiles): Thêm, xem, sửa, xóa
	{Name: "AIProviderProfiles.Insert", Describe: "Quyền tạo AI provider profile", Group: "AI", Category: "AIProviderProfiles"},
	{Name: "AIProviderProfiles.Read", Describe: "Quyền xem danh sách AI provider profiles", Group: "AI", Category: "AIProviderProfiles"},
	{Name: "AIProviderProfiles.Update", Describe: "Quyền cập nhật AI provider profile", Group: "AI", Category: "AIProviderProfiles"},
	{Name: "AIProviderProfiles.Delete", Describe: "Quyền xóa AI provider profile", Group: "AI", Category: "AIProviderProfiles"},

	// Quản lý AI Workflow Runs (collection: ai_workflow_runs): Thêm, xem, sửa, xóa
	{Name: "AIWorkflowRuns.Insert", Describe: "Quyền tạo AI workflow run", Group: "AI", Category: "AIWorkflowRuns"},
	{Name: "AIWorkflowRuns.Read", Describe: "Quyền xem danh sách AI workflow runs", Group: "AI", Category: "AIWorkflowRuns"},
	{Name: "AIWorkflowRuns.Update", Describe: "Quyền cập nhật AI workflow run", Group: "AI", Category: "AIWorkflowRuns"},
	{Name: "AIWorkflowRuns.Delete", Describe: "Quyền xóa AI workflow run", Group: "AI", Category: "AIWorkflowRuns"},

	// Quản lý AI Step Runs (collection: ai_step_runs): Thêm, xem, sửa, xóa
	{Name: "AIStepRuns.Insert", Describe: "Quyền tạo AI step run", Group: "AI", Category: "AIStepRuns"},
	{Name: "AIStepRuns.Read", Describe: "Quyền xem danh sách AI step runs", Group: "AI", Category: "AIStepRuns"},
	{Name: "AIStepRuns.Update", Describe: "Quyền cập nhật AI step run", Group: "AI", Category: "AIStepRuns"},
	{Name: "AIStepRuns.Delete", Describe: "Quyền xóa AI step run", Group: "AI", Category: "AIStepRuns"},

	// Quản lý AI Generation Batches (collection: ai_generation_batches): Thêm, xem, sửa, xóa
	{Name: "AIGenerationBatches.Insert", Describe: "Quyền tạo AI generation batch", Group: "AI", Category: "AIGenerationBatches"},
	{Name: "AIGenerationBatches.Read", Describe: "Quyền xem danh sách AI generation batches", Group: "AI", Category: "AIGenerationBatches"},
	{Name: "AIGenerationBatches.Update", Describe: "Quyền cập nhật AI generation batch", Group: "AI", Category: "AIGenerationBatches"},
	{Name: "AIGenerationBatches.Delete", Describe: "Quyền xóa AI generation batch", Group: "AI", Category: "AIGenerationBatches"},

	// Quản lý AI Candidates (collection: ai_candidates): Thêm, xem, sửa, xóa
	{Name: "AICandidates.Insert", Describe: "Quyền tạo AI candidate", Group: "AI", Category: "AICandidates"},
	{Name: "AICandidates.Read", Describe: "Quyền xem danh sách AI candidates", Group: "AI", Category: "AICandidates"},
	{Name: "AICandidates.Update", Describe: "Quyền cập nhật AI candidate", Group: "AI", Category: "AICandidates"},
	{Name: "AICandidates.Delete", Describe: "Quyền xóa AI candidate", Group: "AI", Category: "AICandidates"},

	// Quản lý AI Runs (collection: ai_runs): Thêm, xem, sửa, xóa
	{Name: "AIRuns.Insert", Describe: "Quyền tạo AI run", Group: "AI", Category: "AIRuns"},
	{Name: "AIRuns.Read", Describe: "Quyền xem danh sách AI runs", Group: "AI", Category: "AIRuns"},
	{Name: "AIRuns.Update", Describe: "Quyền cập nhật AI run", Group: "AI", Category: "AIRuns"},
	{Name: "AIRuns.Delete", Describe: "Quyền xóa AI run", Group: "AI", Category: "AIRuns"},

	// Quản lý AI Workflow Commands (collection: ai_workflow_commands): Thêm, xem, sửa, xóa
	{Name: "AIWorkflowCommands.Insert", Describe: "Quyền tạo AI workflow command", Group: "AI", Category: "AIWorkflowCommands"},
	{Name: "AIWorkflowCommands.Read", Describe: "Quyền xem danh sách AI workflow commands", Group: "AI", Category: "AIWorkflowCommands"},
	{Name: "AIWorkflowCommands.Update", Describe: "Quyền cập nhật AI workflow command", Group: "AI", Category: "AIWorkflowCommands"},
	{Name: "AIWorkflowCommands.Delete", Describe: "Quyền xóa AI workflow command", Group: "AI", Category: "AIWorkflowCommands"},

	// ==================================== WEBHOOK LOGS MODULE ===========================================
	// Quản lý Webhook Log: Thêm, xem, sửa, xóa (để debug và tracking webhooks)
	{Name: "WebhookLog.Insert", Describe: "Quyền tạo webhook log", Group: "Webhook", Category: "WebhookLog"},
	{Name: "WebhookLog.Read", Describe: "Quyền xem danh sách webhook logs", Group: "Webhook", Category: "WebhookLog"},
	{Name: "WebhookLog.Update", Describe: "Quyền cập nhật webhook log", Group: "Webhook", Category: "WebhookLog"},
	{Name: "WebhookLog.Delete", Describe: "Quyền xóa webhook log", Group: "Webhook", Category: "WebhookLog"},
}

// InitPermission khởi tạo các quyền mặc định cho hệ thống
// Chỉ tạo mới các quyền chưa tồn tại trong database
// Returns:
//   - error: Lỗi nếu có trong quá trình khởi tạo
func (h *InitService) InitPermission() error {
	// Duyệt qua danh sách quyền mặc định
	for _, permission := range InitialPermissions {
		// Kiểm tra quyền đã tồn tại chưa
		filter := bson.M{"name": permission.Name}
		_, err := h.permissionService.BaseServiceMongoImpl.FindOne(context.TODO(), filter, nil)

		// Bỏ qua nếu có lỗi khác ErrNotFound
		if err != nil && err != common.ErrNotFound {
			continue
		}

		// Tạo mới quyền nếu chưa tồn tại
		if err == common.ErrNotFound {
			// Set IsSystem = true cho tất cả permissions được tạo trong init
			permission.IsSystem = true
			// Sử dụng context cho phép insert system data trong quá trình init
			// Context cho phép insert system data trong init (từ package services)
			initCtx := basesvc.WithSystemDataInsertAllowed(context.TODO())
			_, err = h.permissionService.BaseServiceMongoImpl.InsertOne(initCtx, permission)
			if err != nil {
				return fmt.Errorf("failed to insert permission %s: %v", permission.Name, err)
			}
		}
	}
	return nil
}

// InitRootOrganization khởi tạo Organization System (Level -1)
// System organization là tổ chức cấp cao nhất, chứa Administrator, không có parent, không thể xóa
// System thay thế ROOT_GROUP cũ
// Returns:
//   - error: Lỗi nếu có trong quá trình khởi tạo
func (h *InitService) InitRootOrganization() error {
	log := logger.GetAppLogger()

	// Kiểm tra System Organization đã tồn tại chưa
	systemFilter := bson.M{
		"type":  authmodels.OrganizationTypeSystem,
		"level": -1,
		"code":  "SYSTEM",
	}

	log.Infof("🔍 [INIT] Checking for System Organization with filter: type=%s, level=%d, code=%s",
		authmodels.OrganizationTypeSystem, -1, "SYSTEM")

	_, err := h.organizationService.BaseServiceMongoImpl.FindOne(context.TODO(), systemFilter, nil)
	if err != nil && err != common.ErrNotFound {
		// Log chi tiết lỗi
		log.Errorf("❌ [INIT] Failed to check system organization: %v", err)
		log.Errorf("❌ [INIT] Error type: %T", err)
		log.Errorf("❌ [INIT] Error details: %+v", err)

		// Kiểm tra nếu là lỗi MongoDB connection
		if commonErr, ok := err.(*common.Error); ok {
			log.Errorf("❌ [INIT] Error code: %s", commonErr.Code.Code)
			log.Errorf("❌ [INIT] Error message: %s", commonErr.Message)
			if commonErr.Details != nil {
				log.Errorf("❌ [INIT] Error details: %v", commonErr.Details)
			}
		}

		return fmt.Errorf("failed to check system organization: %v", err)
	}

	// Nếu đã tồn tại, không cần tạo mới
	if err == nil {
		log.Info("✅ [INIT] System Organization already exists, skipping creation")
		return nil
	}

	if err == common.ErrNotFound {
		log.Info("ℹ️  [INIT] System Organization not found, will create new one")
	}

	// Tạo mới System Organization
	log.Info("🔄 [INIT] Creating new System Organization...")
	systemOrgModel := authmodels.Organization{
		Name:     "Hệ Thống",
		Code:     "SYSTEM",
		Type:     authmodels.OrganizationTypeSystem,
		ParentID: nil, // System không có parent
		Path:     "/system",
		Level:    -1,
		IsActive: true,
		IsSystem: true, // Đánh dấu là dữ liệu hệ thống
	}

	log.Infof("📝 [INIT] System Organization model: Name=%s, Code=%s, Type=%s, Level=%d",
		systemOrgModel.Name, systemOrgModel.Code, systemOrgModel.Type, systemOrgModel.Level)

	// Sử dụng context cho phép insert system data trong quá trình init
	// Context cho phép insert system data trong init (từ package services)
	initCtx := basesvc.WithSystemDataInsertAllowed(context.TODO())
	log.Info("💾 [INIT] Inserting System Organization into database...")
	_, err = h.organizationService.BaseServiceMongoImpl.InsertOne(initCtx, systemOrgModel)
	if err != nil {
		log.Errorf("❌ [INIT] Failed to create system organization: %v", err)
		log.Errorf("❌ [INIT] Error type: %T", err)
		log.Errorf("❌ [INIT] Error details: %+v", err)

		// Kiểm tra nếu là lỗi MongoDB connection
		if commonErr, ok := err.(*common.Error); ok {
			log.Errorf("❌ [INIT] Error code: %s", commonErr.Code.Code)
			log.Errorf("❌ [INIT] Error message: %s", commonErr.Message)
			if commonErr.Details != nil {
				log.Errorf("❌ [INIT] Error details: %v", commonErr.Details)
			}
		}

		return fmt.Errorf("failed to create system organization: %v", err)
	}

	log.Info("✅ [INIT] System Organization created successfully")
	return nil
}

// GetRootOrganization lấy System Organization (Level -1) - tổ chức cấp cao nhất
// Returns:
//   - *authmodels.Organization: System Organization
//   - error: Lỗi nếu có
func (h *InitService) GetRootOrganization() (*authmodels.Organization, error) {
	filter := bson.M{
		"type":  authmodels.OrganizationTypeSystem,
		"level": -1,
		"code":  "SYSTEM",
	}
	org, err := h.organizationService.BaseServiceMongoImpl.FindOne(context.TODO(), filter, nil)
	if err != nil {
		return nil, fmt.Errorf("system organization not found: %v", err)
	}

	var modelOrg authmodels.Organization
	bsonBytes, _ := bson.Marshal(org)
	err = bson.Unmarshal(bsonBytes, &modelOrg)
	if err != nil {
		return nil, common.ErrInvalidFormat
	}

	return &modelOrg, nil
}

// InitRole khởi tạo vai trò Administrator mặc định
// Tạo vai trò và gán tất cả các quyền cho vai trò này
// Role Administrator phải thuộc System Organization (Level -1)
// Lưu ý: Role chỉ có OwnerOrganizationID (đã bỏ OrganizationID)
//   - OwnerOrganizationID: Phân quyền sở hữu dữ liệu + Logic business
func (h *InitService) InitRole() error {
	// Lấy System Organization (Level -1)
	rootOrg, err := h.GetRootOrganization()
	if err != nil {
		return fmt.Errorf("failed to get system organization: %v", err)
	}

	// Kiểm tra vai trò Administrator đã tồn tại chưa
	adminRole, err := h.roleService.BaseServiceMongoImpl.FindOne(context.TODO(), bson.M{"name": "Administrator"}, nil)
	if err != nil && err != common.ErrNotFound {
		return err
	}

	var modelRole authmodels.Role
	roleExists := false

	if err == nil {
		// Nếu đã tồn tại, kiểm tra và cập nhật OwnerOrganizationID nếu cần
		bsonBytes, _ := bson.Marshal(adminRole)
		err = bson.Unmarshal(bsonBytes, &modelRole)
		if err == nil {
			roleExists = true
			// Nếu chưa có OwnerOrganizationID, cập nhật
			if modelRole.OwnerOrganizationID.IsZero() {
				updateData := bson.M{
					"ownerOrganizationId": rootOrg.ID, // Phân quyền dữ liệu + Logic business
				}
				_, err = h.roleService.BaseServiceMongoImpl.UpdateOne(context.TODO(), bson.M{"_id": modelRole.ID}, bson.M{"$set": updateData}, nil)
				if err != nil {
					return fmt.Errorf("failed to update administrator role with organization: %v", err)
				}
			}
		}
	}

	// Nếu chưa tồn tại, tạo mới vai trò Administrator
	if !roleExists {
		newAdminRole := authmodels.Role{
			Name:                "Administrator",
			Describe:            "Vai trò quản trị hệ thống",
			OwnerOrganizationID: rootOrg.ID, // Phân quyền dữ liệu + Logic business
			IsSystem:            true,       // Đánh dấu là dữ liệu hệ thống
		}

		// Lưu vai trò vào database
		// Sử dụng context cho phép insert system data trong quá trình init
		// Context cho phép insert system data trong init (từ package services)
		initCtx := basesvc.WithSystemDataInsertAllowed(context.TODO())
		adminRole, err = h.roleService.BaseServiceMongoImpl.InsertOne(initCtx, newAdminRole)
		if err != nil {
			return fmt.Errorf("failed to create administrator role: %v", err)
		}

		// Chuyển đổi sang model để sử dụng
		bsonBytes, _ := bson.Marshal(adminRole)
		err = bson.Unmarshal(bsonBytes, &modelRole)
		if err != nil {
			return fmt.Errorf("failed to decode administrator role: %v", err)
		}
	}

	// Đảm bảo role Administrator có đầy đủ tất cả permissions
	// Lấy danh sách tất cả các quyền
	permissions, err := h.permissionService.BaseServiceMongoImpl.Find(context.TODO(), bson.M{}, nil)
	if err != nil {
		return fmt.Errorf("failed to get permissions: %v", err)
	}

	// Gán tất cả quyền cho vai trò Administrator với Scope = 1 (Tổ chức đó và tất cả các tổ chức con)
	for _, permissionData := range permissions {
		var modelPermission authmodels.Permission
		bsonBytes, _ := bson.Marshal(permissionData)
		err := bson.Unmarshal(bsonBytes, &modelPermission)
		if err != nil {
			continue // Bỏ qua permission không decode được
		}

		// Kiểm tra quyền đã được gán chưa
		filter := bson.M{
			"roleId":       modelRole.ID,
			"permissionId": modelPermission.ID,
		}

		existingRP, err := h.rolePermissionService.BaseServiceMongoImpl.FindOne(context.TODO(), filter, nil)
		if err != nil && err != common.ErrNotFound {
			continue // Bỏ qua nếu có lỗi khác ErrNotFound
		}

		// Nếu chưa có quyền, thêm mới với Scope = 1 (Tổ chức đó và tất cả các tổ chức con)
		if err == common.ErrNotFound {
			rolePermission := authmodels.RolePermission{
				RoleID:       modelRole.ID,
				PermissionID: modelPermission.ID,
				Scope:        1, // Scope = 1: Tổ chức đó và tất cả các tổ chức con - Vì thuộc Root, sẽ xem tất cả
			}
			_, err = h.rolePermissionService.BaseServiceMongoImpl.InsertOne(context.TODO(), rolePermission)
			if err != nil {
				continue // Bỏ qua nếu insert thất bại
			}
		} else {
			// Nếu đã có, kiểm tra scope - nếu là 0 thì cập nhật thành 1 (để admin có quyền xem tất cả)
			var existingModelRP authmodels.RolePermission
			bsonBytes, _ := bson.Marshal(existingRP)
			err = bson.Unmarshal(bsonBytes, &existingModelRP)
			if err == nil && existingModelRP.Scope == 0 {
				// Cập nhật scope từ 0 → 1 (chỉ tổ chức → tổ chức + các tổ chức con)
				updateData := bson.M{
					"$set": bson.M{
						"scope": 1,
					},
				}
				_, err = h.rolePermissionService.BaseServiceMongoImpl.UpdateOne(context.TODO(), bson.M{"_id": existingModelRP.ID}, updateData, nil)
				if err != nil {
					// Log error nhưng tiếp tục với permission tiếp theo
					continue
				}
			}
		}
	}

	return nil
}

// CheckPermissionForAdministrator kiểm tra và cập nhật quyền cho vai trò Administrator
// Đảm bảo vai trò Administrator có đầy đủ tất cả các quyền trong hệ thống
func (h *InitService) CheckPermissionForAdministrator() (err error) {
	// Kiểm tra vai trò Administrator có tồn tại không
	role, err := h.roleService.BaseServiceMongoImpl.FindOne(context.TODO(), bson.M{"name": "Administrator"}, nil)
	if err != nil && err != common.ErrNotFound {
		return err
	}
	// Nếu chưa có vai trò Administrator, tạo mới
	if err == common.ErrNotFound {
		return h.InitRole()
	}

	// Chuyển đổi dữ liệu sang model
	var modelRole authmodels.Role
	bsonBytes, _ := bson.Marshal(role)
	err = bson.Unmarshal(bsonBytes, &modelRole)
	if err != nil {
		return common.ErrInvalidFormat
	}

	// Lấy danh sách tất cả các quyền
	permissions, err := h.permissionService.BaseServiceMongoImpl.Find(context.TODO(), bson.M{}, nil)
	if err != nil {
		return common.ErrInvalidInput
	}

	// Kiểm tra và cập nhật từng quyền cho vai trò Administrator
	for _, permissionData := range permissions {
		var modelPermission authmodels.Permission
		bsonBytes, _ := bson.Marshal(permissionData)
		err := bson.Unmarshal(bsonBytes, &modelPermission)
		if err != nil {
			// Log error nhưng tiếp tục với permission tiếp theo
			_ = fmt.Errorf("failed to decode permission: %v", err)
			continue
		}

		// Kiểm tra quyền đã được gán chưa (không filter scope)
		filter := bson.M{
			"roleId":       modelRole.ID,
			"permissionId": modelPermission.ID,
		}

		existingRP, err := h.rolePermissionService.BaseServiceMongoImpl.FindOne(context.TODO(), filter, nil)
		if err != nil && err != common.ErrNotFound {
			continue
		}

		// Nếu chưa có quyền, thêm mới với Scope = 1 (Tổ chức đó và tất cả các tổ chức con)
		if err == common.ErrNotFound {
			rolePermission := authmodels.RolePermission{
				RoleID:       modelRole.ID,
				PermissionID: modelPermission.ID,
				Scope:        1, // Scope = 1: Tổ chức đó và tất cả các tổ chức con - Vì thuộc Root, sẽ xem tất cả
			}
			_, err = h.rolePermissionService.BaseServiceMongoImpl.InsertOne(context.TODO(), rolePermission)
			if err != nil {
				// Log error nhưng tiếp tục với permission tiếp theo
				_ = fmt.Errorf("failed to insert role permission: %v", err)
				continue
			}
		} else {
			// Nếu đã có, kiểm tra scope - nếu là 0 thì cập nhật thành 1 (để admin có quyền xem tất cả)
			var existingModelRP authmodels.RolePermission
			bsonBytes, _ := bson.Marshal(existingRP)
			err = bson.Unmarshal(bsonBytes, &existingModelRP)
			if err == nil && existingModelRP.Scope == 0 {
				// Cập nhật scope từ 0 → 1 (chỉ tổ chức → tổ chức + các tổ chức con)
				updateData := bson.M{
					"$set": bson.M{
						"scope": 1,
					},
				}
				_, err = h.rolePermissionService.BaseServiceMongoImpl.UpdateOne(context.TODO(), bson.M{"_id": existingModelRP.ID}, updateData, nil)
				if err != nil {
					// Log error nhưng tiếp tục với permission tiếp theo
					_ = fmt.Errorf("failed to update role permission scope: %v", err)
				}
			}
		}
	}

	return nil
}

// SetAdministrator gán quyền Administrator cho một người dùng
// Trả về lỗi nếu người dùng không tồn tại hoặc đã có quyền Administrator
func (h *InitService) SetAdministrator(userID primitive.ObjectID) (result interface{}, err error) {
	// Kiểm tra user có tồn tại không
	user, err := h.userService.BaseServiceMongoImpl.FindOneById(context.TODO(), userID)
	if err != nil {
		return nil, err
	}

	// Kiểm tra role Administrator có tồn tại không
	role, err := h.roleService.BaseServiceMongoImpl.FindOne(context.TODO(), bson.M{"name": "Administrator"}, nil)
	if err != nil && err != common.ErrNotFound {
		return nil, err
	}

	// Nếu chưa có role Administrator, tạo mới
	if err == common.ErrNotFound {
		err = h.InitRole()
		if err != nil {
			return nil, err
		}

		role, err = h.roleService.BaseServiceMongoImpl.FindOne(context.TODO(), bson.M{"name": "Administrator"}, nil)
		if err != nil {
			return nil, err
		}
	}

	// Kiểm tra userRole đã tồn tại chưa
	_, err = h.userRoleService.BaseServiceMongoImpl.FindOne(context.TODO(), bson.M{"userId": user.ID, "roleId": role.ID}, nil)
	// Kiểm tra nếu userRole đã tồn tại
	if err == nil {
		// Nếu không có lỗi, tức là đã tìm thấy userRole, trả về lỗi đã định nghĩa
		return nil, common.ErrUserAlreadyAdmin
	}

	// Xử lý các lỗi khác ngoài ErrNotFound
	if err != common.ErrNotFound {
		return nil, err
	}

	// Nếu userRole chưa tồn tại (err == utility.ErrNotFound), tạo mới
	userRole := authmodels.UserRole{
		UserID: user.ID,
		RoleID: role.ID,
	}
	result, err = h.userRoleService.BaseServiceMongoImpl.InsertOne(context.TODO(), userRole)
	if err != nil {
		return nil, err
	}

	// Đảm bảo role Administrator có đầy đủ tất cả các quyền trong hệ thống
	// Gọi CheckPermissionForAdministrator để cập nhật quyền cho role Administrator
	err = h.CheckPermissionForAdministrator()
	if err != nil {
		// Log lỗi nhưng không fail việc set administrator
		// Vì role đã được gán, chỉ là quyền có thể chưa được cập nhật đầy đủ
		_ = fmt.Errorf("failed to check permissions for administrator: %v", err)
	}

	return result, nil
}

// InitAdminUser tạo user admin tự động từ Firebase UID (nếu có config)
// Sử dụng khi có FIREBASE_ADMIN_UID trong config
// User sẽ được tạo từ Firebase và tự động gán role Administrator
func (h *InitService) InitAdminUser(firebaseUID string) error {
	if firebaseUID == "" {
		return nil // Không có config, bỏ qua
	}

	// Kiểm tra user đã tồn tại chưa
	filter := bson.M{"firebaseUid": firebaseUID}
	existingUser, err := h.userService.BaseServiceMongoImpl.FindOne(context.TODO(), filter, nil)
	if err != nil && err != common.ErrNotFound {
		return fmt.Errorf("failed to check existing admin user: %v", err)
	}

	var userID primitive.ObjectID

	// Nếu user chưa tồn tại, tạo từ Firebase
	if err == common.ErrNotFound {
		// Lấy thông tin user từ Firebase
		firebaseUser, err := utility.GetUserByUID(context.TODO(), firebaseUID)
		if err != nil {
			return fmt.Errorf("failed to get user from Firebase: %v", err)
		}

		// Tạo user mới
		currentTime := time.Now().Unix()
		newUser := &authmodels.User{
			FirebaseUID:   firebaseUID,
			Email:         firebaseUser.Email,
			EmailVerified: firebaseUser.EmailVerified,
			Phone:         firebaseUser.PhoneNumber,
			PhoneVerified: firebaseUser.PhoneNumber != "",
			Name:          firebaseUser.DisplayName,
			AvatarURL:     firebaseUser.PhotoURL,
			IsBlock:       false,
			Tokens:        []authmodels.Token{},
			CreatedAt:     currentTime,
			UpdatedAt:     currentTime,
		}

		createdUser, err := h.userService.BaseServiceMongoImpl.InsertOne(context.TODO(), *newUser)
		if err != nil {
			return fmt.Errorf("failed to create admin user: %v", err)
		}

		userID = createdUser.ID
	} else {
		// User đã tồn tại
		userID = existingUser.ID
	}

	// Gán role Administrator cho user
	_, err = h.SetAdministrator(userID)
	if err != nil && err != common.ErrUserAlreadyAdmin {
		return fmt.Errorf("failed to set administrator role: %v", err)
	}

	return nil
}

// GetInitStatus kiểm tra trạng thái khởi tạo hệ thống
// Trả về thông tin về các đơn vị cơ bản đã được khởi tạo chưa
func (h *InitService) GetInitStatus() (map[string]interface{}, error) {
	status := make(map[string]interface{})

	// Kiểm tra Organization Root
	_, err := h.GetRootOrganization()
	status["organization"] = map[string]interface{}{
		"initialized": err == nil,
		"error": func() string {
			if err != nil {
				return err.Error()
			} else {
				return ""
			}
		}(),
	}

	// Kiểm tra Permissions
	permissions, err := h.permissionService.BaseServiceMongoImpl.Find(context.TODO(), bson.M{}, nil)
	permissionCount := 0
	if err == nil {
		permissionCount = len(permissions)
	}
	status["permissions"] = map[string]interface{}{
		"initialized": err == nil && permissionCount > 0,
		"count":       permissionCount,
		"error": func() string {
			if err != nil {
				return err.Error()
			} else {
				return ""
			}
		}(),
	}

	// Kiểm tra Role Administrator và admin users
	adminRole, err := h.roleService.BaseServiceMongoImpl.FindOne(context.TODO(), bson.M{"name": "Administrator"}, nil)
	status["roles"] = map[string]interface{}{
		"initialized": err == nil,
		"error": func() string {
			if err != nil && err != common.ErrNotFound {
				return err.Error()
			} else {
				return ""
			}
		}(),
	}
	adminUserCount := 0
	if err == nil {
		var modelRole authmodels.Role
		bsonBytes, _ := bson.Marshal(adminRole)
		if err := bson.Unmarshal(bsonBytes, &modelRole); err == nil {
			userRoles, err := h.userRoleService.BaseServiceMongoImpl.Find(context.TODO(), bson.M{"roleId": modelRole.ID}, nil)
			if err == nil {
				adminUserCount = len(userRoles)
			}
		}
	}
	status["adminUsers"] = map[string]interface{}{
		"count":    adminUserCount,
		"hasAdmin": adminUserCount > 0,
	}

	return status, nil
}

// HasAnyAdministrator kiểm tra xem hệ thống đã có administrator chưa
// Returns:
//   - bool: true nếu đã có ít nhất một administrator
//   - error: Lỗi nếu có
func (h *InitService) HasAnyAdministrator() (bool, error) {
	// Kiểm tra role Administrator có tồn tại không
	adminRole, err := h.roleService.BaseServiceMongoImpl.FindOne(context.TODO(), bson.M{"name": "Administrator"}, nil)
	if err != nil {
		if err == common.ErrNotFound {
			return false, nil // Chưa có role Administrator
		}
		return false, err
	}

	// Chuyển đổi sang model
	var modelRole authmodels.Role
	bsonBytes, _ := bson.Marshal(adminRole)
	if err := bson.Unmarshal(bsonBytes, &modelRole); err != nil {
		return false, err
	}

	// Kiểm tra có user nào có role Administrator không
	userRoles, err := h.userRoleService.BaseServiceMongoImpl.Find(context.TODO(), bson.M{"roleId": modelRole.ID}, nil)
	if err != nil {
		return false, err
	}

	return len(userRoles) > 0, nil
}

// InitNotificationData khởi tạo dữ liệu mặc định cho hệ thống notification
// Tạo các sender và template mặc định (global), các thông tin như token/password sẽ để trống để admin bổ sung sau
// Returns:
//   - error: Lỗi nếu có trong quá trình khởi tạo
func (h *InitService) InitNotificationData() error {
	// Sử dụng context cho phép insert system data trong quá trình init
	// Context cho phép insert system data trong init (từ package services)
	ctx := basesvc.WithSystemDataInsertAllowed(context.TODO())
	currentTime := time.Now().Unix()
	var err error

	// ==================================== 0. LẤY SYSTEM ORGANIZATION =============================================
	// Dữ liệu mẫu notification là dữ liệu hệ thống, thuộc về System Organization (level -1)
	systemOrg, err := h.GetRootOrganization()
	if err != nil {
		return fmt.Errorf("failed to get system organization: %v", err)
	}

	// ==================================== 0.1. KHỞI TẠO TEAM MẶC ĐỊNH CHO NOTIFICATION =============================================
	// Lưu ý: Không cần tạo Tech Team nữa vì channels hệ thống thuộc về System Organization trực tiếp
	// Tech Team vẫn có thể được tạo nếu cần cho mục đích khác, nhưng không bắt buộc cho notification channels

	// ==================================== 1. KHỞI TẠO NOTIFICATION SENDERS CHO SYSTEM ORGANIZATION =============================================
	// Senders là dữ liệu hệ thống, thuộc về System Organization để có thể được share với tất cả organizations
	// Sender cho Email
	emailSenderFilter := bson.M{
		"ownerOrganizationId": systemOrg.ID,
		"channelType":         "email",
		"name":                "Email Sender Mặc Định",
	}
	_, err = h.notificationSenderService.FindOne(ctx, emailSenderFilter, nil)
	if err != nil && err != common.ErrNotFound {
		return fmt.Errorf("failed to check existing email sender: %v", err)
	}
	if err == common.ErrNotFound {
		emailSender := notifmodels.NotificationChannelSender{
			OwnerOrganizationID: &systemOrg.ID, // Thuộc về System Organization (dữ liệu hệ thống) - Phân quyền dữ liệu
			ChannelType:         "email",
			Name:                "Email Sender Mặc Định",
			Description:         "Cấu hình sender email mặc định của hệ thống. Dùng để gửi thông báo qua email. Admin cần cấu hình SMTP credentials trước khi sử dụng.",
			IsActive:            false, // Tắt mặc định, admin cần cấu hình token/password trước khi bật
			IsSystem:            true,  // Đánh dấu là dữ liệu hệ thống, không thể xóa
			SMTPHost:            "",    // Admin cần bổ sung
			SMTPPort:            587,   // Port mặc định
			SMTPUsername:        "",    // Admin cần bổ sung
			SMTPPassword:        "",    // Admin cần bổ sung
			FromEmail:           "",    // Admin cần bổ sung
			FromName:            "",    // Admin cần bổ sung
			CreatedAt:           currentTime,
			UpdatedAt:           currentTime,
		}
		_, err = h.notificationSenderService.InsertOne(ctx, emailSender)
		if err != nil {
			return fmt.Errorf("failed to create email sender: %v", err)
		}
	}

	// Sender cho Telegram
	telegramSenderFilter := bson.M{
		"ownerOrganizationId": systemOrg.ID,
		"channelType":         "telegram",
		"name":                "Telegram Bot Mặc Định",
	}
	_, err = h.notificationSenderService.FindOne(ctx, telegramSenderFilter, nil)
	if err != nil && err != common.ErrNotFound {
		return fmt.Errorf("failed to check existing telegram sender: %v", err)
	}
	if err == common.ErrNotFound {
		// Lấy bot token và username từ config (nếu có)
		botToken := ""
		botUsername := ""
		isActive := false
		if global.MongoDB_ServerConfig != nil {
			botToken = global.MongoDB_ServerConfig.TelegramBotToken
			botUsername = global.MongoDB_ServerConfig.TelegramBotUsername
			// Tự động bật nếu có bot token
			if botToken != "" {
				isActive = true
			}
		}

		telegramSender := notifmodels.NotificationChannelSender{
			OwnerOrganizationID: &systemOrg.ID, // Thuộc về System Organization (dữ liệu hệ thống) - Phân quyền dữ liệu
			ChannelType:         "telegram",
			Name:                "Telegram Bot Mặc Định",
			Description:         "Cấu hình Telegram bot mặc định của hệ thống. Dùng để gửi thông báo qua Telegram. Bot token có thể được cấu hình từ environment variables.",
			IsActive:            isActive,    // Tự động bật nếu có bot token từ env, ngược lại tắt mặc định
			IsSystem:            true,        // Đánh dấu là dữ liệu hệ thống, không thể xóa
			BotToken:            botToken,    // Lấy từ env nếu có, ngược lại để trống
			BotUsername:         botUsername, // Lấy từ env nếu có, ngược lại để trống
			CreatedAt:           currentTime,
			UpdatedAt:           currentTime,
		}
		_, err = h.notificationSenderService.InsertOne(ctx, telegramSender)
		if err != nil {
			return fmt.Errorf("failed to create telegram sender: %v", err)
		}
	}

	// Sender cho Webhook
	webhookSenderFilter := bson.M{
		"ownerOrganizationId": systemOrg.ID,
		"channelType":         "webhook",
		"name":                "Webhook Sender Mặc Định",
	}
	_, err = h.notificationSenderService.FindOne(ctx, webhookSenderFilter, nil)
	if err != nil && err != common.ErrNotFound {
		return fmt.Errorf("failed to check existing webhook sender: %v", err)
	}
	if err == common.ErrNotFound {
		webhookSender := notifmodels.NotificationChannelSender{
			OwnerOrganizationID: &systemOrg.ID, // Thuộc về System Organization (dữ liệu hệ thống) - Phân quyền dữ liệu
			ChannelType:         "webhook",
			Name:                "Webhook Sender Mặc Định",
			Description:         "Cấu hình webhook sender mặc định của hệ thống. Dùng để gửi thông báo qua webhook đến các hệ thống bên ngoài. Admin cần cấu hình trước khi sử dụng.",
			IsActive:            false, // Tắt mặc định, admin cần cấu hình trước khi bật
			IsSystem:            true,  // Đánh dấu là dữ liệu hệ thống, không thể xóa
			CreatedAt:           currentTime,
			UpdatedAt:           currentTime,
		}
		_, err = h.notificationSenderService.InsertOne(ctx, webhookSender)
		if err != nil {
			return fmt.Errorf("failed to create webhook sender: %v", err)
		}
	}

	// ==================================== 2. KHỞI TẠO NOTIFICATION TEMPLATES CHO SYSTEM ORGANIZATION =============================================
	// Templates là dữ liệu hệ thống, thuộc về System Organization để có thể được share với tất cả organizations

	// ==================================== 3. KHỞI TẠO NOTIFICATION CHANNELS CHO SYSTEM ORGANIZATION =============================================
	// Channels hệ thống thuộc về System Organization để có thể được share hoặc sử dụng trực tiếp
	// Channels hệ thống thuộc về System Organization để có thể được share hoặc sử dụng trực tiếp
	// Channel Email hệ thống cho System Organization
	systemEmailChannelFilter := bson.M{
		"ownerOrganizationId": systemOrg.ID,
		"channelType":         "email",
		"name":                "System Email Channel",
	}
	_, err = h.notificationChannelService.FindOne(ctx, systemEmailChannelFilter, nil)
	if err == common.ErrNotFound {
		systemEmailChannel := notifmodels.NotificationChannel{
			OwnerOrganizationID: systemOrg.ID, // Thuộc về System Organization - Phân quyền dữ liệu
			ChannelType:         "email",
			Name:                "System Email Channel",
			Description:         "Kênh email hệ thống thuộc System Organization. Dùng để nhận thông báo hệ thống qua email. Có thể được share với tất cả organizations. Admin cần cấu hình danh sách email recipients trước khi sử dụng.",
			IsActive:            false,      // Tắt mặc định, admin cần cấu hình recipients trước khi bật
			IsSystem:            true,       // Đánh dấu là dữ liệu hệ thống, không thể xóa
			Recipients:          []string{}, // Admin cần bổ sung email addresses
			CreatedAt:           currentTime,
			UpdatedAt:           currentTime,
		}
		_, err = h.notificationChannelService.InsertOne(ctx, systemEmailChannel)
		if err != nil {
			return fmt.Errorf("failed to create system email channel: %v", err)
		}
	}

	// Channel Telegram hệ thống cho System Organization
	systemTelegramChannelFilter := bson.M{
		"ownerOrganizationId": systemOrg.ID,
		"channelType":         "telegram",
		"name":                "System Telegram Channel",
	}
	_, err = h.notificationChannelService.FindOne(ctx, systemTelegramChannelFilter, nil)
	if err == common.ErrNotFound {
		// Lấy chat IDs từ config (nếu có)
		chatIDs := []string{}
		isActive := false
		if global.MongoDB_ServerConfig != nil && global.MongoDB_ServerConfig.TelegramChatIDs != "" {
			// Parse chat IDs từ string (phân cách bằng dấu phẩy)
			chatIDStrings := strings.Split(global.MongoDB_ServerConfig.TelegramChatIDs, ",")
			for _, chatID := range chatIDStrings {
				chatID = strings.TrimSpace(chatID)
				if chatID != "" {
					chatIDs = append(chatIDs, chatID)
				}
			}
			// Tự động bật nếu có ít nhất 1 chat ID
			if len(chatIDs) > 0 {
				isActive = true
			}
		}

		systemTelegramChannel := notifmodels.NotificationChannel{
			OwnerOrganizationID: systemOrg.ID, // Thuộc về System Organization - Phân quyền dữ liệu
			ChannelType:         "telegram",
			Name:                "System Telegram Channel",
			Description:         "Kênh Telegram hệ thống thuộc System Organization. Dùng để nhận thông báo hệ thống qua Telegram. Có thể được share với tất cả organizations. Chat IDs có thể được cấu hình từ environment variables.",
			IsActive:            isActive, // Tự động bật nếu có chat IDs từ env, ngược lại tắt mặc định
			IsSystem:            true,     // Đánh dấu là dữ liệu hệ thống, không thể xóa
			ChatIDs:             chatIDs,  // Lấy từ env nếu có, ngược lại để trống
			CreatedAt:           currentTime,
			UpdatedAt:           currentTime,
		}
		_, err = h.notificationChannelService.InsertOne(ctx, systemTelegramChannel)
		if err != nil {
			return fmt.Errorf("failed to create system telegram channel: %v", err)
		}
	}

	// Channel Webhook hệ thống cho System Organization
	systemWebhookChannelFilter := bson.M{
		"ownerOrganizationId": systemOrg.ID,
		"channelType":         "webhook",
		"name":                "System Webhook Channel",
	}
	_, err = h.notificationChannelService.FindOne(ctx, systemWebhookChannelFilter, nil)
	if err == common.ErrNotFound {
		systemWebhookChannel := notifmodels.NotificationChannel{
			OwnerOrganizationID: systemOrg.ID, // Thuộc về System Organization - Phân quyền dữ liệu
			ChannelType:         "webhook",
			Name:                "System Webhook Channel",
			Description:         "Kênh webhook hệ thống thuộc System Organization. Dùng để nhận thông báo hệ thống qua webhook đến các hệ thống bên ngoài. Có thể được share với tất cả organizations. Admin cần cấu hình webhook URL trước khi sử dụng.",
			IsActive:            false,               // Tắt mặc định, admin cần cấu hình webhook URL trước khi bật
			IsSystem:            true,                // Đánh dấu là dữ liệu hệ thống, không thể xóa
			WebhookURL:          "",                  // Admin cần bổ sung webhook URL
			WebhookHeaders:      map[string]string{}, // Admin có thể bổ sung headers nếu cần
			CreatedAt:           currentTime,
			UpdatedAt:           currentTime,
		}
		_, err = h.notificationChannelService.InsertOne(ctx, systemWebhookChannel)
		if err != nil {
			return fmt.Errorf("failed to create system webhook channel: %v", err)
		}
	}

	// ==================================== 4. KHỞI TẠO TEMPLATES CHO CÁC EVENT CẤP HỆ THỐNG =============================================
	systemEvents := []struct {
		eventType string
		subject   string
		content   string
		variables []string
	}{
		{
			eventType: "system_startup",
			subject:   "Hệ thống đã khởi động",
			content: `Xin chào,

Hệ thống đã được khởi động thành công.

Thông tin:
- Thời gian: {{timestamp}}
- Phiên bản: {{version}}
- Môi trường: {{environment}}

Trân trọng,
Hệ thống thông báo`,
			variables: []string{"timestamp", "version", "environment"},
		},
		{
			eventType: "system_shutdown",
			subject:   "Cảnh báo: Hệ thống đang tắt",
			content: `Xin chào,

Hệ thống đang được tắt.

Thông tin:
- Thời gian: {{timestamp}}
- Lý do: {{reason}}

Trân trọng,
Hệ thống thông báo`,
			variables: []string{"timestamp", "reason"},
		},
		{
			eventType: "system_error",
			subject:   "🚨 Lỗi hệ thống nghiêm trọng",
			content: `Xin chào,

Hệ thống đã gặp lỗi nghiêm trọng.

Thông tin lỗi:
- Thời gian: {{timestamp}}
- Loại lỗi: {{errorType}}
- Mô tả: {{errorMessage}}
- Chi tiết: {{errorDetails}}

Vui lòng kiểm tra và xử lý ngay lập tức.

Trân trọng,
Hệ thống thông báo`,
			variables: []string{"timestamp", "errorType", "errorMessage", "errorDetails"},
		},
		{
			eventType: "system_warning",
			subject:   "⚠️ Cảnh báo hệ thống",
			content: `Xin chào,

Hệ thống có cảnh báo cần chú ý.

Thông tin:
- Thời gian: {{timestamp}}
- Loại cảnh báo: {{warningType}}
- Mô tả: {{warningMessage}}

Vui lòng kiểm tra và xử lý.

Trân trọng,
Hệ thống thông báo`,
			variables: []string{"timestamp", "warningType", "warningMessage"},
		},
		{
			eventType: "database_error",
			subject:   "🚨 Lỗi kết nối Database",
			content: `Xin chào,

Hệ thống gặp lỗi khi kết nối với Database.

Thông tin lỗi:
- Thời gian: {{timestamp}}
- Database: {{databaseName}}
- Lỗi: {{errorMessage}}

Vui lòng kiểm tra kết nối database ngay lập tức.

Trân trọng,
Hệ thống thông báo`,
			variables: []string{"timestamp", "databaseName", "errorMessage"},
		},
		{
			eventType: "api_error",
			subject:   "⚠️ Lỗi API",
			content: `Xin chào,

Hệ thống gặp lỗi khi xử lý API request.

Thông tin:
- Thời gian: {{timestamp}}
- Endpoint: {{endpoint}}
- Method: {{method}}
- Lỗi: {{errorMessage}}
- Status Code: {{statusCode}}

Vui lòng kiểm tra và xử lý.

Trân trọng,
Hệ thống thông báo`,
			variables: []string{"timestamp", "endpoint", "method", "errorMessage", "statusCode"},
		},
		{
			eventType: "backup_completed",
			subject:   "✅ Backup hoàn tất",
			content: `Xin chào,

Quá trình backup đã hoàn tất thành công.

Thông tin:
- Thời gian: {{timestamp}}
- Loại backup: {{backupType}}
- Kích thước: {{backupSize}}
- Vị trí: {{backupLocation}}

Trân trọng,
Hệ thống thông báo`,
			variables: []string{"timestamp", "backupType", "backupSize", "backupLocation"},
		},
		{
			eventType: "backup_failed",
			subject:   "❌ Backup thất bại",
			content: `Xin chào,

Quá trình backup đã thất bại.

Thông tin:
- Thời gian: {{timestamp}}
- Loại backup: {{backupType}}
- Lỗi: {{errorMessage}}

Vui lòng kiểm tra và thử lại.

Trân trọng,
Hệ thống thông báo`,
			variables: []string{"timestamp", "backupType", "errorMessage"},
		},
		{
			eventType: "rate_limit_exceeded",
			subject:   "⚠️ Vượt quá Rate Limit",
			content: `Xin chào,

Hệ thống đã vượt quá rate limit.

Thông tin:
- Thời gian: {{timestamp}}
- Endpoint: {{endpoint}}
- IP: {{ipAddress}}
- Số request: {{requestCount}}
- Giới hạn: {{rateLimit}}

Vui lòng kiểm tra và điều chỉnh.

Trân trọng,
Hệ thống thông báo`,
			variables: []string{"timestamp", "endpoint", "ipAddress", "requestCount", "rateLimit"},
		},
		{
			eventType: "security_alert",
			subject:   "🚨 Cảnh báo bảo mật",
			content: `Xin chào,

Hệ thống phát hiện hoạt động đáng ngờ hoặc vi phạm bảo mật.

Thông tin:
- Thời gian: {{timestamp}}
- Loại cảnh báo: {{alertType}}
- Mô tả: {{alertMessage}}
- IP: {{ipAddress}}
- User: {{username}}

Vui lòng kiểm tra và xử lý ngay lập tức.

Trân trọng,
Hệ thống thông báo`,
			variables: []string{"timestamp", "alertType", "alertMessage", "ipAddress", "username"},
		},
		{
			eventType: "system_resource_overload",
			subject:   "⚠️ [Cảnh báo VPS] Tài nguyên quá tải",
			content: `Xin chào,

VPS đang quá tải tài nguyên (CPU, RAM hoặc dung lượng disk).

Thông tin:
- Thời gian: {{timestamp}}
- Trạng thái: {{state}}
- CPU: {{cpuPercent}}%
- RAM: {{ramPercent}}%
- Disk: {{diskPercent}}%

Hệ thống đã tự động throttle workers. Vui lòng kiểm tra và xử lý.

Trân trọng,
Hệ thống thông báo`,
			variables: []string{"timestamp", "state", "cpuPercent", "ramPercent", "diskPercent"},
		},
	}

	// Tạo templates cho mỗi system event (Email, Telegram, Webhook)
	for _, event := range systemEvents {
		// Email template
		emailFilter := bson.M{
			"ownerOrganizationId": systemOrg.ID,
			"eventType":           event.eventType,
			"channelType":         "email",
		}
		_, err = h.notificationTemplateService.FindOne(ctx, emailFilter, nil)
		if err == common.ErrNotFound {
			template := notifmodels.NotificationTemplate{
				OwnerOrganizationID: &systemOrg.ID, // Thuộc về System Organization (dữ liệu hệ thống) - Phân quyền dữ liệu
				EventType:           event.eventType,
				ChannelType:         "email",
				Description:         fmt.Sprintf("Template email mặc định cho event '%s'. Được tạo tự động khi khởi tạo hệ thống.", event.eventType),
				Subject:             event.subject,
				Content:             event.content,
				Variables:           event.variables,
				IsActive:            true,
				IsSystem:            true, // Đánh dấu là dữ liệu hệ thống, không thể xóa
				CreatedAt:           currentTime,
				UpdatedAt:           currentTime,
			}
			_, err = h.notificationTemplateService.InsertOne(ctx, template)
			if err != nil {
				return fmt.Errorf("failed to create %s email template: %v", event.eventType, err)
			}
		}

		// Telegram template
		telegramFilter := bson.M{
			"ownerOrganizationId": systemOrg.ID,
			"eventType":           event.eventType,
			"channelType":         "telegram",
		}
		_, err = h.notificationTemplateService.FindOne(ctx, telegramFilter, nil)
		if err == common.ErrNotFound {
			// Convert content to Telegram format (Markdown)
			telegramContent := event.content
			telegramContent = fmt.Sprintf("*%s*\n\n%s", event.subject, telegramContent)
			// Replace bullet points with Telegram format
			telegramContent = strings.ReplaceAll(telegramContent, "- ", "• ")

			template := notifmodels.NotificationTemplate{
				OwnerOrganizationID: &systemOrg.ID, // Thuộc về System Organization (dữ liệu hệ thống) - Phân quyền dữ liệu
				EventType:           event.eventType,
				ChannelType:         "telegram",
				Description:         fmt.Sprintf("Template Telegram mặc định cho event '%s'. Được tạo tự động khi khởi tạo hệ thống.", event.eventType),
				Subject:             "",
				Content:             telegramContent,
				Variables:           event.variables,
				IsActive:            true,
				IsSystem:            true, // Đánh dấu là dữ liệu hệ thống, không thể xóa
				CreatedAt:           currentTime,
				UpdatedAt:           currentTime,
			}
			_, err = h.notificationTemplateService.InsertOne(ctx, template)
			if err != nil {
				return fmt.Errorf("failed to create %s telegram template: %v", event.eventType, err)
			}
		}

		// Webhook template (JSON format)
		webhookFilter := bson.M{
			"ownerOrganizationId": systemOrg.ID,
			"eventType":           event.eventType,
			"channelType":         "webhook",
		}
		_, err = h.notificationTemplateService.FindOne(ctx, webhookFilter, nil)
		if err == common.ErrNotFound {
			// Create JSON template with all variables
			jsonVars := make([]string, 0)
			for _, v := range event.variables {
				jsonVars = append(jsonVars, fmt.Sprintf(`"%s":"{{%s}}"`, v, v))
			}
			jsonContent := fmt.Sprintf(`{"eventType":"%s",%s}`, event.eventType, strings.Join(jsonVars, ","))

			template := notifmodels.NotificationTemplate{
				OwnerOrganizationID: &systemOrg.ID, // Thuộc về System Organization (dữ liệu hệ thống) - Phân quyền dữ liệu
				EventType:           event.eventType,
				ChannelType:         "webhook",
				Description:         fmt.Sprintf("Template webhook (JSON) mặc định cho event '%s'. Được tạo tự động khi khởi tạo hệ thống.", event.eventType),
				Subject:             "",
				Content:             jsonContent,
				Variables:           event.variables,
				IsActive:            true,
				IsSystem:            true, // Đánh dấu là dữ liệu hệ thống, không thể xóa
				CreatedAt:           currentTime,
				UpdatedAt:           currentTime,
			}
			_, err = h.notificationTemplateService.InsertOne(ctx, template)
			if err != nil {
				return fmt.Errorf("failed to create %s webhook template: %v", event.eventType, err)
			}
		}
	}

	// ==================================== 5. KHỞI TẠO ROUTING RULES MẶC ĐỊNH =============================================
	// Tạo routing rules để kết nối system events với System Organization channels
	// Lưu ý: Routing rules thuộc về System Organization (ownerOrganizationId = systemOrg.ID)
	// và gửi notification cho System Organization (organizationIds = [systemOrg.ID]) để sử dụng channels hệ thống
	// Lưu ý: Nếu có lỗi duplicate, chỉ log warning và tiếp tục (không return error)
	for _, event := range systemEvents {
		// Query với eventType cụ thể và ownerOrganizationId để kiểm tra duplicate
		routingFilter := bson.M{
			"eventType":           event.eventType, // Query trực tiếp với string (EventType giờ là string, không phải *string)
			"ownerOrganizationId": systemOrg.ID,    // Filter theo ownerOrganizationId để tránh duplicate
		}
		existingRule, err := h.notificationRoutingService.FindOne(ctx, routingFilter, nil)
		if err == common.ErrNotFound {
			// Chưa có rule cho eventType này, tạo mới
			routingRule := notifmodels.NotificationRoutingRule{
				OwnerOrganizationID: systemOrg.ID,    // Thuộc về System Organization (phân quyền dữ liệu)
				EventType:           event.eventType, // EventType giờ là string, không phải *string
				Description:         fmt.Sprintf("Routing rule mặc định cho event '%s'. Gửi thông báo đến System Organization qua tất cả các kênh hệ thống (email, telegram, webhook). Được tạo tự động khi khởi tạo hệ thống.", event.eventType),
				OrganizationIDs:     []primitive.ObjectID{systemOrg.ID},       // System Organization nhận notification (logic nghiệp vụ) - sử dụng channels hệ thống
				ChannelTypes:        []string{"email", "telegram", "webhook"}, // Tất cả channel types
				IsActive:            false,                                    // Tắt mặc định, admin cần bật sau khi cấu hình channels
				IsSystem:            true,                                     // Đánh dấu là dữ liệu hệ thống, không thể xóa
				CreatedAt:           currentTime,
				UpdatedAt:           currentTime,
			}
			createdRule, err := h.notificationRoutingService.InsertOne(ctx, routingRule)
			if err != nil {
				// Kiểm tra xem có phải lỗi duplicate key không
				if errors.Is(err, common.ErrMongoDuplicate) {
					// Lỗi duplicate key - rule đã tồn tại (có thể do race condition hoặc query không tìm thấy)
					// Thử query lại với nhiều cách khác nhau để tìm rule đã tồn tại
					var existingRule notifmodels.NotificationRoutingRule
					var queryErr error

					// Cách 1: Query với filter ban đầu
					existingRule, queryErr = h.notificationRoutingService.FindOne(ctx, routingFilter, nil)
					if queryErr != nil {
						// Cách 2: Query chỉ với ownerOrganizationId và eventType (không dùng filter phức tạp)
						simpleFilter := bson.M{
							"ownerOrganizationId": systemOrg.ID,
							"eventType":           event.eventType,
						}
						existingRule, queryErr = h.notificationRoutingService.FindOne(ctx, simpleFilter, nil)
					}

					if queryErr == nil {
						logrus.WithFields(logrus.Fields{
							"eventType": event.eventType,
							"ruleId":    existingRule.ID.Hex(),
						}).Infof("ℹ️  [INIT] Routing rule for eventType '%s' already exists (detected via duplicate key error), skipping...", event.eventType)
					} else {
						// Không thể query lại rule đã tồn tại, nhưng duplicate key error cho thấy rule đã tồn tại
						// Log info thay vì warning vì đây là trường hợp bình thường (rule đã tồn tại)
						logrus.WithFields(logrus.Fields{
							"eventType": event.eventType,
							"error":     err.Error(),
						}).Infof("ℹ️  [INIT] Routing rule for eventType '%s' already exists (duplicate key detected, không thể query lại nhưng rule đã tồn tại), skipping...", event.eventType)
					}
				} else {
					// Lỗi khác, log warning và tiếp tục
					logrus.WithError(err).Warnf("⚠️ [INIT] Failed to create routing rule for %s, tiếp tục...", event.eventType)
				}
				// Không return error, tiếp tục với event tiếp theo
			} else {
				logrus.WithFields(logrus.Fields{
					"eventType": event.eventType,
					"ruleId":    createdRule.ID.Hex(),
				}).Infof("✅ [INIT] Created routing rule for eventType '%s'", event.eventType)
			}
		} else if err != nil {
			// Lỗi khác khi query, log warning và tiếp tục
			logrus.WithError(err).Warnf("⚠️ [INIT] Failed to check existing routing rule for %s, tiếp tục...", event.eventType)
		} else {
			// Rule đã tồn tại, log info
			logrus.WithFields(logrus.Fields{
				"eventType": event.eventType,
				"ruleId":    existingRule.ID.Hex(),
			}).Infof("ℹ️  [INIT] Routing rule for eventType '%s' already exists, skipping...", event.eventType)
		}
	}

	// ==================================== 6. TẠO ORGANIZATION SHARE ĐỂ SHARE DỮ LIỆU NOTIFICATION =============================================
	// Share dữ liệu notification (senders, templates) từ System Organization đến tất cả organizations khác
	// Đây là dữ liệu hệ thống, cần được share để các organizations có thể sử dụng
	// Phân biệt:
	// - Phân quyền dữ liệu: Senders/Templates thuộc System Organization (ownerOrganizationId = systemOrg.ID)
	// - Logic kinh doanh: Senders/Templates được share với tất cả organizations để sử dụng
	logrus.WithFields(logrus.Fields{
		"systemOrgID": systemOrg.ID.Hex(),
	}).Info("🔄 [INIT] Initializing notification data share for System Organization")
	err = h.initNotificationDataShare(ctx, systemOrg.ID, currentTime)
	if err != nil {
		logrus.WithError(err).Error("❌ [INIT] Failed to initialize notification data share")
		return fmt.Errorf("failed to initialize notification data share: %v", err)
	}
	logrus.Info("✅ [INIT] Notification data share initialized successfully")

	return nil
}

// initNotificationDataShare tạo OrganizationShare để share dữ liệu notification từ System Organization
// đến tất cả organizations (sử dụng "share all" với ToOrgIDs = [])
func (h *InitService) initNotificationDataShare(ctx context.Context, systemOrgID primitive.ObjectID, currentTime int64) error {
	logrus.Info("🔄 [INIT] Initializing notification data share...")

	// Permissions cần share cho notification data
	notificationPermissions := []string{
		"NotificationSender.Read",
		"NotificationTemplate.Read",
	}

	// Kiểm tra share đã tồn tại chưa
	// Tìm share có ownerOrganizationId = systemOrgID và ToOrgIDs rỗng (share với tất cả)
	existingShareFilter := bson.M{
		"ownerOrganizationId": systemOrgID,
		"$or": []bson.M{
			{"toOrgIds": bson.M{"$exists": false}},
			{"toOrgIds": bson.M{"$size": 0}},
			{"toOrgIds": nil},
		},
	}
	existingShares, err := h.organizationShareService.BaseServiceMongoImpl.Find(ctx, existingShareFilter, nil)
	if err != nil {
		logrus.WithError(err).Error("❌ [INIT] Failed to check existing notification share")
		return fmt.Errorf("failed to check existing notification share: %v", err)
	}

	logrus.WithFields(logrus.Fields{
		"systemOrgID": systemOrgID.Hex(),
		"foundShares": len(existingShares),
		"filter":      existingShareFilter,
	}).Debug("🔍 [INIT] Checking for existing notification shares")

	// Tìm share có cùng permissions hoặc share tất cả permissions
	var existingShare *authmodels.OrganizationShare
	for i := range existingShares {
		share := existingShares[i]
		logrus.WithFields(logrus.Fields{
			"shareID":         share.ID.Hex(),
			"permissionNames": share.PermissionNames,
			"toOrgIDs":        share.ToOrgIDs,
		}).Debug("🔍 [INIT] Checking share")

		// Nếu share có permissionNames rỗng/nil → share tất cả permissions → phù hợp
		if len(share.PermissionNames) == 0 {
			existingShare = &share
			logrus.WithFields(logrus.Fields{
				"shareID": share.ID.Hex(),
			}).Debug("✅ [INIT] Found share with empty permissionNames (share all)")
			break
		}
		// Nếu share có cùng permissions → phù hợp
		if len(share.PermissionNames) == len(notificationPermissions) {
			hasAllPerms := true
			permMap := make(map[string]bool)
			for _, p := range share.PermissionNames {
				permMap[p] = true
			}
			for _, p := range notificationPermissions {
				if !permMap[p] {
					hasAllPerms = false
					break
				}
			}
			if hasAllPerms {
				existingShare = &share
				logrus.WithFields(logrus.Fields{
					"shareID": share.ID.Hex(),
				}).Debug("✅ [INIT] Found share with matching permissions")
				break
			}
		}
	}

	if existingShare == nil {
		// Chưa có share phù hợp, tạo mới với "share all" (ToOrgIDs = [])
		logrus.Info("📝 [INIT] No existing notification share found, creating new one")
		share := authmodels.OrganizationShare{
			OwnerOrganizationID: systemOrgID,
			ToOrgIDs:            []primitive.ObjectID{}, // Share với tất cả organizations (empty array = share all)
			PermissionNames:     notificationPermissions,
			Description:         "Share dữ liệu notification (senders và templates) từ System Organization để tất cả các tổ chức có thể sử dụng. Được tạo tự động khi khởi tạo hệ thống.",
			CreatedAt:           currentTime,
			CreatedBy:           primitive.NilObjectID, // System-initiated share
		}
		logrus.WithFields(logrus.Fields{
			"ownerOrgID":  systemOrgID.Hex(),
			"toOrgIDs":    share.ToOrgIDs,
			"permissions": share.PermissionNames,
		}).Debug("📝 [INIT] Attempting to insert notification share")

		createdShare, err := h.organizationShareService.BaseServiceMongoImpl.InsertOne(ctx, share)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"ownerOrgID": systemOrgID.Hex(),
			}).Error("❌ [INIT] Failed to insert notification data share")
			return fmt.Errorf("failed to create notification data share: %v", err)
		}
		// Log để debug
		logrus.WithFields(logrus.Fields{
			"shareID":     createdShare.ID.Hex(),
			"ownerOrgID":  systemOrgID.Hex(),
			"toOrgIDs":    "[] (share all)",
			"permissions": notificationPermissions,
			"description": share.Description,
		}).Info("✅ [INIT] Created notification data share")
	} else {
		// Đã có share, kiểm tra xem có cần cập nhật description, permissions, hoặc toOrgIDs không
		needsUpdate := false
		updateData := bson.M{}

		// Kiểm tra toOrgIDs - nếu không có hoặc nil, set thành [] (share all)
		if existingShare.ToOrgIDs == nil {
			updateData["toOrgIDs"] = []primitive.ObjectID{}
			needsUpdate = true
		}

		// Kiểm tra description
		if existingShare.Description == "" {
			updateData["description"] = "Share dữ liệu notification (senders và templates) từ System Organization để tất cả các tổ chức có thể sử dụng. Được tạo tự động khi khởi tạo hệ thống."
			needsUpdate = true
		}

		// Kiểm tra permissions - nếu share có permissionNames rỗng (share all) nhưng cần share cụ thể
		// thì không cần update (vì share all đã bao gồm)
		// Nếu share có permissions khác, cần merge hoặc update
		if len(existingShare.PermissionNames) > 0 {
			// Kiểm tra xem có đủ notification permissions chưa
			hasAllNotificationPerms := true
			permMap := make(map[string]bool)
			for _, p := range existingShare.PermissionNames {
				permMap[p] = true
			}
			for _, p := range notificationPermissions {
				if !permMap[p] {
					hasAllNotificationPerms = false
					break
				}
			}
			// Nếu chưa có đủ notification permissions, merge vào
			if !hasAllNotificationPerms {
				mergedPerms := append(existingShare.PermissionNames, notificationPermissions...)
				// Loại bỏ duplicates
				uniquePerms := []string{}
				seenPerms := make(map[string]bool)
				for _, p := range mergedPerms {
					if !seenPerms[p] {
						seenPerms[p] = true
						uniquePerms = append(uniquePerms, p)
					}
				}
				updateData["permissionNames"] = uniquePerms
				needsUpdate = true
			}
		}

		if needsUpdate {
			updateFilter := bson.M{"_id": existingShare.ID}
			_, err = h.organizationShareService.BaseServiceMongoImpl.UpdateOne(ctx, updateFilter, bson.M{"$set": updateData}, nil)
			if err != nil {
				return fmt.Errorf("failed to update notification share: %v", err)
			}
			// Log để debug
			logrus.WithFields(logrus.Fields{
				"shareID": existingShare.ID.Hex(),
				"updates": updateData,
			}).Info("✅ [INIT] Updated notification data share")
		} else {
			// Log để debug - share đã tồn tại và không cần update
			logrus.WithFields(logrus.Fields{
				"shareID":     existingShare.ID.Hex(),
				"ownerOrgID":  systemOrgID.Hex(),
				"toOrgIDs":    existingShare.ToOrgIDs,
				"permissions": existingShare.PermissionNames,
			}).Info("ℹ️  [INIT] Notification data share already exists")
		}
	}

	return nil
}

// SetCTALibraryService gán CTA Library service (gọi từ handler init để tránh import cycle)
func (h *InitService) SetCTALibraryService(svc CTALibraryIniter) {
	h.ctaLibraryService = svc
}

// InitCTALibrary khởi tạo các CTA Library mặc định cho hệ thống
// Chỉ tạo các CTA cần thiết cho các system events mặc định (system_error, security_alert, etc.)
// Returns:
//   - error: Lỗi nếu có trong quá trình khởi tạo
func (h *InitService) InitCTALibrary() error {
	if h.ctaLibraryService == nil {
		return nil // Chưa inject, bỏ qua (sẽ inject từ handler init)
	}
	// Sử dụng context cho phép insert system data trong quá trình init
	// Context cho phép insert system data trong init (từ package services)
	ctx := basesvc.WithSystemDataInsertAllowed(context.TODO())
	currentTime := time.Now().Unix()

	// Lấy System Organization
	systemOrg, err := h.GetRootOrganization()
	if err != nil {
		return fmt.Errorf("failed to get system organization: %v", err)
	}

	// Danh sách các CTA mặc định cho system events
	// Chỉ tạo các CTA cần thiết cho các system events (system_error, security_alert, database_error, etc.)
	defaultCTAs := []struct {
		code        string
		label       string
		action      string
		style       string
		variables   []string
		description string
	}{
		{
			code:        "contact_support",
			label:       "Liên hệ hỗ trợ",
			action:      "/support/contact",
			style:       "secondary",
			variables:   []string{},
			description: "CTA để liên hệ bộ phận hỗ trợ. Dùng trong các system events cần hỗ trợ như system_error, security_alert, database_error.",
		},
	}

	// Tạo từng CTA mặc định
	for _, ctaData := range defaultCTAs {
		// Kiểm tra CTA đã tồn tại chưa
		ctaFilter := bson.M{
			"ownerOrganizationId": systemOrg.ID,
			"code":                ctaData.code,
		}
		existingCTA, err := h.ctaLibraryService.FindOne(ctx, ctaFilter, nil)
		if err != nil && err != common.ErrNotFound {
			// Lỗi khác, bỏ qua CTA này
			continue
		}

		if err == common.ErrNotFound {
			// Chưa có CTA, tạo mới
			cta := ctamodels.CTALibrary{
				OwnerOrganizationID: systemOrg.ID, // Thuộc về System Organization (dữ liệu hệ thống) - Phân quyền dữ liệu
				Code:                ctaData.code,
				Label:               ctaData.label,
				Action:              ctaData.action,
				Style:               ctaData.style,
				Variables:           ctaData.variables,
				Description:         ctaData.description,
				IsActive:            true,
				IsSystem:            true, // Đánh dấu là dữ liệu hệ thống, không thể xóa
				CreatedAt:           currentTime,
				UpdatedAt:           currentTime,
			}
			_, err = h.ctaLibraryService.InsertOne(ctx, cta)
			if err != nil {
				// Log lỗi nhưng không dừng quá trình init
				continue
			}
		} else {
			// Đã có CTA, kiểm tra xem có cần cập nhật Description không
			if existingCTA.Description == "" {
				updateFilter := bson.M{"_id": existingCTA.ID}
				updateData := bson.M{
					"$set": bson.M{
						"description": ctaData.description,
					},
				}
				_, err = h.ctaLibraryService.UpdateOne(ctx, updateFilter, updateData, nil)
				if err != nil {
					// Log lỗi nhưng không dừng quá trình init
					continue
				}
			}
		}
	}

	return nil
}

// InitAIData khởi tạo dữ liệu mặc định cho hệ thống AI workflow (Module 2)
// Tạo provider profiles, prompt templates, steps, và workflows mẫu
// Returns:
//   - error: Lỗi nếu có trong quá trình khởi tạo
func (h *InitService) InitAIData() error {
	// Sử dụng context cho phép insert system data trong quá trình init
	ctx := basesvc.WithSystemDataInsertAllowed(context.TODO())
	currentTime := time.Now().UnixMilli()

	// Lấy System Organization
	systemOrg, err := h.GetRootOrganization()
	if err != nil {
		return fmt.Errorf("failed to get system organization: %v", err)
	}

	// 1. Khởi tạo AI Provider Profiles
	if err := h.initAIProviderProfiles(ctx, systemOrg.ID, currentTime); err != nil {
		logrus.WithError(err).Warn("Failed to initialize AI provider profiles")
		// Không dừng quá trình init, chỉ log warning
	}

	// 2. Khởi tạo AI Prompt Templates (cần provider profiles)
	if err := h.initAIPromptTemplates(ctx, systemOrg.ID, currentTime); err != nil {
		logrus.WithError(err).Warn("Failed to initialize AI prompt templates")
		// Không dừng quá trình init, chỉ log warning
	}

	// 3. Khởi tạo AI Steps (cần prompt templates)
	if err := h.initAISteps(ctx, systemOrg.ID, currentTime); err != nil {
		logrus.WithError(err).Warn("Failed to initialize AI steps")
		// Không dừng quá trình init, chỉ log warning
	}

	// 4. Khởi tạo AI Workflows (cần steps)
	if err := h.initAIWorkflows(ctx, systemOrg.ID, currentTime); err != nil {
		logrus.WithError(err).Warn("Failed to initialize AI workflows")
		// Không dừng quá trình init, chỉ log warning
	}

	// 5. Khởi tạo AI Workflow Commands (cần workflows và steps)
	if err := h.initAIWorkflowCommands(ctx, systemOrg.ID, currentTime); err != nil {
		logrus.WithError(err).Warn("Failed to initialize AI workflow commands")
		// Không dừng quá trình init, chỉ log warning
	}

	return nil
}

// initAIProviderProfiles khởi tạo các AI provider profiles mẫu
// Tạo profiles cho OpenAI, Anthropic, Google (API keys để trống, admin sẽ cập nhật sau)
func (h *InitService) initAIProviderProfiles(ctx context.Context, systemOrgID primitive.ObjectID, currentTime int64) error {
	defaultProviders := []struct {
		name            string
		description     string
		provider        string
		defaultModel    string
		availableModels []string
		pricingConfig   map[string]interface{}
	}{
		{
			name:            "OpenAI Production",
			description:     "OpenAI provider profile mặc định cho production (API key cần được cấu hình)",
			provider:        aimodels.AIProviderTypeOpenAI,
			defaultModel:    "gpt-4",
			availableModels: []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"},
			pricingConfig: map[string]interface{}{
				"gpt-4": map[string]interface{}{
					"input":  0.03,
					"output": 0.06,
				},
				"gpt-4-turbo": map[string]interface{}{
					"input":  0.01,
					"output": 0.03,
				},
				"gpt-3.5-turbo": map[string]interface{}{
					"input":  0.0015,
					"output": 0.002,
				},
			},
		},
		{
			name:            "Anthropic Production",
			description:     "Anthropic (Claude) provider profile mặc định cho production (API key cần được cấu hình)",
			provider:        aimodels.AIProviderTypeAnthropic,
			defaultModel:    "claude-3-opus",
			availableModels: []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"},
			pricingConfig: map[string]interface{}{
				"claude-3-opus": map[string]interface{}{
					"input":  0.015,
					"output": 0.075,
				},
				"claude-3-sonnet": map[string]interface{}{
					"input":  0.003,
					"output": 0.015,
				},
				"claude-3-haiku": map[string]interface{}{
					"input":  0.00025,
					"output": 0.00125,
				},
			},
		},
		{
			name:            "Google Production",
			description:     "Google (Gemini) provider profile mặc định cho production (API key cần được cấu hình)",
			provider:        aimodels.AIProviderTypeGoogle,
			defaultModel:    "gemini-pro",
			availableModels: []string{"gemini-pro", "gemini-pro-vision"},
			pricingConfig: map[string]interface{}{
				"gemini-pro": map[string]interface{}{
					"input":  0.0005,
					"output": 0.0015,
				},
			},
		},
		{
			name:            "Google AI Studio",
			description:     "Google AI Studio provider profile với các models Gemini mới nhất (gemini-1.5-pro, gemini-1.5-flash). API key cần được cấu hình từ Google AI Studio (https://aistudio.google.com/)",
			provider:        aimodels.AIProviderTypeGoogle,
			defaultModel:    "gemini-1.5-pro",
			availableModels: []string{"gemini-1.5-pro", "gemini-1.5-flash", "gemini-1.5-pro-latest", "gemini-1.5-flash-latest"},
			pricingConfig: map[string]interface{}{
				"gemini-1.5-pro": map[string]interface{}{
					"input":  0.00125,
					"output": 0.005,
				},
				"gemini-1.5-flash": map[string]interface{}{
					"input":  0.000075,
					"output": 0.0003,
				},
				"gemini-1.5-pro-latest": map[string]interface{}{
					"input":  0.00125,
					"output": 0.005,
				},
				"gemini-1.5-flash-latest": map[string]interface{}{
					"input":  0.000075,
					"output": 0.0003,
				},
			},
		},
	}

	for _, providerData := range defaultProviders {
		// Kiểm tra provider profile đã tồn tại chưa
		profileFilter := bson.M{
			"ownerOrganizationId": systemOrgID,
			"name":                providerData.name,
		}
		existingProfile, err := h.aiProviderProfileService.FindOne(ctx, profileFilter, nil)
		if err != nil && err != common.ErrNotFound {
			continue // Lỗi khác, bỏ qua
		}

		if err == common.ErrNotFound {
			// Chưa có, tạo mới
			defaultTemp := 0.7
			defaultMaxTokens := 2000
			profile := aimodels.AIProviderProfile{
				OwnerOrganizationID: systemOrgID,
				Name:                providerData.name,
				Description:         providerData.description,
				Provider:            providerData.provider,
				Status:              aimodels.AIProviderProfileStatusInactive, // Inactive vì chưa có API key
				APIKey:              "",                                     // Để trống, admin sẽ cập nhật sau
				APIKeyEncrypted:     false,
				AvailableModels:     providerData.availableModels,
				Config: &aimodels.AIConfig{
					Model:         providerData.defaultModel,
					Temperature:   &defaultTemp,
					MaxTokens:     &defaultMaxTokens,
					PricingConfig: providerData.pricingConfig,
				},
				CreatedAt: currentTime,
				UpdatedAt: currentTime,
			}
			_, err = h.aiProviderProfileService.InsertOne(ctx, profile)
			if err != nil {
				logrus.WithError(err).Warnf("Failed to create provider profile: %s", providerData.name)
				continue
			}
		} else {
			// Đã có, có thể update description nếu cần
			var existingProfileModel aimodels.AIProviderProfile
			bsonBytes, _ := bson.Marshal(existingProfile)
			if err := bson.Unmarshal(bsonBytes, &existingProfileModel); err == nil {
				if existingProfileModel.Description == "" {
					updateFilter := bson.M{"_id": existingProfileModel.ID}
					updateData := bson.M{
						"$set": bson.M{
							"description": providerData.description,
							"updatedAt":   currentTime,
						},
					}
					_, _ = h.aiProviderProfileService.UpdateOne(ctx, updateFilter, updateData, nil)
				}
			}
		}
	}

	return nil
}

// initAIPromptTemplates khởi tạo các AI prompt templates mẫu
// Tạo templates cho GENERATE, JUDGE, STEP_GENERATION
func (h *InitService) initAIPromptTemplates(ctx context.Context, systemOrgID primitive.ObjectID, currentTime int64) error {
	// Lấy provider profile mặc định (OpenAI Production)
	providerProfileID, _ := h.getProviderProfileByName(ctx, systemOrgID, "OpenAI Production")

	defaultTemplates := []struct {
		name        string
		description string
		type_       string
		version     string
		prompt      string
		variables   []aimodels.AIPromptTemplateVariable
		provider    *aimodels.AIPromptTemplateProvider // Provider info (profileId, config) - override từ provider profile defaultConfig
	}{
		// Template GENERATE chung (có thể dùng cho tất cả level transitions)
		// ĐƠN GIẢN HÓA: Mỗi step chỉ tạo 1 nội dung duy nhất (không còn candidates[])
		{
			name:        "Tạo Nội Dung - Mẫu Chung",
			description: "Template mẫu chung để tạo 1 nội dung cho bất kỳ cấp độ nào (STP, Insight, Content Line, Gene, Script). Mỗi lần chạy chỉ trả về 1 nội dung.",
			type_:       aimodels.AIPromptTemplateTypeGenerate,
			version:     "1.0.0",
			prompt: `Bạn là một chuyên gia content strategy với nhiều năm kinh nghiệm. Nhiệm vụ của bạn là tạo ra 1 nội dung chất lượng cao cho {{targetTypeName}} dựa trên {{parentTypeName}}.

📋 THÔNG TIN ĐẦU VÀO:

Nội dung {{parentTypeName}}:
{{parentText}}
{{#if targetAudience}}

🎯 Đối tượng mục tiêu: {{targetAudience}}
{{/if}}
{{#if metadata.industry}}

🏢 Ngành nghề: {{metadata.industry}}
{{/if}}
{{#if metadata.productType}}

📦 Loại sản phẩm: {{metadata.productType}}
{{/if}}
{{#if metadata.tone}}

💬 Tone mong muốn: {{metadata.tone}}
{{/if}}

✅ YÊU CẦU:

1. Tạo 1 nội dung {{targetTypeName}} chất lượng: nội dung chính đầy đủ, có thể kèm tên ngắn gọn và tóm tắt nếu phù hợp.
2. Nội dung phải phù hợp với đối tượng mục tiêu và context đã cho.
3. Tuân thủ quy tắc: {{targetTypeName}} phải phát triển logic từ {{parentTypeName}}, không được tách rời.`,
			variables: []aimodels.AIPromptTemplateVariable{
				{Name: "parentText", Required: true, Description: "Text của parent node (system tự lấy từ parentNode.Text)"},
				{Name: "parentTypeName", Required: true, Description: "Tên loại parent (Pillar, STP, Insight, etc.)"},
				{Name: "targetTypeName", Required: true, Description: "Tên loại target (STP, Insight, Content Line, etc.)"},
				{Name: "targetAudience", Required: false, Description: "Đối tượng mục tiêu (B2B, B2C, B2B2C)"},
			},
			provider: &aimodels.AIPromptTemplateProvider{
				ProfileID: providerProfileID,
				Config: &aimodels.AIConfig{
					Model:       "gpt-4",
					Temperature: func() *float64 { v := 0.7; return &v }(), // Temperature cho generation
					MaxTokens:   func() *int { v := 2000; return &v }(),
				},
			},
		},
		// Template GENERATE cho Pillar (L1, cấp trên cùng, không có parent)
		// ĐƠN GIẢN HÓA: Mỗi step chỉ tạo 1 Pillar duy nhất
		{
			name:        "Tạo Pillar (L1)",
			description: "Template để tạo 1 Pillar (L1 - cấp trên cùng) từ context (ngành, đối tượng, sản phẩm). Pillar là nền tảng chiến lược nội dung, không cần parent.",
			type_:       aimodels.AIPromptTemplateTypeGenerate,
			version:     "1.0.0",
			prompt: `Bạn là một chuyên gia content strategy với nhiều năm kinh nghiệm. Nhiệm vụ của bạn là tạo ra 1 Pillar (L1 - cấp nội dung trên cùng) chất lượng cao dựa trên context đã cho. Pillar là nền tảng chiến lược, định hướng toàn bộ chuỗi nội dung phía dưới.

📋 THÔNG TIN ĐẦU VÀO:

{{#if metadata.targetAudience}}
🎯 Đối tượng mục tiêu: {{metadata.targetAudience}}
{{/if}}
{{#if metadata.industry}}
🏢 Ngành nghề: {{metadata.industry}}
{{/if}}
{{#if metadata.productType}}
📦 Loại sản phẩm/dịch vụ: {{metadata.productType}}
{{/if}}
{{#if metadata.tone}}
💬 Tone mong muốn: {{metadata.tone}}
{{/if}}
{{#if metadata.brandName}}
🏷️ Tên thương hiệu: {{metadata.brandName}}
{{/if}}

✅ YÊU CẦU:

1. Tạo 1 Pillar chất lượng: mô tả chiến lược nội dung, phạm vi và định hướng; có thể kèm tên ngắn gọn và tóm tắt.
2. Pillar phải phù hợp với đối tượng mục tiêu và context (ngành, sản phẩm).
3. Đảm bảo tính khả thi, rõ ràng để có thể triển khai thành STP → Insight → Content Line → Gene → Script.`,
			variables: []aimodels.AIPromptTemplateVariable{
				{Name: "targetAudience", Required: false, Description: "Đối tượng mục tiêu (B2B, B2C, B2B2C)"},
			},
			provider: &aimodels.AIPromptTemplateProvider{
				ProfileID: providerProfileID,
				Config: &aimodels.AIConfig{
					Model:       "gpt-4",
					Temperature: func() *float64 { v := 0.7; return &v }(),
					MaxTokens:   func() *int { v := 2000; return &v }(),
				},
			},
		},
		// Template GENERATE cho từng level transition - mỗi step chỉ tạo 1 nội dung
		{
			name:        "Tạo STP từ Pillar",
			description: "Template để tạo 1 STP (Segmentation, Targeting, Positioning) từ Pillar. STP bao gồm 3 thành phần: Segmentation, Targeting, Positioning.",
			type_:       aimodels.AIPromptTemplateTypeGenerate,
			version:     "1.0.0",
			prompt: `Bạn là một chuyên gia marketing strategy với nhiều năm kinh nghiệm. Nhiệm vụ của bạn là tạo ra 1 STP (Segmentation, Targeting, Positioning) chất lượng cao từ Pillar.

📋 THÔNG TIN ĐẦU VÀO:

Nội dung Pillar:
{{parentText}}
{{#if targetAudience}}

🎯 Đối tượng mục tiêu: {{targetAudience}}
{{/if}}
{{#if metadata.industry}}

🏢 Ngành nghề: {{metadata.industry}}
{{/if}}

✅ YÊU CẦU:

1. Tạo 1 STP đầy đủ 3 thành phần: Segmentation (Phân khúc), Targeting (Đối tượng), Positioning (Định vị).
2. STP phải logic, phù hợp với Pillar và đối tượng mục tiêu.
3. Đảm bảo tính thực tế, khả thi và có tính phân biệt rõ ràng.`,
			variables: []aimodels.AIPromptTemplateVariable{
				{Name: "parentText", Required: true, Description: "Text của parent node (Pillar)"},
				{Name: "targetAudience", Required: false, Description: "Đối tượng mục tiêu (B2B, B2C, B2B2C)"},
			},
			provider: &aimodels.AIPromptTemplateProvider{
				ProfileID: providerProfileID,
				Config: &aimodels.AIConfig{
					Model:       "gpt-4",
					Temperature: func() *float64 { v := 0.7; return &v }(), // Temperature cho generation
					MaxTokens:   func() *int { v := 2000; return &v }(),
				},
			},
		},
		{
			name:        "Tạo Insight từ STP",
			description: "Template để tạo 1 Insight (góc nhìn sâu sắc) từ STP. Insight là thông tin chi tiết về đối tượng mục tiêu, nhu cầu, hành vi và động cơ.",
			type_:       aimodels.AIPromptTemplateTypeGenerate,
			version:     "1.0.0",
			prompt: `Bạn là một chuyên gia consumer insights với khả năng phân tích sâu sắc về hành vi và tâm lý khách hàng. Nhiệm vụ của bạn là tạo ra 1 Insight (góc nhìn sâu sắc) chất lượng cao từ STP.

📋 THÔNG TIN ĐẦU VÀO:

Nội dung STP:
{{parentText}}
{{#if targetAudience}}

🎯 Đối tượng mục tiêu: {{targetAudience}}
{{/if}}

✅ YÊU CẦU:

1. Tạo 1 Insight: thông tin chi tiết, sâu sắc về đối tượng mục tiêu; góc nhìn mới mẻ, có giá trị thực tế; dựa trên hành vi, nhu cầu, động cơ khách hàng.
2. Insight phải logic, phù hợp với STP và đối tượng mục tiêu.
3. Đảm bảo tính độc đáo và có tính ứng dụng cao.

💡 GỢI Ý: Insight tốt thường trả lời "Tại sao khách hàng lại hành động như vậy?" hoặc "Điều gì thực sự thúc đẩy họ?"`,
			variables: []aimodels.AIPromptTemplateVariable{
				{Name: "parentText", Required: true, Description: "Text của parent node (STP)"},
				{Name: "targetAudience", Required: false, Description: "Đối tượng mục tiêu"},
			},
			provider: &aimodels.AIPromptTemplateProvider{
				ProfileID: providerProfileID,
				Config: &aimodels.AIConfig{
					Model:       "gpt-4",
					Temperature: func() *float64 { v := 0.7; return &v }(), // Temperature cho generation
					MaxTokens:   func() *int { v := 2000; return &v }(),
				},
			},
		},
		{
			name:        "Tạo Content Line từ Insight",
			description: "Template để tạo 1 Content Line (dòng nội dung) từ Insight. Content Line là dòng nội dung cụ thể, có thể triển khai thành content thực tế.",
			type_:       aimodels.AIPromptTemplateTypeGenerate,
			version:     "1.0.0",
			prompt: `Bạn là một chuyên gia content creation với khả năng biến insights thành nội dung hấp dẫn. Nhiệm vụ của bạn là tạo ra 1 Content Line (dòng nội dung) cụ thể, có thể triển khai ngay từ Insight.

📋 THÔNG TIN ĐẦU VÀO:

Nội dung Insight:
{{parentText}}
{{#if targetAudience}}

🎯 Đối tượng mục tiêu: {{targetAudience}}
{{/if}}

✅ YÊU CẦU:

1. Tạo 1 Content Line: dòng nội dung cụ thể, rõ ràng, có thể sử dụng ngay; dựa trên Insight đã cho; phù hợp đối tượng mục tiêu; có thể triển khai thành content thực tế.
2. Content Line phải có chủ đề rõ ràng, góc tiếp cận cụ thể, thông điệp chính.
3. Đảm bảo tính sáng tạo, độc đáo và thực tế.

💡 GỢI Ý: Content Line tốt trả lời "Nội dung này sẽ nói gì với khách hàng?" và "Tại sao họ sẽ quan tâm?"`,
			variables: []aimodels.AIPromptTemplateVariable{
				{Name: "parentText", Required: true, Description: "Text của parent node (Insight)"},
				{Name: "targetAudience", Required: false, Description: "Đối tượng mục tiêu"},
			},
			provider: &aimodels.AIPromptTemplateProvider{
				ProfileID: providerProfileID,
				Config: &aimodels.AIConfig{
					Model:       "gpt-4",
					Temperature: func() *float64 { v := 0.7; return &v }(), // Temperature cho generation
					MaxTokens:   func() *int { v := 2000; return &v }(),
				},
			},
		},
		{
			name:        "Tạo Gene từ Content Line",
			description: "Template để tạo 1 Gene (DNA của nội dung) từ Content Line. Gene định nghĩa tone, style và đặc điểm đặc trưng của nội dung.",
			type_:       aimodels.AIPromptTemplateTypeGenerate,
			version:     "1.0.0",
			prompt: `Bạn là một chuyên gia brand voice và content style với khả năng định nghĩa DNA của nội dung. Nhiệm vụ của bạn là tạo ra 1 Gene (DNA của nội dung) từ Content Line.

📋 THÔNG TIN ĐẦU VÀO:

Nội dung Content Line:
{{parentText}}
{{#if targetAudience}}

🎯 Đối tượng mục tiêu: {{targetAudience}}
{{/if}}

✅ YÊU CẦU:

1. Tạo 1 Gene: Tone (Giọng điệu), Style (Phong cách), Characteristics (Đặc điểm đặc trưng).
2. Gene phải phù hợp với Content Line và đối tượng mục tiêu, tạo sự nhất quán và dễ nhận biết.
3. Đảm bảo tính sáng tạo, độc đáo và có thể áp dụng thực tế.

💡 GỢI Ý: Gene tốt giống như "DNA" của thương hiệu - mọi nội dung đều mang đặc điểm này.`,
			variables: []aimodels.AIPromptTemplateVariable{
				{Name: "parentText", Required: true, Description: "Text của parent node (Content Line)"},
				{Name: "targetAudience", Required: false, Description: "Đối tượng mục tiêu"},
			},
			provider: &aimodels.AIPromptTemplateProvider{
				ProfileID: providerProfileID,
				Config: &aimodels.AIConfig{
					Model:       "gpt-4",
					Temperature: func() *float64 { v := 0.7; return &v }(), // Temperature cho generation
					MaxTokens:   func() *int { v := 2000; return &v }(),
				},
			},
		},
		{
			name:        "Tạo Script từ Gene",
			description: "Template để tạo 1 Script (kịch bản) từ Gene. Script bao gồm Hook, Body và Call-to-Action.",
			type_:       aimodels.AIPromptTemplateTypeGenerate,
			version:     "1.0.0",
			prompt: `Bạn là một chuyên gia scriptwriting và video production với khả năng tạo ra kịch bản hấp dẫn. Nhiệm vụ của bạn là tạo ra 1 Script (kịch bản) chi tiết từ Gene.

📋 THÔNG TIN ĐẦU VÀO:

Nội dung Gene:
{{parentText}}
{{#if targetAudience}}

🎯 Đối tượng mục tiêu: {{targetAudience}}
{{/if}}

✅ YÊU CẦU:

1. Tạo 1 Script kịch bản chi tiết với 3 phần: Hook (3 giây đầu), Body (Nội dung chính), Call-to-Action.
2. Script phải tuân theo tone và style trong Gene, phù hợp đối tượng mục tiêu.
3. Có tính hấp dẫn cao, dễ hiểu và có thể sử dụng ngay để quay video hoặc tạo nội dung.

💡 GỢI Ý: Script tốt có Hook cực mạnh 3 giây đầu, Body logic và hấp dẫn, CTA rõ ràng.`,
			variables: []aimodels.AIPromptTemplateVariable{
				{Name: "parentText", Required: true, Description: "Text của parent node (Gene)"},
				{Name: "targetAudience", Required: false, Description: "Đối tượng mục tiêu"},
			},
			provider: &aimodels.AIPromptTemplateProvider{
				ProfileID: providerProfileID,
				Config: &aimodels.AIConfig{
					Model:       "gpt-4",
					Temperature: func() *float64 { v := 0.7; return &v }(), // Temperature cho generation
					MaxTokens:   func() *int { v := 2500; return &v }(),    // Script cần nhiều tokens hơn
				},
			},
		},
		// Template JUDGE chung (dùng cho tất cả level transitions)
		{
			name:        "Đánh Giá Nội Dung",
			description: "Template để đánh giá và chấm điểm 1 nội dung dựa trên các tiêu chí: Relevance, Clarity, Engagement và Accuracy. Mỗi lần chạy chỉ đánh giá 1 nội dung.",
			type_:       aimodels.AIPromptTemplateTypeJudge,
			version:     "1.0.0",
			prompt: `Bạn là một chuyên gia đánh giá content với khả năng phân tích sâu sắc và công bằng. Nhiệm vụ của bạn là đánh giá và chấm điểm nội dung sau một cách khách quan và chi tiết.

📋 NỘI DUNG CẦN ĐÁNH GIÁ:

{{#if metadata.title}}
Tiêu đề: {{metadata.title}}
{{/if}}

Nội dung:
{{text}}
{{#if metadata.summary}}

Tóm tắt: {{metadata.summary}}
{{/if}}

📊 TIÊU CHÍ ĐÁNH GIÁ:

Bạn cần đánh giá nội dung dựa trên 4 tiêu chí sau (thang điểm 0-10):
- **Relevance (Độ liên quan)**: Nội dung có liên quan chặt chẽ với parent content và mục tiêu không? ({{criteria.relevance}}/10)
- **Clarity (Độ rõ ràng)**: Nội dung có rõ ràng, dễ hiểu, không mơ hồ không? ({{criteria.clarity}}/10)
- **Engagement (Độ hấp dẫn)**: Nội dung có hấp dẫn, thu hút được sự chú ý không? ({{criteria.engagement}}/10)
- **Accuracy (Độ chính xác)**: Nội dung có chính xác, logic, khả thi không? ({{criteria.accuracy}}/10)
{{#if context.targetAudience}}

🎯 Đối tượng mục tiêu: {{context.targetAudience}}
{{/if}}
{{#if context.industry}}

🏢 Ngành nghề: {{context.industry}}
{{/if}}

✅ YÊU CẦU:

1. Tính điểm cho từng tiêu chí (relevance, clarity, engagement, accuracy) - thang điểm 0-10
2. Tính điểm tổng thể (score) - trung bình có trọng số của các tiêu chí (0-10)
3. Cung cấp feedback chi tiết: điểm mạnh và điểm cần cải thiện

📤 ĐỊNH DẠNG KẾT QUẢ (JSON):
{
  "score": 8.5,
  "criteriaScores": {
    "relevance": 9,
    "clarity": 8,
    "engagement": 9,
    "accuracy": 8
  },
  "feedback": "Nhận xét chi tiết về nội dung: điểm mạnh và điểm cần cải thiện..."
}`,
			variables: []aimodels.AIPromptTemplateVariable{
				{Name: "text", Required: true, Description: "Text cần đánh giá (từ GENERATE output)"},
				{Name: "criteria", Required: true, Description: "Tiêu chí đánh giá (relevance, clarity, engagement, accuracy)"},
				{Name: "metadata", Required: false, Description: "Metadata tùy chọn (title, summary, etc.)"},
			},
			provider: &aimodels.AIPromptTemplateProvider{
				ProfileID: providerProfileID,
				Config: &aimodels.AIConfig{
					Model:       "gpt-4",
					Temperature: func() *float64 { v := 0.3; return &v }(), // Temperature thấp hơn cho judging (chính xác hơn)
					MaxTokens:   func() *int { v := 1500; return &v }(),
				},
			},
		},
		// Template STEP_GENERATION (giữ nguyên)
		{
			name:        "Tạo Workflow Steps - Động",
			description: "Template để tạo các bước (steps) động cho workflow dựa trên yêu cầu và context. Template này giúp tự động thiết kế workflow phù hợp với từng tình huống cụ thể, bao gồm số lượng steps, loại steps, dependencies và cấu trúc workflow.",
			type_:       aimodels.AIPromptTemplateTypeStepGeneration,
			version:     "1.0.0",
			prompt: `Bạn là một chuyên gia workflow design. Nhiệm vụ của bạn là tạo ra một kế hoạch workflow với các steps phù hợp.

Context từ parent:
{{parentContext.content}}
{{#if parentContext.type}}

Loại: {{parentContext.type}}
{{/if}}

Yêu cầu:
- Số lượng steps: {{requirements.numberOfSteps}}
- Loại steps cho phép: {{requirements.stepTypes}}
{{#if requirements.focusAreas}}
- Lĩnh vực tập trung: {{requirements.focusAreas}}
{{/if}}
- Độ phức tạp: {{requirements.complexity}}
- Level mục tiêu: {{targetLevel}}
{{#if constraints.maxExecutionTime}}
- Thời gian thực thi tối đa: {{constraints.maxExecutionTime}}s
{{/if}}
{{#if constraints.excludedStepTypes}}
- Loại steps không được dùng: {{constraints.excludedStepTypes}}
{{/if}}

Yêu cầu:
1. Tạo {{requirements.numberOfSteps}} steps phù hợp
2. Mỗi step phải có: name, type, order, inputSchema, outputSchema, description
3. Xác định dependencies giữa các steps
4. Tạo generation plan với workflow structure

Format output (JSON):
{
  "generatedSteps": [
    {
      "stepName": "...",
      "stepType": "GENERATE|JUDGE|STEP_GENERATION",
      "order": 0,
      "inputSchema": {...},
      "outputSchema": {...},
      "description": "...",
      "dependencies": []
    }
  ],
  "generationPlan": {
    "totalSteps": 3,
    "estimatedTime": 120,
    "workflowStructure": {
      "parallelSteps": [],
      "sequentialSteps": [[0, 1, 2]]
    },
    "reasoning": "..."
  }
}`,
			variables: []aimodels.AIPromptTemplateVariable{
				{Name: "parentContext", Required: true, Description: "Context từ parent pillar/step"},
				{Name: "requirements", Required: true, Description: "Yêu cầu generate steps"},
				{Name: "targetLevel", Required: true, Description: "Level mục tiêu (L1-L8)"},
				{Name: "constraints", Required: false, Description: "Ràng buộc cho việc generate"},
			},
			provider: &aimodels.AIPromptTemplateProvider{
				ProfileID: providerProfileID,
				Config: &aimodels.AIConfig{
					Model:       "gpt-4",
					Temperature: func() *float64 { v := 0.8; return &v }(), // Temperature cao hơn cho creativity
					MaxTokens:   func() *int { v := 3000; return &v }(),
				},
			},
		},
	}

	for _, templateData := range defaultTemplates {
		// Kiểm tra template đã tồn tại chưa
		templateFilter := bson.M{
			"ownerOrganizationId": systemOrgID,
			"name":                templateData.name,
			"version":             templateData.version,
		}
		_, err := h.aiPromptTemplateService.FindOne(ctx, templateFilter, nil)
		if err != nil && err != common.ErrNotFound {
			continue // Lỗi khác, bỏ qua
		}

		if err == common.ErrNotFound {
			// Chưa có, tạo mới
			template := aimodels.AIPromptTemplate{
				OwnerOrganizationID: systemOrgID,
				Name:                templateData.name,
				Description:         templateData.description,
				Type:                templateData.type_,
				Version:             templateData.version,
				Prompt:              templateData.prompt,
				Variables:           templateData.variables,
				Provider:            templateData.provider, // Provider info (profileId, config) - override từ provider profile defaultConfig
				Status:              "active",
				CreatedAt:           currentTime,
				UpdatedAt:           currentTime,
			}
			_, err = h.aiPromptTemplateService.InsertOne(ctx, template)
			if err != nil {
				logrus.WithError(err).Warnf("Failed to create prompt template: %s", templateData.name)
				continue
			}
		}
	}

	return nil
}

// initAISteps khởi tạo các AI steps mẫu với standard schemas
// Tạo steps cho tất cả level transitions: L1→L2, L2→L3, L3→L4, L4→L5, L5→L6
// Mỗi transition cần 2 steps: GENERATE + JUDGE
func (h *InitService) initAISteps(ctx context.Context, systemOrgID primitive.ObjectID, currentTime int64) error {
	// Lấy prompt templates
	generatePillarTemplate, _ := h.getPromptTemplateByName(ctx, systemOrgID, "Tạo Pillar (L1)") // Tùy chọn: dùng cho step tạo Pillar (L1)

	generateSTPTemplate, err := h.getPromptTemplateByName(ctx, systemOrgID, "Tạo STP từ Pillar")
	if err != nil {
		logrus.Warn("Generate STP template not found, skipping step creation")
		return nil
	}

	generateInsightTemplate, err := h.getPromptTemplateByName(ctx, systemOrgID, "Tạo Insight từ STP")
	if err != nil {
		logrus.Warn("Generate Insight template not found, skipping step creation")
		return nil
	}

	generateContentLineTemplate, err := h.getPromptTemplateByName(ctx, systemOrgID, "Tạo Content Line từ Insight")
	if err != nil {
		logrus.Warn("Generate Content Line template not found, skipping step creation")
		return nil
	}

	generateGeneTemplate, err := h.getPromptTemplateByName(ctx, systemOrgID, "Tạo Gene từ Content Line")
	if err != nil {
		logrus.Warn("Generate Gene template not found, skipping step creation")
		return nil
	}

	generateScriptTemplate, err := h.getPromptTemplateByName(ctx, systemOrgID, "Tạo Script từ Gene")
	if err != nil {
		logrus.Warn("Generate Script template not found, skipping step creation")
		return nil
	}

	judgeTemplate, err := h.getPromptTemplateByName(ctx, systemOrgID, "Đánh Giá Nội Dung")
	if err != nil {
		logrus.Warn("Judge prompt template not found, skipping step creation")
		return nil
	}

	stepGenTemplate, err := h.getPromptTemplateByName(ctx, systemOrgID, "Tạo Workflow Steps - Động")
	if err != nil {
		logrus.Warn("Step generation prompt template not found, skipping step creation")
		return nil
	}

	// Định nghĩa các steps cho từng level transition
	defaultSteps := []struct {
		name             string
		description      string
		type_            string
		promptTemplateID *primitive.ObjectID
		targetLevel      string
		parentLevel      string
		// KHÔNG có model, temperature, maxTokens - config lưu trong prompt template
	}{
		// L0 → L1: Tạo Pillar (cấp trên cùng, không có parent)
		{
			name:             "Tạo Pillar (L1)",
			description:      "Bước này tạo ra nhiều phương án Pillar (L1 - cấp nội dung trên cùng) từ context (ngành, đối tượng, sản phẩm). Pillar là nền tảng chiến lược, không cần parent. Dùng khi bắt đầu tạo nội dung từ đầu.",
			type_:            aimodels.AIStepTypeGenerate,
			promptTemplateID: generatePillarTemplate,
			targetLevel:      "L1",
			parentLevel:      "",
		},
		{
			name:             "Đánh Giá Phương Án Pillar",
			description:      "Bước này đánh giá và chấm điểm nội dung Pillar đã được tạo ra, dựa trên các tiêu chí: độ rõ ràng, độ khả thi, độ phù hợp với context.",
			type_:            aimodels.AIStepTypeJudge,
			promptTemplateID: judgeTemplate,
			targetLevel:      "L1",
			parentLevel:      "",
		},
		// L1 → L2: Generate STP
		{
			name:             "Tạo STP từ Pillar",
			description:      "Bước này tạo ra nhiều phương án STP (Segmentation, Targeting, Positioning) từ Pillar. Mỗi phương án sẽ bao gồm đầy đủ 3 thành phần: phân khúc khách hàng, đối tượng mục tiêu và định vị sản phẩm/dịch vụ. Bước này giúp xác định chiến lược marketing cơ bản.",
			type_:            aimodels.AIStepTypeGenerate,
			promptTemplateID: generateSTPTemplate,
			targetLevel:      "L2",
			parentLevel:      "L1",
		},
		{
			name:             "Đánh Giá Phương Án STP",
			description:      "Bước này đánh giá và chấm điểm nội dung STP đã được tạo ra, dựa trên các tiêu chí: độ liên quan, độ rõ ràng, độ hấp dẫn và độ chính xác.",
			type_:            aimodels.AIStepTypeJudge,
			promptTemplateID: judgeTemplate,
			targetLevel:      "L2",
			parentLevel:      "L1",
		},
		// L2 → L3: Generate Insight
		{
			name:             "Generate Insight from STP",
			description:      "Step để generate 1 Insight (L3) từ STP (L2)",
			type_:            aimodels.AIStepTypeGenerate,
			promptTemplateID: generateInsightTemplate,
			targetLevel:      "L3",
			parentLevel:      "L2",
		},
		{
			name:             "Đánh Giá Phương Án Insight",
			description:      "Bước này đánh giá và chấm điểm nội dung Insight đã được tạo ra, dựa trên các tiêu chí: độ liên quan, độ rõ ràng, độ hấp dẫn và độ chính xác.",
			type_:            aimodels.AIStepTypeJudge,
			promptTemplateID: judgeTemplate,
			targetLevel:      "L3",
			parentLevel:      "L2",
		},
		// L3 → L4: Generate Content Line
		{
			name:             "Tạo Content Line từ Insight",
			description:      "Bước này tạo ra nhiều phương án Content Line (dòng nội dung) từ Insight đã được chọn. Content Line là những dòng nội dung cụ thể, có thể triển khai trực tiếp thành content thực tế. Bước này giúp biến insights thành nội dung có thể sử dụng ngay.",
			type_:            aimodels.AIStepTypeGenerate,
			promptTemplateID: generateContentLineTemplate,
			targetLevel:      "L4",
			parentLevel:      "L3",
		},
		{
			name:             "Judge Content Line Candidates",
			description:      "Step để đánh giá và chấm điểm 1 Content Line",
			type_:            aimodels.AIStepTypeJudge,
			promptTemplateID: judgeTemplate,
			targetLevel:      "L4",
			parentLevel:      "L3",
		},
		// L4 → L5: Generate Gene
		{
			name:             "Tạo Gene từ Content Line",
			description:      "Bước này tạo ra nhiều phương án Gene (DNA của nội dung) từ Content Line đã được chọn. Gene định nghĩa tone (giọng điệu), style (phong cách) và các đặc điểm đặc trưng của nội dung. Bước này giúp đảm bảo tính nhất quán về phong cách trong tất cả các nội dung được tạo ra.",
			type_:            aimodels.AIStepTypeGenerate,
			promptTemplateID: generateGeneTemplate,
			targetLevel:      "L5",
			parentLevel:      "L4",
		},
		{
			name:             "Judge Gene Candidates",
			description:      "Step để đánh giá và chấm điểm 1 Gene",
			type_:            aimodels.AIStepTypeJudge,
			promptTemplateID: judgeTemplate,
			targetLevel:      "L5",
			parentLevel:      "L4",
		},
		// L5 → L6: Generate Script
		{
			name:             "Tạo Script từ Gene",
			description:      "Bước này tạo ra nhiều phương án Script (kịch bản) từ Gene đã được chọn. Script là kịch bản chi tiết cho video hoặc nội dung đa phương tiện, bao gồm Hook (3 giây đầu thu hút), Body (nội dung chính) và Call-to-Action (lời kêu gọi hành động). Bước này giúp tạo ra kịch bản sẵn sàng để quay video hoặc tạo nội dung.",
			type_:            aimodels.AIStepTypeGenerate,
			promptTemplateID: generateScriptTemplate,
			targetLevel:      "L6",
			parentLevel:      "L5",
		},
		{
			name:             "Đánh Giá Phương Án Script",
			description:      "Bước này đánh giá và chấm điểm nội dung Script đã được tạo ra, dựa trên các tiêu chí: độ liên quan, độ rõ ràng, độ hấp dẫn và độ chính xác.",
			type_:            aimodels.AIStepTypeJudge,
			promptTemplateID: judgeTemplate,
			targetLevel:      "L6",
			parentLevel:      "L5",
		},
		// STEP_GENERATION
		{
			name:             "Tạo Workflow Steps - Động",
			description:      "Bước này tạo ra các bước (steps) động cho workflow dựa trên yêu cầu và context. Bước này giúp tự động thiết kế workflow phù hợp với từng tình huống cụ thể, bao gồm số lượng steps, loại steps, dependencies và cấu trúc workflow.",
			type_:            aimodels.AIStepTypeStepGeneration,
			promptTemplateID: stepGenTemplate,
			targetLevel:      "",
			parentLevel:      "",
		},
	}

	for _, stepData := range defaultSteps {
		// Bỏ qua step nếu không có prompt template (ví dụ: "Tạo Pillar (L1)" khi template chưa init)
		if stepData.promptTemplateID == nil {
			continue
		}
		// Kiểm tra step đã tồn tại chưa
		stepFilter := bson.M{
			"ownerOrganizationId": systemOrgID,
			"name":                stepData.name,
		}
		_, err := h.aiStepService.FindOne(ctx, stepFilter, nil)
		if err != nil && err != common.ErrNotFound {
			continue // Lỗi khác, bỏ qua
		}

		if err == common.ErrNotFound {
			// Chưa có, tạo mới
			// LƯU Ý: InputSchema và OutputSchema sẽ được tự động set từ standard schema trong InsertOne()
			// Không cần set thủ công, đảm bảo schema nhất quán theo (stepType + TargetLevel + ParentLevel)
			step := aimodels.AIStep{
				OwnerOrganizationID: systemOrgID,
				Name:                stepData.name,
				Description:         stepData.description,
				Type:                stepData.type_,
				PromptTemplateID:    stepData.promptTemplateID,
				// InputSchema và OutputSchema sẽ được tự động set từ GetStandardSchema() trong InsertOne()
				TargetLevel:         stepData.targetLevel,
				ParentLevel:         stepData.parentLevel,
				// KHÔNG có ProviderProfileID, Model, Temperature, MaxTokens - config lưu trong prompt template
				Status:    "active",
				CreatedAt: currentTime,
				UpdatedAt: currentTime,
			}
			_, err = h.aiStepService.InsertOne(ctx, step)
			if err != nil {
				logrus.WithError(err).Warnf("Failed to create step: %s", stepData.name)
				continue
			}
		}
	}

	return nil
}

// initAIWorkflows khởi tạo các AI workflows mẫu
// Tạo nhiều workflows cho từng starting level: L1→L6, L2→L6, L3→L6, L4→L6, L5→L6
// Mỗi workflow chỉ chứa steps từ starting level đến L6, đảm bảo RootRefType match với starting level
func (h *InitService) initAIWorkflows(ctx context.Context, systemOrgID primitive.ObjectID, currentTime int64) error {
	logrus.Infof("Starting AI workflows initialization for organization: %s", systemOrgID.Hex())

	// Lấy tất cả các steps cần thiết (bao gồm step tạo Pillar L1)
	stepNames := []string{
		"Tạo Pillar (L1)",
		"Đánh Giá Phương Án Pillar",
		"Tạo STP từ Pillar",
		"Đánh Giá Phương Án STP",
		"Tạo Insight từ STP",
		"Đánh Giá Phương Án Insight",
		"Tạo Content Line từ Insight",
		"Đánh Giá Phương Án Content Line",
		"Tạo Gene từ Content Line",
		"Đánh Giá Phương Án Gene",
		"Tạo Script từ Gene",
		"Đánh Giá Phương Án Script",
	}

	steps := make(map[string]*aimodels.AIStep)
	missingSteps := []string{}
	for _, stepName := range stepNames {
		step, err := h.getStepByName(ctx, systemOrgID, stepName)
		if err != nil {
			logrus.WithError(err).Warnf("Step '%s' not found", stepName)
			missingSteps = append(missingSteps, stepName)
			continue // Tiếp tục tìm các steps khác, không return ngay
		}
		steps[stepName] = step
		logrus.Debugf("Found step: %s (ID: %s)", stepName, step.ID.Hex())
	}

	// Nếu thiếu quá nhiều steps, không tạo workflows
	if len(missingSteps) > 0 {
		logrus.Warnf("Missing %d steps, will skip workflows that require them. Missing: %v", len(missingSteps), missingSteps)
		// Không return, vẫn tiếp tục tạo workflows với các steps có sẵn
	}

	// Nếu không có step nào, return
	if len(steps) == 0 {
		logrus.Error("No steps found, cannot create workflows")
		return fmt.Errorf("no steps found, cannot create workflows")
	}

	logrus.Infof("Found %d/%d steps, proceeding to create workflows", len(steps), len(stepNames))

	// Định nghĩa các workflows cho từng starting level
	workflowDefinitions := []struct {
		name        string
		description string
		version     string
		rootRefType string
		targetLevel string
		stepNames   []string // Tên các steps theo thứ tự
	}{
		// L0 → L1: Tạo Pillar (không cần root, rootRefType rỗng)
		{
			name:        "Tạo Pillar (L1)",
			description: "Workflow tạo Pillar (L1 - cấp nội dung trên cùng) từ context (ngành, đối tượng, sản phẩm). Không cần root content. Dùng khi bắt đầu tạo nội dung từ đầu.",
			version:     "1.0.0",
			rootRefType: "",
			targetLevel: "L1",
			stepNames: []string{
				"Tạo Pillar (L1)",
				"Đánh Giá Phương Án Pillar",
			},
		},
		// L1 → L6: pillar → stp → insight → contentLine → gene → script
		{
			name:        "Quy Trình Tạo Nội Dung - Từ Pillar (L1 đến L6)",
			description: "Workflow đầy đủ để tạo và đánh giá nội dung từ Pillar (L1) đến Script (L6) theo quy trình tuần tự. Workflow này bao gồm 10 bước: tạo và đánh giá STP, Insight, Content Line, Gene, và Script. Phù hợp khi bạn đã có Pillar và muốn tạo ra Script hoàn chỉnh.",
			version:     "1.0.0",
			rootRefType: "pillar",
			targetLevel: "L6",
			stepNames: []string{
				"Tạo STP từ Pillar",
				"Đánh Giá Phương Án STP",
				"Tạo Insight từ STP",
				"Đánh Giá Phương Án Insight",
				"Tạo Content Line từ Insight",
				"Đánh Giá Phương Án Content Line",
				"Tạo Gene từ Content Line",
				"Đánh Giá Phương Án Gene",
				"Tạo Script từ Gene",
				"Đánh Giá Phương Án Script",
			},
		},
		// L2 → L6: stp → insight → contentLine → gene → script
		{
			name:        "Quy Trình Tạo Nội Dung - Từ STP (L2 đến L6)",
			description: "Workflow để tạo và đánh giá nội dung từ STP (L2) đến Script (L6) theo quy trình tuần tự. Workflow này bao gồm 8 bước: tạo và đánh giá Insight, Content Line, Gene, và Script. Phù hợp khi bạn đã có STP và muốn tạo ra Script hoàn chỉnh.",
			version:     "1.0.0",
			rootRefType: "stp",
			targetLevel: "L6",
			stepNames: []string{
				"Tạo Insight từ STP",
				"Đánh Giá Phương Án Insight",
				"Tạo Content Line từ Insight",
				"Đánh Giá Phương Án Content Line",
				"Tạo Gene từ Content Line",
				"Đánh Giá Phương Án Gene",
				"Tạo Script từ Gene",
				"Đánh Giá Phương Án Script",
			},
		},
		// L3 → L6: insight → contentLine → gene → script
		{
			name:        "Quy Trình Tạo Nội Dung - Từ Insight (L3 đến L6)",
			description: "Workflow để tạo và đánh giá nội dung từ Insight (L3) đến Script (L6) theo quy trình tuần tự. Workflow này bao gồm 6 bước: tạo và đánh giá Content Line, Gene, và Script. Phù hợp khi bạn đã có Insight và muốn tạo ra Script hoàn chỉnh.",
			version:     "1.0.0",
			rootRefType: "insight",
			targetLevel: "L6",
			stepNames: []string{
				"Tạo Content Line từ Insight",
				"Đánh Giá Phương Án Content Line",
				"Tạo Gene từ Content Line",
				"Đánh Giá Phương Án Gene",
				"Tạo Script từ Gene",
				"Đánh Giá Phương Án Script",
			},
		},
		// L4 → L6: contentLine → gene → script
		{
			name:        "Quy Trình Tạo Nội Dung - Từ Content Line (L4 đến L6)",
			description: "Workflow để tạo và đánh giá nội dung từ Content Line (L4) đến Script (L6) theo quy trình tuần tự. Workflow này bao gồm 4 bước: tạo và đánh giá Gene và Script. Phù hợp khi bạn đã có Content Line và muốn tạo ra Script hoàn chỉnh.",
			version:     "1.0.0",
			rootRefType: "contentLine",
			targetLevel: "L6",
			stepNames: []string{
				"Tạo Gene từ Content Line",
				"Đánh Giá Phương Án Gene",
				"Tạo Script từ Gene",
				"Đánh Giá Phương Án Script",
			},
		},
		// L5 → L6: gene → script
		{
			name:        "Quy Trình Tạo Nội Dung - Từ Gene (L5 đến L6)",
			description: "Workflow để tạo và đánh giá nội dung từ Gene (L5) đến Script (L6) theo quy trình tuần tự. Workflow này bao gồm 2 bước: tạo và đánh giá Script. Phù hợp khi bạn đã có Gene và muốn tạo ra Script hoàn chỉnh.",
			version:     "1.0.0",
			rootRefType: "gene",
			targetLevel: "L6",
			stepNames: []string{
				"Tạo Script từ Gene",
				"Đánh Giá Phương Án Script",
			},
		},
	}

	// Tạo từng workflow
	createdCount := 0
	skippedCount := 0
	for _, wfDef := range workflowDefinitions {
		logrus.Debugf("Processing workflow: %s", wfDef.name)

		// Kiểm tra workflow đã tồn tại chưa
		workflowFilter := bson.M{
			"ownerOrganizationId": systemOrgID,
			"name":                wfDef.name,
			"version":             wfDef.version,
		}
		_, err := h.aiWorkflowService.FindOne(ctx, workflowFilter, nil)
		if err != nil && err != common.ErrNotFound {
			logrus.WithError(err).Warnf("Failed to check existing workflow: %s", wfDef.name)
			skippedCount++
			continue
		}

		if err == common.ErrNotFound {
			logrus.Debugf("Workflow '%s' not found, creating new one", wfDef.name)
			// Tạo workflow steps từ step names
			workflowSteps := make([]aimodels.AIWorkflowStepReference, 0, len(wfDef.stepNames))
			for order, stepName := range wfDef.stepNames {
				step, exists := steps[stepName]
				if !exists {
					logrus.Warnf("Step '%s' not found in steps map, skipping workflow: %s", stepName, wfDef.name)
					continue
				}
				workflowSteps = append(workflowSteps, aimodels.AIWorkflowStepReference{
					StepID: step.ID.Hex(), // StepID là string
					Order:  order,
					Policy: &aimodels.AIWorkflowStepPolicy{
						RetryCount: 2,
						Timeout:    300, // 5 minutes
						OnFailure:  "stop",
						OnSuccess:  "continue",
						Parallel:   false, // Phải chạy tuần tự
					},
				})
			}

			if len(workflowSteps) == 0 {
				logrus.Warnf("No valid steps found for workflow: %s (required %d steps, found 0)", wfDef.name, len(wfDef.stepNames))
				skippedCount++
				continue
			}

			if len(workflowSteps) < len(wfDef.stepNames) {
				logrus.Warnf("Workflow '%s' missing some steps: required %d, found %d", wfDef.name, len(wfDef.stepNames), len(workflowSteps))
			}

			// Chưa có, tạo mới
			workflow := aimodels.AIWorkflow{
				OwnerOrganizationID: systemOrgID,
				Name:                wfDef.name,
				Description:         wfDef.description,
				Version:             wfDef.version,
				Steps:               workflowSteps,
				RootRefType:         wfDef.rootRefType,
				TargetLevel:         wfDef.targetLevel,
				DefaultPolicy: &aimodels.AIWorkflowStepPolicy{
					RetryCount: 2,
					Timeout:    300,
					OnFailure:  "stop",
					OnSuccess:  "continue",
					Parallel:   false, // Đảm bảo tuần tự
				},
				Status:    "active",
				CreatedAt: currentTime,
				UpdatedAt: currentTime,
			}
			_, err = h.aiWorkflowService.InsertOne(ctx, workflow)
			if err != nil {
				logrus.WithError(err).Errorf("Failed to create workflow: %s", wfDef.name)
				skippedCount++
				continue
			}
			logrus.Infof("✅ Created workflow: %s (RootRefType: %s, TargetLevel: %s, Steps: %d)",
				wfDef.name, wfDef.rootRefType, wfDef.targetLevel, len(workflowSteps))
			createdCount++
		} else {
			logrus.Debugf("Workflow '%s' already exists, skipping", wfDef.name)
			skippedCount++
		}
	}

	logrus.Infof("AI workflows initialization completed: %d created, %d skipped", createdCount, skippedCount)
	return nil
}

// initAIWorkflowCommands khởi tạo các AI workflow commands mẫu
// Tạo các command ví dụ để demo cách sử dụng workflow commands
// Lưu ý: RootRefID sử dụng ObjectID mẫu (vì đây chỉ là init data mẫu, không cần content node thực tế)
func (h *InitService) initAIWorkflowCommands(ctx context.Context, systemOrgID primitive.ObjectID, currentTime int64) error {
	logrus.Infof("Starting AI workflow commands initialization for organization: %s", systemOrgID.Hex())

	// Lấy một vài workflows và steps để tạo command ví dụ
	workflowNames := []string{
		"Quy Trình Tạo Nội Dung - Từ Pillar (L1 đến L6)",
		"Quy Trình Tạo Nội Dung - Từ STP (L2 đến L6)",
		"Quy Trình Tạo Nội Dung - Từ Insight (L3 đến L6)",
	}

	stepNames := []string{
		"Tạo STP từ Pillar",
		"Tạo Insight từ STP",
		"Tạo Content Line từ Insight",
	}

	// Lấy workflows
	workflows := make(map[string]*aimodels.AIWorkflow)
	for _, workflowName := range workflowNames {
		workflow, err := h.getWorkflowByName(ctx, systemOrgID, workflowName)
		if err != nil {
			logrus.Debugf("Workflow '%s' not found, skipping", workflowName)
			continue
		}
		workflows[workflowName] = workflow
		logrus.Debugf("Found workflow: %s (ID: %s)", workflowName, workflow.ID.Hex())
	}

	// Lấy steps
	steps := make(map[string]*aimodels.AIStep)
	for _, stepName := range stepNames {
		step, err := h.getStepByName(ctx, systemOrgID, stepName)
		if err != nil {
			logrus.Debugf("Step '%s' not found, skipping", stepName)
			continue
		}
		steps[stepName] = step
		logrus.Debugf("Found step: %s (ID: %s)", stepName, step.ID.Hex())
	}

	// Nếu không có workflow hoặc step nào, không tạo commands
	if len(workflows) == 0 && len(steps) == 0 {
		logrus.Warn("No workflows or steps found, cannot create workflow commands")
		return nil // Không phải lỗi, chỉ là không có data để tạo
	}

	logrus.Infof("Found %d workflows and %d steps, proceeding to create commands", len(workflows), len(steps))

	// Log chi tiết workflows và steps đã tìm thấy
	if len(workflows) > 0 {
		logrus.Infof("Workflows found: %v", func() []string {
			names := make([]string, 0, len(workflows))
			for name := range workflows {
				names = append(names, name)
			}
			return names
		}())
	}
	if len(steps) > 0 {
		logrus.Infof("Steps found: %v", func() []string {
			names := make([]string, 0, len(steps))
			for name := range steps {
				names = append(names, name)
			}
			return names
		}())
	}

	// Định nghĩa các commands ví dụ
	commandDefinitions := []struct {
		commandType string
		description string
		workflowID  *primitive.ObjectID
		stepID      *primitive.ObjectID
		rootRefType string
		params      map[string]interface{}
	}{
		// START_WORKFLOW commands
		{
			commandType: aimodels.AIWorkflowCommandTypeStartWorkflow,
			description: "Command ví dụ: Bắt đầu workflow từ Pillar (L1) để tạo nội dung đến Script (L6)",
			workflowID: func() *primitive.ObjectID {
				if wf, ok := workflows["Quy Trình Tạo Nội Dung - Từ Pillar (L1 đến L6)"]; ok {
					return &wf.ID
				} else {
					return nil
				}
			}(),
			stepID:      nil,
			rootRefType: "pillar",
			params: map[string]interface{}{
				"priority":    "high",
				"description": "Tạo nội dung marketing từ Pillar mẫu",
			},
		},
		{
			commandType: aimodels.AIWorkflowCommandTypeStartWorkflow,
			description: "Command ví dụ: Bắt đầu workflow từ STP (L2) để tạo nội dung đến Script (L6)",
			workflowID: func() *primitive.ObjectID {
				if wf, ok := workflows["Quy Trình Tạo Nội Dung - Từ STP (L2 đến L6)"]; ok {
					return &wf.ID
				} else {
					return nil
				}
			}(),
			stepID:      nil,
			rootRefType: "stp",
			params: map[string]interface{}{
				"priority":    "medium",
				"description": "Tạo nội dung từ STP đã có sẵn",
			},
		},
		{
			commandType: aimodels.AIWorkflowCommandTypeStartWorkflow,
			description: "Command ví dụ: Bắt đầu workflow từ Insight (L3) để tạo nội dung đến Script (L6)",
			workflowID: func() *primitive.ObjectID {
				if wf, ok := workflows["Quy Trình Tạo Nội Dung - Từ Insight (L3 đến L6)"]; ok {
					return &wf.ID
				} else {
					return nil
				}
			}(),
			stepID:      nil,
			rootRefType: "insight",
			params: map[string]interface{}{
				"priority":    "low",
				"description": "Tạo nội dung từ Insight đã có sẵn",
			},
		},
		// EXECUTE_STEP commands
		{
			commandType: aimodels.AIWorkflowCommandTypeExecuteStep,
			description: "Command ví dụ: Chạy step tạo STP từ Pillar",
			workflowID:  nil,
			stepID: func() *primitive.ObjectID {
				if step, ok := steps["Tạo STP từ Pillar"]; ok {
					return &step.ID
				} else {
					return nil
				}
			}(),
			rootRefType: "pillar",
			params: map[string]interface{}{
				"generateCount": 3, // Tạo 3 phương án STP
				"description":   "Tạo STP từ Pillar mẫu",
			},
		},
		{
			commandType: aimodels.AIWorkflowCommandTypeExecuteStep,
			description: "Command ví dụ: Chạy step tạo Insight từ STP",
			workflowID:  nil,
			stepID: func() *primitive.ObjectID {
				if step, ok := steps["Tạo Insight từ STP"]; ok {
					return &step.ID
				} else {
					return nil
				}
			}(),
			rootRefType: "stp",
			params: map[string]interface{}{
				"generateCount": 5, // Tạo 5 phương án Insight
				"description":   "Tạo Insight từ STP mẫu",
			},
		},
		{
			commandType: aimodels.AIWorkflowCommandTypeExecuteStep,
			description: "Command ví dụ: Chạy step tạo Content Line từ Insight",
			workflowID:  nil,
			stepID: func() *primitive.ObjectID {
				if step, ok := steps["Tạo Content Line từ Insight"]; ok {
					return &step.ID
				} else {
					return nil
				}
			}(),
			rootRefType: "insight",
			params: map[string]interface{}{
				"generateCount": 4, // Tạo 4 phương án Content Line
				"description":   "Tạo Content Line từ Insight mẫu",
			},
		},
	}

	// Tạo từng command
	createdCount := 0
	skippedCount := 0
	for _, cmdDef := range commandDefinitions {
		// Bỏ qua nếu không có workflowID hoặc stepID tương ứng
		if cmdDef.commandType == aimodels.AIWorkflowCommandTypeStartWorkflow && cmdDef.workflowID == nil {
			logrus.Warnf("Skipping START_WORKFLOW command: workflow not found for description: %s", cmdDef.description)
			skippedCount++
			continue
		}
		if cmdDef.commandType == aimodels.AIWorkflowCommandTypeExecuteStep && cmdDef.stepID == nil {
			logrus.Warnf("Skipping EXECUTE_STEP command: step not found for description: %s", cmdDef.description)
			skippedCount++
			continue
		}

		// Validate: Đảm bảo có đủ thông tin trước khi tạo command
		if cmdDef.commandType == "" {
			logrus.Errorf("Invalid command definition: missing commandType for: %s", cmdDef.description)
			skippedCount++
			continue
		}
		if cmdDef.commandType == aimodels.AIWorkflowCommandTypeStartWorkflow {
			if cmdDef.workflowID == nil {
				logrus.Warnf("Skipping START_WORKFLOW command: workflowID is nil for: %s", cmdDef.description)
				skippedCount++
				continue
			}
			if cmdDef.workflowID.IsZero() {
				logrus.Warnf("Skipping START_WORKFLOW command: workflowID is zero for: %s", cmdDef.description)
				skippedCount++
				continue
			}
		}
		if cmdDef.commandType == aimodels.AIWorkflowCommandTypeExecuteStep {
			if cmdDef.stepID == nil {
				logrus.Warnf("Skipping EXECUTE_STEP command: stepID is nil for: %s", cmdDef.description)
				skippedCount++
				continue
			}
			if cmdDef.stepID.IsZero() {
				logrus.Warnf("Skipping EXECUTE_STEP command: stepID is zero for: %s", cmdDef.description)
				skippedCount++
				continue
			}
		}

		// Tạo ObjectID mẫu cho RootRefID (mỗi command có một RootRefID khác nhau để demo)
		// Trong thực tế, RootRefID sẽ là ID của content node thực tế
		sampleRootRefID := primitive.NewObjectID()

		// Kiểm tra command đã tồn tại chưa (dựa trên commandType, workflowID/stepID, và rootRefID)
		commandFilter := bson.M{
			"commandType": cmdDef.commandType,
			"rootRefType": cmdDef.rootRefType,
		}
		if cmdDef.workflowID != nil {
			commandFilter["workflowId"] = cmdDef.workflowID
		}
		if cmdDef.stepID != nil {
			commandFilter["stepId"] = cmdDef.stepID
		}

		// Kiểm tra xem đã có command tương tự chưa (không kiểm tra rootRefID vì mỗi command có rootRefID khác nhau)
		_, err := h.aiWorkflowCommandService.FindOne(ctx, commandFilter, nil)
		if err != nil && err != common.ErrNotFound {
			logrus.WithError(err).Warnf("Failed to check existing command")
			skippedCount++
			continue
		}

		// Chỉ tạo command nếu chưa có command tương tự (cùng commandType, workflowID/stepID, rootRefType)
		// Lưu ý: Trong thực tế, có thể có nhiều command với cùng workflow/step nhưng khác rootRefID
		// Nhưng vì đây là init data mẫu, chúng ta chỉ tạo một command cho mỗi workflow/step
		if err == common.ErrNotFound {
			// Chưa có, tạo mới
			command := aimodels.AIWorkflowCommand{
				OwnerOrganizationID: systemOrgID, // Thuộc về System Organization (dữ liệu hệ thống) - Phân quyền dữ liệu
				CommandType:         cmdDef.commandType,
				Status:              aimodels.AIWorkflowCommandStatusPending, // Mặc định là pending, chờ agent xử lý
				WorkflowID:          cmdDef.workflowID,
				StepID:              cmdDef.stepID,
				RootRefID:           &sampleRootRefID,
				RootRefType:         cmdDef.rootRefType,
				Params:              cmdDef.params,
				CreatedAt:           currentTime,
				Metadata: map[string]interface{}{
					"description": cmdDef.description,
					"initData":    true, // Đánh dấu là init data
				},
			}

			_, err = h.aiWorkflowCommandService.InsertOne(ctx, command)
			if err != nil {
				logrus.WithError(err).Errorf("Failed to create command: %s", cmdDef.description)
				skippedCount++
				continue
			}

			// Log chi tiết thông tin command đã tạo
			workflowIDStr := "nil"
			if cmdDef.workflowID != nil {
				workflowIDStr = cmdDef.workflowID.Hex()
			}
			stepIDStr := "nil"
			if cmdDef.stepID != nil {
				stepIDStr = cmdDef.stepID.Hex()
			}
			logrus.Infof("✅ Created command: %s (Type: %s, WorkflowID: %s, StepID: %s, RootRefType: %s)",
				cmdDef.description, cmdDef.commandType, workflowIDStr, stepIDStr, cmdDef.rootRefType)
			createdCount++
		} else {
			logrus.Debugf("Command already exists, skipping: %s", cmdDef.description)
			skippedCount++
		}
	}

	logrus.Infof("AI workflow commands initialization completed: %d created, %d skipped", createdCount, skippedCount)
	return nil
}

// Helper functions
func (h *InitService) getPromptTemplateByName(ctx context.Context, systemOrgID primitive.ObjectID, name string) (*primitive.ObjectID, error) {
	filter := bson.M{
		"ownerOrganizationId": systemOrgID,
		"name":                name,
	}
	template, err := h.aiPromptTemplateService.FindOne(ctx, filter, nil)
	if err != nil {
		return nil, err
	}
	var templateModel aimodels.AIPromptTemplate
	bsonBytes, _ := bson.Marshal(template)
	if err := bson.Unmarshal(bsonBytes, &templateModel); err != nil {
		return nil, err
	}
	return &templateModel.ID, nil
}

func (h *InitService) getStepByName(ctx context.Context, systemOrgID primitive.ObjectID, name string) (*aimodels.AIStep, error) {
	filter := bson.M{
		"ownerOrganizationId": systemOrgID,
		"name":                name,
	}
	step, err := h.aiStepService.FindOne(ctx, filter, nil)
	if err != nil {
		return nil, err
	}
	var stepModel aimodels.AIStep
	bsonBytes, _ := bson.Marshal(step)
	if err := bson.Unmarshal(bsonBytes, &stepModel); err != nil {
		return nil, err
	}
	return &stepModel, nil
}

func (h *InitService) getWorkflowByName(ctx context.Context, systemOrgID primitive.ObjectID, name string) (*aimodels.AIWorkflow, error) {
	filter := bson.M{
		"ownerOrganizationId": systemOrgID,
		"name":                name,
	}
	workflow, err := h.aiWorkflowService.FindOne(ctx, filter, nil)
	if err != nil {
		return nil, err
	}
	var workflowModel aimodels.AIWorkflow
	bsonBytes, _ := bson.Marshal(workflow)
	if err := bson.Unmarshal(bsonBytes, &workflowModel); err != nil {
		return nil, err
	}
	return &workflowModel, nil
}

func (h *InitService) getProviderProfileByName(ctx context.Context, systemOrgID primitive.ObjectID, name string) (*primitive.ObjectID, error) {
	filter := bson.M{
		"ownerOrganizationId": systemOrgID,
		"name":                name,
	}
	profile, err := h.aiProviderProfileService.FindOne(ctx, filter, nil)
	if err != nil {
		return nil, err
	}
	var profileModel aimodels.AIProviderProfile
	bsonBytes, _ := bson.Marshal(profile)
	if err := bson.Unmarshal(bsonBytes, &profileModel); err != nil {
		return nil, err
	}
	return &profileModel.ID, nil
}

// orderReportMetadata metadata dùng chung cho báo cáo đơn hàng (daily/weekly/monthly).
var orderReportMetadata = map[string]interface{}{
	"description": "Số lượng đơn và tổng số tiền, phân theo nguồn (posData.tags), trạng thái đơn (posData.status), kho (posData.warehouse_info.name), nhân viên tạo đơn (posData.assigning_seller.name).",
	"warehouseDimension": map[string]interface{}{
		"fieldPath": "posData.warehouse_info.name",
	},
	"assigningSellerDimension": map[string]interface{}{
		"fieldPath": "posData.assigning_seller.name",
	},
	"tagDimension": map[string]interface{}{
		"fieldPath":  "posData.tags",
		"nameField": "name",
		"splitMode":  "equal", // Chia đều số lượng và số tiền khi đơn có nhiều tag
	},
	"statusDimension": map[string]interface{}{
		"fieldPath": "posData.status",
	},
	// excludeStatuses: loại trừ đơn hàng đã hủy (6) và đã xóa gần đây (7) khỏi doanh thu
	"excludeStatuses": []interface{}{6, 7},
	// revenueCompletedStatuses: trạng thái đơn được tính vào doanh thu đã hoàn thành (3=Đã nhận hàng, 16=Đã thu tiền)
	"revenueCompletedStatuses": []interface{}{3, 16},
	"totalAmountField": "posData.total_price_after_sub_discount",
	"knownTags": []string{
		"Nguồn.Store-Sài Gòn", "Nguồn.Store-Hà Nội", "Nguồn.Web-Zalo",
		"Nguồn.Web-Shopify", "Nguồn.Bán lại", "Nguồn.Bán sỉ", "Nguồn.Bán mới",
	},
	"knownStatuses": []interface{}{0, 17, 11, 12, 13, 20, 1, 8, 9, 2, 3, 16, 4, 15, 5, 6, 7},
	"statusLabels": map[string]interface{}{
		"0": "Mới", "17": "Chờ xác nhận", "11": "Chờ hàng", "12": "Chờ in",
		"13": "Đã in", "20": "Đã đặt hàng", "1": "Đã xác nhận", "8": "Đang đóng hàng",
		"9": "Chờ lấy hàng", "2": "Đã giao hàng", "3": "Đã nhận hàng", "16": "Đã thu tiền",
		"4": "Đang trả hàng", "15": "Trả hàng một phần", "5": "Đã trả hàng",
		"6": "Đã hủy", "7": "Đã xóa gần đây",
	},
}

// InitReportDefinitions tạo hoặc cập nhật mẫu báo cáo đơn hàng (order_daily, order_weekly, order_monthly) trong report_definitions.
// Báo cáo: thời gian theo posCreatedAt; chỉ tiêu: số lượng đơn, tổng số tiền; thống kê theo posData.tags (chia đều nếu nhiều tag).
func (h *InitService) InitReportDefinitions() error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ReportDefinitions)
	if !ok {
		return fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.ReportDefinitions)
	}
	ctx := context.TODO()
	now := time.Now().Unix()
	metrics := []reportmodels.ReportMetricDefinition{
		// Base metrics: aggregation từ collection
		{OutputKey: "orderCount", Type: "base", AggType: "count", FieldPath: "_id"},
		{OutputKey: "totalAmount", Type: "base", AggType: "sum", FieldPath: "posData.total_price_after_sub_discount"},
		// Derived metrics: công thức từ metric gốc
		{OutputKey: "avgAmount", Type: "derived", FormulaRef: "avg_from_sum_count", Params: map[string]string{"sum": "totalAmount", "count": "orderCount"}, Scope: "total"},
		{OutputKey: "orderCountPct", Type: "derived", FormulaRef: "pct_of_total", Params: map[string]string{"value": "orderCount", "total": "total.orderCount"}, Scope: "perDimension"},
		{OutputKey: "totalAmountPct", Type: "derived", FormulaRef: "pct_of_total", Params: map[string]string{"value": "totalAmount", "total": "total.totalAmount"}, Scope: "perDimension"},
		{OutputKey: "avgAmount", Type: "derived", FormulaRef: "avg_from_sum_count", Params: map[string]string{"sum": "totalAmount", "count": "orderCount"}, Scope: "perDimension"},
	}

	orderSeeds := []struct {
		key         string
		name        string
		periodType  string
		periodLabel string
	}{
		{"order_daily", "Báo cáo đơn hàng chu kỳ ngày", "day", "Theo ngày"},
		{"order_weekly", "Báo cáo đơn hàng chu kỳ tuần", "week", "Theo tuần"},
		{"order_monthly", "Báo cáo đơn hàng chu kỳ tháng", "month", "Theo tháng"},
		{"order_yearly", "Báo cáo đơn hàng chu kỳ năm", "year", "Theo năm"},
	}

	opts := options.Replace().SetUpsert(true)
	for _, s := range orderSeeds {
		seed := reportmodels.ReportDefinition{
			Key:              s.key,
			Name:             s.name,
			PeriodType:       s.periodType,
			PeriodLabel:      s.periodLabel,
			SourceCollection: global.MongoDB_ColNames.PcPosOrders,
			TimeField:        "posCreatedAt",
			TimeFieldUnit:    "millisecond",
			Dimensions:       []string{"ownerOrganizationId"},
			Metrics:          metrics,
			Metadata:         orderReportMetadata,
			IsActive:         true,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		filter := bson.M{"key": s.key}
		if _, err := coll.ReplaceOne(ctx, filter, seed, opts); err != nil {
			return fmt.Errorf("upsert %s: %w", s.key, err)
		}
		logrus.Infof("[INIT] Báo cáo %s (%s) đã được tạo/cập nhật trong report_definitions", s.name, s.key)
	}

	// Báo cáo khách hàng theo chu kỳ (customer_daily, customer_weekly, ...) — dùng engine riêng ComputeCustomerReport.
	// sourceCollection = pc_pos_orders để hook MarkDirty khi đơn thay đổi.
	customerSeeds := []struct {
		key         string
		name        string
		periodType  string
		periodLabel string
	}{
		{"customer_daily", "Báo cáo khách hàng chu kỳ ngày", "day", "Theo ngày"},
		{"customer_weekly", "Báo cáo khách hàng chu kỳ tuần", "week", "Theo tuần"},
		{"customer_monthly", "Báo cáo khách hàng chu kỳ tháng", "month", "Theo tháng"},
		{"customer_yearly", "Báo cáo khách hàng chu kỳ năm", "year", "Theo năm"},
	}

	customerMetrics := []reportmodels.ReportMetricDefinition{} // Engine ComputeCustomerReport tính trực tiếp, không dùng metrics
	customerMetadata := map[string]interface{}{
		"description": "Snapshot khách hàng tại cuối chu kỳ: KPI, phân bố tier, lifecycle. Dùng cho xu hướng Tab Customer.",
	}
	for _, s := range customerSeeds {
		seed := reportmodels.ReportDefinition{
			Key:              s.key,
			Name:             s.name,
			PeriodType:       s.periodType,
			PeriodLabel:      s.periodLabel,
			SourceCollection: global.MongoDB_ColNames.PcPosOrders,
			TimeField:        "posCreatedAt",
			TimeFieldUnit:    "millisecond",
			Dimensions:       []string{"ownerOrganizationId"},
			Metrics:          customerMetrics,
			Metadata:         customerMetadata,
			IsActive:         true,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		filter := bson.M{"key": s.key}
		if _, err := coll.ReplaceOne(ctx, filter, seed, opts); err != nil {
			return fmt.Errorf("upsert %s: %w", s.key, err)
		}
		logrus.Infof("[INIT] Báo cáo %s (%s) đã được tạo/cập nhật trong report_definitions", s.name, s.key)
	}
	return nil
}
