package common

import (
	"errors"

	"go.mongodb.org/mongo-driver/mongo"
)

// HTTP Status Code Constants
const (
	// Success Codes (2xx)
	StatusOK        = 200 // Thành công
	StatusCreated   = 201 // Tạo mới thành công
	StatusAccepted  = 202 // Yêu cầu được chấp nhận
	StatusNoContent = 204 // Thành công nhưng không có nội dung trả về

	// Client Error Codes (4xx)
	StatusBadRequest         = 400 // Yêu cầu không hợp lệ
	StatusUnauthorized       = 401 // Chưa xác thực
	StatusForbidden          = 403 // Không có quyền truy cập
	StatusNotFound           = 404 // Không tìm thấy tài nguyên
	StatusMethodNotAllowed   = 405 // Phương thức HTTP không được hỗ trợ
	StatusConflict           = 409 // Xung đột dữ liệu
	StatusGone               = 410 // Tài nguyên không còn tồn tại
	StatusPreconditionFailed = 412 // Điều kiện tiên quyết không thỏa mãn
	StatusTooManyRequests    = 429 // Quá nhiều yêu cầu

	// Server Error Codes (5xx)
	StatusInternalServerError = 500 // Lỗi server
	StatusNotImplemented      = 501 // Chức năng chưa được triển khai
	StatusBadGateway          = 502 // Gateway không hợp lệ
	StatusServiceUnavailable  = 503 // Dịch vụ không khả dụng
	StatusGatewayTimeout      = 504 // Gateway timeout
)

// Response Messages
const (
	// Success Messages
	MsgSuccess   = "Thao tác thành công"
	MsgCreated   = "Tạo mới thành công"
	MsgAccepted  = "Yêu cầu được chấp nhận"
	MsgNoContent = "Không có nội dung trả về"

	// Error Messages
	MsgBadRequest         = "Yêu cầu không hợp lệ"
	MsgUnauthorized       = "Vui lòng đăng nhập"
	MsgForbidden          = "Không có quyền truy cập"
	MsgNotFound           = "Không tìm thấy tài nguyên"
	MsgMethodNotAllowed   = "Phương thức không được hỗ trợ"
	MsgConflict           = "Xung đột dữ liệu"
	MsgGone               = "Tài nguyên không còn tồn tại"
	MsgPreconditionFailed = "Điều kiện tiên quyết không thỏa mãn"
	MsgTooManyRequests    = "Quá nhiều yêu cầu"
	MsgInternalError      = "Lỗi hệ thống"
	MsgNotImplemented     = "Chức năng chưa được triển khai"
	MsgBadGateway         = "Gateway không hợp lệ"
	MsgServiceUnavailable = "Dịch vụ không khả dụng"
	MsgGatewayTimeout     = "Gateway timeout"

	// Token Messages
	MsgTokenMissing = "Thiếu token xác thực"
	MsgTokenInvalid = "Token không hợp lệ"
	MsgTokenExpired = "Token đã hết hạn"

	// Validation Messages
	MsgValidationError = "Dữ liệu không hợp lệ"
	MsgDatabaseError   = "Lỗi tương tác với cơ sở dữ liệu"
	MsgInvalidFormat   = "Định dạng dữ liệu không hợp lệ"
)

// ErrorCode định nghĩa mã lỗi chi tiết
type ErrorCode struct {
	Code        string // Mã lỗi (ví dụ: AUTH_001)
	Category    string // Phân loại lỗi (ví dụ: Authentication)
	SubCategory string // Phân loại con (ví dụ: Token)
	Description string // Mô tả chi tiết
}

