// Package activity — types dùng chung cho Activity framework.
// Spec: docs-shared/ai-context/folkform/design/activity-framework.md
package activity

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ActivityBase — khung cấu trúc chung, mọi activity đều có.
// Domain model embed với bson:",inline".
type ActivityBase struct {
	ID                  primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	Uid                 string                 `json:"uid" bson:"uid" index:"single:1"` // act_xxx — ID chuẩn 4 lớp
	ActivityType        string                 `json:"activityType" bson:"activityType"`
	Domain              string                 `json:"domain" bson:"domain"`
	OwnerOrganizationID primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId"`
	UnifiedId           string                 `json:"unifiedId" bson:"unifiedId"`
	Source              string                 `json:"source" bson:"source"`
	SourceRef           map[string]interface{} `json:"sourceRef,omitempty" bson:"sourceRef,omitempty"`
	Actor               Actor                  `json:"actor,omitempty" bson:"actor,omitempty"`
	ActivityAt          int64                  `json:"activityAt" bson:"activityAt"`
	Display             Display                `json:"display,omitempty" bson:"display,omitempty"`
	Snapshot            Snapshot               `json:"snapshot,omitempty" bson:"snapshot,omitempty"`
	Context             map[string]interface{} `json:"context,omitempty" bson:"context,omitempty"`
	Changes             []ActivityChangeItem  `json:"changes,omitempty" bson:"changes,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
	CreatedAt           int64                  `json:"createdAt" bson:"createdAt"`
}

// ActivityChangeItem mô tả một thay đổi (field, oldValue, newValue).
// Dùng chung CRM, Ads.
type ActivityChangeItem struct {
	Field    string      `json:"field" bson:"field"`
	OldValue interface{} `json:"oldValue,omitempty" bson:"oldValue,omitempty"`
	NewValue interface{} `json:"newValue,omitempty" bson:"newValue,omitempty"`
}

// Actor — ai gây ra activity
type Actor struct {
	Type string `json:"type" bson:"type"` // user | ai | system | external
	Id   string `json:"id,omitempty" bson:"id,omitempty"`
	Name string `json:"name,omitempty" bson:"name,omitempty"`
}

// Display — hiển thị UI timeline
type Display struct {
	Icon    string `json:"icon,omitempty" bson:"icon,omitempty"`
	Label   string `json:"label,omitempty" bson:"label,omitempty"`
	Subtext string `json:"subtext,omitempty" bson:"subtext,omitempty"`
}

// Snapshot — state tại thời điểm event
type Snapshot struct {
	Profile map[string]interface{} `json:"profile,omitempty" bson:"profile,omitempty"`
	Metrics map[string]interface{} `json:"metrics,omitempty" bson:"metrics,omitempty"`
}

// Các hằng type Actor
const (
	ActorTypeUser    = "user"
	ActorTypeAI      = "ai"
	ActorTypeSystem  = "system"
	ActorTypeExternal = "external"
)

// ToActor chuyển từ legacy actorId + actorName sang Actor.
// actorType: user | ai | system | external
func ToActor(actorId *primitive.ObjectID, actorName, actorType string) Actor {
	a := Actor{Type: actorType, Name: actorName}
	if actorId != nil && !actorId.IsZero() {
		a.Id = actorId.Hex()
	}
	return a
}
