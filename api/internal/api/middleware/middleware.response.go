package middleware

import (
	"errors"
	"meta_commerce/internal/common"

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

// HandleErrorResponse xử lý và trả về error response cho client
// Tách riêng để tránh import cycle với handler package
func HandleErrorResponse(c fiber.Ctx, err error) {
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
}