// Định nghĩa các mã lỗi theo hệ thống phân cấp
var (
	// System Errors (SYS_xxx)
	ErrCodeInternalServer = ErrorCode{
		Code:        "SYS_001",
		Category:    "System",
		SubCategory: "Internal",
		Description: "Lỗi hệ thống nội bộ",
	}

	// Authentication Errors (AUTH_xxx)
	ErrCodeAuth = ErrorCode{
		Code:        "AUTH",
		Category:    "Authentication",
		SubCategory: "General",
		Description: "Lỗi xác thực chung",
	}

	ErrCodeAuthToken = ErrorCode{
		Code:        "AUTH_001",
		Category:    "Authentication",
		SubCategory: "Token",
		Description: "Lỗi liên quan đến token",
	}

	ErrCodeAuthCredentials = ErrorCode{
		Code:        "AUTH_002",
		Category:    "Authentication",
		SubCategory: "Credentials",
		Description: "Lỗi thông tin đăng nhập",
	}

	ErrCodeAuthRole = ErrorCode{
		Code:        "AUTH_003",
		Category:    "Authentication",
		SubCategory: "Role",
		Description: "Lỗi liên quan đến vai trò người dùng",
	}

	// Validation Errors (VAL_xxx)
	ErrCodeValidation = ErrorCode{
		Code:        "VAL",
		Category:    "Validation",
		SubCategory: "General",
		Description: "Lỗi xác thực dữ liệu chung",
	}

	ErrCodeValidationInput = ErrorCode{
		Code:        "VAL_001",
		Category:    "Validation",
		SubCategory: "Input",
		Description: "Lỗi dữ liệu đầu vào",
	}

	ErrCodeValidationFormat = ErrorCode{
		Code:        "VAL_002",
		Category:    "Validation",
		SubCategory: "Format",
		Description: "Lỗi định dạng dữ liệu",
	}

	// Database Errors (DB_xxx)
	ErrCodeDatabase = ErrorCode{
		Code:        "DB",
		Category:    "Database",
		SubCategory: "General",
		Description: "Lỗi cơ sở dữ liệu chung",
	}

	ErrCodeDatabaseConnection = ErrorCode{
		Code:        "DB_001",
		Category:    "Database",
		SubCategory: "Connection",
		Description: "Lỗi kết nối cơ sở dữ liệu",
	}

	ErrCodeDatabaseQuery = ErrorCode{
		Code:        "DB_002",
		Category:    "Database",
		SubCategory: "Query",
		Description: "Lỗi truy vấn dữ liệu",
	}

	// Business Logic Errors (BIZ_xxx)
	ErrCodeBusiness = ErrorCode{
		Code:        "BIZ",
		Category:    "Business",
		SubCategory: "General",
		Description: "Lỗi logic nghiệp vụ chung",
	}

	ErrCodeBusinessState = ErrorCode{
		Code:        "BIZ_001",
		Category:    "Business",
		SubCategory: "State",
		Description: "Lỗi trạng thái nghiệp vụ",
	}

	ErrCodeBusinessOperation = ErrorCode{
		Code:        "BIZ_002",
		Category:    "Business",
		SubCategory: "Operation",
		Description: "Lỗi thao tác nghiệp vụ",
	}
)

// Error định nghĩa cấu trúc lỗi chi tiết
type Error struct {
	Code       ErrorCode // Mã lỗi chi tiết
	Message    string    // Thông báo lỗi
	StatusCode int       // HTTP status code
	Details    any       // Thông tin chi tiết thêm về lỗi
}

// Error trả về message của lỗi
func (e *Error) Error() string {
	return e.Message
}

// Is kiểm tra xem error có phải là target error không (hỗ trợ errors.Is)
func (e *Error) Is(target error) bool {
	if target == nil {
		return false
	}

	// So sánh với ErrNotFound hoặc các error khác
	if targetErr, ok := target.(*Error); ok {
		return e.Code.Code == targetErr.Code.Code && e.Message == targetErr.Message
	}

	// Nếu target là ErrNotFound (kiểu error interface), so sánh trực tiếp
	// Hỗ trợ cả trường hợp target là error interface (ErrNotFound)
	if target == ErrNotFound {
		if errNotFound, ok := ErrNotFound.(*Error); ok {
			return e.Code.Code == errNotFound.Code.Code && e.Message == errNotFound.Message
		}
	}

	// Hỗ trợ errors.Is với wrapped errors - kiểm tra bằng cách so sánh error message
	// Nếu target error có message giống, coi như match
	if target != nil {
		if target.Error() == e.Message && e.Code.Code == ErrCodeDatabaseQuery.Code && e.Message == "Không tìm thấy dữ liệu" {
			return true
		}
	}

	return false
}

// NewError tạo một error mới với đầy đủ thông tin
func NewError(code ErrorCode, message string, statusCode int, details any) error {
	return &Error{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Details:    details,
	}
}

