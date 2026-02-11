package utility

import (
	"bytes"
	"encoding/json"

	"github.com/valyala/fasthttp"

	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
)

// JSON thiết lập header và trả về dữ liệu JSON
func JSON(ctx *fasthttp.RequestCtx, data map[string]interface{}) {

	// Thiết lập Header
	ctx.Response.Header.Set("Content-Type", "application/json; charset=UTF-8")
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")

	// Chuyển đổi dữ liệu thành JSON
	res, err := json.Marshal(data)

	if err != nil {
		logger.GetAppLogger().WithError(err).Error("Error Convert to JSON")
		data["error"] = err
	}

	// Ghi dữ liệu ra output
	ctx.Write(res)

	// Thiết lập mã trạng thái HTTP
	//ctx.SetStatusCode(statusCode)
}

// ResponseType định nghĩa kiểu response
type ResponseType struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
	Code    int         `json:"code"`
}

// Payload tạo payload với trạng thái, dữ liệu và thông điệp
func Payload(isSuccess bool, data interface{}, message string, statusCode ...int) map[string]interface{} {
	response := ResponseType{
		Status:  "error",
		Data:    data,
		Message: message,
		Code:    common.StatusInternalServerError,
	}

	if isSuccess {
		response.Status = "success"
		response.Code = common.StatusOK
	}

	if len(statusCode) > 0 {
		response.Code = statusCode[0]
	}

	result := make(map[string]interface{})
	result["status"] = response.Status
	result["data"] = response.Data
	result["message"] = response.Message
	result["code"] = response.Code

	return result
}

// FinalResponse tạo phản hồi cuối cùng dựa trên kết quả và lỗi
func FinalResponse(result interface{}, err error) map[string]interface{} {
	if err != nil {
		if customErr, ok := err.(*common.Error); ok {
			return Payload(false, customErr, customErr.Message, customErr.StatusCode)
		}
		return Payload(false, common.NewError(common.ErrCodeDatabaseConnection, common.MsgDatabaseError, common.StatusInternalServerError, err), common.MsgDatabaseError)
	} else {
		return Payload(true, result, common.MsgSuccess, common.StatusOK)
	}
}

// Convert2Struct chuyển đổi dữ liệu JSON thành struct
func Convert2Struct(data []byte, myStruct interface{}) map[string]interface{} {
	reader := bytes.NewReader(data)
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	err := decoder.Decode(&myStruct)
	if err != nil {
		return Payload(false, common.NewError(common.ErrCodeValidationFormat, common.MsgInvalidFormat, common.StatusBadRequest, err), common.MsgInvalidFormat)
	}

	return nil
}

// ValidateStruct kiểm tra tính hợp lệ của struct
func ValidateStruct(myStruct interface{}) map[string]interface{} {
	err := global.Validate.Struct(myStruct)
	if err != nil {
		return Payload(false, common.NewError(common.ErrCodeValidationInput, common.MsgValidationError, common.StatusBadRequest, err), common.MsgValidationError)
	}

	return nil
}

// CreateChangeMap tạo bản đồ thay đổi từ struct
func CreateChangeMap(myStruct interface{}, myChange *map[string]interface{}) map[string]interface{} {
	CustomBson := &CustomBson{}
	change, err := CustomBson.Set(myStruct)
	if err != nil {
		return Payload(false, common.NewError(common.ErrCodeValidationInput, common.MsgValidationError, common.StatusBadRequest, err), common.MsgValidationError)
	}

	*myChange = change
	return nil
}

// =================================================	=========================
// P2Float64 chuyển đổi interface thành float64
func P2Float64(input interface{}) float64 {
	jsonNumber, ok := input.(json.Number)
	if !ok {
		return 0
	}
	number, err := jsonNumber.Float64()
	if err != nil {
		return 0
	}

	return number
}

// P2Int64 chuyển đổi interface thành int64
func P2Int64(input interface{}) int64 {
	jsonNumber, ok := input.(json.Number)
	if !ok {
		return 0
	}
	result, err := jsonNumber.Int64()
	if err != nil {
		return 0
	}

	return result
}
