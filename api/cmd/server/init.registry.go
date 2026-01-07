package main

import (
	"meta_commerce/config"
	"meta_commerce/core/global"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

func InitRegistry() {

	logrus.Info("Initialized registry") // Ghi log thông báo đã khởi tạo registry

	// Khởi tạo registry và đăng ký các collections
	err := InitCollections(global.MongoDB_Session, global.MongoDB_ServerConfig)
	if err != nil {
		logrus.Fatalf("Failed to initialize collections: %v", err)
	}
	logrus.Info("Initialized collection registry")
}

// InitCollections khởi tạo và đăng ký các collections MongoDB
func InitCollections(client *mongo.Client, cfg *config.Configuration) error {
	db := client.Database(cfg.MongoDB_DBName_Auth)
	colNames := []string{"auth_users", "auth_permissions", "auth_roles", "auth_role_permissions", "auth_user_roles", "auth_organizations", "auth_organization_shares",
		"agents", "access_tokens", "fb_pages", "fb_conversations", "fb_messages", "fb_message_items", "fb_posts", "fb_customers", "pc_orders", "customers", "pc_pos_customers", "pc_pos_shops", "pc_pos_warehouses", "pc_pos_products", "pc_pos_variations", "pc_pos_categories", "pc_pos_orders",
		"notification_senders", "notification_channels", "notification_templates", "notification_routing_rules",
		"delivery_queue", "delivery_history",
		"cta_library", "cta_tracking",
		"agent_registry", "agent_configs", "agent_commands", "agent_activity_logs"}

	for _, name := range colNames {
		registered, err := global.RegistryCollections.Register(name, db.Collection(name))
		if err != nil {
			logrus.Errorf("Failed to register collection %s: %v", name, err)
			return err
		}

		if registered {
			logrus.Infof("Collection %s registered successfully", name)
		} else {
			logrus.Errorf("Collection %s already registered", name)
		}

	}

	return nil
}
