// Package database - Index bổ sung cho CRM (nested fields, compound) không thể định nghĩa qua model tags.
package database

import (
	"context"
	"strings"

	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CreateCrmAdditionalIndexes tạo các index bổ sung cho CRM (nested fields, compound phức tạp).
// Gọi sau CreateIndexes cho từng collection CRM.
func CreateCrmAdditionalIndexes(ctx context.Context, db *mongo.Database) error {
	// crm_customers: (ownerOrganizationId, sourceIds.pos) sparse — merge findByPosId
	crmCustomers := db.Collection(global.MongoDB_ColNames.CrmCustomers)
	if _, err := crmCustomers.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "ownerOrganizationId", Value: 1},
			{Key: "sourceIds.pos", Value: 1},
		},
		Options: options.Index().SetName("crm_customer_org_pos").SetSparse(true),
	}); err != nil && !isIndexExistsError(err) {
		return err
	}

	// crm_customers: (ownerOrganizationId, sourceIds.fb) sparse — merge findByFbId
	if _, err := crmCustomers.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "ownerOrganizationId", Value: 1},
			{Key: "sourceIds.fb", Value: 1},
		},
		Options: options.Index().SetName("crm_customer_org_fb").SetSparse(true),
	}); err != nil && !isIndexExistsError(err) {
		return err
	}

	// crm_customers: (ownerOrganizationId, phoneNumbers) multikey — merge findByPhone
	if _, err := crmCustomers.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "ownerOrganizationId", Value: 1},
			{Key: "phoneNumbers", Value: 1},
		},
		Options: options.Index().SetName("crm_customer_org_phones"),
	}); err != nil && !isIndexExistsError(err) {
		return err
	}

	// pc_pos_orders: (ownerOrganizationId, status) — aggregate metrics filter
	pcPosOrders := db.Collection(global.MongoDB_ColNames.PcPosOrders)
	if _, err := pcPosOrders.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "ownerOrganizationId", Value: 1},
			{Key: "status", Value: 1},
		},
		Options: options.Index().SetName("pc_pos_order_org_status"),
	}); err != nil && !isIndexExistsError(err) {
		return err
	}

	// pc_pos_orders: (ownerOrganizationId, customerId) — aggregate metrics match
	if _, err := pcPosOrders.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "ownerOrganizationId", Value: 1},
			{Key: "customerId", Value: 1},
		},
		Options: options.Index().SetName("pc_pos_order_org_customer").SetSparse(true),
	}); err != nil && !isIndexExistsError(err) {
		return err
	}

	// pc_pos_orders: (ownerOrganizationId, billPhoneNumber) — aggregate metrics guest orders
	if _, err := pcPosOrders.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "ownerOrganizationId", Value: 1},
			{Key: "billPhoneNumber", Value: 1},
		},
		Options: options.Index().SetName("pc_pos_order_org_billphone").SetSparse(true),
	}); err != nil && !isIndexExistsError(err) {
		return err
	}

	// pc_pos_orders: (ownerOrganizationId, posData.customer.id) — aggregate metrics match nested
	if _, err := pcPosOrders.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "ownerOrganizationId", Value: 1},
			{Key: "posData.customer.id", Value: 1},
		},
		Options: options.Index().SetName("pc_pos_order_org_pos_customer").SetSparse(true),
	}); err != nil && !isIndexExistsError(err) {
		return err
	}

	// fb_conversations: (ownerOrganizationId, customerId) — checkHasConversation
	fbConversations := db.Collection(global.MongoDB_ColNames.FbConvesations)
	if _, err := fbConversations.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "ownerOrganizationId", Value: 1},
			{Key: "customerId", Value: 1},
		},
		Options: options.Index().SetName("fb_conversation_org_customer").SetSparse(true),
	}); err != nil && !isIndexExistsError(err) {
		return err
	}

	return nil
}

func isIndexExistsError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "already exists") || strings.Contains(s, "duplicate")
}
