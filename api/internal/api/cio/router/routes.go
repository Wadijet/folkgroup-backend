// Package ciorouter đăng ký CIO Ingest API — điểm vào thống nhất cho dữ liệu thô (Agent, job).
//
// Một URL: POST /api/v1/cio/ingest với body { "domain", "filter?", "data" }.
// filter (body) merge với ?filter=; data giống payload từng domain (đồng bộ trước đây qua sync-upsert-one / Meta sync-upsert).
package ciorouter

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	ciohdl "meta_commerce/internal/api/cio/handler"
	"meta_commerce/internal/api/middleware"
	apirouter "meta_commerce/internal/api/router"
)

// Register đăng ký POST /cio/ingest lên v1.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	orgContextMiddleware := middleware.OrganizationContextMiddleware()

	ingestHandler, err := ciohdl.NewCioIngestHandler()
	if err != nil {
		return fmt.Errorf("cio ingest handler: %w", err)
	}

	apirouter.RegisterRouteWithMiddleware(v1, "/cio/ingest", "POST", "",
		[]fiber.Handler{middleware.AuthMiddleware(""), orgContextMiddleware},
		ingestHandler.HandleIngest)

	_ = r
	return nil
}
