package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ContentNodeType định nghĩa các loại content node (L1-L6)
const (
	ContentNodeTypePillar      = "pillar"      // L1: Pillar (Trụ cột)
	ContentNodeTypeSTP         = "stp"         // L2: STP (Segmentation, Targeting, Positioning)
	ContentNodeTypeInsight     = "insight"     // L3: Insight (Thông tin chi tiết)
	ContentNodeTypeContentLine = "contentLine" // L4: Content Line (Dòng nội dung)
	ContentNodeTypeGene        = "gene"        // L5: Gene (Gen nội dung)
	ContentNodeTypeScript      = "script"      // L6: Script (Kịch bản)
)

// CreatorType định nghĩa loại người tạo content
const (
	CreatorTypeHuman  = "human"  // Human tạo thủ công
	CreatorTypeAI     = "ai"     // AI tạo tự động
	CreatorTypeHybrid = "hybrid" // Hybrid (AI generate + Human edit)
)

// CreationMethod định nghĩa phương thức tạo content
const (
	CreationMethodManual   = "manual"   // Tạo thủ công
	CreationMethodAI       = "ai"       // AI generate
	CreationMethodWorkflow = "workflow" // Từ workflow run
)

// ContentNode đại diện cho content node (L1-L6: Pillar, STP, Insight, Content Line, Gene, Script)
// Đây là production content đã được duyệt và commit
type ContentNode struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của content node

	// ===== CONTENT HIERARCHY =====
	Type     string              `json:"type" bson:"type" index:"single:1"`                             // Loại content node: pillar, stp, insight, contentLine, gene, script
	ParentID *primitive.ObjectID `json:"parentId,omitempty" bson:"parentId,omitempty" index:"single:1"` // ID của parent node (null nếu là root)
	Name     string              `json:"name,omitempty" bson:"name,omitempty" index:"text"`             // Tên content node (tùy chọn)
	Text     string              `json:"text" bson:"text" index:"text"`                                 // Nội dung text của node

	// ===== CREATION METADATA =====
	CreatorType          string              `json:"creatorType" bson:"creatorType" index:"single:1"`                                       // Loại người tạo: human, ai, hybrid
	CreationMethod       string              `json:"creationMethod" bson:"creationMethod" index:"single:1"`                                 // Phương thức tạo: manual, ai, workflow
	CreatedByRunID       *primitive.ObjectID `json:"createdByRunId,omitempty" bson:"createdByRunId,omitempty" index:"single:1"`             // ID của workflow run tạo ra node này (nếu có, link về Module 2)
	CreatedByStepRunID   *primitive.ObjectID `json:"createdByStepRunId,omitempty" bson:"createdByStepRunId,omitempty" index:"single:1"`     // ID của step run (nếu có, link về Module 2)
	CreatedByCandidateID *primitive.ObjectID `json:"createdByCandidateId,omitempty" bson:"createdByCandidateId,omitempty" index:"single:1"` // ID của candidate được chọn (nếu có, link về Module 2)
	CreatedByBatchID     *primitive.ObjectID `json:"createdByBatchId,omitempty" bson:"createdByBatchId,omitempty" index:"single:1"`         // ID của generation batch (nếu có, link về Module 2)

	// ===== STATUS =====
	Status string `json:"status" bson:"status" index:"single:1"` // Trạng thái: active, archived, deleted

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	// ===== METADATA =====
	Metadata  map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"` // Metadata bổ sung (tùy chọn)
	CreatedAt int64                  `json:"createdAt" bson:"createdAt"`                   // Thời gian tạo
	UpdatedAt int64                  `json:"updatedAt" bson:"updatedAt"`                   // Thời gian cập nhật
}