// Custom errors
var (
	// Authentication Errors
	ErrInvalidCredentials = NewError(ErrCodeAuthCredentials, "Thông tin đăng nhập không chính xác", StatusUnauthorized, nil)
	ErrTokenExpired       = NewError(ErrCodeAuthToken, "Phiên đăng nhập đã hết hạn", StatusUnauthorized, nil)
	ErrTokenInvalid       = NewError(ErrCodeAuthToken, "Token không hợp lệ", StatusUnauthorized, nil)
	ErrTokenMissing       = NewError(ErrCodeAuthToken, "Thiếu token xác thực", StatusUnauthorized, nil)
	ErrUserAlreadyAdmin   = NewError(ErrCodeAuthRole, "Người dùng đã có quyền Administrator", StatusConflict, nil)
	ErrUserNotFound       = NewError(ErrCodeAuthCredentials, "Không tìm thấy thông tin người dùng", StatusNotFound, nil)

	// Validation Errors
	ErrInvalidInput  = NewError(ErrCodeValidationInput, "Dữ liệu đầu vào không hợp lệ", StatusBadRequest, nil)
	ErrInvalidEmail  = NewError(ErrCodeValidationInput, "Email không đúng định dạng", StatusBadRequest, nil)
	ErrWeakPassword  = NewError(ErrCodeValidationInput, "Mật khẩu quá yếu", StatusBadRequest, nil)
	ErrInvalidFormat = NewError(ErrCodeValidationFormat, "Định dạng dữ liệu không hợp lệ", StatusBadRequest, nil)
	ErrRequiredField = NewError(ErrCodeValidationInput, "Thiếu thông tin bắt buộc", StatusBadRequest, nil)

	// Database Errors
	ErrNotFound    = NewError(ErrCodeDatabaseQuery, "Không tìm thấy dữ liệu", StatusNotFound, nil)
	ErrDuplicate   = NewError(ErrCodeDatabaseQuery, "Dữ liệu đã tồn tại", StatusConflict, nil)
	ErrConstraint  = NewError(ErrCodeDatabaseQuery, "Vi phạm ràng buộc dữ liệu", StatusBadRequest, nil)
	ErrConnection  = NewError(ErrCodeDatabaseConnection, "Lỗi kết nối cơ sở dữ liệu", StatusServiceUnavailable, nil)
	ErrTransaction = NewError(ErrCodeDatabaseQuery, "Lỗi giao dịch cơ sở dữ liệu", StatusInternalServerError, nil)

	// Business Logic Errors
	ErrInsufficientFunds = NewError(ErrCodeBusinessOperation, "Số dư không đủ", StatusBadRequest, nil)
	ErrInvalidState      = NewError(ErrCodeBusinessState, "Trạng thái không hợp lệ", StatusBadRequest, nil)
	ErrInvalidOperation  = NewError(ErrCodeBusinessOperation, "Thao tác không hợp lệ", StatusBadRequest, nil)
)

// MongoDB Error Codes
const (
	// Connection Errors (1xx)
	MongoErrConnection = 100 // Lỗi kết nối chung
	MongoErrNetwork    = 101 // Lỗi mạng
	MongoErrTimeout    = 102 // Lỗi timeout

	// Authentication Errors (2xx)
	MongoErrAuth = 200 // Lỗi xác thực chung
	MongoErrUser = 201 // Lỗi người dùng

	// Query Errors (3xx)
	MongoErrQuery  = 300 // Lỗi truy vấn chung
	MongoErrCursor = 301 // Lỗi con trỏ
	MongoErrIndex  = 302 // Lỗi chỉ mục

	// Write Errors (4xx)
	MongoErrWrite     = 400 // Lỗi ghi chung
	MongoErrDuplicate = 401 // Lỗi trùng lặp
	MongoErrConflict  = 402 // Lỗi xung đột

	// System Errors (5xx)
	MongoErrSystem  = 500 // Lỗi hệ thống chung
	MongoErrShard   = 501 // Lỗi phân mảnh
	MongoErrReplica = 502 // Lỗi replica
)

// MongoDB Error Messages
const (
	// Connection Messages
	MsgMongoConnection = "Lỗi kết nối MongoDB"
	MsgMongoNetwork    = "Lỗi mạng khi kết nối MongoDB"
	MsgMongoTimeout    = "Kết nối MongoDB bị timeout"

	// Authentication Messages
	MsgMongoAuth = "Lỗi xác thực MongoDB"
	MsgMongoUser = "Lỗi người dùng MongoDB"

	// Query Messages
	MsgMongoQuery  = "Lỗi truy vấn MongoDB"
	MsgMongoCursor = "Lỗi con trỏ MongoDB"
	MsgMongoIndex  = "Lỗi chỉ mục MongoDB"

	// Write Messages
	MsgMongoWrite     = "Lỗi ghi dữ liệu MongoDB"
	MsgMongoDuplicate = "Dữ liệu trùng lặp trong MongoDB"
	MsgMongoConflict  = "Xung đột khi ghi dữ liệu MongoDB"

	// System Messages
	MsgMongoSystem  = "Lỗi hệ thống MongoDB"
	MsgMongoShard   = "Lỗi phân mảnh MongoDB"
	MsgMongoReplica = "Lỗi replica MongoDB"
)

