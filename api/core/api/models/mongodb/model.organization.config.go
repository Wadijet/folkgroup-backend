package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ConfigKeyMeta là metadata cho từng key config: tên, mô tả, loại dữ liệu, ràng buộc, và khóa không cho cấp dưới ghi đè.
type ConfigKeyMeta struct {
	Name          string `json:"name" bson:"name"`                                     // Tên hiển thị của key (ví dụ "Giờ làm việc", "Múi giờ")
	Description   string `json:"description" bson:"description"`                       // Mô tả chi tiết mục đích và cách dùng
	DataType      string `json:"dataType" bson:"dataType"`                             // Loại dữ liệu: string, number, boolean, object, array
	Constraints   string `json:"constraints,omitempty" bson:"constraints,omitempty"`  // Quy tắc ràng buộc: enum, min, max, pattern... (có thể JSON string hoặc mô tả)
	AllowOverride bool   `json:"allowOverride" bson:"allowOverride"`                   // true = cấp dưới được ghi đè; false = khóa không cho cấp dưới ghi đè
}

// OrganizationConfig lưu cấu hình của từng tổ chức (quan hệ 1:1 với Organization).
// Collection: auth_organization_configs
// Config theo cây: GetResolvedConfig merge từ root xuống org hiện tại; key có ConfigMeta[k].AllowOverride == false thì cấp dưới không ghi đè.
type OrganizationConfig struct {
	ID                   primitive.ObjectID            `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID            `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"unique"` // Tổ chức sở hữu config (phân quyền); 1 document per org
	Config               map[string]interface{}       `json:"config" bson:"config"`                                           // Giá trị từng key
	ConfigMeta           map[string]ConfigKeyMeta      `json:"configMeta,omitempty" bson:"configMeta,omitempty"`               // Metadata từng key: name, mô tả, loại, ràng buộc, AllowOverride
	IsSystem             bool                          `json:"-" bson:"isSystem" index:"single:1"`                             // true = config hệ thống, không cho xóa (chỉ nội bộ)
	CreatedAt            int64                         `json:"createdAt" bson:"createdAt"`
	UpdatedAt            int64                         `json:"updatedAt" bson:"updatedAt"`
}
