// Package models - Constants cho activity domain và mapping.
package models

// Các domain phân loại lịch sử hoạt động.
const (
	ActivityDomainOrder          = "order"
	ActivityDomainConversation   = "conversation"
	ActivityDomainNote           = "note"
	ActivityDomainProfile        = "profile"
	ActivityDomainCustomer       = "customer"
	ActivityDomainAssignment     = "assignment"
	ActivityDomainClassification = "classification"
	ActivityDomainCampaign       = "campaign"
	ActivityDomainSystem         = "system"
)

// ActivityTypeToDomain mapping activityType -> domain (fallback khi không truyền domain).
var ActivityTypeToDomain = map[string]string{
	"order_created": ActivityDomainOrder, "order_completed": ActivityDomainOrder,
	"order_cancelled": ActivityDomainOrder, "order_refunded": ActivityDomainOrder,
	"conversation_started": ActivityDomainConversation, "message_received": ActivityDomainConversation, "message_sent": ActivityDomainConversation,
	"note_added": ActivityDomainNote, "note_updated": ActivityDomainNote, "note_deleted": ActivityDomainNote,
	"profile_viewed": ActivityDomainProfile, "profile_updated": ActivityDomainProfile,
	"customer_merged": ActivityDomainCustomer, "customer_updated": ActivityDomainCustomer, "customer_created": ActivityDomainCustomer,
	"sale_assigned": ActivityDomainAssignment, "sale_unassigned": ActivityDomainAssignment,
	"classification_changed": ActivityDomainClassification,
	"voucher_sent": ActivityDomainCampaign, "sms_sent": ActivityDomainCampaign, "email_sent": ActivityDomainCampaign,
	"sync_completed": ActivityDomainSystem,
}
