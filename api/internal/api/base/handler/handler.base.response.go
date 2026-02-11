package basehdl

import (
	"errors"
	"fmt"
	"meta_commerce/internal/common"
	"runtime/debug"

	"github.com/gofiber/fiber/v3"
)

// JSONResponse trả về JSON response với Content-Type: application/json; charset=utf-8
// Helper function này đảm bảo tất cả JSON responses đều có charset=utf-8 để hỗ trợ UTF-8 encoding đúng cách
func JSONResponse(c fiber.Ctx, statusCode int, data interface{}) error {
	// Set Content-Type với charset=utf-8 trước khi gọi JSON
	c.Set("Content-Type", "application/json; charset=utf-8")
	// Trả về JSON response
	return c.Status(statusCode).JSON(data)
}

// SafeHandler bọc các handler với recover để bắt panic và xử lý lỗi an toàn.
// Hàm này đảm bảo rằng server luôn trả về response cho client, kể cả khi có panic xảy ra.
//
// Parameters:
// - c: Fiber context
// - handler: Function xử lý chính của handler
func (h *BaseHandler[T, CreateInput, UpdateInput]) SafeHandler(c fiber.Ctx, handler func() error) error {
	defer func() {
		if r := recover(); r != nil {
			// Log stack trace để debug
			debug.PrintStack()

			// Trả về lỗi cho client
			h.HandleResponse(c, nil, common.NewError(
				common.ErrCodeInternalServer,
				fmt.Sprintf("Lỗi hệ thống không mong muốn: %v", r),
				common.StatusInternalServerError,
				nil,
			))
		}
	}()
	return handler()
}

// SafeHandlerWrapper wrapper để xử lý errors (dùng bởi domain handler không embed BaseHandler).
func SafeHandlerWrapper(c fiber.Ctx, fn func() error) error {
	if err := fn(); err != nil {
		return err
	}
	return nil
}

// HandleResponse xử lý và chuẩn hóa response trả về cho client.
// Phương thức này đảm bảo format response thống nhất trong toàn bộ ứng dụng.
//
// Parameters:
// - c: Fiber context
// - data: Dữ liệu trả về cho client (có thể là nil nếu chỉ trả về lỗi)
// - err: Lỗi nếu có (nil nếu không có lỗi)
func (h *BaseHandler[T, CreateInput, UpdateInput]) HandleResponse(c fiber.Ctx, data interface{}, err error) {
	if err != nil {
		var customErr *common.Error
		if errors.As(err, &customErr) {
			JSONResponse(c, customErr.StatusCode, fiber.Map{
				"code":    customErr.Code.Code,
				"message": customErr.Message,
				"details": customErr.Details,
				"status":  "error",
			})
			return
		}
		// Nếu không phải custom error, trả về internal server error
		JSONResponse(c, common.StatusInternalServerError, fiber.Map{
			"code":    common.ErrCodeDatabase.Code,
			"message": err.Error(),
			"status":  "error",
		})
		return
	}

	// Trường hợp thành công
	JSONResponse(c, common.StatusOK, fiber.Map{
		"code":    common.StatusOK,
		"message": common.MsgSuccess,
		"data":    data,
		"status":  "success",
	})
}
