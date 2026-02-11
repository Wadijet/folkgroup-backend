// Package router đăng ký các route thuộc domain auth: Admin, System, Auth, RBAC, Init.
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	authhdl "meta_commerce/internal/api/auth/handler"
	basehdl "meta_commerce/internal/api/base/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/initsvc"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký tất cả route auth (admin, system, auth, RBAC, init) lên v1.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	if err := registerAdminRoutes(v1); err != nil {
		return err
	}
	if err := registerSystemRoutes(v1); err != nil {
		return err
	}
	if err := registerAuthRoutes(v1, r); err != nil {
		return err
	}
	if err := registerRBACRoutes(v1, r); err != nil {
		return err
	}
	if err := registerInitRoutes(v1, r); err != nil {
		return err
	}
	return nil
}

func registerAdminRoutes(router fiber.Router) error {
	adminHandler, err := authhdl.NewAdminHandler()
	if err != nil {
		return fmt.Errorf("failed to create admin handler: %w", err)
	}
	blockMiddleware := middleware.AuthMiddleware("User.Block")
	apirouter.RegisterRouteWithMiddleware(router, "/admin/user", "POST", "/block", []fiber.Handler{blockMiddleware}, adminHandler.HandleBlockUser)
	apirouter.RegisterRouteWithMiddleware(router, "/admin/user", "POST", "/unblock", []fiber.Handler{blockMiddleware}, adminHandler.HandleUnBlockUser)
	setRoleMiddleware := middleware.AuthMiddleware("User.SetRole")
	apirouter.RegisterRouteWithMiddleware(router, "/admin/user", "POST", "/role", []fiber.Handler{setRoleMiddleware}, adminHandler.HandleSetRole)
	setAdminMiddleware := middleware.AuthMiddleware("Init.SetAdmin")
	apirouter.RegisterRouteWithMiddleware(router, "/admin/user", "POST", "/set-administrator/:id", []fiber.Handler{setAdminMiddleware}, adminHandler.HandleAddAdministrator)
	apirouter.RegisterRouteWithMiddleware(router, "/admin", "POST", "/sync-administrator-permissions", []fiber.Handler{setAdminMiddleware}, adminHandler.HandleSyncAdministratorPermissions)
	return nil
}

func registerSystemRoutes(router fiber.Router) error {
	systemHandler, err := basehdl.NewSystemHandler()
	if err != nil {
		return fmt.Errorf("failed to create system handler: %w", err)
	}
	router.Get("/system/health", systemHandler.HandleHealth)
	return nil
}

func registerAuthRoutes(router fiber.Router, r *apirouter.Router) error {
	userHandler, err := authhdl.NewUserHandler()
	if err != nil {
		return fmt.Errorf("failed to create user handler: %w", err)
	}
	router.Post("/auth/login/firebase", userHandler.HandleLoginWithFirebase)
	authOnlyMiddleware := middleware.AuthMiddleware("")
	apirouter.RegisterRouteWithMiddleware(router, "/auth", "POST", "/logout", []fiber.Handler{authOnlyMiddleware}, userHandler.HandleLogout)
	apirouter.RegisterRouteWithMiddleware(router, "/auth", "GET", "/profile", []fiber.Handler{authOnlyMiddleware}, userHandler.HandleGetProfile)
	apirouter.RegisterRouteWithMiddleware(router, "/auth", "PUT", "/profile", []fiber.Handler{authOnlyMiddleware}, userHandler.HandleUpdateProfile)
	authRolesMiddleware := middleware.AuthMiddleware("")
	apirouter.RegisterRouteWithMiddleware(router, "/auth", "GET", "/roles", []fiber.Handler{authRolesMiddleware}, userHandler.HandleGetUserRoles)
	return nil
}

