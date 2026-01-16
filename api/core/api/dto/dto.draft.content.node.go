package dto

// DraftContentNodeCreateInput dữ liệu đầu vào khi tạo draft content node
type DraftContentNodeCreateInput struct {
	Type                string                 `json:"type" validate:"required"`                                                      // Loại content node: layer, stp, insight, contentLine, gene, script
	ParentID            string                 `json:"parentId,omitempty" transform:"str_objectid_ptr,optional"`                    // ID của parent node (production) (tùy chọn, dạng string ObjectID)
	ParentDraftID       string                 `json:"parentDraftId,omitempty" transform:"str_objectid_ptr,optional"`              // ID của parent draft node (tùy chọn, dạng string ObjectID)
	Name                string                 `json:"name,omitempty"`                                                                // Tên content node (tùy chọn)
	Text                string                 `json:"text" validate:"required"`                                                     // Nội dung text của node (bắt buộc)
	WorkflowRunID       string                 `json:"workflowRunId,omitempty" transform:"str_objectid_ptr,optional"`              // ID của workflow run tạo ra draft này (tùy chọn, link về Module 2)
	CreatedByRunID       string                 `json:"createdByRunId,omitempty" transform:"str_objectid_ptr,optional"`             // ID của AI run (tùy chọn, link về Module 2)
	CreatedByStepRunID   string                 `json:"createdByStepRunId,omitempty" transform:"str_objectid_ptr,optional"`        // ID của step run (tùy chọn, link về Module 2)
	CreatedByCandidateID string                 `json:"createdByCandidateId,omitempty" transform:"str_objectid_ptr,optional"`      // ID của candidate được chọn (tùy chọn, link về Module 2)
	CreatedByBatchID     string                 `json:"createdByBatchId,omitempty" transform:"str_objectid_ptr,optional"`           // ID của generation batch (tùy chọn, link về Module 2)
	ApprovalStatus       string                 `json:"approvalStatus,omitempty" transform:"string,default=draft"`                  // Trạng thái approval: pending, approved, rejected, draft (mặc định: draft)
	Metadata             map[string]interface{} `json:"metadata,omitempty"`                                                           // Metadata bổ sung (tùy chọn)
}

// DraftContentNodeUpdateInput dữ liệu đầu vào khi cập nhật draft content node
type DraftContentNodeUpdateInput struct {
	Type            string                 `json:"type,omitempty"`                              // Loại content node
	ParentID        string                 `json:"parentId,omitempty" transform:"str_objectid_ptr,optional"` // ID của parent node
	ParentDraftID   string                 `json:"parentDraftId,omitempty" transform:"str_objectid_ptr,optional"` // ID của parent draft node
	Name            string                 `json:"name,omitempty"`                               // Tên content node
	Text            string                 `json:"text,omitempty"`                              // Nội dung text của node
	ApprovalStatus  string                 `json:"approvalStatus,omitempty"`                     // Trạng thái approval
	Metadata        map[string]interface{} `json:"metadata,omitempty"`                        // Metadata bổ sung
}

// CommitDraftNodeParams params từ URL khi commit draft node
type CommitDraftNodeParams struct {
	ID string `uri:"id" validate:"required" transform:"str_objectid"` // Draft Node ID từ URL params - tự động validate và convert sang ObjectID
}
