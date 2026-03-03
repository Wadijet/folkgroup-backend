// Package basehdl - Handler quản lý MongoDB: danh sách collections, xóa toàn bộ, tải export.
package basehdl

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v3"

	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
)

// MongoDbManageHandler xử lý các route quản lý MongoDB (list, delete all, export).
type MongoDbManageHandler struct {
	service *basesvc.MongoDbManageService
}

// NewMongoDbManageHandler tạo handler quản lý MongoDB.
func NewMongoDbManageHandler() (*MongoDbManageHandler, error) {
	svc := basesvc.NewMongoDbManageService()
	return &MongoDbManageHandler{service: svc}, nil
}

// HandleListCollections xử lý GET danh sách collections kèm số documents.
// Trả về: [{ name, docCount }, ...]
func (h *MongoDbManageHandler) HandleListCollections(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		list, err := h.service.ListCollectionsWithCount(c.Context())
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code":    common.ErrCodeDatabase.Code,
				"message": "Không thể lấy danh sách collections",
				"status":  "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code":    common.StatusOK,
			"message": "Thành công",
			"data":    list,
			"status":  "success",
		})
		return nil
	})
}

// HandleDeleteAllDocuments xử lý DELETE xóa toàn bộ documents trong collection.
// Query params: collection (tên collection), confirm=true (bắt buộc để xác nhận)
func (h *MongoDbManageHandler) HandleDeleteAllDocuments(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		collectionName := c.Query("collection")
		if collectionName == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationInput.Code,
				"message": "Thiếu tham số collection",
				"status":  "error",
			})
			return nil
		}
		if c.Query("confirm") != "true" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationInput.Code,
				"message": "Cần thêm confirm=true để xác nhận xóa toàn bộ documents",
				"status":  "error",
			})
			return nil
		}

		deletedCount, err := h.service.DeleteAllDocuments(c.Context(), collectionName)
		if err != nil {
			if errors.Is(err, basesvc.ErrCollectionProtected) {
				c.Status(common.StatusForbidden).JSON(fiber.Map{
					"code":    common.ErrCodeBusinessOperation.Code,
					"message": "Collection này được bảo vệ, không cho phép xóa toàn bộ",
					"status":  "error",
				})
				return nil
			}
			if err == common.ErrNotFound {
				c.Status(common.StatusNotFound).JSON(fiber.Map{
					"code":    common.ErrCodeDatabaseQuery.Code,
					"message": fmt.Sprintf("Collection %s không tồn tại", collectionName),
					"status":  "error",
				})
				return nil
			}
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code":    common.ErrCodeDatabase.Code,
				"message": "Không thể xóa documents",
				"status":  "error",
			})
			return nil
		}

		c.Status(common.StatusOK).JSON(fiber.Map{
			"code":    common.StatusOK,
			"message": fmt.Sprintf("Đã xóa %d documents", deletedCount),
			"data": fiber.Map{
				"collection":    collectionName,
				"deletedCount":  deletedCount,
			},
			"status": "success",
		})
		return nil
	})
}

// HandleExportCollection xử lý GET tải collection dưới dạng file JSON.
// Query params: collection (tên collection), format=json|ndjson (mặc định json)
// format=ndjson: streaming, phù hợp file lớn, một document mỗi dòng.
func (h *MongoDbManageHandler) HandleExportCollection(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		collectionName := c.Query("collection")
		if collectionName == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationInput.Code,
				"message": "Thiếu tham số collection",
				"status":  "error",
			})
			return nil
		}

		format := c.Query("format", "json")
		if format == "ndjson" {
			// Streaming export cho file lớn
			streamFunc, err := h.service.StreamExportFunc(c.Context(), collectionName)
			if err != nil {
				if err == common.ErrNotFound {
					c.Status(common.StatusNotFound).JSON(fiber.Map{
						"code":    common.ErrCodeDatabaseQuery.Code,
						"message": fmt.Sprintf("Collection %s không tồn tại", collectionName),
						"status":  "error",
					})
					return nil
				}
				c.Status(common.StatusInternalServerError).JSON(fiber.Map{
					"code":    common.ErrCodeDatabase.Code,
					"message": "Không thể export collection",
					"status":  "error",
				})
				return nil
			}
			filename := collectionName + ".ndjson"
			c.Set("Content-Type", "application/x-ndjson; charset=utf-8")
			c.Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
			return c.SendStreamWriter(streamFunc)
		}

		// JSON array (toàn bộ vào memory)
		jsonBytes, err := h.service.ExportCollectionAsJSON(c.Context(), collectionName)
		if err != nil {
			if err == common.ErrNotFound {
				c.Status(common.StatusNotFound).JSON(fiber.Map{
					"code":    common.ErrCodeDatabaseQuery.Code,
					"message": fmt.Sprintf("Collection %s không tồn tại", collectionName),
					"status":  "error",
				})
				return nil
			}
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code":    common.ErrCodeDatabase.Code,
				"message": "Không thể export collection",
				"status":  "error",
			})
			return nil
		}

		filename := collectionName + ".json"
		c.Set("Content-Type", "application/json; charset=utf-8")
		c.Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
		return c.Send(jsonBytes)
	})
}

