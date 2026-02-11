package contentdto

// DraftContentNodeCreateInput dữ liệu đầu vào khi tạo draft content node
type DraftContentNodeCreateInput struct {
	Type                string                 `json:"type" validate:"required"`
	ParentID            string                 `json:"parentId,omitempty" transform:"str_objectid_ptr,optional"`
	ParentDraftID       string                 `json:"parentDraftId,omitempty" transform:"str_objectid_ptr,optional"`
	Name                string                 `json:"name,omitempty"`
	Text                string                 `json:"text" validate:"required"`
	WorkflowRunID       string                 `json:"workflowRunId,omitempty" transform:"str_objectid_ptr,optional"`
	CreatedByRunID      string                 `json:"createdByRunId,omitempty" transform:"str_objectid_ptr,optional"`
	CreatedByStepRunID  string                 `json:"createdByStepRunId,omitempty" transform:"str_objectid_ptr,optional"`
	CreatedByCandidateID string                `json:"createdByCandidateId,omitempty" transform:"str_objectid_ptr,optional"`
	CreatedByBatchID    string                 `json:"createdByBatchId,omitempty" transform:"str_objectid_ptr,optional"`
	ApprovalStatus      string                 `json:"approvalStatus,omitempty" transform:"string,default=draft"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// DraftContentNodeUpdateInput dữ liệu đầu vào khi cập nhật draft content node
type DraftContentNodeUpdateInput struct {
	Type           string                 `json:"type,omitempty"`
	ParentID       string                 `json:"parentId,omitempty" transform:"str_objectid_ptr,optional"`
	ParentDraftID  string                 `json:"parentDraftId,omitempty" transform:"str_objectid_ptr,optional"`
	Name           string                 `json:"name,omitempty"`
	Text           string                 `json:"text,omitempty"`
	ApprovalStatus string                 `json:"approvalStatus,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// CommitDraftNodeParams params từ URL khi commit draft node
type CommitDraftNodeParams struct {
	ID string `uri:"id" validate:"required" transform:"str_objectid"`
}

// ApproveDraftParams params từ URL khi approve draft
type ApproveDraftParams struct {
	ID string `uri:"id" validate:"required" transform:"str_objectid"`
}

// RejectDraftParams params từ URL khi reject draft
type RejectDraftParams struct {
	ID string `uri:"id" validate:"required" transform:"str_objectid"`
}

// RejectDraftInput body khi reject (ghi chú tùy chọn)
type RejectDraftInput struct {
	DecisionNote string `json:"decisionNote,omitempty"`
}
