// Package models - CrmActivityHistory thuộc domain CRM (crm_activity_history).
// Lưu lịch sử hoạt động của khách hàng: order, conversation, note, ...
package models

import (
	"meta_commerce/internal/common/activity"
)

// CrmActivityHistory lưu lịch sử hoạt động khách (crm_activity_history).
// Embed ActivityBase — cấu trúc chuẩn. Index tạo bởi CreateCrmActivityIndexes.
type CrmActivityHistory struct {
	activity.ActivityBase `bson:",inline"`
}
