package dto

// ContentNodeCreateInput dữ liệu đầu vào khi tạo content node
type ContentNodeCreateInput struct {
	Type          string                 `json:"type" validate:"required"`                                    // Loại content node: layer, stp, insight, contentLine, gene, script
	ParentID      string                 `json:"parentId,omitempty" transform:"str_objectid_ptr,optional"`    // ID của parent node (tùy chọn, dạng string ObjectID) - tự động convert sang *primitive.ObjectID
	Name          string                 `json:"name,omitempty"`                                                // Tên content node (tùy chọn)
	Text          string                 `json:"text" validate:"required"`                                     // Nội dung text của node (bắt buộc)
	CreatorType   string                 `json:"creatorType,omitempty" transform:"string,default=human"`      // Loại người tạo: human, ai, hybrid (mặc định: human)
	CreationMethod string                `json:"creationMethod,omitempty" transform:"string,default=manual"`   // Phương thức tạo: manual, ai, workflow (mặc định: manual)
	Status        string                 `json:"status,omitempty" transform:"string,default=active"`          // Trạng thái: active, archived, deleted (mặc định: active)
	Metadata      map[string]interface{} `json:"metadata,omitempty"`                                           // Metadata bổ sung (tùy chọn)
}

// ContentNodeUpdateInput dữ liệu đầu vào khi cập nhật content node
type ContentNodeUpdateInput struct {
	Type     string                 `json:"type,omitempty"`                              // Loại content node
	ParentID string                 `json:"parentId,omitempty" transform:"str_objectid_ptr,optional"` // ID của parent node (dạng string ObjectID) - tự động convert sang *primitive.ObjectID
	Name     string                 `json:"name,omitempty"`                              // Tên content node
	Text     string                 `json:"text,omitempty"`                              // Nội dung text của node
	Status   string                 `json:"status,omitempty"`                           // Trạng thái
	Metadata map[string]interface{} `json:"metadata,omitempty"`                        // Metadata bổ sung
}
