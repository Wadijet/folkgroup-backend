// Package router — Route cho module CIX (Contextual Conversation Intelligence).
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	cixhdl "meta_commerce/internal/api/cix/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký route CIX lên v1.
func Register(v1 fiber.Router, _ *apirouter.Router) error {
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	readMiddleware := middleware.AuthMiddleware("CIX.Read")
	analyzeMiddleware := middleware.AuthMiddleware("CIX.Analyze")

	analysisHandler, err := cixhdl.NewCixAnalysisHandler()
	if err != nil {
		return fmt.Errorf("tạo CixAnalysisHandler: %w", err)
	}

	// POST /cix/analyze — Phân tích session
	apirouter.RegisterRouteWithMiddleware(v1, "/cix/analyze", "POST", "", []fiber.Handler{analyzeMiddleware, orgContextMiddleware}, analysisHandler.HandleAnalyzeSession)

	// GET /cix/analysis/:sessionUid — Lấy kết quả phân tích theo session
	apirouter.RegisterRouteWithMiddleware(v1, "/cix", "GET", "/analysis/:sessionUid", []fiber.Handler{readMiddleware, orgContextMiddleware}, analysisHandler.HandleGetAnalysisBySession)

	return nil
}
