package dto

// DraftApprovalCreateInput dữ liệu đầu vào khi tạo approval request
type DraftApprovalCreateInput struct {
	WorkflowRunID     string `json:"workflowRunId,omitempty"`        // ID của workflow run (nếu approve cho workflow run, dạng string ObjectID)
	DraftNodeID       string `json:"draftNodeId,omitempty"`          // ID của draft node (nếu approve cho individual draft, dạng string ObjectID)
	DraftVideoID      string `json:"draftVideoId,omitempty"`         // ID của draft video (nếu approve cho individual draft, dạng string ObjectID)
	DraftPublicationID string `json:"draftPublicationId,omitempty"`  // ID của draft publication (nếu approve cho individual draft, dạng string ObjectID)
	Metadata          map[string]interface{} `json:"metadata,omitempty"` // Metadata bổ sung (tùy chọn)
}

// DraftApprovalUpdateInput dữ liệu đầu vào khi cập nhật approval (approve/reject)
type DraftApprovalUpdateInput struct {
	Status      string `json:"status,omitempty"`                      // Trạng thái: pending, approved, rejected
	DecisionNote string `json:"decisionNote,omitempty"`              // Ghi chú về quyết định (tùy chọn)
	Metadata    map[string]interface{} `json:"metadata,omitempty"`    // Metadata bổ sung
}