// MongoDB Specific Errors
var (
	// Connection Errors
	ErrMongoConnection = NewError(ErrCodeDatabaseConnection, MsgMongoConnection, StatusServiceUnavailable, nil)
	ErrMongoNetwork    = NewError(ErrCodeDatabaseConnection, MsgMongoNetwork, StatusServiceUnavailable, nil)
	ErrMongoTimeout    = NewError(ErrCodeDatabaseConnection, MsgMongoTimeout, StatusServiceUnavailable, nil)

	// Authentication Errors
	ErrMongoAuth = NewError(ErrCodeAuth, MsgMongoAuth, StatusUnauthorized, nil)
	ErrMongoUser = NewError(ErrCodeAuth, MsgMongoUser, StatusUnauthorized, nil)

	// Query Errors
	ErrMongoQuery  = NewError(ErrCodeDatabaseQuery, MsgMongoQuery, StatusInternalServerError, nil)
	ErrMongoCursor = NewError(ErrCodeDatabaseQuery, MsgMongoCursor, StatusNotFound, nil)
	ErrMongoIndex  = NewError(ErrCodeDatabaseQuery, MsgMongoIndex, StatusBadRequest, nil)

	// Write Errors
	ErrMongoWrite     = NewError(ErrCodeDatabaseQuery, MsgMongoWrite, StatusInternalServerError, nil)
	ErrMongoDuplicate = NewError(ErrCodeDatabaseQuery, MsgMongoDuplicate, StatusConflict, nil)
	ErrMongoConflict  = NewError(ErrCodeDatabaseQuery, MsgMongoConflict, StatusConflict, nil)

	// System Errors
	ErrMongoSystem  = NewError(ErrCodeDatabase, MsgMongoSystem, StatusInternalServerError, nil)
	ErrMongoShard   = NewError(ErrCodeDatabase, MsgMongoShard, StatusServiceUnavailable, nil)
	ErrMongoReplica = NewError(ErrCodeDatabase, MsgMongoReplica, StatusServiceUnavailable, nil)
)

// ConvertMongoError chuyển đổi lỗi MongoDB sang lỗi hệ thống
func ConvertMongoError(err error) error {
	if err == nil {
		return nil
	}

	// Kiểm tra ErrNotFound trước - không convert lỗi này
	// Thử nhiều cách để đảm bảo nhận diện đúng

	// Cách 1: Kiểm tra bằng errors.Is (hỗ trợ wrapped errors)
	if errors.Is(err, ErrNotFound) {
		return err
	}

	// Cách 2: Kiểm tra xem có phải là ErrNotFound bằng cách so sánh error code và message
	if errNotFound, ok := err.(*Error); ok {
		if errNotFound.Code.Code == ErrCodeDatabaseQuery.Code &&
			errNotFound.Message == "Không tìm thấy dữ liệu" {
			return err
		}
	}

	// Cách 3: Kiểm tra bằng error message (cho trường hợp wrapped errors)
	errMsg := err.Error()
	if errMsg == "Không tìm thấy dữ liệu" || errMsg == ErrNotFound.Error() {
		return ErrNotFound
	}

	// Cách 4: Kiểm tra bằng cách so sánh với ErrNotFound trực tiếp
	if err == ErrNotFound {
		return err
	}

	// Kiểm tra các loại lỗi MongoDB cụ thể
	var mongoErr mongo.CommandError
	if errors.As(err, &mongoErr) {
		switch {
		// Connection Errors
		case mongoErr.Code >= 100 && mongoErr.Code < 200:
			return ErrMongoConnection
		// Authentication Errors
		case mongoErr.Code >= 200 && mongoErr.Code < 300:
			return ErrMongoAuth
		// Query Errors
		case mongoErr.Code >= 300 && mongoErr.Code < 400:
			return ErrMongoQuery
		// Write Errors
		case mongoErr.Code >= 400 && mongoErr.Code < 500:
			return ErrMongoWrite
		// System Errors
		case mongoErr.Code >= 500:
			return ErrMongoSystem
		}
	}

	// Kiểm tra các lỗi MongoDB khác
	if mongo.IsDuplicateKeyError(err) {
		return ErrMongoDuplicate
	}
	if mongo.IsNetworkError(err) {
		return ErrMongoNetwork
	}
	if mongo.IsTimeout(err) {
		return ErrMongoTimeout
	}

	// Nếu không tìm thấy lỗi cụ thể, trả về lỗi hệ thống chung
	return NewError(ErrCodeDatabase, "Lỗi kết nối cơ sở dữ liệu", StatusInternalServerError, err)
}
