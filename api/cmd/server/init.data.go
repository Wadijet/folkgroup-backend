package main

import (
	"meta_commerce/core/api/services"
	"meta_commerce/core/global"
	"meta_commerce/core/logger"
)

func InitDefaultData() {
	log := logger.GetAppLogger()
	log.Info("🔄 [INIT] Starting InitDefaultData...")
	
	initService, err := services.NewInitService()
	if err != nil {
		log.Fatalf("Failed to initialize init service: %v", err)
	}

	// 1. Khởi tạo Organization Root (PHẢI LÀM TRƯỚC)
	log.Info("🔄 [INIT] Step 1: Initializing root organization...")
	if err := initService.InitRootOrganization(); err != nil {
		log.Fatalf("Failed to initialize root organization: %v", err)
	}
	log.Info("✅ [INIT] Step 1: Root organization initialized")

	// 2. Khởi tạo Permissions (tạo các quyền mới nếu chưa có, bao gồm Customer, FbMessageItem, ...)
	log.Info("🔄 [INIT] Step 2: Initializing permissions...")
	if err := initService.InitPermission(); err != nil {
		log.Fatalf("Failed to initialize permissions: %v", err)
	}
	log.Info("✅ [INIT] Step 2: Permissions initialized/updated successfully")

	// 3. Tạo Role Administrator (nếu chưa có) + Đảm bảo đầy đủ Permission cho Administrator
	// Tự động gán tất cả quyền trong hệ thống (bao gồm quyền mới) cho role Administrator
	if err := initService.CheckPermissionForAdministrator(); err != nil {
		log.Warnf("Failed to check permissions for administrator: %v", err)
	} else {
		log.Info("Administrator role permissions synchronized successfully")
	}

	// 4. Tạo user admin tự động từ Firebase UID (nếu có config) - Tùy chọn
	// Lưu ý: User phải đã tồn tại trong Firebase Authentication
	// Nếu không có FIREBASE_ADMIN_UID, user đầu tiên login sẽ tự động trở thành admin
	if global.MongoDB_ServerConfig.FirebaseAdminUID != "" {
		if err := initService.InitAdminUser(global.MongoDB_ServerConfig.FirebaseAdminUID); err != nil {
			log.Warnf("Failed to initialize admin user from Firebase UID: %v", err)
			log.Info("User đầu tiên login sẽ tự động trở thành admin")
		} else {
			log.Info("Admin user initialized successfully from Firebase UID")
		}
	} else {
		log.Info("FIREBASE_ADMIN_UID not set")
		log.Info("User đầu tiên login sẽ tự động trở thành admin (First user becomes admin)")
	}

	// 5. Khởi tạo Tech Team mặc định (nếu chưa có)
	// Tạo team "Tech Team" thuộc System Organization để sử dụng cho các mục đích khác nhau
	log.Info("🔄 [INIT] Step 5: Initializing default Tech Team...")
	techTeam, err := initService.InitDefaultNotificationTeam()
	if err != nil {
		log.WithError(err).Error("❌ [INIT] Step 5: Failed to initialize Tech Team")
		log.Warnf("Failed to initialize Tech Team: %v", err)
	} else {
		log.Infof("✅ [INIT] Step 5: Tech Team initialized successfully (ID: %s)", techTeam.ID.Hex())
	}

	// 6. Khởi tạo dữ liệu mặc định cho hệ thống notification
	// Tạo các sender và template mặc định (global), các thông tin như token/password sẽ để trống để admin bổ sung sau
	log.Info("🔄 [INIT] Step 6: Initializing notification data...")
	if err := initService.InitNotificationData(); err != nil {
		log.WithError(err).Error("❌ [INIT] Step 6: Failed to initialize notification data")
		log.Warnf("Failed to initialize notification data: %v", err)
	} else {
		log.Info("✅ [INIT] Step 6: Notification data initialized successfully")
	}

	// 7. Khởi tạo CTA Library mặc định
	// Tạo các CTA templates phổ biến để có thể reuse trong notification templates
	log.Info("🔄 [INIT] Step 7: Initializing CTA library...")
	if err := initService.InitCTALibrary(); err != nil {
		log.WithError(err).Error("❌ [INIT] Step 7: Failed to initialize CTA library")
		log.Warnf("Failed to initialize CTA library: %v", err)
	} else {
		log.Info("✅ [INIT] Step 7: CTA library initialized successfully")
	}

	// 8. Khởi tạo dữ liệu mặc định cho hệ thống AI workflow (Module 2)
	// Tạo provider profiles, prompt templates, steps, và workflows mẫu
	log.Info("🔄 [INIT] Step 8: Initializing AI workflow data...")
	if err := initService.InitAIData(); err != nil {
		log.WithError(err).Error("❌ [INIT] Step 8: Failed to initialize AI workflow data")
		log.Warnf("Failed to initialize AI workflow data: %v", err)
	} else {
		log.Info("✅ [INIT] Step 8: AI workflow data initialized successfully")
	}
	
	log.Info("✅ [INIT] InitDefaultData completed successfully")
}
