// Package metahdl - Handler cho Meta token exchange (short-lived → long-lived).
package metahdl

import (
	"github.com/gofiber/fiber/v3"

	basehdl "meta_commerce/internal/api/base/handler"
	metad "meta_commerce/internal/api/meta/dto"
	metasvc "meta_commerce/internal/api/meta/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// MetaTokenHandler xử lý request đổi token Meta (short-lived → long-lived).
type MetaTokenHandler struct{}

// NewMetaTokenHandler tạo MetaTokenHandler.
func NewMetaTokenHandler() *MetaTokenHandler {
	return &MetaTokenHandler{}
}

// maskToken trả về prefix token để log, không expose full token.
func maskToken(s string) string {
	if len(s) <= 20 {
		return "***"
	}
	return s[:20] + "..."
}

// HandleExchangeToken xử lý POST /meta/token/exchange.
// Nhận shortLivedToken, đổi sang long-lived (~60 ngày), lưu vào file (META_TOKEN_FILE).
// Cần META_APP_ID và META_APP_SECRET trong config.
func (h *MetaTokenHandler) HandleExchangeToken(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input metad.MetaTokenExchangeInput
		if err := c.Bind().JSON(&input); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "Dữ liệu gửi lên không đúng định dạng JSON",
				"status":  "error",
			})
			return nil
		}
		if input.ShortLivedToken == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "shortLivedToken không được để trống",
				"status":  "error",
			})
			return nil
		}
		cfg := global.MongoDB_ServerConfig
		if cfg == nil || cfg.MetaAppID == "" || cfg.MetaAppSecret == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "Chưa cấu hình META_APP_ID và META_APP_SECRET. Thêm vào env hoặc config.",
				"status":  "error",
			})
			return nil
		}
		filePath := cfg.MetaTokenFile
		if filePath == "" {
			filePath = "config/meta_token.json"
		}
		token, err := metasvc.ExchangeAndSaveMetaToken(c.Context(), cfg.MetaAppID, cfg.MetaAppSecret, input.ShortLivedToken, filePath)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "Đổi token thất bại: " + err.Error(),
				"status":  "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code":    common.StatusOK,
			"message": "Đã đổi token dài hạn và lưu vào " + filePath + ". Server sẽ dùng token này khi khởi động lại.",
			"data": fiber.Map{
				"savedTo": filePath,
				"token":   maskToken(token), // Chỉ trả prefix, không expose full token
			},
			"status": "success",
		})
		return nil
	})
}
