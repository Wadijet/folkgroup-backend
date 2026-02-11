package authdto

// UserCreateInput đầu vào tạo người dùng (CRUD).
type UserCreateInput struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required"`
}

// UserSetWorkingRoleInput đầu vào đăng nhập người dùng.
type UserSetWorkingRoleInput struct {
	RoleID string `json:"roleId" validate:"required"`
}

// UserLogoutInput đầu vào đăng xuất người dùng.
type UserLogoutInput struct {
	Hwid string `json:"hwid" validate:"required"`
}

// UserChangeInfoInput đầu vào thay đổi thông tin người dùng.
type UserChangeInfoInput struct {
	Name string `json:"name"`
}

// BlockUserInput đầu vào khóa người dùng.
type BlockUserInput struct {
	Email string `json:"email" validate:"required"`
	Note  string `json:"note" validate:"required"`
}

// UnBlockUserInput đầu vào mở khóa người dùng.
type UnBlockUserInput struct {
	Email string `json:"email" validate:"required"`
}

// FirebaseLoginInput đầu vào đăng nhập bằng Firebase ID token.
type FirebaseLoginInput struct {
	IDToken string `json:"idToken" validate:"required"`
	Hwid    string `json:"hwid" validate:"required"`
}