func registerRBACRoutes(router fiber.Router, r *apirouter.Router) error {
	userHandler, err := authhdl.NewUserHandler()
	if err != nil {
		return fmt.Errorf("failed to create user handler: %w", err)
	}
	r.RegisterCRUDRoutes(router, "/user", userHandler, apirouter.ReadOnlyConfig, "User")

	permHandler, err := authhdl.NewPermissionHandler()
	if err != nil {
		return fmt.Errorf("failed to create permission handler: %w", err)
	}
	r.RegisterCRUDRoutes(router, "/permission", permHandler, apirouter.ReadOnlyConfig, "Permission")

	roleHandler, err := authhdl.NewRoleHandler()
	if err != nil {
		return fmt.Errorf("failed to create role handler: %w", err)
	}
	r.RegisterCRUDRoutes(router, "/role", roleHandler, apirouter.ReadWriteConfig, "Role")

	rolePermHandler, err := authhdl.NewRolePermissionHandler()
	if err != nil {
		return fmt.Errorf("failed to create role permission handler: %w", err)
	}
	rolePermUpdateMiddleware := middleware.AuthMiddleware("RolePermission.Update")
	apirouter.RegisterRouteWithMiddleware(router, "/role-permission", "PUT", "/update-role", []fiber.Handler{rolePermUpdateMiddleware}, rolePermHandler.HandleUpdateRolePermissions)
	r.RegisterCRUDRoutes(router, "/role-permission", rolePermHandler, apirouter.ReadWriteConfig, "RolePermission")

	userRoleHandler, err := authhdl.NewUserRoleHandler()
	if err != nil {
		return fmt.Errorf("failed to create user role handler: %w", err)
	}
	userRoleUpdateMiddleware := middleware.AuthMiddleware("UserRole.Update")
	apirouter.RegisterRouteWithMiddleware(router, "/user-role", "PUT", "/update-user-roles", []fiber.Handler{userRoleUpdateMiddleware}, userRoleHandler.HandleUpdateUserRoles)
	r.RegisterCRUDRoutes(router, "/user-role", userRoleHandler, apirouter.ReadWriteConfig, "UserRole")

	organizationHandler, err := authhdl.NewOrganizationHandler()
	if err != nil {
		return fmt.Errorf("failed to create organization handler: %w", err)
	}
	r.RegisterCRUDRoutes(router, "/organization", organizationHandler, apirouter.ReadWriteConfig, "Organization")

	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	orgConfigReadMiddleware := middleware.AuthMiddleware("OrganizationConfig.Read")
	orgConfigItemHandler, err := authhdl.NewOrganizationConfigItemHandler()
	if err != nil {
		return fmt.Errorf("failed to create organization config item handler: %w", err)
	}
	r.RegisterCRUDRoutes(router, "/organization-config", orgConfigItemHandler, apirouter.OrgConfigItemConfig, "OrganizationConfig")
	apirouter.RegisterRouteWithMiddleware(router, "/organization-config", "GET", "/resolved", []fiber.Handler{orgConfigReadMiddleware, orgContextMiddleware}, orgConfigItemHandler.GetResolved)

	organizationShareHandler, err := authhdl.NewOrganizationShareHandler()
	if err != nil {
		return fmt.Errorf("failed to create organization share handler: %w", err)
	}
	r.RegisterCRUDRoutes(router, "/organization-share", organizationShareHandler, apirouter.ReadWriteConfig, "OrganizationShare")
	return nil
}

func registerInitRoutes(router fiber.Router, r *apirouter.Router) error {
	initService, err := initsvc.NewInitService()
	if err == nil {
		hasAdmin, err := initService.HasAnyAdministrator()
		if err == nil && hasAdmin {
			return nil
		}
	}
	initHandler, err := authhdl.NewInitHandler()
	if err != nil {
		return fmt.Errorf("failed to create init handler: %w", err)
	}
	router.Get("/init/status", initHandler.HandleInitStatus)
	router.Post("/init/organization", initHandler.HandleInitOrganization)
	router.Post("/init/permissions", initHandler.HandleInitPermissions)
	router.Post("/init/roles", initHandler.HandleInitRoles)
	router.Post("/init/admin-user", initHandler.HandleInitAdminUser)
	router.Post("/init/all", initHandler.HandleInitAll)
	router.Post("/init/set-administrator/:id", initHandler.HandleSetAdministrator)
	return nil
}
