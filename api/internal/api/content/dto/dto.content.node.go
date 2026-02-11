package contentdto

// ContentNodeCreateInput dữ liệu đầu vào khi tạo content node
type ContentNodeCreateInput struct {
	Type           string                 `json:"type" validate:"required"`
	ParentID       string                 `json:"parentId,omitempty" transform:"str_objectid_ptr,optional"`
	Name           string                 `json:"name,omitempty"`
	Text           string                 `json:"text" validate:"required"`
	CreatorType    string                 `json:"creatorType,omitempty" transform:"string,default=human"`
	CreationMethod string                 `json:"creationMethod,omitempty" transform:"string,default=manual"`
	Status         string                 `json:"status,omitempty" transform:"string,default=active"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ContentNodeUpdateInput dữ liệu đầu vào khi cập nhật content node
type ContentNodeUpdateInput struct {
	Type     string                 `json:"type,omitempty"`
	ParentID string                 `json:"parentId,omitempty" transform:"str_objectid_ptr,optional"`
	Name     string                 `json:"name,omitempty"`
	Text     string                 `json:"text,omitempty"`
	Status   string                 `json:"status,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ContentNodeTreeParams params từ URL khi lấy tree của content node
type ContentNodeTreeParams struct {
	ID string `uri:"id" validate:"required" transform:"str_objectid"`
}
