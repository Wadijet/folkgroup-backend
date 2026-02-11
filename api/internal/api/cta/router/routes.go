// Package router đăng ký các route thuộc domain CTA: Library (CRUD).
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	ctahdl "meta_commerce/internal/api/cta/handler"
	apirouter "meta_commerce/internal/api/router"
)

// Register đăng ký tất cả route CTA lên v1.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	ctaLibraryHandler, err := ctahdl.NewCTALibraryHandler()
	if err != nil {
		return fmt.Errorf("create CTA library handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/cta/library", ctaLibraryHandler, apirouter.ReadWriteConfig, "CTALibrary")
	return nil
}
