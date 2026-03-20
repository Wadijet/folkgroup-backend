// Package identity — models và helpers cho hệ thống 4 lớp ID (uid, sourceIds, links).
package identity

import "go.mongodb.org/mongo-driver/bson/primitive"

// ExternalRef tham chiếu ID tại hệ ngoài (pos, facebook, zalo).
type ExternalRef struct {
	Source string `json:"source" bson:"source"` // pos, facebook, zalo, shopify
	ID     string `json:"id" bson:"id"`
}

// LinkItem schema chuẩn 1 link — hỗ trợ uid (resolved) và externalRefs (pending).
type LinkItem struct {
	Uid          string        `json:"uid" bson:"uid"`
	ExternalRefs []ExternalRef `json:"externalRefs" bson:"externalRefs"`
	Role         string        `json:"role,omitempty" bson:"role,omitempty"`
	Status       string        `json:"status" bson:"status"` // resolved | pending_resolution | conflict | detached
	Confidence   float64       `json:"confidence,omitempty" bson:"confidence,omitempty"`
}

const (
	LinkStatusResolved          = "resolved"
	LinkStatusPendingResolution = "pending_resolution"
	LinkStatusConflict          = "conflict"
	LinkStatusDetached          = "detached"
)

// DocWithIdentity interface cho document cần enrich — helper dùng reflection nếu cần.
type DocWithIdentity interface {
	GetObjectID() primitive.ObjectID
	SetUid(string)
	GetUid() string
	GetSourceIds() map[string]string
	SetSourceIds(map[string]string)
	GetLinks() map[string]LinkItem
	SetLinks(map[string]LinkItem)
}