// HandleImportFile xử lý POST import từ file upload (multipart).
// Query param: collection (tên collection)
// Form field: file (file NDJSON - một document mỗi dòng, phù hợp file lớn)
func (h *MongoDbManageHandler) HandleImportFile(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		collectionName := c.Query("collection")
		if collectionName == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationInput.Code,
				"message": "Thiếu tham số collection",
				"status":  "error",
			})
			return nil
		}

		file, err := c.FormFile("file")
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationInput.Code,
				"message": "Thiếu file upload (form field: file)",
				"status":  "error",
			})
			return nil
		}

		f, err := file.Open()
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code":    common.ErrCodeDatabase.Code,
				"message": "Không thể đọc file",
				"status":  "error",
			})
			return nil
		}
		defer f.Close()

		insertedCount, err := h.service.ImportCollectionFromNDJSONStream(c.Context(), collectionName, f)
		if err != nil {
			if err == common.ErrNotFound {
				c.Status(common.StatusNotFound).JSON(fiber.Map{
					"code":    common.ErrCodeDatabaseQuery.Code,
					"message": fmt.Sprintf("Collection %s không tồn tại", collectionName),
					"status":  "error",
				})
				return nil
			}
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": err.Error(),
				"status":  "error",
			})
			return nil
		}

		c.Status(common.StatusOK).JSON(fiber.Map{
			"code":    common.StatusOK,
			"message": fmt.Sprintf("Đã import %d documents từ file", insertedCount),
			"data": fiber.Map{
				"collection":    collectionName,
				"insertedCount": insertedCount,
			},
			"status": "success",
		})
		return nil
	})
}

// HandleImportCollection xử lý POST import documents từ JSON vào collection.
// Query param: collection (tên collection)
// Body: JSON array các documents (định dạng tương thích export, hỗ trợ Extended JSON $oid).
func (h *MongoDbManageHandler) HandleImportCollection(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		collectionName := c.Query("collection")
		if collectionName == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationInput.Code,
				"message": "Thiếu tham số collection",
				"status":  "error",
			})
			return nil
		}

		body := c.Body()
		if len(body) == 0 {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationInput.Code,
				"message": "Thiếu body JSON (array documents)",
				"status":  "error",
			})
			return nil
		}

		insertedCount, err := h.service.ImportCollectionFromJSON(c.Context(), collectionName, body)
		if err != nil {
			if err == common.ErrNotFound {
				c.Status(common.StatusNotFound).JSON(fiber.Map{
					"code":    common.ErrCodeDatabaseQuery.Code,
					"message": fmt.Sprintf("Collection %s không tồn tại", collectionName),
					"status":  "error",
				})
				return nil
			}
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": err.Error(),
				"status":  "error",
			})
			return nil
		}

		c.Status(common.StatusOK).JSON(fiber.Map{
			"code":    common.StatusOK,
			"message": fmt.Sprintf("Đã import %d documents", insertedCount),
			"data": fiber.Map{
				"collection":     collectionName,
				"insertedCount":  insertedCount,
			},
			"status": "success",
		})
		return nil
	})
}
